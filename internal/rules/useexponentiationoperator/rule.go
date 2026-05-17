// Package useexponentiationoperator implements
// use-exponentiation-operator: `Math.pow(a, b)` predates the `**`
// operator. The operator is shorter, reads as math, and gets the
// same answer.
package useexponentiationoperator

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-exponentiation-operator"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	obj, name := propParts(callee)
	if name != "pow" || obj == nil {
		return
	}
	if obj.Kind() != wrapperchecker.KindIdentifier || obj.SourceText() != "Math" {
		return
	}
	if isShadowed(n, "Math") {
		return
	}
	ctx.Report(n, "use the `**` operator instead of Math.pow")
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	prev := start
	for p := start.Parent(); p != nil; p = p.Parent() {
		if declaresInScope(p, prev, name) {
			return true
		}
		prev = p
	}
	return false
}

func declaresInScope(scope *wrapperchecker.Node, child *wrapperchecker.Node, name string) bool {
	if isFunctionLike(scope) {
		found := false
		scope.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindParameter && declIdentName(c) == name {
				found = true
				return true
			}
			if c.Kind() == wrapperchecker.KindIdentifier && c.SourceText() == name {
				// Named FunctionExpression — `var f = function Math() {...}`.
				found = true
				return true
			}
			return false
		})
		if found {
			return true
		}
		// Fall through to scan the function body for variable decls.
	}
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindFunctionDeclaration:
			if declIdentName(c) == name {
				found = true
				return true
			}
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(cc *wrapperchecker.Node) bool {
				if cc.Kind() != wrapperchecker.KindVariableDeclarationList {
					return false
				}
				cc.ForEachChild(func(d *wrapperchecker.Node) bool {
					if d.Kind() == wrapperchecker.KindVariableDeclaration && declIdentName(d) == name {
						found = true
						return true
					}
					return false
				})
				return found
			})
		case wrapperchecker.KindBlock, wrapperchecker.KindIfStatement, wrapperchecker.KindForStatement,
			wrapperchecker.KindForInStatement, wrapperchecker.KindForOfStatement,
			wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
			// Recurse into nested non-function blocks.
			if c != child {
				if declaresInScope(c, child, name) {
					found = true
					return true
				}
			}
		}
		return found
	})
	return found
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

func declIdentName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
		return first.SourceText()
	}
	return ""
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
}
