// Package usegooglefontpreconnect implements
// use-google-font-preconnect: flag `<link href="https://fonts.gstatic.com">`
// (and variants) that don't carry `rel="preconnect"`. Google Fonts'
// stylesheet domain (gstatic.com) needs an early TCP/TLS handshake
// to avoid a layout-shifting font swap; `preconnect` opens that
// connection during HTML parse.
package usegooglefontpreconnect

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-google-font-preconnect"

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
	if jsxTagName(el) != "link" {
		return
	}
	href := getStringAttribute(el, "href")
	if !strings.Contains(href, "fonts.gstatic.com") {
		return
	}
	if getStringAttribute(el, "rel") == "preconnect" {
		return
	}
	ctx.Report(el, `<link href="...gstatic.com"> needs rel="preconnect" to avoid font-swap layout shift`)
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

func getStringAttribute(el *wrapperchecker.Node, name string) string {
	var value string
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttributes {
			return false
		}
		c.ForEachChild(func(attr *wrapperchecker.Node) bool {
			if attr.Kind() != wrapperchecker.KindJsxAttribute {
				return false
			}
			if jsxAttributeName(attr) != name {
				return false
			}
			value = jsxAttributeStringValue(attr)
			return true
		})
		return true
	})
	return value
}

func jsxAttributeName(attr *wrapperchecker.Node) string {
	var name string
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func jsxAttributeStringValue(attr *wrapperchecker.Node) string {
	var out string
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral:
			out = c.LiteralText()
			return true
		case wrapperchecker.KindJsxExpression:
			c.ForEachChild(func(e *wrapperchecker.Node) bool {
				if e.Kind() == wrapperchecker.KindStringLiteral ||
					e.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
					out = e.LiteralText()
					return true
				}
				return false
			})
			return true
		}
		return false
	})
	return out
}
