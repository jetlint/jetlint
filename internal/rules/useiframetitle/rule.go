// Package useiframetitle implements use-iframe-title: every <iframe>
// needs a non-empty `title` attribute so screen-reader users can
// understand what's embedded.
package useiframetitle

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-iframe-title"

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
	if jsxutil.TagName(el) != "iframe" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	attr := jsxutil.FindAttribute(attrs, "title")
	if attr == nil {
		ctx.Report(el, "<iframe> needs a title attribute for screen readers")
		return
	}
	if s, ok := jsxutil.AttributeStringValue(attr); ok && s == "" {
		ctx.Report(attr, "<iframe title=\"\"> is no help — describe what's embedded")
	}
}
