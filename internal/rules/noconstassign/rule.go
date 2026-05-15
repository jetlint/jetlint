// Package noconstassign implements the no-const-assign rule: writing
// to a `const` (or `using` / `await using`) binding is a runtime
// TypeError. TypeScript catches this under strict configurations, but
// the rule still pays for plain-JS files and for declarations the
// checker can't statically resolve.
package noconstassign

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-const-assign"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.IsAssignmentTarget() {
		return
	}
	var sym *wrapperchecker.Symbol
	if parent := n.Parent(); parent != nil && parent.Kind() == wrapperchecker.KindShorthandPropertyAssignment {
		sym = ctx.Checker().ShorthandAssignmentValueSymbol(parent)
	} else {
		sym = ctx.Checker().SymbolOf(n)
	}
	if sym == nil {
		return
	}
	if !symbolIsImmutable(sym) {
		return
	}
	ctx.Report(n, fmt.Sprintf("'%s' is constant.", n.LiteralText()))
}

func symbolIsImmutable(sym *wrapperchecker.Symbol) bool {
	for _, decl := range sym.Declarations() {
		if declOriginatesInConstBinding(decl) {
			return true
		}
	}
	return false
}

// declOriginatesInConstBinding walks up from a declaration site to the
// nearest VariableDeclarationList and reports whether that list is
// const / using / await using. This covers both the plain
// `const x = …` shape and destructured bindings like
// `const {a: x} = …` where the symbol's declaration is the inner
// Identifier or BindingElement rather than a VariableDeclaration.
func declOriginatesInConstBinding(decl *wrapperchecker.Node) bool {
	for cur := decl; cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindVariableDeclaration:
			if cur.IsConstVariableDeclaration() {
				return true
			}
			parent := cur.Parent()
			if parent != nil && (parent.IsUsingDeclaration() || parent.IsAwaitUsingDeclaration()) {
				return true
			}
		case wrapperchecker.KindVariableDeclarationList:
			if cur.IsUsingDeclaration() || cur.IsAwaitUsingDeclaration() {
				return true
			}
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression,
			wrapperchecker.KindSourceFile:
			return false
		}
	}
	return false
}
