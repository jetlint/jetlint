// Package useariapropssupportedbyrole implements
// use-aria-props-supported-by-role: each ARIA role declares which
// aria-* properties it supports. Mixing in one that isn't supported
// (e.g. aria-checked on a link) confuses AT users — the property is
// ignored on devices that respect the spec and misinterpreted on
// ones that don't.
package useariapropssupportedbyrole

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-aria-props-supported-by-role"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// Roles that support each gated aria-* attribute. Global aria-* (label,
// labelledby, etc.) are supported by every role and aren't gated here.
var rolesSupporting = map[string]map[string]bool{
	"aria-checked": {
		"checkbox": true, "menuitemcheckbox": true, "menuitemradio": true,
		"option": true, "radio": true, "switch": true, "treeitem": true,
	},
	"aria-selected": {
		"gridcell": true, "option": true, "row": true, "tab": true,
		"columnheader": true, "rowheader": true, "treeitem": true,
	},
	"aria-expanded": {
		"application": true, "button": true, "checkbox": true, "combobox": true,
		"gridcell": true, "link": true, "listbox": true, "menuitem": true,
		"row": true, "rowheader": true, "tab": true, "treeitem": true,
		"document": true, "section": true, "sectionhead": true, "window": true,
		"alertdialog": true, "dialog": true, "tabpanel": true, "menu": true,
		"menubar": true, "tree": true,
	},
	"aria-pressed": {"button": true},
	"aria-valuemin": {
		"slider": true, "scrollbar": true, "spinbutton": true,
		"separator": true, "progressbar": true, "meter": true,
	},
	"aria-valuemax": {
		"slider": true, "scrollbar": true, "spinbutton": true,
		"separator": true, "progressbar": true, "meter": true,
	},
	"aria-valuenow": {
		"slider": true, "scrollbar": true, "spinbutton": true,
		"separator": true, "progressbar": true, "meter": true,
	},
	"aria-modal":           {"dialog": true, "alertdialog": true},
	"aria-multiselectable": {"grid": true, "listbox": true, "tree": true, "tablist": true},
	"aria-readonly": {
		"checkbox": true, "combobox": true, "grid": true, "gridcell": true,
		"listbox": true, "radiogroup": true, "slider": true, "spinbutton": true,
		"switch": true, "textbox": true, "columnheader": true, "rowheader": true,
		"searchbox": true,
	},
}

// implicitRole returns the role we infer for a tag, taking attribute
// hints into account (a[href] → link, input[type=...] → various).
func implicitRole(tag string, attrs *wrapperchecker.Node) string {
	switch tag {
	case "a", "area":
		if jsxutil.FindAttribute(attrs, "href") != nil {
			return "link"
		}
		return ""
	case "link":
		if jsxutil.FindAttribute(attrs, "href") != nil {
			return "link"
		}
	case "article":
		return "article"
	case "aside":
		return "complementary"
	case "nav":
		return "navigation"
	case "main":
		return "main"
	case "header":
		return "banner"
	case "footer":
		return "contentinfo"
	case "section":
		return "region"
	case "button":
		return "button"
	case "select":
		return "combobox"
	case "textarea":
		return "textbox"
	case "ul", "ol":
		return "list"
	case "li":
		return "listitem"
	case "details":
		return "group"
	case "dialog":
		return "dialog"
	case "fieldset":
		return "group"
	case "figure":
		return "figure"
	case "form":
		return "form"
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return "heading"
	case "hr":
		return "separator"
	case "html":
		return "document"
	case "img":
		if alt := jsxutil.FindAttribute(attrs, "alt"); alt != nil {
			if v, ok := jsxutil.AttributeStringValue(alt); ok && v == "" {
				return "presentation"
			}
			return "img"
		}
		return "img"
	case "input":
		t := ""
		if ta := jsxutil.FindAttribute(attrs, "type"); ta != nil {
			t, _ = jsxutil.AttributeStringValue(ta)
		}
		switch t {
		case "button", "submit", "reset", "image":
			return "button"
		case "checkbox":
			return "checkbox"
		case "radio":
			return "radio"
		case "range":
			return "slider"
		case "search":
			return "searchbox"
		case "number":
			return "spinbutton"
		case "email", "tel", "url", "text", "":
			return "textbox"
		}
	case "menu":
		// menu[type=toolbar] historically had role=toolbar; otherwise list.
		if ta := jsxutil.FindAttribute(attrs, "type"); ta != nil {
			if v, _ := jsxutil.AttributeStringValue(ta); v == "toolbar" {
				return "toolbar"
			}
		}
		return "list"
	case "menuitem":
		if ta := jsxutil.FindAttribute(attrs, "type"); ta != nil {
			if v, _ := jsxutil.AttributeStringValue(ta); v == "checkbox" {
				return "menuitemcheckbox"
			} else if v == "radio" {
				return "menuitemradio"
			}
		}
		return "menuitem"
	case "meter":
		return "meter"
	case "option":
		return "option"
	case "output":
		return "status"
	case "progress":
		return "progressbar"
	case "table":
		return "table"
	case "tbody", "tfoot", "thead":
		return "rowgroup"
	case "td":
		return "cell"
	case "th":
		return "columnheader"
	case "tr":
		return "row"
	case "summary":
		return "button"
	}
	return ""
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil || jsxutil.HasSpread(attrs) {
		return
	}
	role := ""
	if r := jsxutil.FindAttribute(attrs, "role"); r != nil {
		if v, ok := jsxutil.AttributeStringValue(r); ok && v != "" {
			role = v
		}
	}
	if role == "" {
		role = implicitRole(tag, attrs)
	}
	if role == "" {
		return
	}
	// For each gated aria-* prop, verify the role is in the allowlist.
	for prop, allowed := range rolesSupporting {
		if a := jsxutil.FindAttribute(attrs, prop); a != nil {
			if !allowed[role] {
				ctx.Report(a, prop+" isn't supported by role \""+role+"\"")
			}
		}
	}
}
