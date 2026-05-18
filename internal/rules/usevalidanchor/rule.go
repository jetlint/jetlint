// Package usevalidanchor implements use-valid-anchor: an <a> tag is
// a link; without a real href it's an empty hole keyboard users
// can't tab to and AT users can't follow. Hash-only or
// javascript: hrefs are the same problem in disguise.
package usevalidanchor

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-valid-anchor"

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
	if jsxutil.TagName(el) != "a" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	href := jsxutil.FindAttribute(attrs, "href")
	if href == nil {
		ctx.Report(el, "<a> needs an href")
		return
	}
	// Bare `href` attribute, `href={null}`, or `href={undefined}`.
	v, ok := jsxutil.AttributeStringValue(href)
	if !ok {
		if jsxutil.AttributeIsNullishExpression(href) || isBareAttribute(href) {
			ctx.Report(href, "<a href> needs a real value")
			return
		}
		// Other expressions: assume runtime value is meaningful.
		return
	}
	if v == "" {
		ctx.Report(href, "<a href=\"\"> isn't a usable link")
		return
	}
	if v == "#" {
		ctx.Report(href, "<a href=\"#\"> isn't a destination — use a <button> or a real URL")
		return
	}
	if strings.HasPrefix(strings.ToLower(v), "javascript:") {
		ctx.Report(href, "<a href=\"javascript:…\"> hides the link target — use a <button> for actions")
		return
	}
}

func isBareAttribute(attr *wrapperchecker.Node) bool {
	hasInitializer := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			return false
		}
		hasInitializer = true
		return true
	})
	return !hasInitializer
}
