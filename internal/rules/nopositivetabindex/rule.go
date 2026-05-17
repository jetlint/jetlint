// Package nopositivetabindex implements no-positive-tabindex: flag
// tabIndex > 0. Positive tabindex overrides the document's natural
// focus order, which makes pages unpredictable for keyboard and
// screen-reader users. `0` (focusable in order) and `-1`
// (programmatically focusable) are fine.
package nopositivetabindex

import (
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-positive-tabindex"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
		wrapperchecker.KindCallExpression:        visitCreate,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	attr := jsxutil.FindAttribute(jsxutil.AttributesNode(el), "tabIndex")
	if attr == nil {
		return
	}
	v, ok := tabIndexValue(attr)
	if ok && v > 0 {
		ctx.Report(attr, "positive tabindex disrupts the document's natural focus order")
	}
}

func tabIndexValue(attr *wrapperchecker.Node) (int, bool) {
	// Walk the attribute's value child directly. We deliberately
	// skip template literals because biome treats them as
	// potentially dynamic (the lone-literal case can be unfolded
	// at build time, but the rule doesn't try).
	seenName := false
	var out int
	var has bool
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName && c.Kind() == wrapperchecker.KindIdentifier {
			seenName = true
			return false
		}
		// Quoted string attribute value: `tabIndex="2"`.
		if c.Kind() == wrapperchecker.KindStringLiteral {
			if n, ok := parseInt(c.LiteralText()); ok {
				out = n
				has = true
			}
			return true
		}
		if c.Kind() != wrapperchecker.KindJsxExpression {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			switch e.Kind() {
			case wrapperchecker.KindNumericLiteral:
				if n, ok := parseInt(e.LiteralText()); ok {
					out = n
					has = true
				}
			case wrapperchecker.KindPrefixUnaryExpression:
				// negative numbers come in as -<literal>
				if e.PrefixUnaryOperator() == "-" {
					op := e.PrefixUnaryOperand()
					if op != nil && op.Kind() == wrapperchecker.KindNumericLiteral {
						if n, ok := parseInt(op.LiteralText()); ok {
							out = -n
							has = true
						}
					}
				}
			case wrapperchecker.KindStringLiteral:
				if n, ok := parseInt(e.LiteralText()); ok {
					out = n
					has = true
				}
			// Template literals are dynamic — skip per biome.
			}
			return true
		})
		return true
	})
	return out, has
}

func parseInt(s string) (int, bool) {
	s = strings.Trim(strings.TrimSpace(s), `"'`)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// visitCreate handles `React.createElement("div", { tabIndex: 1 })`
// — biome's fixture exercises this form too.
func visitCreate(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "createElement" {
		return
	}
	args := call.CallArguments()
	if len(args) < 2 {
		return
	}
	props := args[1]
	if props.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return
	}
	props.ForEachChild(func(p *wrapperchecker.Node) bool {
		if p.Kind() != wrapperchecker.KindPropertyAssignment {
			return false
		}
		if p.PropertyName() != "tabIndex" {
			return false
		}
		init := p.PropertyInitializer()
		if init == nil {
			return false
		}
		if n, ok := objectLitNumeric(init); ok && n > 0 {
			ctx.Report(p, "positive tabindex disrupts the document's natural focus order")
		}
		return false
	})
}

func objectLitNumeric(e *wrapperchecker.Node) (int, bool) {
	switch e.Kind() {
	case wrapperchecker.KindNumericLiteral:
		return parseInt(e.LiteralText())
	case wrapperchecker.KindStringLiteral:
		return parseInt(e.LiteralText())
	// Template literals are skipped — dynamic per biome.
	case wrapperchecker.KindPrefixUnaryExpression:
		if e.PrefixUnaryOperator() == "-" {
			op := e.PrefixUnaryOperand()
			if op != nil && op.Kind() == wrapperchecker.KindNumericLiteral {
				if n, ok := parseInt(op.LiteralText()); ok {
					return -n, true
				}
			}
		}
	}
	return 0, false
}
