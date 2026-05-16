// Package norenderreturnvalue implements no-render-return-value:
// `ReactDOM.render(...)` returns a component instance, but using
// that return value couples the caller to a deprecated React 16
// behavior — modern React (18+) returns `undefined` and recommends
// the `ref` callback. Capturing the value into a variable, an
// arrow body, or an expression context is a port-blocker.
//
// Recognized callees:
//   - bare `ReactDOM.render(...)` (assumes the global), unless
//     `ReactDOM` is shadowed by a local binding;
//   - `DefaultImport.render(...)` where `DefaultImport` is the
//     default import from `"react-dom"`;
//   - the named `render` import from `"react-dom"` (including
//     `import { render as alias }`).
//
// "Captured" means the call is used in a context that consumes its
// value: assignment RHS, variable initializer, return statement,
// concise arrow body, ternary, logical operator operand, object
// property value, etc. A bare `render(...)` statement is fine.
package norenderreturnvalue

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-render-return-value"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	defaultName, namedRender := collectReactDomImports(src)
	reactDOMShadowed := nameIsBoundInFile(src, "ReactDOM")
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n.Kind() == wrapperchecker.KindCallExpression {
			if isReactDomRender(n.CalleeExpression(), defaultName, namedRender, reactDOMShadowed) &&
				callIsCaptured(n) {
				ctx.Report(n, "don't capture the return value of ReactDOM.render — modern React no longer returns one")
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

// collectReactDomImports scans top-level imports for `"react-dom"`
// and returns the default-import local name (if any) and the local
// name bound to the named `render` export (if any).
func collectReactDomImports(src *wrapperchecker.Node) (defaultName, namedRender string) {
	src.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindImportDeclaration {
			return false
		}
		spec := stmt.ModuleSpecifier()
		if spec == nil || spec.LiteralText() != "react-dom" {
			return false
		}
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindImportClause {
				return false
			}
			c.ForEachChild(func(cc *wrapperchecker.Node) bool {
				switch cc.Kind() {
				case wrapperchecker.KindIdentifier:
					defaultName = cc.LiteralText()
				default:
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
						if original == "render" {
							namedRender = local
						}
						return false
					})
				}
				return false
			})
			return false
		})
		return false
	})
	return defaultName, namedRender
}

func isReactDomRender(callee *wrapperchecker.Node, defaultName, namedRender string, reactDOMShadowed bool) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		// Bare identifier callee: only the named-import alias
		// counts; an unimported `render` could be anything.
		return namedRender != "" && callee.LiteralText() == namedRender
	case wrapperchecker.KindPropertyAccessExpression:
		if callee.PropertyAccessName() != "render" {
			return false
		}
		recv := callee.PropertyAccessReceiver()
		if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		name := recv.LiteralText()
		if defaultName != "" && name == defaultName {
			return true
		}
		if name == "ReactDOM" && !reactDOMShadowed {
			return true
		}
	}
	return false
}

// callIsCaptured reports whether the call's value flows to a
// consumer. Unwraps ParenthesizedExpression to see the real
// surrounding node. A direct ExpressionStatement parent is the
// only "discarded" form.
func callIsCaptured(call *wrapperchecker.Node) bool {
	p := call.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return false
	}
	return p.Kind() != wrapperchecker.KindExpressionStatement
}

// nameIsBoundInFile reports whether any binding named `name`
// appears in the source file, excluding bindings whose declaring
// import is from `"react-dom"` (those re-export the global and
// shouldn't count as a shadowing rename).
func nameIsBoundInFile(src *wrapperchecker.Node, name string) bool {
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if found {
			return true
		}
		switch c.Kind() {
		case wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindParameter,
			wrapperchecker.KindBindingElement,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration:
			if bindingName(c) == name {
				found = true
				return true
			}
		}
		c.ForEachChild(walk)
		return found
	}
	src.ForEachChild(walk)
	return found
}

func bindingName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
