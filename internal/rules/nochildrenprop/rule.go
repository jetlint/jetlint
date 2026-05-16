// Package nochildrenprop implements no-children-prop: in React, the
// `children` prop is meant to be the element's children, not a
// regular attribute. Passing `children` through the props object
// (or as a JSX attribute) bypasses React's element-tree assembly
// and produces surprising hot-reload, key, and reconciliation
// behavior.
//
// Both shapes are flagged:
//   - JSX `<Component children={...} />`;
//   - `createElement(type, { children: ... })`, including
//     `React.createElement(...)` and any import like
//     `import { createElement as alias } from "react"` whose
//     alias is used as the callee.
//
// `cloneElement` legitimately accepts `children` in its props
// object, so it stays unflagged. Library names other than `react`
// aren't tracked — this rule only knows React's contract.
package nochildrenprop

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-children-prop"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	aliases := collectCreateElementAliases(src)
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		switch n.Kind() {
		case wrapperchecker.KindJsxAttribute:
			if jsxAttributeName(n) == "children" {
				ctx.Report(n, "don't pass `children` as a prop — nest it inside the element")
			}
		case wrapperchecker.KindCallExpression:
			if isCreateElementCallee(n.CalleeExpression(), aliases) {
				args := n.CallArguments()
				if len(args) >= 2 && hasChildrenProperty(args[1]) {
					ctx.Report(n, "don't pass `children` in the createElement props object")
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c)
		return false
	})
}

// collectCreateElementAliases scans top-level imports and returns
// the set of local names that refer to React's createElement. The
// default member `createElement` always counts; aliased imports
// (`import { createElement as h }`) add their local name.
func collectCreateElementAliases(src *wrapperchecker.Node) map[string]bool {
	aliases := map[string]bool{"createElement": true}
	src.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindImportDeclaration {
			return false
		}
		spec := stmt.ModuleSpecifier()
		if spec == nil || spec.LiteralText() != "react" {
			return false
		}
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindImportClause {
				return false
			}
			c.ForEachChild(func(cc *wrapperchecker.Node) bool {
				cc.ForEachChild(func(spec *wrapperchecker.Node) bool {
					if spec.Kind() != wrapperchecker.KindImportSpecifier {
						return false
					}
					// `propertyName as localName`: collect when
					// propertyName is `createElement`. If there's
					// no `as` clause the local name IS the
					// property name, which also collects.
					var ids []*wrapperchecker.Node
					spec.ForEachChild(func(n *wrapperchecker.Node) bool {
						if n.Kind() == wrapperchecker.KindIdentifier {
							ids = append(ids, n)
						}
						return false
					})
					if len(ids) == 0 {
						return false
					}
					original := ids[0].LiteralText()
					local := ids[len(ids)-1].LiteralText()
					if original == "createElement" {
						aliases[local] = true
					}
					return false
				})
				return false
			})
			return false
		})
		return false
	})
	return aliases
}

// jsxAttributeName returns the attribute name (the `foo` in
// `<X foo="bar" />`), or "" for namespaced / computed attributes
// where the name isn't a simple identifier.
func jsxAttributeName(attr *wrapperchecker.Node) string {
	var name string
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// isCreateElementCallee reports whether the callee names a known
// createElement (or a property access whose property is
// `createElement`, e.g. `React.createElement`).
func isCreateElementCallee(callee *wrapperchecker.Node, aliases map[string]bool) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return aliases[callee.LiteralText()]
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == "createElement"
	}
	return false
}

// hasChildrenProperty reports whether the second argument to
// createElement is an object literal containing a property with
// the key `children`.
func hasChildrenProperty(arg *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindPropertyAssignment &&
			c.PropertyName() == "children" {
			found = true
			return true
		}
		// Shorthand: `{ children }` shows up as a different node
		// kind in the wrapper; cover the case by checking its
		// first identifier child too.
		var name string
		c.ForEachChild(func(n *wrapperchecker.Node) bool {
			if n.Kind() == wrapperchecker.KindIdentifier {
				name = n.LiteralText()
				return true
			}
			return false
		})
		if name == "children" {
			found = true
			return true
		}
		return false
	})
	return found
}
