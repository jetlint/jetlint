// Package usearrowfunction implements use-arrow-function: a
// `function () {}` callback that doesn't use `this`, `arguments`, or
// `new.target` reads more directly as an arrow.
package usearrowfunction

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-arrow-function"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip generators.
	if isGenerator(n) {
		return
	}
	// Skip named function expressions — biome considers them intentional.
	if hasName(n) {
		return
	}
	// Skip in contexts where arrow can't substitute (extends clause, etc.).
	if !isReplaceableContext(n) {
		return
	}
	if usesDynamicThis(n) || hasThisParameter(n) {
		return
	}
	if referencesArguments(n) && !shadowsArguments(n) {
		return
	}
	if usesNewTarget(n) {
		return
	}
	// Skip `export default function() {}` — that's idiomatic for module exports.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindExportAssignment {
		return
	}
	ctx.Report(n, "convert this function expression to an arrow function")
}

func isGenerator(n *wrapperchecker.Node) bool {
	// Source text starts with "function*" (whitespace possible).
	src := n.SourceText()
	for i := 0; i < len(src); i++ {
		if src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == 'a' || src[i] == 's' || src[i] == 'y' || src[i] == 'n' || src[i] == 'c' {
			continue
		}
		if src[i] == 'f' && i+8 <= len(src) && src[i:i+8] == "function" {
			j := i + 8
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			return j < len(src) && src[j] == '*'
		}
		return false
	}
	return false
}

func hasName(n *wrapperchecker.Node) bool {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first != nil && first.Kind() == wrapperchecker.KindIdentifier
}

func isReplaceableContext(n *wrapperchecker.Node) bool {
	p := n.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return false
	}
	// Common safe contexts: variable initializer, assignment, call argument, return.
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration, wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindReturnStatement, wrapperchecker.KindCallExpression,
		wrapperchecker.KindNewExpression, wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindArrowFunction:
		return true
	}
	return false
}

func hasThisParameter(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			var name *wrapperchecker.Node
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if name == nil {
					name = d
				}
				return false
			})
			if name != nil && (name.Kind() == wrapperchecker.KindThisKeyword || (name.Kind() == wrapperchecker.KindIdentifier && name.SourceText() == "this")) {
				found = true
			}
		}
		return false
	})
	return found
}

func usesDynamicThis(n *wrapperchecker.Node) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindThisKeyword:
			found = true
			return
		case wrapperchecker.KindFunctionExpression, wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration, wrapperchecker.KindConstructor,
			wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor,
			wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			// Don't descend.
			return
		}
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			walk(d)
			return false
		})
	}
	// Only check the body, not the parameters/return-type.
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	if body != nil {
		walk(body)
	}
	return found
}

func shadowsArguments(n *wrapperchecker.Node) bool {
	// True if the function body locally declares a variable called `arguments`.
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	if body == nil {
		return false
	}
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindFunctionExpression, wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration, wrapperchecker.KindConstructor,
			wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor,
			wrapperchecker.KindArrowFunction:
			return
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindVariableDeclarationList {
					d.ForEachChild(func(decl *wrapperchecker.Node) bool {
						var first *wrapperchecker.Node
						decl.ForEachChild(func(x *wrapperchecker.Node) bool {
							if first == nil {
								first = x
							}
							return false
						})
						if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == "arguments" {
							found = true
						}
						return false
					})
				}
				return false
			})
		}
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			walk(d)
			return false
		})
	}
	walk(body)
	return found
}

func usesNewTarget(n *wrapperchecker.Node) bool {
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	if body == nil {
		return false
	}
	src := body.SourceText()
	for i := 0; i+10 <= len(src); i++ {
		if src[i] == 'n' && src[i:i+10] == "new.target" {
			return true
		}
	}
	return false
}

func referencesArguments(n *wrapperchecker.Node) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			if c.SourceText() == "arguments" {
				found = true
			}
			return
		case wrapperchecker.KindFunctionExpression, wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration, wrapperchecker.KindConstructor,
			wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor:
			return
		}
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			walk(d)
			return false
		})
	}
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	if body != nil {
		walk(body)
	}
	return found
}
