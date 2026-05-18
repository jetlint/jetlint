// Package usehtmllang implements use-html-lang: the <html> element
// must declare a `lang` attribute so screen readers can pick the
// right pronunciation rules. An empty lang value doesn't count.
package usehtmllang

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-html-lang"

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
	if jsxutil.TagName(el) != "html" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	attr := jsxutil.FindAttribute(attrs, "lang")
	if attr == nil {
		ctx.Report(el, "<html> needs a lang attribute for screen readers")
		return
	}
	if s, ok := jsxutil.AttributeStringValue(attr); ok && s == "" {
		ctx.Report(attr, "<html lang=\"\"> defeats the purpose — set a real language code")
	}
}
