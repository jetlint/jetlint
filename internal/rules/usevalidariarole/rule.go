// Package usevalidariarole implements use-valid-aria-role: the
// `role` attribute must name a role defined in WAI-ARIA. Typos or
// invented roles are silently ignored by assistive tech.
package usevalidariarole

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-valid-aria-role"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// validRoles is the WAI-ARIA 1.2 role catalog.
var validRoles = map[string]bool{
	"alert": true, "alertdialog": true, "application": true, "article": true,
	"banner": true, "button": true, "cell": true, "checkbox": true,
	"columnheader": true, "combobox": true, "complementary": true, "contentinfo": true,
	"definition": true, "dialog": true, "directory": true, "document": true,
	"feed": true, "figure": true, "form": true, "grid": true, "gridcell": true,
	"group": true, "heading": true, "img": true, "link": true, "list": true,
	"listbox": true, "listitem": true, "log": true, "main": true, "marquee": true,
	"math": true, "menu": true, "menubar": true, "menuitem": true, "menuitemcheckbox": true,
	"menuitemradio": true, "navigation": true, "none": true, "note": true,
	"option": true, "presentation": true, "progressbar": true, "radio": true,
	"radiogroup": true, "region": true, "row": true, "rowgroup": true, "rowheader": true,
	"scrollbar": true, "search": true, "searchbox": true, "separator": true,
	"slider": true, "spinbutton": true, "status": true, "switch": true,
	"tab": true, "table": true, "tablist": true, "tabpanel": true, "term": true,
	"textbox": true, "timer": true, "toolbar": true, "tooltip": true, "tree": true,
	"treegrid": true, "treeitem": true, "blockquote": true, "caption": true,
	"code": true, "deletion": true, "emphasis": true, "generic": true,
	"insertion": true, "meter": true, "paragraph": true, "strong": true,
	"subscript": true, "superscript": true, "time": true,
	"graphics-document": true, "graphics-object": true, "graphics-symbol": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	attrs := jsxutil.AttributesNode(el)
	roleAttr := jsxutil.FindAttribute(attrs, "role")
	if roleAttr == nil {
		return
	}
	v, ok := jsxutil.AttributeStringValue(roleAttr)
	if !ok {
		return
	}
	if v == "" {
		ctx.Report(roleAttr, "empty role attribute — pick a WAI-ARIA role or remove")
		return
	}
	// Multi-role tokens are allowed; the FIRST one a UA finds in
	// the catalog wins. If none match, flag.
	for tok := range strings.FieldsSeq(v) {
		if validRoles[tok] {
			return
		}
	}
	ctx.Report(roleAttr, "unknown ARIA role — check the WAI-ARIA spec for valid names")
}
