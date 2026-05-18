// Package noenum implements no-enum: TS enums emit non-trivial
// runtime code, are tricky to tree-shake, and clash with the ES
// modules pattern of the rest of the language. Use string-literal
// unions or `as const` objects.
package noenum

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-enum"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindEnumDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// `const enum` is covered by no-const-enum; this rule only flags
	// regular enums.
	hasConst := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindConstKeyword {
			hasConst = true
		}
		return false
	})
	if hasConst {
		return
	}
	ctx.Report(n, "enums emit runtime code and are hard to tree-shake — use a union or `as const` object")
}
