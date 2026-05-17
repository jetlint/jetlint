// Package usevalidariaprops implements use-valid-aria-props: aria-*
// attributes must match the WAI-ARIA spec. Typos (`aria-labell`,
// `aria-labeledby`) silently do nothing.
package usevalidariaprops

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-valid-aria-props"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// validAriaProps lists every ARIA attribute name in the WAI-ARIA 1.2
// spec. Anything starting with `aria-` outside this set is flagged.
var validAriaProps = map[string]bool{
	"aria-activedescendant": true, "aria-atomic": true, "aria-autocomplete": true,
	"aria-busy": true, "aria-checked": true, "aria-colcount": true,
	"aria-colindex": true, "aria-colspan": true, "aria-controls": true,
	"aria-current": true, "aria-describedby": true, "aria-description": true,
	"aria-details": true, "aria-disabled": true, "aria-dropeffect": true,
	"aria-errormessage": true, "aria-expanded": true, "aria-flowto": true,
	"aria-grabbed": true, "aria-haspopup": true, "aria-hidden": true,
	"aria-invalid": true, "aria-keyshortcuts": true, "aria-label": true,
	"aria-labelledby": true, "aria-level": true, "aria-live": true,
	"aria-modal": true, "aria-multiline": true, "aria-multiselectable": true,
	"aria-orientation": true, "aria-owns": true, "aria-placeholder": true,
	"aria-posinset": true, "aria-pressed": true, "aria-readonly": true,
	"aria-relevant": true, "aria-required": true, "aria-roledescription": true,
	"aria-rowcount": true, "aria-rowindex": true, "aria-rowspan": true,
	"aria-selected": true, "aria-setsize": true, "aria-sort": true,
	"aria-valuemax": true, "aria-valuemin": true, "aria-valuenow": true,
	"aria-valuetext": true, "aria-braillelabel": true, "aria-brailleroledescription": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil {
		return
	}
	attrs.ForEachChild(func(a *wrapperchecker.Node) bool {
		if a.Kind() != wrapperchecker.KindJsxAttribute {
			return false
		}
		name := jsxutil.AttributeName(a)
		if !strings.HasPrefix(name, "aria-") {
			return false
		}
		if !validAriaProps[name] {
			ctx.Report(a, "unknown ARIA attribute — check spelling against the WAI-ARIA spec")
		}
		return false
	})
}
