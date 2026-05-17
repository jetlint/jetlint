// Package noexcessivelinesperfunction implements
// no-excessive-lines-per-function: a function whose body spans many
// lines is harder to keep in your head — split it up.
package noexcessivelinesperfunction

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-excessive-lines-per-function"

// Default threshold; smaller bodies pass.
const defaultMaxLines = 50

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	if body == nil {
		return
	}
	lines := 1
	src := body.SourceText()
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lines++
		}
	}
	if lines > defaultMaxLines {
		ctx.Report(n, "function body exceeds the configured line limit")
	}
}
