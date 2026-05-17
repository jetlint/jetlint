// Package nocommenttext implements no-comment-text: `// ...` and
// `/* ... */` inside JSX *text* aren't comments — they're rendered to
// the DOM as literal text. Wrap them in `{/* ... */}`.
package nocommenttext

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-comment-text"

const (
	kindJsxText               = wrapperchecker.Kind(11)
	kindJsxTextAllWhiteSpaces = wrapperchecker.Kind(12)
)

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	scan(ctx, n, src)
}

func scan(ctx *engine.Context, sf *wrapperchecker.Node, src string) {
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		switch n.Kind() {
		case kindJsxText, kindJsxTextAllWhiteSpaces:
			start, end := n.Pos(), n.End()
			if start < 0 || end > len(src) || start >= end {
				return
			}
			text := src[start:end]
			if strings.Contains(text, "//") || strings.Contains(text, "/*") {
				ctx.Report(n, "JSX text contains a comment-like sequence — wrap in `{/* … */}`")
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(sf)
}
