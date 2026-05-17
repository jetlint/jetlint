// Package nolabelwithoutcontrol implements no-label-without-control:
// a <label> must be tied to a form control AND carry visible text
// (or aria-label/aria-labelledby). Without both, screen-reader users
// hear an unlabeled input — or a label that points to nothing.
package nolabelwithoutcontrol

import (
	"strings"
	"unicode"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-label-without-control"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visitOpening,
		wrapperchecker.KindJsxSelfClosingElement: visitSelfClosing,
	}
}

var controlTags = map[string]bool{
	"input": true, "textarea": true, "select": true,
	"meter": true, "output": true, "progress": true, "button": true,
}

func visitSelfClosing(ctx *engine.Context, el *wrapperchecker.Node) {
	if jsxutil.TagName(el) != "label" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	if hasAriaLabel(attrs) && hasFor(attrs) {
		return
	}
	ctx.Report(el, "<label> needs visible text and a control association (for/htmlFor or nested control)")
}

func visitOpening(ctx *engine.Context, el *wrapperchecker.Node) {
	if jsxutil.TagName(el) != "label" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	parent := el.Parent()
	if parent == nil {
		return
	}
	forAttr := hasFor(attrs)
	ariaLabel := hasAriaLabel(attrs)
	text := hasTextDescendant(parent, el)
	control := hasControlDescendant(parent, el)
	hasText := text || ariaLabel
	hasControl := control || forAttr
	if hasText && hasControl {
		return
	}
	ctx.Report(el, "<label> needs visible text and a control association (for/htmlFor or nested control)")
}

func hasFor(attrs *wrapperchecker.Node) bool {
	return jsxutil.FindAttribute(attrs, "for") != nil || jsxutil.FindAttribute(attrs, "htmlFor") != nil
}

func hasAriaLabel(attrs *wrapperchecker.Node) bool {
	return jsxutil.FindAttribute(attrs, "aria-label") != nil ||
		jsxutil.FindAttribute(attrs, "aria-labelledby") != nil
}

// hasTextDescendant returns true if the label element (whose opening
// tag is `opening` and whose container is `parent`) has any non-empty
// text, expression, or descendant element carrying accessible text.
func hasTextDescendant(parent *wrapperchecker.Node, opening *wrapperchecker.Node) bool {
	if hasPlainJsxText(parent, opening) {
		return true
	}
	found := false
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c == opening {
			return false
		}
		if walkForText(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// hasPlainJsxText scans the parent JsxElement's source text for plain
// text content (outside child elements and expressions). JsxText
// nodes are not exposed as a constant by the wrapper, so we look at
// raw source for a letter that isn't inside `<...>` or `{...}`.
func hasPlainJsxText(parent *wrapperchecker.Node, opening *wrapperchecker.Node) bool {
	src := parent.SourceText()
	openSrc := opening.SourceText()
	_, inner, ok := strings.Cut(src, openSrc)
	if !ok {
		return false
	}
	// Trim trailing </label>.
	if end := strings.LastIndex(inner, "</"); end >= 0 {
		inner = inner[:end]
	}
	depth := 0
	braceDepth := 0
	for _, r := range inner {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		default:
			if depth == 0 && braceDepth == 0 && unicode.IsLetter(r) {
				return true
			}
		}
	}
	return false
}

func walkForText(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindJsxExpression:
		// Empty {} is just punctuation; an expression with any child is text.
		seen := false
		n.ForEachChild(func(_ *wrapperchecker.Node) bool { seen = true; return true })
		return seen
	case wrapperchecker.KindJsxOpeningElement, wrapperchecker.KindJsxSelfClosingElement:
		attrs := jsxutil.AttributesNode(n)
		if attrs != nil {
			if jsxutil.FindAttribute(attrs, "aria-label") != nil ||
				jsxutil.FindAttribute(attrs, "aria-labelledby") != nil {
				return true
			}
			if jsxutil.TagName(n) == "img" {
				if a := jsxutil.FindAttribute(attrs, "alt"); a != nil {
					if v, ok := jsxutil.AttributeStringValue(a); ok && v != "" {
						return true
					}
				}
			}
		}
		return false
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if walkForText(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

func hasControlDescendant(parent *wrapperchecker.Node, opening *wrapperchecker.Node) bool {
	found := false
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c == opening {
			return false
		}
		if walkForControl(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

func walkForControl(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if k := n.Kind(); k == wrapperchecker.KindJsxOpeningElement || k == wrapperchecker.KindJsxSelfClosingElement {
		if controlTags[jsxutil.TagName(n)] {
			return true
		}
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if walkForControl(c) {
			found = true
			return true
		}
		return false
	})
	return found
}
