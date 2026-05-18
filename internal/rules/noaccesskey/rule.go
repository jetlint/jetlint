// Package noaccesskey implements no-access-key: flag JSX
// `<element accessKey="...">` on built-in HTML elements. Access keys
// override the user's browser shortcuts; the resulting collisions
// make sites less accessible, not more.
//
// Component-flavored tags (uppercase or member access — `<Input />`,
// `<RadioGroup.Radio />`) are out of scope: the rule can't tell what
// the component does with the prop.
package noaccesskey

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-access-key"

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
	attr := jsxutil.FindAttribute(jsxutil.AttributesNode(el), "accessKey")
	if attr == nil {
		return
	}
	// `accessKey={undefined}` / `accessKey={null}` is the idiomatic
	// way to disable the prop while keeping its name in place.
	if jsxutil.AttributeIsNullishExpression(attr) {
		return
	}
	ctx.Report(attr, "accessKey on a DOM element collides with browser shortcuts — remove it")
}
