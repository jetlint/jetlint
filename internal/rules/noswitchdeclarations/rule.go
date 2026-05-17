// Package noswitchdeclarations implements no-switch-declarations:
// reject lexical declarations (`let`, `const`, `function`, `class`,
// `enum`, `interface`, `type`) placed directly under a `case` or
// `default` clause without being wrapped in a block. Because case
// clauses share a single lexical scope, the binding leaks into
// subsequent cases and is also visible to the cases above it via
// hoisting/temporal-dead-zone effects, which is almost never what
// the author meant.
//
// `var` declarations are intentionally allowed — `var` is
// function-scoped, and flagging it would be redundant noise. A
// nested Block (`case 1: { let x = 1; break; }`) gives the binding
// its own scope and passes.
package noswitchdeclarations

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-switch-declarations"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCaseClause:    visit,
		wrapperchecker.KindDefaultClause: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// CaseClause/DefaultClause children: optional case-expression
	// followed by statements. ForEachChild visits them in source
	// order; we only flag statements that are lexical declarations.
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if isLexicalDeclaration(c) {
			ctx.Report(c, "wrap this declaration in a block — case clauses share a scope")
		}
		return false
	})
}

// isLexicalDeclaration reports whether n is a statement-level
// declaration that introduces a binding visible across the whole
// switch's clause group. `var` is excluded because its scope is the
// enclosing function, not the case.
func isLexicalDeclaration(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration:
		return true
	case wrapperchecker.KindVariableStatement:
		return isBlockScopedVarStatement(n)
	}
	return false
}

// isBlockScopedVarStatement returns true for `let` / `const`
// VariableStatements. The wrapper exposes IsConstVariableDeclaration
// for const but not let; rather than reach into NodeFlags we scan the
// declaration keyword from source text — it's the first identifier
// of the statement and unambiguous in JS/TS grammar.
func isBlockScopedVarStatement(n *wrapperchecker.Node) bool {
	src := strings.TrimLeft(n.SourceText(), " \t\n\r")
	// Allow modifiers like `export` before the keyword.
	for _, mod := range []string{"export "} {
		if rest, ok := strings.CutPrefix(src, mod); ok {
			src = strings.TrimLeft(rest, " \t")
		}
	}
	return startsWithKeyword(src, "let") || startsWithKeyword(src, "const")
}

func startsWithKeyword(src, kw string) bool {
	if !strings.HasPrefix(src, kw) {
		return false
	}
	if len(src) == len(kw) {
		return true
	}
	next := src[len(kw)]
	// Keyword must be followed by whitespace or an identifier-break
	// character so `letter` isn't mistaken for `let er`.
	return next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '['
}
