// Package noimplicitboolean implements no-implicit-boolean: JSX
// boolean attributes default to `true` when written bare (`<input
// disabled />`). Be explicit (`disabled={true}`) so the intent is
// visible in the diff and grep.
package noimplicitboolean

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-implicit-boolean"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxAttribute: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// A bare attribute has only the Identifier child — no initializer.
	count := 0
	n.ForEachChild(func(_ *wrapperchecker.Node) bool {
		count++
		return false
	})
	if count == 1 {
		ctx.Report(n, "JSX boolean attribute is implicit — write `={true}` explicitly")
	}
}
