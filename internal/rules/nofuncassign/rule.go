// Package nofuncassign implements the no-func-assign rule: reassigning
// a function declaration's name (or the name of a named function
// expression from within its own body) is almost always a mistake. The
// declaration introduces a binding that points at the function value;
// overwriting it loses access to the function from later code.
//
// TypeScript catches this at compile time when strict mode is on, so
// the rule mainly matters for JavaScript and loosely-typed TS code.
package nofuncassign

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-func-assign"

// New constructs a nofuncassign rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
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
	if !symbolIsFunction(sym) {
		return
	}
	ctx.Report(n, fmt.Sprintf("'%s' is a function.", n.LiteralText()))
}

// symbolIsFunction reports whether the symbol was introduced by a
// FunctionDeclaration (`function foo() {…}` at statement level) or by
// a named FunctionExpression (`const a = function bar() {…}` — `bar`
// is only visible inside the function body, and reassigning it from
// inside has no effect on the outer binding).
func symbolIsFunction(sym *wrapperchecker.Symbol) bool {
	for _, decl := range sym.Declarations() {
		switch decl.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression:
			return true
		}
	}
	return false
}
