// Package nowith implements the no-with rule: `with` makes scope
// unresolvable at parse time and is banned in strict mode. Forbid it
// everywhere.
//
// Matches oxlint and ESLint's no-with: zero configurable options.
// Reports each `with` statement at the statement node.
package nowith

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-with"

// kindWithStatement mirrors `ast.KindWithStatement`, which the TS-go
// wrapper does not re-export. The AST walk still produces nodes
// reporting this numeric kind, so handler dispatch works directly.
const kindWithStatement wrapperchecker.Kind = 255

// New constructs a no-with rule.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		kindWithStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	ctx.Report(n, "Unexpected use of `with` statement.")
}
