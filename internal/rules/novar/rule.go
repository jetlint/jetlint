// Package novar implements no-var: `var` declarations have function
// scope, hoist to the top of their function, and allow redeclaration
// — all of which combine to make refactors risky. `let` and `const`
// stay where you wrote them.
package novar

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-var"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclarationList: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !startsWithVar(n.SourceText()) {
		return
	}
	if insideAmbientGlobal(n) {
		return
	}
	ctx.Report(n, "use `let` or `const` instead of `var`")
}

func startsWithVar(src string) bool {
	for len(src) > 0 && (src[0] == ' ' || src[0] == '\t' || src[0] == '\n') {
		src = src[1:]
	}
	return len(src) >= 4 && src[:4] == "var "
}

func insideAmbientGlobal(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindModuleDeclaration {
			return true
		}
	}
	return false
}
