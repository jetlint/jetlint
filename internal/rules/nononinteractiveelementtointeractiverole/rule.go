// Package nononinteractiveelementtointeractiverole implements
// no-noninteractive-element-to-interactive-role: a non-interactive
// HTML element (h1, li, main, …) should not carry an interactive
// ARIA role (button, checkbox, …) — keyboard semantics won't follow
// and AT users get a half-broken control.
package nononinteractiveelementtointeractiverole

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-noninteractive-element-to-interactive-role"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var nonInteractiveTags = map[string]bool{
	"main": true, "article": true, "aside": true, "blockquote": true, "body": true,
	"br": true, "caption": true, "dd": true, "details": true, "dir": true,
	"dfn": true, "dl": true, "dt": true, "fieldset": true, "figcaption": true,
	"figure": true, "footer": true, "form": true, "frame": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"hr": true, "iframe": true, "img": true, "label": true, "legend": true,
	"li": true, "mark": true, "marquee": true, "menu": true, "meter": true,
	"nav": true, "ol": true, "optgroup": true, "output": true, "p": true,
	"pre": true, "progress": true, "ruby": true, "table": true, "tbody": true,
	"td": true, "tfoot": true, "th": true, "thead": true, "time": true, "ul": true,
}

var interactiveRoles = map[string]bool{
	"button": true, "checkbox": true, "combobox": true, "gridcell": true,
	"link": true, "listbox": true, "menuitem": true, "menuitemcheckbox": true,
	"menuitemradio": true, "option": true, "radio": true, "row": true,
	"scrollbar": true, "searchbox": true, "slider": true, "spinbutton": true,
	"switch": true, "tab": true, "textbox": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !nonInteractiveTags[tag] {
		return
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
	ctx.Report(r, "interactive role on non-interactive <"+tag+"> — keyboard semantics don't carry over")
}
