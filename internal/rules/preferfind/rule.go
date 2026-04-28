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
		wrapperchecker.KindCallExpression:          visitCall,
	}
}

// visitCall handles `arr.filter(...).at(0)`.
func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.IsOptionalChain() {
		return
	}
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "at" {
		return
	}
	args := n.CallArguments()
	if len(args) != 1 {
		return
	}
	if !isZero(ctx, args[0]) {
		return
	}
	atRecv := callee.PropertyAccessReceiver()
	if atRecv == nil || atRecv.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	filterCallee := atRecv.CalleeExpression()
	if filterCallee == nil || filterCallee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if filterCallee.PropertyAccessName() != "filter" {
		return
	}
	filterRecv := filterCallee.PropertyAccessReceiver()
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
	ctx.Report(n, "use .find() instead of .filter().at(0)")
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
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral:
		if n.LiteralText() == "0" {
			return true
		}
	case wrapperchecker.KindBigIntLiteral:
		t := n.LiteralText()
		if t == "0n" || t == "0" {
			return true
		}
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		// `arr.filter(...)['0']` indexes the same slot.
		if n.LiteralText() == "0" {
			return true
		}
	case wrapperchecker.KindPrefixUnaryExpression:
		// `-0` and `-0n` are still zero.
		if n.PrefixUnaryOperator() == "-" {
			if inner := n.FirstChild(); inner != nil {
				return isZero(ctx, inner)
			}
		}
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return false
	}
	if v, ok := t.NumericLiteralValue(); ok && v == 0 {
		return true
	}
	if t.IsBigIntLike() {
		// 0n typed via const reference: `const zero = 0n; arr[zero]`.
		s := t.String()
		if s == "0n" || s == "-0n" {
			return true
		}
	}
	return false
}
