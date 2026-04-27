// Package nounsafememberaccess implements the no-unsafe-member-access rule:
// flag `x.foo` or `x[key]` where x has type any.
package nounsafememberaccess

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-member-access"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
		wrapperchecker.KindElementAccessExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	recv := n.FirstChild()
	if recv == nil {
		return
	}
	t := ctx.TypeOf(recv)
	if t == nil {
		return
	}
	if t.IsAny() {
		ctx.Report(n, "member access on a value of type `any` defeats the type checker")
	}
}
