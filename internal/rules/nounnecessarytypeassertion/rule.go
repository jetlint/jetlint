// Package nounnecessarytypeassertion implements the
// no-unnecessary-type-assertion rule: flag `x as T` when x is
// already exactly T, and `x!` when x's type is already non-nullable.
package nounnecessarytypeassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-type-assertion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAsExpression:             visitAs,
		wrapperchecker.KindTypeAssertionExpression: visitAs,
		wrapperchecker.KindNonNullExpression:        visitNonNull,
	}
}

func visitAs(ctx *engine.Context, n *wrapperchecker.Node) {
	var src, annot *wrapperchecker.Node
	switch n.Kind() {
	case wrapperchecker.KindAsExpression:
		src = n.AsExpressionSource()
		annot = n.AsExpressionTarget()
	case wrapperchecker.KindTypeAssertionExpression:
		src = n.TypeAssertionSource()
		annot = n.TypeAssertionTarget()
	}
	if src == nil || annot == nil {
		return
	}
	// `as const` is meaningful on most expressions because it preserves
	// literal/readonly inference at widening boundaries. On a literal
	// expression that already has the narrow type, however, the
	// assertion is redundant (`const a = 1 as const` — the variable
	// already would have type `1` from the const initializer).
	if isAsConst(annot) {
		if !isLiteralExpression(src) {
			return
		}
		ctx.Report(n, "type assertion is unnecessary — the literal already has this exact type")
		return
	}
	srcT := ctx.TypeOf(src)
	target := ctx.Checker().TypeFromTypeNode(annot)
	if srcT == nil || target == nil {
		return
	}
	// Asserting to a literal/tuple type often preserves narrower
	// inference at a widening boundary (`let z = c as 1` keeps the
	// literal type when `c` would otherwise widen to `number`).
	// Skip those — but a literal expression source (`3 as 3`)
	// already has the narrow type and gains nothing from the
	// assertion.
	if isLiteralOrTupleAssertion(target) && !isLiteralExpression(src) {
		return
	}
	if srcT.String() == target.String() {
		ctx.Report(n, "type assertion is unnecessary — the source already has this type")
	}
}

// isLiteralOrTupleAssertion reports whether the assertion target is
// a literal type, tuple, or readonly tuple. These can preserve
// narrower typing at widening boundaries even when the source's
// inferred type currently matches.
func isLiteralOrTupleAssertion(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsTupleType() {
		return true
	}
	s := t.String()
	switch {
	case t.IsBooleanLike() && (s == "true" || s == "false"):
		return true
	case t.IsStringLike() && s != "string":
		return true
	case t.IsNumberLike() && s != "number":
		return true
	case t.IsBigIntLike() && s != "bigint":
		return true
	}
	return false
}

func isAsConst(annot *wrapperchecker.Node) bool {
	// `as const` has TypeReference shape with the `const` keyword.
	if annot.Kind() != wrapperchecker.KindTypeReference {
		return false
	}
	var name string
	annot.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name == "const"
}

func isLiteralExpression(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		return true
	}
	return false
}

func visitNonNull(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	// The non-null assertion is redundant when the surrounding context
	// already accepts the source's full type (`let y: T | null = x!`
	// where `x: T | null` discards the assertion's effect; `nonNull(x!)`
	// when `nonNull` takes the same `T | null`). Use assignability —
	// if the unnarrowed source already fits the contextual type, the
	// `!` was a no-op.
	if ctxT := contextualNullableType(ctx, n); ctxT != nil &&
		typeContainsNullable(ctxT) && t.IsAssignableTo(ctxT) {
		ctx.Report(n, "non-null assertion is unnecessary — the contextual type already accepts the value's nullable members")
		return
	}
	// `unknown` includes null/undefined but the assertion still
	// narrows callers' usage — typescript-eslint doesn't flag.
	if t.IsUnknown() || t.IsAny() {
		return
	}
	// `void` carries undefined-as-the-only-inhabitant semantics —
	// `foo()!` over a `void`-returning call is treated as legitimate.
	if t.IsVoid() {
		return
	}
	if !t.IsNullOrUndefined() && !typeContainsNullable(t) {
		ctx.Report(n, "non-null assertion is unnecessary — the value is already non-nullable")
	}
}

// contextualNullableType returns the type the assertion's value will be
// assigned/passed into, where defined. For arguments this is the
// parameter's type; for variable declarations the annotation; etc.
func contextualNullableType(ctx *engine.Context, n *wrapperchecker.Node) *wrapperchecker.Type {
	p := n.Parent()
	if p == nil {
		return nil
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		// `const y: T = x!` — annotation type, if present.
		if t := p.VariableDeclarationType(); t != nil {
			return ctx.Checker().TypeFromTypeNode(t)
		}
	case wrapperchecker.KindPropertyDeclaration:
		if t := p.PropertyDeclarationType(); t != nil {
			return ctx.Checker().TypeFromTypeNode(t)
		}
	case wrapperchecker.KindParameter:
		if t := p.ParameterTypeAnnotation(); t != nil {
			return ctx.Checker().TypeFromTypeNode(t)
		}
	case wrapperchecker.KindCallExpression, wrapperchecker.KindNewExpression:
		// Find the argument index of `n` and ask the checker.
		args := p.CallArguments()
		for i, a := range args {
			if sameNodeIdentity(a, n) {
				return ctx.Checker().ContextualTypeForArgument(p, i)
			}
		}
	case wrapperchecker.KindBinaryExpression:
		if p.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
			if l := p.BinaryLeft(); l != nil && !sameNodeIdentity(l, n) {
				return ctx.TypeOf(l)
			}
		}
	case wrapperchecker.KindReturnStatement:
		// Return type — best-effort: get the contextual type of the
		// return expression itself.
		return ctx.Checker().ContextualTypeOf(n)
	}
	return ctx.Checker().ContextualTypeOf(n)
}

func sameNodeIdentity(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	af, asl, asc, ael, aec := a.SourceRange()
	bf, bsl, bsc, bel, bec := b.SourceRange()
	return af == bf && asl == bsl && asc == bsc && ael == bel && aec == bec
}

func typeContainsNullable(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsNullOrUndefined() || t.IsVoid() || t.IsUnknown() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContainsNullable(m) {
				return true
			}
		}
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeContainsNullable(c)
		}
	}
	return false
}
