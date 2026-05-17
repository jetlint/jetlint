// Package nodangerouslysetinnerhtml implements the lint rule that
// flags React's escape-hatch innerHTML prop. The prop bypasses
// React's XSS escaping; this rule is purely a static detector and
// performs no actual rendering.
//
// Detected shapes:
//   - JSX `<X PROP_NAME={...} />`,
//   - `React.createElement(...)` with the prop in the props object,
//   - `createElement(...)` called via a binding imported from "react".
package nodangerouslysetinnerhtml

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dangerously-set-inner-html"
const propName = "dangerouslySetInnerHTML"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	aliases := collectReactCreateElement(src)
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		switch n.Kind() {
		case wrapperchecker.KindJsxAttribute:
			if jsxAttributeName(n) == propName {
				ctx.Report(n, "this React prop bypasses XSS escaping")
			}
		case wrapperchecker.KindCallExpression:
			if isCreateElementCallee(n.CalleeExpression(), aliases) {
				args := n.CallArguments()
				if len(args) >= 2 && hasFlaggedProperty(args[1]) {
					ctx.Report(n, "createElement props with this key bypass React's XSS escaping")
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

func collectReactCreateElement(src *wrapperchecker.Node) map[string]bool {
	out := map[string]bool{}
	// reactShadowed: another module imported under the name
	// "React". That binding wins over the implicit React convention,
	// so we skip `React.createElement(...)` flags in that case.
	reactShadowed := false
	src.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindImportDeclaration {
			return false
		}
		spec := stmt.ModuleSpecifier()
		if spec == nil {
			return false
		}
		if spec.LiteralText() != "react" {
			// Check for a default import named React.
			stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() != wrapperchecker.KindImportClause {
					return false
				}
				c.ForEachChild(func(cc *wrapperchecker.Node) bool {
					if cc.Kind() == wrapperchecker.KindIdentifier && cc.LiteralText() == "React" {
						reactShadowed = true
						return true
					}
					return false
				})
				return false
			})
			return false
		}
		stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() != wrapperchecker.KindImportClause {
				return false
			}
			c.ForEachChild(func(cc *wrapperchecker.Node) bool {
				if cc.Kind() != wrapperchecker.KindNamedImports {
					return false
				}
				cc.ForEachChild(func(spec *wrapperchecker.Node) bool {
					if spec.Kind() != wrapperchecker.KindImportSpecifier {
						return false
					}
					var orig, local string
					i := 0
					spec.ForEachChild(func(n *wrapperchecker.Node) bool {
						if n.Kind() == wrapperchecker.KindIdentifier {
							if i == 0 {
								orig = n.LiteralText()
								local = orig
							} else {
								local = n.LiteralText()
							}
							i++
						}
						return false
					})
					if orig == "createElement" {
						out[local] = true
					}
					return false
				})
				return false
			})
			return false
		})
		return false
	})
	if reactShadowed {
		out["__react_shadowed__"] = true
	}
	return out
}

func isCreateElementCallee(callee *wrapperchecker.Node, aliases map[string]bool) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return aliases[callee.LiteralText()]
	case wrapperchecker.KindPropertyAccessExpression:
		if callee.PropertyAccessName() != "createElement" {
			return false
		}
		recv := callee.PropertyAccessReceiver()
		if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		if recv.LiteralText() != "React" {
			return false
		}
		// `React.createElement(...)` is flagged when React isn't
		// shadowed by an import from a non-react module.
		return !aliases["__react_shadowed__"]
	}
	return false
}

func hasFlaggedProperty(arg *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindMethodDeclaration:
			if c.PropertyName() == propName {
				found = true
				return true
			}
		}
		return false
	})
	return found
}

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
