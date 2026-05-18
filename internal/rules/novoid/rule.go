// Package novoid implements no-void: the `void` operator forces an
// expression to evaluate to `undefined`. The cases where you'd want
// that are too narrow — explicit `undefined` is clearer.
package novoid

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-void"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVoidExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	ctx.Report(n, "`void` operator — return `undefined` directly")
}
