// Package noglobalisnan implements no-global-is-nan: global `isNaN`
// coerces its argument first (`isNaN("foo")` is `true`!). Use
// `Number.isNaN` for a check that doesn't lie.
package noglobalisnan

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-global-is-nan"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := unwrap(firstChild(n))
	name := referencedName(callee)
	if name != "isNaN" {
		return
	}
	if isShadowed(n, "isNaN") {
		return
	}
	ctx.Report(n, "global `isNaN` coerces its argument — use `Number.isNaN`")
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

func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = firstChild(n)
	}
	return n
}

// referencedName extracts the function name from a callee that may be
// a bare identifier, parenthesized, or `<global>.isNaN` / `<global>["isNaN"]`.
func referencedName(n *wrapperchecker.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case wrapperchecker.KindIdentifier:
		return n.SourceText()
	case wrapperchecker.KindPropertyAccessExpression:
		obj, name := propParts(n)
		if isGlobalRef(obj) {
			return name
		}
	case wrapperchecker.KindElementAccessExpression:
		obj, name := elemParts(n)
		if isGlobalRef(obj) {
			return name
		}
	}
	return ""
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

func elemParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	obj, key := propParts(n)
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'' || key[0] == '`') {
		key = key[1 : len(key)-1]
	}
	return obj, key
}

func isGlobalRef(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	n = unwrap(n)
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindIdentifier {
		s := n.SourceText()
		return s == "globalThis" || s == "window" || s == "self"
	}
	if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
		obj, name := propParts(n)
		if name == "window" || name == "globalThis" || name == "self" {
			return isGlobalRef(obj)
		}
	}
	return false
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	prev := start
	for n := start.Parent(); n != nil; n = n.Parent() {
		if declaresInScope(n, prev, name) {
			return true
		}
		prev = n
	}
	return false
}

// declaresInScope checks if scope itself declares `name` (parameter,
// variable, function), without descending into nested function-like
// scopes. `child` is the previous descendant we came up through; we
// must avoid recursing back into it.
func declaresInScope(scope *wrapperchecker.Node, child *wrapperchecker.Node, name string) bool {
	if scope.Kind() == wrapperchecker.KindFunctionDeclaration ||
		scope.Kind() == wrapperchecker.KindFunctionExpression ||
		scope.Kind() == wrapperchecker.KindMethodDeclaration ||
		scope.Kind() == wrapperchecker.KindArrowFunction {
		// Check parameters directly.
		found := false
		scope.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindParameter && declName(c) == name {
				found = true
				return true
			}
			return false
		})
		return found
	}
	// For block-like scopes, look at direct statements only.
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c == child {
			return false
		}
		// Don't dive into other functions.
		switch c.Kind() {
		case wrapperchecker.KindFunctionDeclaration:
			if declName(c) == name {
				found = true
				return true
			}
			return false
		case wrapperchecker.KindClassDeclaration:
			if declName(c) == name {
				found = true
				return true
			}
			return false
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(cc *wrapperchecker.Node) bool {
				if cc.Kind() != wrapperchecker.KindVariableDeclarationList {
					return false
				}
				cc.ForEachChild(func(d *wrapperchecker.Node) bool {
					if d.Kind() == wrapperchecker.KindVariableDeclaration && declName(d) == name {
						found = true
						return true
					}
					return false
				})
				return found
			})
		}
		return found
	})
	return found
}

func declName(n *wrapperchecker.Node) string {
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
