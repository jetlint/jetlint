// Package usekeywithmouseevents implements use-key-with-mouse-events:
// `onMouseOver` / `onMouseOut` should be paired with `onFocus` /
// `onBlur` so keyboard users get the same affordance.
package usekeywithmouseevents

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-key-with-mouse-events"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	if !jsxutil.IsHTMLElement(jsxutil.TagName(el)) {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	if jsxutil.FindAttribute(attrs, "onMouseOver") != nil && jsxutil.FindAttribute(attrs, "onFocus") == nil {
		ctx.Report(el, "onMouseOver without onFocus leaves keyboard users without the hover affordance")
		return
	}
	if jsxutil.FindAttribute(attrs, "onMouseOut") != nil && jsxutil.FindAttribute(attrs, "onBlur") == nil {
		ctx.Report(el, "onMouseOut without onBlur leaves keyboard users without the leave affordance")
	}
}
