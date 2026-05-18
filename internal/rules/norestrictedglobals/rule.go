// Package norestrictedglobals implements no-restricted-globals: flag
// uses of globals that are reserved for browser callbacks (e.g.,
// `event`, `error`) since they shadow each other and are easy to
// confuse.
package norestrictedglobals

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-restricted-globals"

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
	if name != "event" && name != "error" {
		return
	}
	// Skip when the identifier is a binding, property name, or import specifier.
	p := n.Parent()
	if p == nil {
		return
	}
	switch p.Kind() {
	case wrapperchecker.KindParameter, wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindBindingElement, wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertyAssignment, wrapperchecker.KindShorthandPropertyAssignment,
		wrapperchecker.KindImportSpecifier, wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindImportClause, wrapperchecker.KindExportSpecifier,
		wrapperchecker.KindMethodDeclaration, wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration, wrapperchecker.KindTypeReference,
		wrapperchecker.KindTypeAliasDeclaration, wrapperchecker.KindInterfaceDeclaration:
		return
	case wrapperchecker.KindPropertyAccessExpression, wrapperchecker.KindQualifiedName:
		// Skip when identifier is the property name (right side).
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
	if isDeclared(n, name) {
		return
	}
	ctx.Report(n, "`"+name+"` is a restricted global — use a more specific name")
}

func isDeclared(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor:
			if functionHasParam(p, name) {
				return true
			}
		case wrapperchecker.KindBlock:
			if blockDeclares(p, name) {
				return true
			}
		case wrapperchecker.KindSourceFile:
			return sourceFileDeclares(p, name)
		}
	}
	return false
}

func functionHasParam(fn *wrapperchecker.Node, name string) bool {
	found := false
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			var pn *wrapperchecker.Node
			c.ForEachChild(func(p *wrapperchecker.Node) bool {
				if pn == nil {
					pn = p
				}
				return false
			})
			if pn != nil && pn.Kind() == wrapperchecker.KindIdentifier && pn.SourceText() == name {
				found = true
			}
		}
		return false
	})
	return found
}

func blockDeclares(block *wrapperchecker.Node, name string) bool {
	found := false
	block.ForEachChild(func(c *wrapperchecker.Node) bool {
		if declaresInStatement(c, name) {
			found = true
			return true
		}
		return false
	})
	return found
}

func sourceFileDeclares(sf *wrapperchecker.Node, name string) bool {
	found := false
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		if declaresInStatement(c, name) {
			found = true
			return true
		}
		return false
	})
	return found
}

func declaresInStatement(c *wrapperchecker.Node, name string) bool {
	switch c.Kind() {
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration:
		var first *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if first == nil && d.Kind() == wrapperchecker.KindIdentifier {
				first = d
			}
			return false
		})
		return first != nil && first.SourceText() == name
	case wrapperchecker.KindVariableStatement:
		found := false
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
					if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == name {
						found = true
					}
					return false
				})
			}
			return false
		})
		return found
	}
	return false
}
