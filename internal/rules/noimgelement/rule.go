// Package noimgelement implements no-img-element: in a Next.js
// project the raw `<img>` element bypasses Next's image
// optimization pipeline (responsive sizing, lazy loading,
// AVIF/WebP). Use `next/image` instead.
//
// `<picture><img></picture>` is allowed: a `<picture>` parent
// signals that the developer is already handling responsive sources
// manually, which `next/image` would interfere with.
package noimgelement

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-img-element"

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
	if jsxTagName(el) != "img" {
		return
	}
	// Bare `<img />` (no attributes) is a placeholder, not a real
	// usage — biome treats it as out-of-scope for this rule.
	if !hasAnyAttribute(el) {
		return
	}
	if hasPictureAncestor(el) {
		return
	}
	ctx.Report(el, "<img> bypasses Next.js image optimization — use `next/image`")
}

func hasAnyAttribute(el *wrapperchecker.Node) bool {
	found := false
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxAttributes {
			c.ForEachChild(func(a *wrapperchecker.Node) bool {
				if a.Kind() == wrapperchecker.KindJsxAttribute {
					found = true
					return true
				}
				return false
			})
			return true
		}
		return false
	})
	return found
}

func jsxTagName(el *wrapperchecker.Node) string {
	var name string
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// hasPictureAncestor walks parents looking for a JsxElement whose
// opening tag is `<picture>`. The wrapper doesn't surface
// KindJsxElement directly, so we check each ancestor's source text
// prefix instead — `<picture` is unambiguous and cheap.
func hasPictureAncestor(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		text := strings.TrimSpace(p.SourceText())
		if strings.HasPrefix(text, "<picture>") || strings.HasPrefix(text, "<picture ") {
			return true
		}
	}
	return false
}
