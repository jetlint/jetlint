// Package noparameterproperties implements no-parameter-properties:
// `constructor(public name: string)` is a TS shorthand that conflates
// arg list with class field list. Spelling each field separately
// keeps the class shape obvious.
package noparameterproperties

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-parameter-properties"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindParameter: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	hasModifier := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPublicKeyword, wrapperchecker.KindPrivateKeyword,
			wrapperchecker.KindProtectedKeyword, wrapperchecker.KindReadonlyKeyword:
			hasModifier = true
		}
		return false
	})
	if !hasModifier {
		return
	}
	ctx.Report(n, "parameter property — declare the class field separately")
}
