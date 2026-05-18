// Package noconsole implements no-console: `console.*` calls leak
// into production builds and slow them down. Use a real logger.
package noconsole

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-console"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
		wrapperchecker.KindElementAccessExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil {
		return
	}
	// Direct `console.X`
	if first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == "console" {
		// Only report once per outer expression — skip if this access is
		// itself the object of an enclosing access.
		if p := n.Parent(); p != nil {
			if p.Kind() == wrapperchecker.KindPropertyAccessExpression ||
				p.Kind() == wrapperchecker.KindElementAccessExpression {
				var pf *wrapperchecker.Node
				p.ForEachChild(func(c *wrapperchecker.Node) bool {
					if pf == nil {
						pf = c
					}
					return false
				})
				if pf == n {
					return
				}
			}
		}
		if isShadowed(n, "console") {
			return
		}
		ctx.Report(n, "remove the console call — use a real logger")
		return
	}
	// `globalThis.console`, `window.console`, `self.console`
	if first.Kind() == wrapperchecker.KindPropertyAccessExpression {
		_, name := propertyAccessParts(first)
		if name == "console" && isGlobalObject(propertyAccessOwner(first)) {
			ctx.Report(n, "remove the console call — use a real logger")
		}
	}
}

func propertyAccessOwner(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first
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

// isShadowed checks ancestors for a local declaration of `name`.
func isShadowed(start *wrapperchecker.Node, name string) bool {
	for n := start.Parent(); n != nil; n = n.Parent() {
		found := false
		walk(n, func(c *wrapperchecker.Node) bool {
			if found {
				return false
			}
			switch c.Kind() {
			case wrapperchecker.KindVariableDeclaration, wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindParameter:
				if declName(c) == name {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
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

func isGlobalObject(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	s := n.SourceText()
	return s == "globalThis" || s == "window" || s == "self"
}
