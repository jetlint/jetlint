// Package useenuminitializers implements use-enum-initializers: enum
// members without explicit values change when members are reordered —
// always give them initializers.
package useenuminitializers

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-enum-initializers"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindEnumDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if isDeclared(n) {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindEnumMember {
			return false
		}
		hasInit := false
		idx := 0
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if idx > 0 {
				hasInit = true
			}
			idx++
			return false
		})
		if !hasInit {
			ctx.Report(c, "enum member needs an explicit initializer")
		}
		return false
	})
}

func isDeclared(enum *wrapperchecker.Node) bool {
	if enum.HasDeclareModifier() {
		return true
	}
	for p := enum.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindModuleDeclaration && p.HasDeclareModifier() {
			return true
		}
	}
	return false
}
