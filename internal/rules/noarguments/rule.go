// Package noarguments implements no-arguments: prefer rest parameters
// (`...args`) over the `arguments` object. `arguments` doesn't work in
// arrow functions, isn't a real array, and obscures the function's
// signature.
package noarguments

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-arguments"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.SourceText() != "arguments" {
		return
	}
	// Skip identifiers used as declaration names: `let arguments = 1`,
	// parameter names, property names, etc.
	p := n.Parent()
	if p == nil {
		return
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindPropertyAccessExpression,
		wrapperchecker.KindPropertySignature,
		wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindBindingElement:
		// For PropertyAccessExpression, only skip if `arguments` is the
		// property side (rare). The reads we care about (arguments[i],
		// arguments.length) live elsewhere.
		if p.Kind() == wrapperchecker.KindPropertyAccessExpression {
			// In `arguments.length`, `arguments` is the object side.
			// We DO want to flag that. Detect by being the first child.
			var first *wrapperchecker.Node
			p.ForEachChild(func(c *wrapperchecker.Node) bool {
				if first == nil {
					first = c
				}
				return false
			})
			if first == n {
				ctx.Report(n, "prefer rest parameters (`...args`) over the `arguments` object")
			}
			return
		}
		return
	}
	ctx.Report(n, "prefer rest parameters (`...args`) over the `arguments` object")
}
