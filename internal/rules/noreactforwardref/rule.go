// Package noreactforwardref implements no-react-forward-ref: as of
// React 19, function components accept `ref` as a regular prop and
// `forwardRef` is no longer needed.
package noreactforwardref

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-react-forward-ref"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil {
		return
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		if callee.SourceText() == "forwardRef" {
			ctx.Report(n, "forwardRef is no longer needed in React 19 — pass `ref` as a regular prop")
		}
	case wrapperchecker.KindPropertyAccessExpression:
		_, name := propParts(callee)
		if name == "forwardRef" {
			ctx.Report(n, "forwardRef is no longer needed in React 19 — pass `ref` as a regular prop")
		}
	}
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

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
}
