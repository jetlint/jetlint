// Package noexplicitany implements no-explicit-any: writing `any`
// turns off the type checker for that value. Use `unknown` and a
// narrowing check, or a real type.
package noexplicitany

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-explicit-any"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAnyKeyword: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// `<T extends any>` is a no-op constraint, idiomatic in some places.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindTypeParameter {
		return
	}
	ctx.Report(n, "`any` disables the type checker — use `unknown` and narrow")
}
