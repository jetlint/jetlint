// Package novuedataobjectdeclaration implements
// no-vue-data-object-declaration: Vue's `data` option must be a
// function so each component instance gets its own state — an
// object literal makes all instances share one mutable object,
// which silently corrupts state across renders. The rule fires
// when a Vue component descriptor (`export default { ... }`,
// `createApp({ ... })`, or `new Vue({ ... })`) declares `data`
// as anything other than a function/arrow/shorthand method.
package novuedataobjectdeclaration

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-vue-data-object-declaration"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindObjectLiteralExpression: visit,
	}
}

func visit(ctx *engine.Context, obj *wrapperchecker.Node) {
	if !isVueComponentDescriptor(obj) {
		return
	}
	prop := findDataProperty(obj)
	if prop == nil {
		return
	}
	if dataPropertyIsFunction(prop) {
		return
	}
	ctx.Report(prop, "`data` must be a function so each Vue instance gets its own state")
}

// isVueComponentDescriptor checks if the object literal is the
// payload Vue would consume as a component definition: the top of
// an `export default ...`, the first argument to a `createApp` /
// `Vue.createApp` call, or the first argument to `new Vue(...)`.
func isVueComponentDescriptor(obj *wrapperchecker.Node) bool {
	p := unwrapParens(obj.Parent())
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindExportAssignment:
		return true
	case wrapperchecker.KindCallExpression:
		return isVueFactoryCall(p, obj)
	case wrapperchecker.KindNewExpression:
		return isNewVueCall(p, obj)
	}
	return false
}

func unwrapParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.Parent()
	}
	return n
}

func isVueFactoryCall(call, arg *wrapperchecker.Node) bool {
	if !nodeIsFirstArgOf(call, arg) {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return callee.LiteralText() == "createApp"
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == "createApp"
	}
	return false
}

func isNewVueCall(newExpr, arg *wrapperchecker.Node) bool {
	if !nodeIsFirstArgOf(newExpr, arg) {
		return false
	}
	callee := newExpr.CalleeExpression()
	if callee == nil {
		return false
	}
	if callee.Kind() == wrapperchecker.KindIdentifier && callee.LiteralText() == "Vue" {
		return true
	}
	return false
}

// nodeIsFirstArgOf reports whether arg is the first argument of the
// given call/new expression (skipping any wrapping parentheses).
func nodeIsFirstArgOf(call, arg *wrapperchecker.Node) bool {
	var first *wrapperchecker.Node
	seenCallee := false
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		if first == nil && c.Kind() != wrapperchecker.KindTypeReference {
			first = c
			return true
		}
		return false
	})
	if first == nil {
		return false
	}
	if first.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		first.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner != nil {
			first = inner
		}
	}
	return first.Pos() == arg.Pos() && first.End() == arg.End()
}

func findDataProperty(obj *wrapperchecker.Node) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	obj.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindShorthandPropertyAssignment:
			if propertyNameIs(c, "data") {
				found = c
				return true
			}
		}
		return false
	})
	return found
}

func propertyNameIs(prop *wrapperchecker.Node, name string) bool {
	var match bool
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			if c.LiteralText() == name {
				match = true
			}
			return true
		}
		return false
	})
	return match
}

func dataPropertyIsFunction(prop *wrapperchecker.Node) bool {
	if prop.Kind() == wrapperchecker.KindMethodDeclaration {
		return true
	}
	var value *wrapperchecker.Node
	seenName := false
	prop.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName {
			if c.Kind() == wrapperchecker.KindIdentifier {
				seenName = true
			}
			return false
		}
		value = c
		return true
	})
	if value == nil {
		return false
	}
	for value.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		value.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			return false
		}
		value = inner
	}
	switch value.Kind() {
	case wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction:
		return true
	}
	return false
}
