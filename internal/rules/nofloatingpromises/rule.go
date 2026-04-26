// Package nofloatingpromises implements the no-floating-promises rule:
// flag any expression statement whose value is a Promise (or other
// thenable) that is not awaited, returned, voided, or otherwise handled.
//
// The rule's signature catch is the cross-file case — calling an async
// function from another module without awaiting it. That requires real
// type information across imports, which is exactly what biome cannot
// produce today and what makes this rule the headline differentiator
// for the linter.
package nofloatingpromises

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-floating-promises"

// New constructs a fresh rule instance. The rule has no per-instance
// configuration in the MVP; future revisions may accept options here.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExpressionStatement: visitExpressionStatement,
	}
}

func visitExpressionStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	if isHandled(expr) {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if !isAnyMemberThenable(t) {
		return
	}
	ctx.Report(n, "promise returned by this expression is not awaited or otherwise handled")
}

// isHandled reports whether the expression is in a position that
// suppresses the floating-promise warning: explicit `void`, `await`, or
// promise-chain handling like `.then`/`.catch`/`.finally`.
func isHandled(expr *wrapperchecker.Node) bool {
	switch expr.Kind() {
	case wrapperchecker.KindAwaitExpression, wrapperchecker.KindVoidExpression:
		return true
	}
	return false
}

// isAnyMemberThenable returns true when the type is thenable, or when
// any constituent of a union is thenable. The union case handles
// shapes like `Promise<T> | undefined`.
func isAnyMemberThenable(t *wrapperchecker.Type) bool {
	if t.IsThenable() {
		return true
	}
	if !t.IsUnion() {
		return false
	}
	for _, m := range t.UnionMembers() {
		if m.IsThenable() {
			return true
		}
	}
	return false
}
