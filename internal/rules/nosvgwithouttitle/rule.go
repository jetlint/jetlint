// Package nosvgwithouttitle implements no-svg-without-title: <svg>
// elements should expose an accessible name. Acceptable forms:
// a <title> child, role="img" with aria-label/aria-labelledby, or
// a graphics-* role.
package nosvgwithouttitle

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-svg-without-title"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement: visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	if jsxutil.TagName(el) != "svg" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	// aria-hidden=true svg is decorative; skip.
	if h := jsxutil.FindAttribute(attrs, "aria-hidden"); h != nil {
		if v, ok := jsxutil.AttributeStringValue(h); ok && v == "true" {
			return
		}
	}
	role := ""
	if r := jsxutil.FindAttribute(attrs, "role"); r != nil {
		if v, ok := jsxutil.AttributeStringValue(r); ok {
			role = v
		}
	}
	roleTokens := strings.Fields(role)
	hasGraphicsRole := false
	for _, t := range roleTokens {
		if strings.HasPrefix(t, "graphics-") {
			hasGraphicsRole = true
		}
	}
	if hasGraphicsRole {
		// graphics-* roles still need a <title>; the rule accepts when
		// title is direct child OR aria-label present.
		if jsxutil.FindAttribute(attrs, "aria-label") != nil ||
			jsxutil.FindAttribute(attrs, "aria-labelledby") != nil ||
			hasTitleChild(el) {
			return
		}
		ctx.Report(el, "<svg> needs a <title> or aria-label for an accessible name")
		return
	}
	hasImgRole := false
	for _, t := range roleTokens {
		if t == "img" {
			hasImgRole = true
		}
	}
	if hasImgRole {
		// role="img" requires aria-label or aria-labelledby.
		if jsxutil.FindAttribute(attrs, "aria-label") != nil ||
			jsxutil.FindAttribute(attrs, "aria-labelledby") != nil {
			return
		}
		ctx.Report(el, "<svg role=\"img\"> needs aria-label or aria-labelledby")
		return
	}
	if hasTitleChild(el) {
		return
	}
	ctx.Report(el, "<svg> needs a <title> child for an accessible name")
}

func hasTitleChild(opening *wrapperchecker.Node) bool {
	parent := opening.Parent()
	if parent == nil {
		return false
	}
	found := false
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c == opening {
			return false
		}
		// Look at the opening tag of each child JSX element (direct child only).
		if t := jsxutil.TagName(c); t == "title" {
			found = true
			return true
		}
		// JsxElement has its own opening child; ForEachChild on the element
		// container's children gives us each top-level child element.
		c.ForEachChild(func(cc *wrapperchecker.Node) bool {
			if jsxutil.TagName(cc) == "title" {
				found = true
				return true
			}
			return false
		})
		return found
	})
	return found
}
