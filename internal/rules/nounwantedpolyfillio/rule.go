// Package nounwantedpolyfillio implements no-unwanted-polyfillio:
// flag polyfill.io `<script src>` (or `next/script` `<Script>` or
// `<NextScript>`) entries that polyfill features modern browsers
// already ship. Loading these wastes a network round-trip and
// bytes for code that will never run.
//
// The allowlist matches biome: `AbortController` and
// `IntersectionObserver` are still worth polyfilling on
// occasionally-relevant target audiences; everything else
// (Array/Object/Promise/ES2015 etc.) is considered unwanted.
package nounwantedpolyfillio

import (
	"net/url"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unwanted-polyfillio"

var allowedFeatures = map[string]bool{
	"AbortController":      true,
	"IntersectionObserver": true,
}

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
	switch jsxTagName(el) {
	case "script", "Script", "NextScript":
	default:
		return
	}
	src := getStringAttribute(el, "src")
	if src == "" || !isPolyfillIO(src) {
		return
	}
	if unwantedFeatures(src) {
		ctx.Report(el, "polyfill.io URL includes features that modern browsers already ship — drop or narrow the feature set")
	}
}

func isPolyfillIO(src string) bool {
	return strings.Contains(src, "polyfill.io") || strings.Contains(src, "polyfill-fastly.io")
}

func unwantedFeatures(src string) bool {
	u, err := url.Parse(src)
	if err != nil {
		return true
	}
	feat := u.Query().Get("features")
	if feat == "" {
		// A polyfill.io URL with no `features` arg defaults to a
		// broad ES set — definitely unwanted.
		return true
	}
	for f := range strings.SplitSeq(feat, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !allowedFeatures[f] {
			return true
		}
	}
	return false
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

// getStringAttribute returns the literal string value of the named
// attribute, or "" if absent / non-literal.
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

// jsxAttributeStringValue extracts the literal string value of an
// attribute, unwrapping `{ "..." }` JSX expression containers as
// well as direct string-literal attribute values.
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
