// Package useariapropsforrole implements use-aria-props-for-role:
// each interactive ARIA role declares "required" properties (e.g.
// role="checkbox" requires aria-checked). Without them, AT users get
// a half-described widget.
package useariapropsforrole

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-aria-props-for-role"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// Required ARIA props by role (per WAI-ARIA 1.2).
var requiredProps = map[string][]string{
	"checkbox":         {"aria-checked"},
	"switch":           {"aria-checked"},
	"radio":            {"aria-checked"},
	"menuitemcheckbox": {"aria-checked"},
	"menuitemradio":    {"aria-checked"},
	"option":           {"aria-selected"},
	"combobox":         {"aria-controls", "aria-expanded"},
	"heading":          {"aria-level"},
	"slider":           {"aria-valuemin", "aria-valuemax", "aria-valuenow"},
	"scrollbar":        {"aria-valuemin", "aria-valuemax", "aria-valuenow", "aria-orientation", "aria-controls"},
	"spinbutton":       {"aria-valuemin", "aria-valuemax", "aria-valuenow"},
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	if !jsxutil.IsHTMLElement(jsxutil.TagName(el)) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil {
		return
	}
	r := jsxutil.FindAttribute(attrs, "role")
	if r == nil {
		return
	}
	role, ok := jsxutil.AttributeStringValue(r)
	if !ok {
		return
	}
	required, has := requiredProps[role]
	if !has {
		return
	}
	for _, prop := range required {
		if jsxutil.FindAttribute(attrs, prop) == nil {
			ctx.Report(r, "role=\""+role+"\" requires "+prop)
			return
		}
	}
}
