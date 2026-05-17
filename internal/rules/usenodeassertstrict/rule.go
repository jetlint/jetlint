// Package usenodeassertstrict implements use-node-assert-strict: the
// loose `assert` builtin coerces operands. `node:assert/strict` (or
// `assert.strict`) uses strict equality, matching test-runner
// expectations.
package usenodeassertstrict

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-node-assert-strict"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindStringLiteral {
			return false
		}
		s := strings.Trim(c.SourceText(), `"'`+"`")
		if s == "assert" || s == "node:assert" {
			ctx.Report(c, "use `node:assert/strict` for strict-equality assertions")
		}
		return false
	})
}
