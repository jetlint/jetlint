// Package nouselessundefinedinitialization implements
// no-useless-undefined-initialization: `let x = undefined` is the
// default — say `let x` instead.
package nouselessundefinedinitialization

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-undefined-initialization"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Only flag `let` / `var` — `const x = undefined` is fine (a constant).
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindVariableDeclarationList {
		return
	}
	if !startsWithLet(p.SourceText()) {
		return
	}
	// Skip if the containing statement is exported / used as a module
	// prop (Svelte pattern, etc.).
	if stmt := p.Parent(); stmt != nil {
		if hasExportModifier(stmt) || isExportedByName(stmt, declName(n)) {
			return
		}
	}
	var init *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && init == nil {
			init = c
		}
		idx++
		return false
	})
	if init == nil {
		return
	}
	if init.Kind() == wrapperchecker.KindIdentifier && init.SourceText() == "undefined" {
		ctx.Report(n, "`undefined` initializer is redundant — drop the `= undefined`")
	}
}

func hasExportModifier(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindExportKeyword {
			found = true
		}
		return false
	})
	return found
}

func isExportedByName(stmt *wrapperchecker.Node, name string) bool {
	if name == "" {
		return false
	}
	// Walk up to SourceFile, then scan for `export { name }` declarations.
	root := stmt
	for root.Parent() != nil {
		root = root.Parent()
	}
	found := false
	walk(root, func(c *wrapperchecker.Node) bool {
		if found {
			return false
		}
		if c.Kind() == wrapperchecker.KindExportSpecifier {
			var first *wrapperchecker.Node
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if first == nil {
					first = d
				}
				return false
			})
			if first != nil && first.SourceText() == name {
				found = true
			}
		}
		if c.Kind() == wrapperchecker.KindExportAssignment {
			// `export default name`
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if d.Kind() == wrapperchecker.KindIdentifier && d.SourceText() == name {
					found = true
				}
				return false
			})
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

func startsWithLet(src string) bool {
	for len(src) > 0 && (src[0] == ' ' || src[0] == '\t' || src[0] == '\n') {
		src = src[1:]
	}
	return strings.HasPrefix(src, "let ") || strings.HasPrefix(src, "var ")
}
