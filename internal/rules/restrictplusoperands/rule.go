// Package restrictplusoperands implements the restrict-plus-operands
// rule: flag `a + b` where the operands aren't both string-like or
// both numeric.
package restrictplusoperands

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "restrict-plus-operands"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	if op != wrapperchecker.KindPlusToken && op != wrapperchecker.KindPlusEqualsToken {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	lt := ctx.TypeOf(left)
	rt := ctx.TypeOf(right)
	if lt == nil || rt == nil {
		return
	}
	lk := classify(lt)
	rk := classify(rt)
	// Report each side that doesn't fit. Left and right are evaluated
	// independently; the combined-kind check is only meaningful when
	// both sides classify cleanly.
	if lk == "" {
		ctx.Report(left, "operand of `+` has a type that doesn't safely compose under string concatenation or numeric addition")
	}
	if rk == "" {
		ctx.Report(right, "operand of `+` has a type that doesn't safely compose under string concatenation or numeric addition")
	}
	if lk != "" && rk != "" && lk != rk {
		ctx.Report(n, "operands of `+` are different kinds: "+lk+" + "+rk)
	}
}

// classify reduces a type to one of the categories the `+` operator
// treats as safe to combine with itself: "string", "number", or
// "bigint". Returns "" for anything else (any/unknown/object/etc.) so
// the caller can flag those defensively.
func classify(t *wrapperchecker.Type) string {
	if t.IsAny() || t.IsUnknown() {
		return ""
	}
	if t.IsStringLike() {
		return "string"
	}
	if t.IsNumberLike() {
		return "number"
	}
	if t.IsBigIntLike() {
		return "bigint"
	}
	if t.IsUnion() {
		var seen string
		for _, m := range t.UnionMembers() {
			c := classify(m)
			if c == "" {
				return ""
			}
			if seen == "" {
				seen = c
				continue
			}
			if seen != c {
				return ""
			}
		}
		return seen
	}
	if t.IsIntersection() {
		// `{} & string` — pick the primitive component.
		for _, m := range t.IntersectionMembers() {
			if c := classify(m); c != "" {
				return c
			}
		}
		return ""
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return classify(c)
		}
	}
	return ""
}
