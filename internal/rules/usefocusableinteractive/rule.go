// Package usefocusableinteractive implements use-focusable-interactive:
// an element carrying an interactive ARIA role must also be focusable.
// Without tabIndex (or being inherently focusable) the keyboard never
// reaches it.
package usefocusableinteractive

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-focusable-interactive"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var interactiveRoles = map[string]bool{
	"button": true, "checkbox": true, "combobox": true, "gridcell": true,
	"link": true, "menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"option": true, "radio": true, "scrollbar": true, "searchbox": true,
	"slider": true, "spinbutton": true, "switch": true, "tab": true, "textbox": true,
	"tree": true, "treeitem": true,
}

// Tags that are naturally focusable — adding tabIndex is unnecessary.
var nativelyFocusable = map[string]bool{
	"button": true, "input": true, "select": true, "textarea": true,
	"area": true, "audio": true, "video": true, "iframe": true, "summary": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) || nativelyFocusable[tag] {
		return
	}
	// <a href> is focusable.
	if tag == "a" {
		if jsxutil.FindAttribute(jsxutil.AttributesNode(el), "href") != nil {
			return
		}
	}
	attrs := jsxutil.AttributesNode(el)
	r := jsxutil.FindAttribute(attrs, "role")
	if r == nil {
		return
	}
	v, ok := jsxutil.AttributeStringValue(r)
	if !ok || !interactiveRoles[v] {
		return
	}
	if jsxutil.FindAttribute(attrs, "tabIndex") != nil {
		return
	}
	ctx.Report(el, "element with interactive role needs tabIndex so keyboard users can reach it")
}
