// Package noalert implements no-alert: alert/confirm/prompt block
// the UI thread and ship a non-stylable native dialog. They're fine
// in throwaway scripts but never in shipped product code.
package noalert

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-alert"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

var bannedNames = map[string]bool{"alert": true, "confirm": true, "prompt": true}

func visit(ctx *engine.Context, call *wrapperchecker.Node) {
	expr := callee(call)
	if expr == nil {
		return
	}
	name := identifierName(expr)
	if name == "" {
		return
	}
	if !bannedNames[name] {
		return
	}
	// Skip if shadowed by a local declaration in scope.
	if isShadowed(call, name) {
		return
	}
	ctx.Report(call, name+" blocks the UI thread and can't be styled — use a real dialog component")
}

// callee returns the call's callee node.
func callee(call *wrapperchecker.Node) *wrapperchecker.Node {
	var out *wrapperchecker.Node
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if out == nil {
			out = c
		}
		return false
	})
	return out
}

// identifierName extracts the function name from:
//   - bare identifier:           alert
//   - parenthesized:             (alert)
//   - property access:           window.alert
//   - element access:            window["alert"]
func identifierName(n *wrapperchecker.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case wrapperchecker.KindIdentifier:
		return n.SourceText()
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		return identifierName(inner)
	case wrapperchecker.KindPropertyAccessExpression:
		obj, name := propertyAccessParts(n)
		if isWindowOrGlobal(obj) {
			return name
		}
	case wrapperchecker.KindElementAccessExpression:
		obj, name := elementAccessParts(n)
		if isWindowOrGlobal(obj) {
			return name
		}
	}
	return ""
}

func propertyAccessParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
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

func elementAccessParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	obj, key := propertyAccessParts(n)
	// `key` is a literal like "alert"; strip quotes.
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'' || key[0] == '`') {
		key = key[1 : len(key)-1]
	}
	return obj, key
}

func isWindowOrGlobal(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	s := n.SourceText()
	return s == "window" || s == "globalThis" || s == "self"
}

// isShadowed walks ancestors looking for a local declaration of `name`.
func isShadowed(start *wrapperchecker.Node, name string) bool {
	for n := start.Parent(); n != nil; n = n.Parent() {
		if hasLocalDeclaration(n, name) {
			return true
		}
	}
	return false
}

// hasLocalDeclaration checks a scope-owning node's direct children for
// a declaration named `name`.
func hasLocalDeclaration(scope *wrapperchecker.Node, name string) bool {
	found := false
	walk(scope, func(n *wrapperchecker.Node) bool {
		if found {
			return false
		}
		switch n.Kind() {
		case wrapperchecker.KindVariableDeclaration:
			if declName(n) == name {
				found = true
			}
		case wrapperchecker.KindFunctionDeclaration:
			if declName(n) == name {
				found = true
			}
		}
		return !found
	})
	return found
}

func walk(n *wrapperchecker.Node, fn func(*wrapperchecker.Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c, fn)
		return false
	})
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
