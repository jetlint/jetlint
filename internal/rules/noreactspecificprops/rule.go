// Package noreactspecificprops implements no-react-specific-props:
// in non-React projects (e.g., Solid), use the HTML names `class` /
// `for` instead of React's `className` / `htmlFor`.
package noreactspecificprops

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-react-specific-props"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxAttribute: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var name *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name == nil {
			name = c
		}
		return false
	})
	if name == nil || name.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	switch name.SourceText() {
	case "className":
		ctx.Report(name, "use `class` instead of React's `className`")
	case "htmlFor":
		ctx.Report(name, "use `for` instead of React's `htmlFor`")
	}
}
