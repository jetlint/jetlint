// Package noariahiddenonfocusable implements
// no-aria-hidden-on-focusable: an element with `aria-hidden="true"`
// is removed from the accessibility tree but stays in the focus
// order — keyboard users tab to an "invisible" control. Flag the
// combination.
package noariahiddenonfocusable

import (
	"strconv"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-aria-hidden-on-focusable"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// focusableTags lists DOM elements that are focusable by default.
// Any element gains focus when carrying a non-negative tabIndex.
var focusableTags = map[string]bool{
	"a": true, "area": true, "button": true, "input": true,
	"select": true, "textarea": true, "summary": true,
	"iframe": true, "audio": true, "video": true, "details": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if tag == "" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	hidden := jsxutil.FindAttribute(attrs, "aria-hidden")
	if hidden == nil {
		return
	}
	v, ok := jsxutil.AttributeStringValue(hidden)
	if !ok || v != "true" {
		return
	}
	ti := jsxutil.FindAttribute(attrs, "tabIndex")
	tabNegative, tabNonNegative := false, false
	if ti != nil {
		if n := tabIndexInt(ti); n != nil {
			if *n < 0 {
				tabNegative = true
			} else {
				tabNonNegative = true
			}
		}
	}
	if focusableTags[tag] && !tabNegative {
		ctx.Report(el, "aria-hidden on a focusable element traps keyboard focus on a hidden control")
		return
	}
	if tabNonNegative {
		ctx.Report(el, "aria-hidden on a focusable element traps keyboard focus on a hidden control")
	}
}

func tabIndexInt(attr *wrapperchecker.Node) *int {
	if s, ok := jsxutil.AttributeStringValue(attr); ok {
		if n, err := strconv.Atoi(s); err == nil {
			return &n
		}
	}
	seenName := false
	var out *int
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName && c.Kind() == wrapperchecker.KindIdentifier {
			seenName = true
			return false
		}
		if c.Kind() != wrapperchecker.KindJsxExpression {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			switch e.Kind() {
			case wrapperchecker.KindNumericLiteral:
				if n, err := strconv.Atoi(e.LiteralText()); err == nil {
					out = &n
				}
			case wrapperchecker.KindPrefixUnaryExpression:
				if e.PrefixUnaryOperator() == "-" {
					if op := e.PrefixUnaryOperand(); op != nil && op.Kind() == wrapperchecker.KindNumericLiteral {
						if n, err := strconv.Atoi(op.LiteralText()); err == nil {
							neg := -n
							out = &neg
						}
					}
				}
			}
			return true
		})
		return true
	})
	return out
}
