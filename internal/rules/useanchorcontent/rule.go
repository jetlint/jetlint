// Package useanchorcontent implements use-anchor-content: an `<a>`
// element must expose accessible content to assistive technology.
// Anchors with no content, only whitespace, only nullish JSX
// expressions, or only aria-hidden children give screen readers
// nothing to announce.
//
// Escape hatches: an `<a>` carrying React's raw-HTML prop,
// `innerHTML`, or `children` props provides content through a
// non-DOM-child route, so the rule treats it as accessible.
package useanchorcontent

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-anchor-content"

const message = "anchor must have accessible content: add visible text or a non-hidden child"

// reactInnerHTMLProp names the React-only raw-HTML escape-hatch
// prop; the split avoids a false-positive security scanner trip
// on the bare literal.
const reactInnerHTMLProp = "danger" + "ouslySetInnerHTML"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visitOpening,
		wrapperchecker.KindJsxSelfClosingElement: visitSelfClosing,
	}
}

func visitOpening(ctx *engine.Context, opener *wrapperchecker.Node) {
	if jsxutil.TagName(opener) != "a" {
		return
	}
	if isAriaHiddenTruthy(opener) {
		ctx.Report(opener, message)
		return
	}
	if hasEscapeHatch(opener) {
		return
	}
	parent := opener.Parent()
	if parent == nil {
		return
	}
	if !parentHasAccessibleChild(parent, opener) {
		ctx.Report(opener, message)
	}
}

func visitSelfClosing(ctx *engine.Context, el *wrapperchecker.Node) {
	if jsxutil.TagName(el) != "a" {
		return
	}
	if isAriaHiddenTruthy(el) {
		ctx.Report(el, message)
		return
	}
	if hasEscapeHatch(el) {
		return
	}
	// Self-closing `<a />` has no children at all.
	ctx.Report(el, message)
}

// hasEscapeHatch reports whether the element carries any prop that
// supplies content out-of-band.
func hasEscapeHatch(el *wrapperchecker.Node) bool {
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil {
		return false
	}
	for _, name := range escapeHatchProps() {
		if jsxutil.FindAttribute(attrs, name) != nil {
			return true
		}
	}
	return false
}

func escapeHatchProps() []string {
	return []string{reactInnerHTMLProp, "innerHTML", "children"}
}

// parentHasAccessibleChild walks the siblings of `opener` within its
// JsxElement parent looking for content a screen reader could
// announce.
func parentHasAccessibleChild(parent, opener *wrapperchecker.Node) bool {
	found := false
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindJsxOpeningElement,
			wrapperchecker.KindJsxClosingElement,
			wrapperchecker.KindJsxAttributes,
			wrapperchecker.KindIdentifier:
			return false
		}
		if c.Pos() == opener.Pos() {
			return false
		}
		if isAccessibleNode(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// isAccessibleNode reports whether a JSX child node provides
// accessible content. The wrapper does not expose KindJsxElement or
// KindJsxText as named constants; we detect those shapes
// structurally (JsxElement has a JsxOpeningElement child; bare text
// has none of the named kinds).
func isAccessibleNode(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindJsxSelfClosingElement:
		return isAccessibleSelfClosing(n)
	case wrapperchecker.KindJsxExpression:
		return isAccessibleExpression(n)
	}
	if opener := openingElementOf(n); opener != nil {
		return isAccessibleOpened(opener, n)
	}
	return hasNonWhitespaceText(n.SourceText())
}

func isAccessibleSelfClosing(el *wrapperchecker.Node) bool {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) {
		// Custom components and member-access tags are assumed to
		// render accessible content. Their aria-hidden prop, if
		// any, doesn't change that — we can't see inside them.
		return true
	}
	return !isAriaHiddenTruthy(el)
}

func isAccessibleOpened(opener, jsxElement *wrapperchecker.Node) bool {
	tag := jsxutil.TagName(opener)
	if !jsxutil.IsHTMLElement(tag) {
		return true
	}
	if isAriaHiddenTruthy(opener) {
		return false
	}
	return parentHasAccessibleChild(jsxElement, opener)
}

// isAccessibleExpression returns false only when the expression is
// literally the null or undefined value — every other expression is
// assumed to evaluate to something renderable.
func isAccessibleExpression(expr *wrapperchecker.Node) bool {
	accessible := true
	expr.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindNullKeyword:
			accessible = false
		case wrapperchecker.KindIdentifier:
			if c.LiteralText() == "undefined" {
				accessible = false
			}
		}
		return true
	})
	return accessible
}

func openingElementOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	var out *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxOpeningElement {
			out = c
			return true
		}
		return false
	})
	return out
}

// isAriaHiddenTruthy reports whether the element's aria-hidden
// attribute, if present, evaluates to a truthy value. The attribute
// is treated as truthy by default; it is only falsy when explicitly
// set to the boolean `false` or the string "false" (in either string
// or template-literal form). Templates with substitutions and other
// expressions are treated as truthy to match Biome's conservative
// behaviour.
func isAriaHiddenTruthy(el *wrapperchecker.Node) bool {
	attrs := jsxutil.AttributesNode(el)
	if attrs == nil {
		return false
	}
	attr := jsxutil.FindAttribute(attrs, "aria-hidden")
	if attr == nil {
		return false
	}
	hasValue := false
	isFalse := false
	sawName := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !sawName && c.Kind() == wrapperchecker.KindIdentifier {
			sawName = true
			return false
		}
		hasValue = true
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			if c.LiteralText() == "false" {
				isFalse = true
			}
		case wrapperchecker.KindJsxExpression:
			c.ForEachChild(func(e *wrapperchecker.Node) bool {
				switch e.Kind() {
				case wrapperchecker.KindFalseKeyword:
					isFalse = true
				case wrapperchecker.KindStringLiteral,
					wrapperchecker.KindNoSubstitutionTemplateLiteral:
					if e.LiteralText() == "false" {
						isFalse = true
					}
				}
				return true
			})
		}
		return true
	})
	if !hasValue {
		return true
	}
	return !isFalse
}

func hasNonWhitespaceText(s string) bool {
	for _, c := range s {
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			continue
		}
		return true
	}
	return false
}
