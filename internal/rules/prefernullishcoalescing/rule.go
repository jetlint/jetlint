// Package prefernullishcoalescing implements the
// prefer-nullish-coalescing rule: flag `x || y` where x is nullable —
// the `??` operator handles only null/undefined and avoids
// unintentionally treating other falsy values as the fallback trigger.
package prefernullishcoalescing

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-nullish-coalescing"

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
	if op != wrapperchecker.KindBarBarToken && op != wrapperchecker.KindBarBarEqualsToken {
		return
	}
	left := n.BinaryLeft()
	if left == nil {
		return
	}
	t := ctx.TypeOf(left)
	if t == nil {
		return
	}
	if !typeIsNullable(t) {
		return
	}
	if op == wrapperchecker.KindBarBarEqualsToken {
		ctx.Report(n, "use ??= for nullable values; ||= treats other falsy values as missing too")
		return
	}
	ctx.Report(n, "use ?? for nullable values; || treats other falsy values as missing too")
}

// typeIsNullable reports whether t is a union containing null or
// undefined and at least one non-nullable member. Bare null/undefined
// alone or non-unions don't make `||` a misuse.
func typeIsNullable(t *wrapperchecker.Type) bool {
	if !t.IsUnion() {
		return false
	}
	hasNullable := false
	hasNonNullable := false
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() {
			hasNullable = true
		} else {
			hasNonNullable = true
		}
	}
	return hasNullable && hasNonNullable
}
