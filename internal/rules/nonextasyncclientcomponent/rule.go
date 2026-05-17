// Package nonextasyncclientcomponent implements
// no-next-async-client-component: in Next.js, a file marked with
// "use client" runs in the browser and React doesn't support async
// client components — turning an async function into one breaks
// hydration. The rule fires only when the source file opens with
// the `"use client"` directive and a component-cased function
// (PascalCase) is declared as async.
package nonextasyncclientcomponent

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-next-async-client-component"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visitFunction,
		wrapperchecker.KindFunctionExpression:  visitFunction,
		wrapperchecker.KindArrowFunction:       visitFunction,
		wrapperchecker.KindMethodDeclaration:   visitFunction,
	}
}

func visitFunction(ctx *engine.Context, fn *wrapperchecker.Node) {
	if !isAsync(fn) {
		return
	}
	if !fileIsUseClient(fn) {
		return
	}
	if !isComponent(fn) {
		return
	}
	ctx.Report(fn, "async components are not supported in client components; remove `async` or convert this file to a server component")
}

func isAsync(fn *wrapperchecker.Node) bool {
	var found bool
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindAsyncKeyword {
			found = true
			return true
		}
		return false
	})
	return found
}

// fileIsUseClient walks up to the source file and checks whether
// the first statement is a string literal expression with the
// `use client` directive.
func fileIsUseClient(n *wrapperchecker.Node) bool {
	root := n
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return false
	}
	var firstHit bool
	var useClient bool
	root.ForEachChild(func(c *wrapperchecker.Node) bool {
		if firstHit {
			return true
		}
		firstHit = true
		if c.Kind() != wrapperchecker.KindExpressionStatement {
			return true
		}
		c.ForEachChild(func(inner *wrapperchecker.Node) bool {
			if inner.Kind() == wrapperchecker.KindStringLiteral && inner.LiteralText() == "use client" {
				useClient = true
				return true
			}
			return false
		})
		return true
	})
	return useClient
}

// isComponent reports whether the function should be treated as a
// React component for the purposes of this rule: it has a name
// (declaration, named expression, assigned variable, method, or
// object property) that starts with an uppercase letter.
func isComponent(fn *wrapperchecker.Node) bool {
	name := functionLikeName(fn)
	if name == "" {
		return false
	}
	c := name[0]
	return c >= 'A' && c <= 'Z'
}

func functionLikeName(fn *wrapperchecker.Node) string {
	switch fn.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindMethodDeclaration:
		var n string
		fn.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				n = c.LiteralText()
				return true
			}
			return false
		})
		if n != "" {
			return n
		}
	}
	parent := fn.Parent()
	for parent != nil && parent.Kind() == wrapperchecker.KindParenthesizedExpression {
		parent = parent.Parent()
	}
	if parent == nil {
		return ""
	}
	switch parent.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindPropertyAssignment:
		var n string
		parent.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				n = c.LiteralText()
				return true
			}
			return false
		})
		return n
	case wrapperchecker.KindBinaryExpression:
		var n string
		parent.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && n == "" {
				n = c.LiteralText()
				return true
			}
			return false
		})
		return n
	}
	return ""
}
