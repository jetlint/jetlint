// Package usejsxkeyiniterable implements use-jsx-key-in-iterable:
// React needs a stable `key` on each element of a list so it can
// reconcile re-renders correctly. JSX elements (or
// `React.createElement(...)` calls) that appear inside an array
// literal, a `.map` / `.forEach` / `.flatMap` callback,
// `Array.from(..., cb)`, or `React.Children.map(..., cb)` are
// flagged when they don't carry a `key` prop.
//
// `React.cloneElement` and any other element factory called in
// those contexts is treated like `createElement` — biome flags
// `React.cloneElement(c)` inside `React.Children.map(...)` without
// `key`, so we mirror that.
package usejsxkeyiniterable

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-jsx-key-in-iterable"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visitJsxOpening,
		wrapperchecker.KindJsxSelfClosingElement: visitJsxSelfClosing,
		wrapperchecker.KindCallExpression:        visitCall,
	}
}

func visitJsxOpening(ctx *engine.Context, n *wrapperchecker.Node) {
	checkJsx(ctx, n, n)
}

func visitJsxSelfClosing(ctx *engine.Context, n *wrapperchecker.Node) {
	checkJsx(ctx, n, n)
}

func checkJsx(ctx *engine.Context, anchor, elem *wrapperchecker.Node) {
	if !inIterableContext(elem) {
		return
	}
	if hasKeyAttribute(elem) {
		return
	}
	ctx.Report(anchor, "JSX element in an iterable needs a `key` prop for React to track re-renders")
}

// visitCall handles `React.createElement(...)` and
// `React.cloneElement(...)` (and the unqualified forms) the same
// way: when used inside an iterable, the props object must include
// a `key` entry.
func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isReactElementFactoryCallee(n.CalleeExpression()) {
		return
	}
	if !inIterableContext(n) {
		return
	}
	args := n.CallArguments()
	if len(args) >= 2 && objectHasKeyProp(args[1]) {
		return
	}
	ctx.Report(n, "createElement/cloneElement in an iterable needs a `key` in its props")
}

// inIterableContext walks ancestors and reports whether n appears
// either as an array-literal element or inside a callback to one of
// the iteration helpers React requires keys for.
func inIterableContext(n *wrapperchecker.Node) bool {
	cur := n
	for {
		p := cur.Parent()
		if p == nil {
			return false
		}
		switch p.Kind() {
		case wrapperchecker.KindArrowFunction,
			wrapperchecker.KindFunctionExpression:
			// Walk up further: if this function is itself the
			// argument to an iteration helper, we're in an
			// iterable context.
			pp := p.Parent()
			for pp != nil && pp.Kind() == wrapperchecker.KindParenthesizedExpression {
				pp = pp.Parent()
			}
			if pp == nil {
				return false
			}
			if pp.Kind() == wrapperchecker.KindCallExpression &&
				isIterationCallee(pp.CalleeExpression()) {
				return true
			}
			cur = pp
			continue
		case wrapperchecker.KindReturnStatement,
			wrapperchecker.KindBlock,
			wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindConditionalExpression,
			wrapperchecker.KindBinaryExpression,
			wrapperchecker.KindSpreadElement,
			wrapperchecker.KindCallExpression,
			wrapperchecker.KindExpressionStatement,
			wrapperchecker.KindArrayLiteralExpression,
			wrapperchecker.KindPropertyAccessExpression,
			wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindIfStatement,
			wrapperchecker.KindSwitchStatement,
			wrapperchecker.KindCaseClause,
			wrapperchecker.KindDefaultClause:
			cur = p
			continue
		}
		return false
	}
}

// isIterationCallee reports whether the callee names one of the
// JSX-producing iteration helpers React tracks (`.map`, `.forEach`,
// `.flatMap`, `Array.from`, `Children.map`).
func isIterationCallee(callee *wrapperchecker.Node) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		// `map(...)` bare — too generic; biome doesn't flag.
		return false
	case wrapperchecker.KindPropertyAccessExpression:
		name := callee.PropertyAccessName()
		return name == "map" || name == "forEach" || name == "flatMap" ||
			name == "from"
	}
	return false
}

func hasKeyAttribute(elem *wrapperchecker.Node) bool {
	found := false
	elem.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttributes {
			return false
		}
		c.ForEachChild(func(attr *wrapperchecker.Node) bool {
			if attr.Kind() != wrapperchecker.KindJsxAttribute {
				return false
			}
			var name string
			attr.ForEachChild(func(n *wrapperchecker.Node) bool {
				if n.Kind() == wrapperchecker.KindIdentifier {
					name = n.LiteralText()
					return true
				}
				return false
			})
			if name == "key" {
				found = true
				return true
			}
			return false
		})
		return true
	})
	return found
}

func isReactElementFactoryCallee(callee *wrapperchecker.Node) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		name := callee.LiteralText()
		return name == "createElement" || name == "cloneElement"
	case wrapperchecker.KindPropertyAccessExpression:
		name := callee.PropertyAccessName()
		return name == "createElement" || name == "cloneElement"
	}
	return false
}

func objectHasKeyProp(arg *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyAssignment:
			if c.PropertyName() == "key" {
				found = true
				return true
			}
		case wrapperchecker.KindShorthandPropertyAssignment:
			var name string
			c.ForEachChild(func(n *wrapperchecker.Node) bool {
				if n.Kind() == wrapperchecker.KindIdentifier {
					name = n.LiteralText()
					return true
				}
				return false
			})
			if name == "key" {
				found = true
				return true
			}
		}
		return false
	})
	return found
}
