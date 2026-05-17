// Package noredundantalt implements no-redundant-alt: `alt` text on
// <img> should describe the image, not announce that it's an image.
// Words like "photo", "image", and "picture" are redundant — screen
// readers already say "image". Substring matches that happen to live
// inside other words (e.g. "photography") are fine.
package noredundantalt

import (
	"regexp"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-redundant-alt"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// Word-boundary match on "photo", "image", or "picture", case-insensitive.
var redundantWord = regexp.MustCompile(`(?i)\b(photo|image|picture)\b`)

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	if jsxutil.TagName(el) != "img" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	alt := jsxutil.FindAttribute(attrs, "alt")
	if alt == nil {
		return
	}
	// Aria-hidden truthy → element is decorative; alt content doesn't matter.
	if h := jsxutil.FindAttribute(attrs, "aria-hidden"); h != nil {
		v, ok := jsxutil.AttributeStringValue(h)
		if ok && v != "false" {
			return
		}
		if !ok {
			// Bare attribute (`aria-hidden`) or expression — treat as decorative
			// unless it's clearly `{false}`.
			if !ariaHiddenIsExplicitFalse(h) {
				return
			}
		}
	}
	text, hadStaticText, ok := altStaticText(alt)
	if !ok {
		return
	}
	if !hadStaticText {
		return
	}
	if redundantWord.MatchString(text) {
		ctx.Report(alt, "alt text shouldn't say \"photo\"/\"image\"/\"picture\" — the screen reader already announces it")
	}
}

func ariaHiddenIsExplicitFalse(attr *wrapperchecker.Node) bool {
	// Look inside the JsxExpression for `false`.
	isFalse := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxExpression {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			if e.Kind() == wrapperchecker.KindFalseKeyword {
				isFalse = true
			}
			return true
		})
		return true
	})
	return isFalse
}

// altStaticText returns the static portion of the alt value (string literal,
// no-substitution template, or the fixed parts of a template literal).
// hadStaticText is false if the attribute was a bare `alt`, undefined/null,
// a function, or a pure identifier with no literal content.
// ok is false if the attribute should be ignored entirely (no value to check).
func altStaticText(attr *wrapperchecker.Node) (text string, hadStaticText bool, ok bool) {
	if s, has := jsxutil.AttributeStringValue(attr); has {
		return s, true, true
	}
	// Iterate JsxExpression child.
	var out strings.Builder
	hasStatic := false
	bailout := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxExpression {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			switch e.Kind() {
			case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
				out.WriteString(e.LiteralText())
				hasStatic = true
			case wrapperchecker.KindTemplateExpression:
				// Walk template parts.
				e.ForEachChild(func(t *wrapperchecker.Node) bool {
					switch t.Kind() {
					case wrapperchecker.KindTemplateHead, wrapperchecker.KindTemplateMiddle, wrapperchecker.KindTemplateTail:
						out.WriteString(t.LiteralText())
						hasStatic = true
					}
					return true
				})
			case wrapperchecker.KindNullKeyword, wrapperchecker.KindUndefinedKeyword,
				wrapperchecker.KindIdentifier, wrapperchecker.KindPropertyAccessExpression,
				wrapperchecker.KindCallExpression, wrapperchecker.KindArrowFunction,
				wrapperchecker.KindFunctionExpression:
				bailout = true
			}
			return true
		})
		return true
	})
	if bailout && !hasStatic {
		return "", false, false
	}
	return out.String(), hasStatic, true
}
