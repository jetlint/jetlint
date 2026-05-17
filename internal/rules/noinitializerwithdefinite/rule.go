// Package noinitializerwithdefinite implements
// no-initializer-with-definite: TypeScript's definite-assignment
// assertion `!` declares "I'll assign this before use" — so pairing
// it with an initializer is contradictory. The initializer already
// proves the variable is assigned, making the `!` redundant noise.
package noinitializerwithdefinite

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-initializer-with-definite"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visit,
	}
}

func visit(ctx *engine.Context, decl *wrapperchecker.Node) {
	if decl.VariableDeclarationInitializer() == nil {
		return
	}
	if !hasDefiniteAssertion(decl) {
		return
	}
	ctx.Report(decl, "remove the `!` definite-assignment assertion or the initializer; they're contradictory")
}

// hasDefiniteAssertion inspects the source text of a variable decl
// for a `!` between the binding identifier and its type annotation
// (e.g., `let a!: number = 5`). It scans the prefix of the decl's
// source up to `=`, which avoids being misled by `!` inside the
// initializer expression itself.
func hasDefiniteAssertion(decl *wrapperchecker.Node) bool {
	text := decl.SourceText()
	if eq := strings.Index(text, "="); eq >= 0 {
		text = text[:eq]
	}
	return strings.Contains(text, "!")
}
