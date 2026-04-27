// Package nounsafecall implements the no-unsafe-call rule: flag `x()`
// where x has type `any`.
package nounsafecall

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-call"

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
	t := ctx.TypeOf(callee)
	if t == nil {
		return
	}
	if t.IsAny() {
		ctx.Report(callee, "calling a value of type `any` defeats the type checker")
	}
}
