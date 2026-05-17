// Package usearia implements use-aria-activedescendant-with-tabindex:
// an element with `aria-activedescendant` must also be focusable —
// it must carry `tabIndex` or be an inherently focusable HTML tag.
package usearia

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-aria-activedescendant-with-tabindex"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var focusableTags = map[string]bool{
	"a": true, "area": true, "button": true, "input": true,
	"select": true, "textarea": true, "summary": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	if jsxutil.FindAttribute(attrs, "aria-activedescendant") == nil {
		return
	}
	if focusableTags[tag] {
		return
	}
	if jsxutil.FindAttribute(attrs, "tabIndex") != nil {
		return
	}
	ctx.Report(el, "aria-activedescendant requires the element to be focusable — add tabIndex")
}
