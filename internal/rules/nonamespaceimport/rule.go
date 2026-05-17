// Package nonamespaceimport implements no-namespace-import: flag
// `import * as foo from "mod"`. Namespace imports defeat tree-shaking
// because bundlers must conservatively keep every export from the
// module in case any property of the namespace is read.
//
// `import type * as ...` is allowed: type imports are erased before
// bundling so tree-shaking is unaffected.
package nonamespaceimport

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-namespace-import"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
	}
}

func visit(ctx *engine.Context, decl *wrapperchecker.Node) {
	var clause *wrapperchecker.Node
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindImportClause {
			clause = c
			return true
		}
		return false
	})
	if clause == nil || isTypeOnlyImportClause(clause) {
		return
	}
	clause.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindNamespaceImport {
			ctx.Report(c, "namespace import (`* as`) defeats tree-shaking — prefer named imports")
			return true
		}
		return false
	})
}

// isTypeOnlyImportClause checks whether the import declaration was
// written as `import type ...`. The wrapper doesn't surface the
// IsTypeOnly flag on ImportClause, so we peek at the raw source
// text just past the leading `import` keyword.
func isTypeOnlyImportClause(clause *wrapperchecker.Node) bool {
	text := strings.TrimSpace(clause.SourceText())
	return strings.HasPrefix(text, "type ") || strings.HasPrefix(text, "type{") || text == "type"
}
