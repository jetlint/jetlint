// Package usenumbernamespace implements use-number-namespace: the
// global `NaN`, `Infinity`, `parseInt`, `parseFloat`, and `isNaN`
// also exist on `Number` as `Number.NaN`, `Number.parseInt`, … —
// using the namespaced names is unambiguous and survives shadowing.
package usenumbernamespace

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-number-namespace"

var globalNames = map[string]bool{
	"NaN":        true,
	"Infinity":   true,
	"parseInt":   true,
	"parseFloat": true,
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	name := n.SourceText()
	if !globalNames[name] {
		return
	}
	p := n.Parent()
	if p == nil {
		return
	}
	// Skip when this identifier isn't a reference.
	switch p.Kind() {
	case wrapperchecker.KindParameter, wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindBindingElement, wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration, wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause, wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindExportSpecifier, wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindPropertyDeclaration, wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindTypeAliasDeclaration, wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeReference, wrapperchecker.KindTypeParameter:
		return
	case wrapperchecker.KindPropertyAccessExpression, wrapperchecker.KindQualifiedName:
		// Skip property name (the right side).
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		if first != nil && first.Pos() != n.Pos() {
			return
		}
	}
	if isShadowed(n, name) {
		return
	}
	ctx.Report(n, "use `Number."+name+"` instead of the global `"+name+"`")
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		shadowed := false
		// Check params for function-like nodes.
		if isFunctionLike(p) {
			p.ForEachChild(func(c *wrapperchecker.Node) bool {
				if c.Kind() == wrapperchecker.KindParameter {
					var firstParam *wrapperchecker.Node
					c.ForEachChild(func(d *wrapperchecker.Node) bool {
						if firstParam == nil {
							firstParam = d
						}
						return false
					})
					if bindingPatternBinds(firstParam, name) {
						shadowed = true
						return true
					}
				}
				return false
			})
			if shadowed {
				return true
			}
		}
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declaresName(c, name) {
				shadowed = true
				return true
			}
			return false
		})
		if shadowed {
			return true
		}
	}
	return false
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor:
		return true
	}
	return false
}

// bindingPatternBinds walks a binding pattern (an identifier, array-,
// or object-pattern) and returns true if `name` appears as one of the
// binding identifiers — skipping initializer expressions, which are
// uses rather than bindings.
func bindingPatternBinds(n *wrapperchecker.Node, name string) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindIdentifier:
		return n.SourceText() == name
	case wrapperchecker.KindObjectBindingPattern, wrapperchecker.KindArrayBindingPattern:
		found := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if bindingPatternBinds(c, name) {
				found = true
				return true
			}
			return false
		})
		return found
	case wrapperchecker.KindBindingElement:
		// First identifier is property name (or binding); second (when
		// preceded by a colon) is rename. Walk first child only — but
		// stop before initializer.
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		return bindingPatternBinds(first, name)
	}
	return false
}

func declaresName(c *wrapperchecker.Node, name string) bool {
	switch c.Kind() {
	case wrapperchecker.KindVariableStatement:
		hit := false
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
					if bindingPatternBinds(first, name) {
						hit = true
					}
					return false
				})
			}
			return false
		})
		return hit
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration:
		var first *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if first == nil && d.Kind() == wrapperchecker.KindIdentifier {
				first = d
			}
			return false
		})
		return first != nil && first.SourceText() == name
	}
	return false
}
