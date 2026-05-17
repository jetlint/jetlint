// Package noemptytypeparameters implements no-empty-type-parameters:
// `type A<> = {}` or `interface B<> {}` declares no type parameters
// at all — the brackets are a left-over from a deleted parameter.
package noemptytypeparameters

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-empty-type-parameters"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTypeAliasDeclaration:  visit,
		wrapperchecker.KindInterfaceDeclaration:  visit,
		wrapperchecker.KindFunctionDeclaration:   visit,
		wrapperchecker.KindMethodDeclaration:     visit,
		wrapperchecker.KindClassDeclaration:      visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Source-based check: look for empty `<>` after the declared name.
	if strings.Contains(n.SourceText(), "<>") {
		ctx.Report(n, "empty type parameter list `<>` — remove it or add a parameter")
	}
}
