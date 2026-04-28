// Package returnawait implements the return-await rule: flag
// `return await X` where X isn't a Promise — the await is dead.
package returnawait

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "return-await"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement: visit,
		wrapperchecker.KindArrowFunction:   visitArrow,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	checkReturnedAwait(ctx, expr)
}

func visitArrow(ctx *engine.Context, n *wrapperchecker.Node) {
	body := n.FunctionBody()
	if body == nil || body.Kind() == wrapperchecker.KindBlock {
		// Block-bodied arrows go through the ReturnStatement handler.
		return
	}
	checkReturnedAwait(ctx, body)
}

func checkReturnedAwait(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr.Kind() != wrapperchecker.KindAwaitExpression {
		return
	}
	operand := expr.FirstChild()
	if operand == nil {
		return
	}
	t := ctx.TypeOf(operand)
	if t == nil {
		return
	}
	if isThenableLike(t) {
		return
	}
	ctx.Report(expr, "await of a non-promise value in a return position has no effect; remove `await`")
}

func isThenableLike(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	if t.IsPromise() || t.IsThenable() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isThenableLike(m) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isThenableLike(m) {
				return true
			}
		}
	}
	return false
}
