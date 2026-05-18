// Package noconstenum implements no-const-enum: `const enum` requires
// inlining at use sites, which breaks when consumers aren't compiled
// together (especially across module boundaries). A regular enum or
// a typed object literal is portable.
package noconstenum

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-const-enum"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindEnumDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	hasConst := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindConstKeyword {
			hasConst = true
		}
		return false
	})
	if hasConst {
		ctx.Report(n, "`const enum` can't be safely consumed across module boundaries — use a regular enum")
	}
}
