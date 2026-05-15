// Package noasyncpromiseexecutor implements the
// no-async-promise-executor rule: passing an async function to the
// Promise constructor is almost always a mistake — the Promise
// resolves immediately with the async function's returned Promise,
// not with the value passed to `resolve`, and any rejection inside
// the async body is swallowed unless the caller awaits the wrapper.
package noasyncpromiseexecutor

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-async-promise-executor"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNewExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, newExpr *wrapperchecker.Node) {
	callee := stripParens(newExpr.CalleeExpression())
	if callee == nil {
		return
	}
	if callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.LiteralText() != "Promise" {
		return
	}
	args := newExpr.CallArguments()
	if len(args) == 0 {
		return
	}
	executor := stripParens(args[0])
	if executor == nil {
		return
	}
	if !wrapperchecker.IsAsyncFunction(executor) {
		return
	}
	ctx.Report(executor, "async function should not be used as a Promise executor")
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
