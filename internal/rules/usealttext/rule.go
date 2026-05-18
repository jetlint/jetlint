// Package usealttext implements use-alt-text: images and media-like
// embedded content (<img>, <area>, <input type="image">, <object>)
// must carry an accessible label. The label can come from `alt`,
// `aria-label`, `aria-labelledby`, or `aria-hidden` (for <object>,
// also `title` or visible child content). Without any of these,
// assistive technology has nothing to announce.
//
// The rule visits self-closing JSX elements only; <object>foo</object>
// and friends carry their label in their children, which the parser
// only exposes via the surrounding JsxElement.
package usealttext

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-alt-text"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	switch tag {
	case "img", "area":
		if !hasMediaLabel(attrs) {
			ctx.Report(el, "missing accessible label: add `alt`, `aria-label`, `aria-labelledby`, or `aria-hidden`")
		}
	case "input":
		if isInputTypeImage(attrs) && !hasMediaLabel(attrs) {
			ctx.Report(el, "missing accessible label on input[type=image]: add `alt`, `aria-label`, `aria-labelledby`, or `aria-hidden`")
		}
	case "object":
		if !hasObjectLabel(attrs) {
			ctx.Report(el, "missing accessible label on `<object />`: add `title`, `aria-label`, `aria-labelledby`, `aria-hidden`, or visible child content")
		}
	}
}

func hasMediaLabel(attrs *wrapperchecker.Node) bool {
	if attrs == nil {
		return false
	}
	return jsxutil.FindAttribute(attrs, "alt") != nil ||
		jsxutil.FindAttribute(attrs, "aria-label") != nil ||
		jsxutil.FindAttribute(attrs, "aria-labelledby") != nil ||
		jsxutil.FindAttribute(attrs, "aria-hidden") != nil
}

func hasObjectLabel(attrs *wrapperchecker.Node) bool {
	if attrs == nil {
		return false
	}
	return jsxutil.FindAttribute(attrs, "title") != nil ||
		jsxutil.FindAttribute(attrs, "aria-label") != nil ||
		jsxutil.FindAttribute(attrs, "aria-labelledby") != nil ||
		jsxutil.FindAttribute(attrs, "aria-hidden") != nil
}

func isInputTypeImage(attrs *wrapperchecker.Node) bool {
	t := jsxutil.FindAttribute(attrs, "type")
	if t == nil {
		return false
	}
	v, ok := jsxutil.AttributeStringValue(t)
	return ok && v == "image"
}
