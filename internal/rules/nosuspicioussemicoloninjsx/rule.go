// Package nosuspicioussemicoloninjsx implements no-suspicious-semicolon-in-jsx:
// a bare `;` inside JSX children is almost always a typo; it gets
// rendered as the text ";".
package nosuspicioussemicoloninjsx

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-suspicious-semicolon-in-jsx"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

const (
	kindJsxText = wrapperchecker.Kind(11)
)

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n.Kind() == kindJsxText {
			start, end := n.Pos(), n.End()
			if start >= 0 && end <= len(src) && start < end {
				text := strings.TrimSpace(src[start:end])
				if text == ";" {
					ctx.Report(n, "stray `;` inside JSX children — usually a typo")
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(n)
}
