// Package preferreducetypeparameter implements the
// prefer-reduce-type-parameter rule: flag `arr.reduce(fn, init as T)`
// in favor of `arr.reduce<T>(fn, init)`.
package preferreducetypeparameter

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-reduce-type-parameter"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		if callee.PropertyAccessName() != "reduce" {
			return
		}
	case wrapperchecker.KindElementAccessExpression:
		idx := callee.ElementAccessIndex()
		if idx == nil {
			return
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			if idx.LiteralText() != "reduce" {
				return
			}
		default:
			return
		}
	default:
		return
	}
	args := n.CallArguments()
	if len(args) != 2 {
		return
	}
	// Second argument must be a type-cast — both `init as T` and the
	// older `<T>init` forms are equivalent rewrite targets.
	switch args[1].Kind() {
	case wrapperchecker.KindAsExpression, wrapperchecker.KindTypeAssertionExpression:
	default:
		return
	}
	// Receiver must be an array-like (or tuple), since reduce on a
	// custom Reducer interface may genuinely need the cast.
	var recv *wrapperchecker.Node
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression {
		recv = callee.PropertyAccessReceiver()
	} else {
		recv = callee.ElementAccessReceiver()
	}
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !isArrayLike(rt) {
		return
	}
	// Casts to a type parameter (or whose target *contains* a type
	// parameter) can't be rewritten as `reduce<T>` cleanly: the
	// generic instantiation would need to flow from the surrounding
	// function's type parameters, which the rule can't synthesize.
	castT := ctx.TypeOf(args[1])
	if castT != nil && typeContainsTypeParameter(castT, 4) {
		return
	}
	// Casts to a generic shape parameterized by a union of literal
	// types — e.g. `Record<'a' | 'b', boolean>` — are typically
	// preserving information that `reduce<T>` would lose. The
	// reducer body widens to `string` keys at runtime, but the cast
	// asserts a narrower compile-time shape. Inspect the SYNTACTIC
	// target so the alias's structure (e.g. `Record<'a' | 'b', T>`)
	// survives instead of being expanded to its mapped form.
	var targetNode *wrapperchecker.Node
	if args[1].Kind() == wrapperchecker.KindAsExpression {
		targetNode = args[1].AsExpressionTarget()
	} else {
		targetNode = args[1].TypeAssertionTarget()
	}
	if targetNode != nil && typeRefHasLiteralUnionArg(targetNode) {
		return
	}
	ctx.Report(args[1], "use `reduce<T>(...)` to declare the accumulator type instead of casting the initial value")
}

// typeRefHasLiteralUnionArg reports whether the type-annotation node
// is a TypeReference (or contains one) whose syntactic type
// arguments include a union of literal types — `Record<'a' | 'b',
// boolean>` qualifies; `number[]` does not.
func typeRefHasLiteralUnionArg(annot *wrapperchecker.Node) bool {
	if annot == nil {
		return false
	}
	if annot.Kind() == wrapperchecker.KindUnionType {
		// A bare union annotation: ensure all its members are literal
		// type-nodes.
		allLiteral := true
		annot.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindLiteralType {
				allLiteral = false
				return true
			}
			return false
		})
		return allLiteral
	}
	found := false
	annot.ForEachChild(func(c *wrapperchecker.Node) bool {
		if typeRefHasLiteralUnionArg(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// containsLiteralUnionTypeArgument reports whether t (or any of its
// nested type arguments) is a union whose members are all literal
// types — string-literals, number-literals, boolean-literals, or
// bigint-literals.
func containsLiteralUnionTypeArgument(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsUnion() {
		allLiteral := true
		for _, m := range t.UnionMembers() {
			s := m.String()
			if s == "string" || s == "number" || s == "bigint" || s == "boolean" {
				allLiteral = false
				break
			}
			if !(m.IsStringLike() || m.IsNumberLike() || m.IsBigIntLike() || m.IsBooleanLike()) {
				allLiteral = false
				break
			}
		}
		if allLiteral && len(t.UnionMembers()) > 1 {
			return true
		}
	}
	for _, a := range t.TypeArguments() {
		if containsLiteralUnionTypeArgument(a, depth-1) {
			return true
		}
	}
	return false
}

// typeContainsTypeParameter reports whether t is a type parameter
// or contains one in its generic arguments. Bounds on type parameter
// arguments make reduce<T>'s generic-inference reach more than the
// rule wants to second-guess.
func typeContainsTypeParameter(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsTypeParameter() {
		return true
	}
	for _, a := range t.TypeArguments() {
		if typeContainsTypeParameter(a, depth-1) {
			return true
		}
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContainsTypeParameter(m, depth-1) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if typeContainsTypeParameter(m, depth-1) {
				return true
			}
		}
	}
	return false
}

func isArrayLike(t *wrapperchecker.Type) bool {
	if t == nil || t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsIntersection() {
		// An intersection is array-like when every member is itself
		// array-like. `[number, number] & number[]` is just an array
		// — but `number[] & Reducer` adds a custom reduce signature
		// where the cast is meaningful, so we skip.
		for _, m := range t.IntersectionMembers() {
			if !isArrayLike(m) {
				return false
			}
		}
		return true
	}
	if t.IsTupleType() || t.IsArrayLikeType() || t.ArrayElementType() != nil {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isArrayLike(m) {
				return false
			}
		}
		return true
	}
	return false
}
