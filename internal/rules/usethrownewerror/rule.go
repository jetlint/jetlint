// Package usethrownewerror implements use-throw-new-error: `throw
// Error("x")` works but reads ambiguously. `throw new Error("x")`
// matches every other constructor call and is unambiguous.
package usethrownewerror

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-throw-new-error"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindThrowStatement: visit,
	}
}

var errorNames = map[string]bool{
	"Error": true, "TypeError": true, "RangeError": true, "ReferenceError": true,
	"SyntaxError": true, "URIError": true, "EvalError": true, "AggregateError": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var arg *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		arg = c
		return false
	})
	if arg == nil || arg.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	callee := firstChild(arg)
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if !errorNames[callee.SourceText()] {
		return
	}
	ctx.Report(n, "throw `new "+callee.SourceText()+"(...)` — explicit constructor reads better")
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}
