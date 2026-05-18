// Package usebuttontype implements use-button-type: a <button>
// without an explicit `type` defaults to "submit", which submits a
// surrounding <form> on click. The bug doesn't surface until the
// component is dropped into a form. Be explicit.
package usebuttontype

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-button-type"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var allowedTypes = map[string]bool{"button": true, "submit": true, "reset": true}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if tag != "button" && tag != "input" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	if tag == "input" {
		// input always has a type; only flag input[type] missing for actionable inputs.
		// Other input types have their own defaults; not the rule's concern.
		return
	}
	t := jsxutil.FindAttribute(attrs, "type")
	if t == nil {
		ctx.Report(el, "<button> needs an explicit type (button, submit, or reset)")
		return
	}
	v, ok := jsxutil.AttributeStringValue(t)
	if !ok {
		return
	}
	if !allowedTypes[v] {
		ctx.Report(t, "<button type=\""+v+"\"> isn't valid — use button/submit/reset")
	}
}
