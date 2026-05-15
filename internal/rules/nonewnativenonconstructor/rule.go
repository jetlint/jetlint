// Package nonewnativenonconstructor implements the
// no-new-native-nonconstructor rule: `Symbol` and `BigInt` are
// global *functions*, not classes. Invoking them with `new` throws
// a TypeError. The capitalized names tempt readers to treat them
// like constructors; this rule catches that mistake.
package nonewnativenonconstructor

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-new-native-nonconstructor"

// targets enumerates the globals that cannot be invoked with `new`.
var targets = map[string]bool{
	"Symbol": true,
	"BigInt": true,
}

// New constructs a nonewnativenonconstructor rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNewExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	name := callee.LiteralText()
	if !targets[name] {
		return
	}
	if hasEnclosingBinding(callee, name) {
		return
	}
	ctx.Report(n, "'"+name+"' cannot be called as a constructor.")
}

// hasEnclosingBinding reports whether any scope enclosing `n` binds
// `name`. Used to skip the rule when the global is shadowed by a
// user-defined declaration.
func hasEnclosingBinding(n *wrapperchecker.Node, name string) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if !isScopeNode(p) {
			continue
		}
		if scopeBinds(p, name) {
			return true
		}
	}
	return false
}

func scopeBinds(scope *wrapperchecker.Node, name string) bool {
	switch scope.Kind() {
	case wrapperchecker.KindFunctionExpression, wrapperchecker.KindClassExpression:
		if functionExpressionName(scope) == name {
			return true
		}
	}
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if declarationBindsName(c, name) {
			found = true
			return true
		}
		return false
	})
	if found {
		return true
	}
	switch scope.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		for _, p := range scope.FunctionParameters() {
			if declarationBindsName(p, name) {
				return true
			}
		}
	}
	return false
}

func isScopeNode(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindSourceFile,
		wrapperchecker.KindBlock,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindModuleDeclaration,
		wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement,
		wrapperchecker.KindCatchClause:
		return true
	}
	return false
}

func declarationBindsName(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableStatement,
		wrapperchecker.KindVariableDeclarationList:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declarationBindsName(c, name) {
				hit = true
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			if c.Kind() == wrapperchecker.KindObjectBindingPattern ||
				c.Kind() == wrapperchecker.KindArrayBindingPattern {
				if patternBindsName(c, name) {
					hit = true
					return true
				}
			}
			return false
		})
		return hit
	case wrapperchecker.KindImportDeclaration:
		return patternBindsName(n, name)
	}
	return false
}

func patternBindsName(n *wrapperchecker.Node, name string) bool {
	hit := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
			hit = true
			return true
		}
		if patternBindsName(c, name) {
			hit = true
			return true
		}
		return false
	})
	return hit
}

func functionExpressionName(n *wrapperchecker.Node) string {
	name := ""
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
