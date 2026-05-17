// Package usesolidforcomponent implements use-solid-for-component:
// `data.map(d => <li>...)` inside JSX recreates every child on
// every render. Solid's `<For each>` keeps stable identities and
// is the idiomatic way to render a list.
//
// The rule fires when a `.map(...)` callback returns JSX and the
// call expression itself is inside a JSX expression container.
package usesolidforcomponent

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-solid-for-component"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil {
		return
	}
	if !isMapCall(callee) {
		return
	}
	if !insideJsxExpression(call) {
		return
	}
	args := call.CallArguments()
	if len(args) == 0 {
		return
	}
	cb := args[0]
	if !callbackReturnsJsx(cb) {
		return
	}
	ctx.Report(call, "use Solid's `<For each>` instead of `.map(...)` for rendering lists in JSX")
}

func isMapCall(callee *wrapperchecker.Node) bool {
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression {
		return callee.PropertyAccessName() == "map"
	}
	// `data?.map(...)` — optional chain shows up as the same access
	// kind in tsgo's wrapper, so the property name check above
	// already covers it.
	return false
}

// insideJsxExpression returns true if the call's nearest non-paren
// ancestor is a JSX expression container.
func insideJsxExpression(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindJsxExpression:
			return true
		case wrapperchecker.KindParenthesizedExpression:
			continue
		default:
			return false
		}
	}
	return false
}

// callbackReturnsJsx reports whether the function/arrow returns a
// JSX element. Handles both `x => <jsx/>` (concise body) and
// `x => { return <jsx/>; }` shapes.
func callbackReturnsJsx(cb *wrapperchecker.Node) bool {
	if cb.Kind() != wrapperchecker.KindArrowFunction &&
		cb.Kind() != wrapperchecker.KindFunctionExpression {
		return false
	}
	body := cb.FunctionBody()
	if body == nil {
		return false
	}
	if isJsxNode(body) {
		return true
	}
	if body.Kind() != wrapperchecker.KindBlock {
		return false
	}
	found := false
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindReturnStatement {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			if isJsxNode(e) {
				found = true
				return true
			}
			return false
		})
		return found
	})
	return found
}

func isJsxNode(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindJsxOpeningElement,
		wrapperchecker.KindJsxSelfClosingElement,
		wrapperchecker.KindJsxClosingElement,
		wrapperchecker.KindJsxExpression:
		return true
	}
	// JsxElement and JsxFragment kinds aren't exposed by the wrapper
	// — match them by source-text prefix as a fallback.
	t := n.SourceText()
	if len(t) > 0 && t[0] == '<' {
		return true
	}
	return false
}
