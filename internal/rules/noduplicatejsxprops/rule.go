// Package noduplicatejsxprops implements no-duplicate-jsx-props:
// duplicated attribute names on the same JSX tag silently drop all
// but the last one — usually not what the author meant.
package noduplicatejsxprops

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-duplicate-jsx-props"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil {
		return
	}
	seen := map[string]bool{}
	attrs.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttribute {
			return false
		}
		name := jsxutil.AttributeName(c)
		if name == "" {
			return false
		}
		if seen[name] {
			ctx.Report(c, "duplicate JSX attribute `"+name+"`")
			return false
		}
		seen[name] = true
		return false
	})
}
