// Package noariaunsupportedelements implements
// no-aria-unsupported-elements: <meta>, <html>, <script>, <style>
// have no accessibility semantics in the page. Adding `role` or
// `aria-*` attributes to them is meaningless and confuses tooling.
package noariaunsupportedelements

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-aria-unsupported-elements"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var unsupportedTags = map[string]bool{"meta": true, "html": true, "script": true, "style": true}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	if !unsupportedTags[jsxutil.TagName(el)] {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil {
		return
	}
	attrs.ForEachChild(func(a *wrapperchecker.Node) bool {
		if a.Kind() != wrapperchecker.KindJsxAttribute {
			return false
		}
		name := jsxutil.AttributeName(a)
		if name == "role" || strings.HasPrefix(name, "aria-") {
			ctx.Report(a, "role / aria-* attributes have no effect on this element")
		}
		return false
	})
}
