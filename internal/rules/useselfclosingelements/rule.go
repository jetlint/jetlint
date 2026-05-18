// Package useselfclosingelements implements use-self-closing-elements:
// `<X></X>` (no children) reads as `<X />`.
package useselfclosingelements

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-self-closing-elements"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	p := n.Parent()
	if p == nil {
		return
	}
	// Parent has children: openingElement, ...children, closingElement.
	hasChild := false
	idx := 0
	var closing *wrapperchecker.Node
	p.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 {
			if c.Kind() == wrapperchecker.KindJsxClosingElement {
				closing = c
			} else {
				hasChild = true
			}
		}
		idx++
		return false
	})
	if closing == nil || hasChild {
		return
	}
	ctx.Report(p, "empty JSX element collapses to self-closing form")
}
