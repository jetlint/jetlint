// Package usekeywithclickevents implements use-key-with-click-events:
// non-interactive HTML elements with `onClick` also need a keyboard
// handler (`onKeyUp`, `onKeyDown`, or `onKeyPress`) so keyboard users
// can activate them. Already-interactive tags (button, input, etc.)
// handle keyboard activation natively. Elements hidden from the
// accessibility tree are skipped.
package usekeywithclickevents

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-key-with-click-events"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var interactiveTags = map[string]bool{
	"a": true, "area": true, "button": true, "input": true, "option": true,
	"select": true, "textarea": true, "summary": true, "details": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) || interactiveTags[tag] {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	if jsxutil.FindAttribute(attrs, "onClick") == nil {
		return
	}
	if h := jsxutil.FindAttribute(attrs, "aria-hidden"); h != nil {
		if v, ok := jsxutil.AttributeStringValue(h); !ok || v == "true" {
			return
		}
	}
	// role="presentation" / role="none" opts out of a11y semantics
	// entirely — the rule doesn't apply.
	if r := jsxutil.FindAttribute(attrs, "role"); r != nil {
		if v, ok := jsxutil.AttributeStringValue(r); ok && (v == "presentation" || v == "none") {
			return
		}
	}
	if jsxutil.FindAttribute(attrs, "onKeyUp") != nil ||
		jsxutil.FindAttribute(attrs, "onKeyDown") != nil ||
		jsxutil.FindAttribute(attrs, "onKeyPress") != nil {
		return
	}
	ctx.Report(el, "onClick on a non-interactive element needs a paired key handler for keyboard users")
}
