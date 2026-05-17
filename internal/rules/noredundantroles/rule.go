// Package noredundantroles implements no-redundant-roles: setting an
// explicit `role` that matches the element's implicit role adds noise
// without changing semantics. e.g. `<button role="button">`.
package noredundantroles

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-redundant-roles"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// Tag → set of implicit ARIA roles. Some tags have multiple
// possible roles depending on attributes (handled below for input).
var implicitRoles = map[string]map[string]bool{
	"article":  {"article": true},
	"button":   {"button": true},
	"h1":       {"heading": true},
	"h2":       {"heading": true},
	"h3":       {"heading": true},
	"h4":       {"heading": true},
	"h5":       {"heading": true},
	"h6":       {"heading": true},
	"dialog":   {"dialog": true},
	"figure":   {"figure": true},
	"form":     {"form": true},
	"fieldset": {"group": true},
	"img":      {"img": true, "presentation": true, "none": true},
	"ol":       {"list": true},
	"ul":       {"list": true},
	"li":       {"listitem": true},
	"nav":      {"navigation": true},
	"tr":       {"row": true},
	"tbody":    {"rowgroup": true},
	"tfoot":    {"rowgroup": true},
	"thead":    {"rowgroup": true},
	"table":    {"table": true},
	"textarea": {"textbox": true},
	"a":        {"link": true},
	// Generic elements — all map to implicit "generic".
	"b":     {"generic": true},
	"bdi":   {"generic": true},
	"bdo":   {"generic": true},
	"data":  {"generic": true},
	"div":   {"generic": true},
	"i":     {"generic": true},
	"pre":   {"generic": true},
	"q":     {"generic": true},
	"samp":  {"generic": true},
	"small": {"generic": true},
	"span":  {"generic": true},
	"u":     {"generic": true},
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	r := jsxutil.FindAttribute(attrs, "role")
	if r == nil {
		return
	}
	role, ok := jsxutil.AttributeStringValue(r)
	if !ok {
		return
	}
	// Special-case <a href> → link, <select> multiple → listbox, etc.
	implicit := implicitRoles[tag]
	if tag == "a" {
		if jsxutil.FindAttribute(attrs, "href") == nil {
			implicit = nil
		}
	}
	if tag == "input" {
		t := ""
		if ta := jsxutil.FindAttribute(attrs, "type"); ta != nil {
			t, _ = jsxutil.AttributeStringValue(ta)
		}
		switch t {
		case "button", "submit", "reset":
			implicit = map[string]bool{"button": true}
		case "checkbox":
			implicit = map[string]bool{"checkbox": true}
		case "radio":
			implicit = map[string]bool{"radio": true}
		case "range":
			implicit = map[string]bool{"slider": true}
		case "search":
			implicit = map[string]bool{"searchbox": true}
		case "text", "":
			implicit = map[string]bool{"textbox": true}
		case "email", "tel", "url":
			implicit = map[string]bool{"textbox": true}
		case "number":
			implicit = map[string]bool{"spinbutton": true}
		}
	}
	if tag == "select" {
		multiple := jsxutil.FindAttribute(attrs, "multiple") != nil
		sizeAttr := jsxutil.FindAttribute(attrs, "size")
		size := ""
		if sizeAttr != nil {
			size, _ = jsxutil.AttributeStringValue(sizeAttr)
		}
		if multiple || (size != "" && size != "1") {
			implicit = map[string]bool{"listbox": true}
		} else {
			implicit = map[string]bool{"combobox": true}
		}
	}
	if implicit[role] {
		ctx.Report(r, "role=\""+role+"\" is the implicit role for <"+tag+">; remove it")
	}
}
