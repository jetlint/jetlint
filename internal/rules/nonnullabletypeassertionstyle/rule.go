// Package nonnullabletypeassertionstyle implements the
// non-nullable-type-assertion-style rule: flag `x as T` where x is
// `T | null | undefined`-shaped — those should use the non-null
// assertion `x!` instead.
package nonnullabletypeassertionstyle

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "non-nullable-type-assertion-style"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAsExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.AsExpressionSource()
	annot := n.AsExpressionTarget()
	if expr == nil || annot == nil {
		return
	}
	src := ctx.TypeOf(expr)
	target := ctx.Checker().TypeFromTypeNode(annot)
	if src == nil || target == nil {
		return
	}
	if !src.IsUnion() {
		return
	}
	stripped := stripNullables(src)
	if stripped == nil {
		return
	}
	if stripped.String() == target.String() {
		ctx.Report(n, "use the non-null assertion `!` instead of `as` when the value is just T | null | undefined")
	}
}

// stripNullables returns the union with null/undefined members
// removed. Nil if the union has no nullables (the cast isn't a
// nullability-only narrowing), if it leaves nothing, or if multiple
// non-null members remain (can't represent as a single comparable
// type without re-resolving the union).
func stripNullables(t *wrapperchecker.Type) *wrapperchecker.Type {
	if !t.IsUnion() {
		return nil
	}
	var nonNullables []*wrapperchecker.Type
	hadNullable := false
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() {
			hadNullable = true
			continue
		}
		nonNullables = append(nonNullables, m)
	}
	if !hadNullable || len(nonNullables) != 1 {
		return nil
	}
	return nonNullables[0]
}
