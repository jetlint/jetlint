// Package useuniqueelementids implements use-unique-element-ids:
// hardcoded `id="foo"` attributes on JSX elements (or in
// `createElement` props from React) collide whenever the component
// is rendered more than once. React provides `useId()` for exactly
// this case; the rule pushes authors toward it.
//
// JSX side is straightforward — any `<X id="literal">` shape is
// flagged. The createElement side requires the call's callee to
// resolve to React's createElement: the bare global, an alias
// imported from `"react"`, or a property access whose name is
// `createElement` (covers `React.createElement`).
//
// When a `createElement` import comes from a non-React module
// (`import { createElement } from "not-react"`), the rule stays
// quiet — that's a different element factory.
package useuniqueelementids

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-unique-element-ids"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	reactCreate := collectReactCreateElementAliases(src)
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		switch n.Kind() {
		case wrapperchecker.KindJsxOpeningElement,
			wrapperchecker.KindJsxSelfClosingElement:
			if attr := jsxAttribute(n, "id"); attr != nil {
				if v := jsxAttributeStringValue(attr); v != "" {
					ctx.Report(attr, "hardcoded id collides on re-render — use useId() or a prop")
				}
			}
		case wrapperchecker.KindCallExpression:
			if isReactCreateElement(n.CalleeExpression(), reactCreate) {
				args := n.CallArguments()
				if len(args) >= 2 && objectHasStringIdProp(args[1]) {
					ctx.Report(n, "hardcoded id collides on re-render — use useId() or a prop")
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

// collectReactCreateElementAliases returns the local names that
// resolve to React's createElement. `React.createElement` (via
// property access) is always recognized at the call site; this
// table covers the bare `createElement` symbol and any alias
// from `import { createElement as X } from "react"`. When no
// React import exists, the bare name is NOT included — biome's
// fixture relies on that to keep `createElement` from a
// non-React module unflagged.
func collectReactCreateElementAliases(src *wrapperchecker.Node) map[string]bool {
	aliases := map[string]bool{}
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

func isReactCreateElement(callee *wrapperchecker.Node, aliases map[string]bool) bool {
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

// jsxAttribute returns the named attribute node on a JSX element,
// or nil if absent.
func jsxAttribute(elem *wrapperchecker.Node, name string) *wrapperchecker.Node {
	var got *wrapperchecker.Node
	elem.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttributes {
			return false
		}
		c.ForEachChild(func(attr *wrapperchecker.Node) bool {
			if attr.Kind() != wrapperchecker.KindJsxAttribute {
				return false
			}
			var aname string
			attr.ForEachChild(func(n *wrapperchecker.Node) bool {
				if n.Kind() == wrapperchecker.KindIdentifier {
					aname = n.LiteralText()
					return true
				}
				return false
			})
			if aname == name {
				got = attr
				return true
			}
			return false
		})
		return true
	})
	return got
}

// jsxAttributeStringValue returns the literal string value of a
// JSX attribute (either `id="foo"` or `id={"foo"}`), or "" when
// the value is dynamic, missing, or a different shape.
func jsxAttributeStringValue(attr *wrapperchecker.Node) string {
	var value string
	count := 0
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		count++
		// Skip the name (first child); inspect subsequent
		// children for the value.
		if count == 1 {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral:
			value = c.LiteralText()
		case wrapperchecker.KindJsxExpression:
			c.ForEachChild(func(e *wrapperchecker.Node) bool {
				if e.Kind() == wrapperchecker.KindStringLiteral ||
					e.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
					value = e.LiteralText()
					return true
				}
				return false
			})
		}
		return true
	})
	return value
}

func objectHasStringIdProp(arg *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindPropertyAssignment {
			return false
		}
		if c.PropertyName() != "id" {
			return false
		}
		init := c.PropertyInitializer()
		if init == nil {
			return false
		}
		if init.Kind() == wrapperchecker.KindStringLiteral ||
			init.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
			found = true
			return true
		}
		return false
	})
	return found
}
