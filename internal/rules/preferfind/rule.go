// Package preferfind implements the prefer-find rule: flag
// `arr.filter(p)[0]` in favor of `arr.find(p)`.
package preferfind

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-find"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindElementAccessExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip optional-chained access — `arr.filter(...)?.[0]` is OK; the
	// optional chain is meaningful when the filter receiver itself is
	// nullable.
	if n.IsOptionalChain() {
		return
	}
	idx := n.ElementAccessIndex()
	if idx == nil {
		return
	}
	if !isZero(ctx, idx) {
		return
	}
	recv := n.ElementAccessReceiver()
	if recv == nil || recv.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	callee := recv.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "filter" {
		return
	}
	filterRecv := callee.PropertyAccessReceiver()
	if filterRecv == nil {
		return
	}
	rt := ctx.TypeOf(filterRecv)
	if rt == nil {
		return
	}
	if rt.ArrayElementType() == nil && !rt.IsArrayLikeType() && !rt.IsTupleType() {
		return
	}
	ctx.Report(n, "use .find() instead of .filter()[0]")
}

func isZero(ctx *engine.Context, n *wrapperchecker.Node) bool {
	if n.Kind() == wrapperchecker.KindNumericLiteral && n.LiteralText() == "0" {
		return true
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return false
	}
	if v, ok := t.NumericLiteralValue(); ok && v == 0 {
		return true
	}
	return false
}
