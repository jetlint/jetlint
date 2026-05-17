// Package nointeractiveelementtononinteractiverole implements
// no-interactive-element-to-noninteractive-role: an interactive
// element (button, a[href], input, …) shouldn't claim a
// non-interactive role like "img" or "listitem".
package nointeractiveelementtononinteractiverole

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-interactive-element-to-noninteractive-role"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// Tags whose default semantics are interactive.
// `input` is conditional on `type` — handled below.
var interactiveTags = map[string]bool{
	"button": true, "menuitem": true, "option": true, "select": true,
	"textarea": true, "tr": true,
}

// Roles considered non-interactive (subset that biome flags here).
var nonInteractiveRoles = map[string]bool{
	"article": true, "banner": true, "complementary": true, "contentinfo": true,
	"definition": true, "directory": true, "document": true, "feed": true,
	"figure": true, "img": true, "list": true, "listitem": true, "main": true,
	"navigation": true, "none": true, "note": true, "region": true, "search": true,
	"separator": true, "status": true, "term": true, "timer": true, "tooltip": true,
	"presentation": true, "marquee": true, "log": true, "alert": true, "alertdialog": true,
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
	v, ok := jsxutil.AttributeStringValue(r)
	if !ok || !nonInteractiveRoles[v] {
		return
	}
	interactive := interactiveTags[tag]
	if tag == "a" || tag == "area" {
		interactive = jsxutil.FindAttribute(attrs, "href") != nil
	}
	if tag == "input" {
		t := ""
		if ta := jsxutil.FindAttribute(attrs, "type"); ta != nil {
			t, _ = jsxutil.AttributeStringValue(ta)
		}
		interactive = t != "hidden"
	}
	if !interactive {
		return
	}
	ctx.Report(r, "non-interactive role on interactive <"+tag+"> hides its semantics from AT")
}
