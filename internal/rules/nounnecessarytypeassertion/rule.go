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
	// `as const` widens-or-narrows literal types; the assertion is
	// generally meaningful even when the source already has a literal
	// type, because the operator preserves the literal context.
	if isAsConst(annot) {
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

func typeContainsNullable(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsNullOrUndefined() || t.IsVoid() {
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
