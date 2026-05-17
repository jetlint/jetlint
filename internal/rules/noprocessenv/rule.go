// Package noprocessenv implements no-process-env: scattering
// `process.env.X` throughout the codebase makes config opaque. Wrap
// access in a single typed module so callers see real names instead
// of strings.
package noprocessenv

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-process-env"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	obj, name := parts(n)
	if name != "env" || obj == nil {
		return
	}
	if obj.Kind() != wrapperchecker.KindIdentifier || obj.SourceText() != "process" {
		return
	}
	if isShadowed(n, "process") {
		return
	}
	// Skip nested access reports — only flag the outer once.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindPropertyAccessExpression {
		var pf *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if pf == nil {
				pf = c
			}
			return false
		})
		if pf == n {
			// outer will be reported instead.
			ctx.Report(n, "process.env access — wrap in a typed config module")
			return
		}
	}
	ctx.Report(n, "process.env access — wrap in a typed config module")
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for n := start.Parent(); n != nil; n = n.Parent() {
		found := false
		walk(n, func(c *wrapperchecker.Node) bool {
			if found {
				return false
			}
			switch c.Kind() {
			case wrapperchecker.KindVariableDeclaration, wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindParameter:
				if declName(c) == name {
					found = true
				}
			case wrapperchecker.KindImportDeclaration:
				// Imports from "process" / "node:process" are the real node
				// `process` — not shadowing.
				if importedFromProcess(c) {
					return false
				}
				if importIntroduces(c, name) {
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

// importedFromProcess returns true if the ImportDeclaration imports
// from "process" or "node:process".
func importedFromProcess(decl *wrapperchecker.Node) bool {
	found := false
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStringLiteral {
			s := c.SourceText()
			if s == `"process"` || s == `'process'` ||
				s == `"node:process"` || s == `'node:process'` {
				found = true
			}
		}
		return false
	})
	return found
}

// importIntroduces returns true if `decl` introduces a binding named `name`.
func importIntroduces(decl *wrapperchecker.Node, name string) bool {
	found := false
	walk(decl, func(c *wrapperchecker.Node) bool {
		if found {
			return false
		}
		if c.Kind() == wrapperchecker.KindImportClause || c.Kind() == wrapperchecker.KindNamespaceImport ||
			c.Kind() == wrapperchecker.KindImportSpecifier {
			if declName(c) == name {
				found = true
			}
		}
		return !found
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

func parts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
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
