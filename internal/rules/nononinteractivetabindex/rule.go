// Package nononinteractivetabindex implements
// no-noninteractive-tabindex: a positive (or zero) tabIndex on a
// non-interactive element puts a non-actionable focus stop in the
// page, confusing screen readers and keyboard users. Negative
// tabIndex (programmatic-only focus) is fine, as is any tabIndex
// on an inherently interactive tag or an element carrying an
// interactive ARIA role.
package nononinteractivetabindex

import (
	"strconv"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-noninteractive-tabindex"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

var interactiveTags = map[string]bool{
	"a": true, "area": true, "button": true, "input": true, "option": true,
	"select": true, "textarea": true, "summary": true, "details": true,
}

var interactiveRoles = map[string]bool{
	"button": true, "checkbox": true, "combobox": true, "link": true, "menuitem": true,
	"menuitemcheckbox": true, "menuitemradio": true, "option": true, "radio": true,
	"searchbox": true, "slider": true, "spinbutton": true, "switch": true, "tab": true,
	"textbox": true, "treeitem": true, "scrollbar": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !jsxutil.IsHTMLElement(tag) || interactiveTags[tag] {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	ti := jsxutil.FindAttribute(attrs, "tabIndex")
	if ti == nil {
		return
	}
	if r := jsxutil.FindAttribute(attrs, "role"); r != nil {
		if v, ok := jsxutil.AttributeStringValue(r); ok && interactiveRoles[v] {
			return
		}
	}
	if n := tabIndexInt(ti); n != nil && *n < 0 {
		return
	}
	ctx.Report(ti, "tabIndex on a non-interactive element puts a non-actionable focus stop in the page")
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
