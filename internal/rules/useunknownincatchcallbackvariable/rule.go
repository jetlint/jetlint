// Package useunknownincatchcallbackvariable implements the
// use-unknown-in-catch-callback-variable rule: in `.catch(cb)` and
// `.then(_, cb)`, the callback's error parameter should be typed as
// `unknown` so callers must narrow before use.
package useunknownincatchcallbackvariable

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "use-unknown-in-catch-callback-variable"

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
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	args := n.CallArguments()
	switch callee.PropertyAccessName() {
	case "catch":
		if len(args) >= 1 {
			checkCatchCallback(ctx, callee, args[0])
		}
	case "then":
		if len(args) >= 2 {
			checkCatchCallback(ctx, callee, args[1])
		}
	}
}

func checkCatchCallback(ctx *engine.Context, callee, cb *wrapperchecker.Node) {
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !isPromiseLike(rt) {
		return
	}
	if cb.Kind() != wrapperchecker.KindArrowFunction &&
		cb.Kind() != wrapperchecker.KindFunctionExpression {
		return
	}
	param := firstParameter(cb)
	if param == nil {
		return
	}
	annot := param.ParameterTypeAnnotation()
	if annot == nil {
		ctx.Report(param, "use `unknown` for catch-callback parameters; the rejection value is untyped at runtime")
		return
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return
	}
	if t.IsUnknown() {
		return
	}
	ctx.Report(param, "use `unknown` for catch-callback parameters; the rejection value is untyped at runtime")
}

func firstParameter(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			first = c
			return true
		}
		return false
	})
	return first
}

func isPromiseLike(t *wrapperchecker.Type) bool {
	if t.IsPromise() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				return true
			}
		}
	}
	if t.IsThenable() {
		return true
	}
	return false
}
