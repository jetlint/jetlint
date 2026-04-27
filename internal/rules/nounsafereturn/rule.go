// Package nounsafereturn implements the no-unsafe-return rule: flag a
// `return X` where X has type any but the function's declared return
// type is more specific.
package nounsafereturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-return"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil || !t.IsAny() {
		return
	}
	ctx.Report(expr, "returning an `any` value defeats the function's declared return type")
}
