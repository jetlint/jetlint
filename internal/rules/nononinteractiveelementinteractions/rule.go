// Package nononinteractiveelementinteractions implements
// no-noninteractive-element-interactions: putting a click/key handler
// on a non-interactive HTML element (div, h1, li, …) is semantically
// wrong — it asks AT software to expose something interactive while
// the element itself doesn't advertise that role. Either change the
// tag or add an interactive role + tabIndex.
package nononinteractiveelementinteractions

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-noninteractive-element-interactions"

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
	"a": true, "area": true, "button": true, "input": true, "menuitem": true,
	"option": true, "select": true, "textarea": true, "summary": true,
	"tr": true,
}

var interactiveRoles = map[string]bool{
	"button": true, "checkbox": true, "combobox": true, "gridcell": true,
	"link": true, "menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"option": true, "radio": true, "row": true, "scrollbar": true, "searchbox": true,
	"slider": true, "spinbutton": true, "switch": true, "tab": true, "textbox": true,
	"listbox": true, "tree": true, "treeitem": true,
}

// Handlers the rule cares about. onClick alone is enough for the
// biome compatibility test, but we cover the obvious siblings so the
// rule does what its name promises.
var interactiveHandlers = []string{
	"onClick", "onMouseDown", "onMouseUp", "onKeyDown", "onKeyUp", "onKeyPress",
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) || interactiveTags[tag] {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	// role="presentation"/"none" opts out of a11y semantics.
	if r := jsxutil.FindAttribute(attrs, "role"); r != nil {
		if v, ok := jsxutil.AttributeStringValue(r); ok {
			if v == "presentation" || v == "none" || interactiveRoles[v] {
				return
			}
		}
	}
	// aria-hidden truthy = not in the a11y tree.
	if h := jsxutil.FindAttribute(attrs, "aria-hidden"); h != nil {
		if v, ok := jsxutil.AttributeStringValue(h); !ok || v != "false" {
			return
		}
	}
	for _, name := range interactiveHandlers {
		if handler := jsxutil.FindAttribute(attrs, name); handler != nil {
			ctx.Report(handler, name+" on a non-interactive element fakes an interactive control — use a real <button>/<a> or add an interactive role + tabIndex")
			return
		}
	}
}
