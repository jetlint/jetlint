// Package nounnecessarycondition implements the no-unnecessary-condition
// rule: flag conditional positions whose test type is provably
// constant (always-true or always-false).
package nounnecessarycondition

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-condition"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           visitIf,
		wrapperchecker.KindWhileStatement:        visitWhile,
		wrapperchecker.KindDoStatement:           visitWhile,
		wrapperchecker.KindConditionalExpression: visitConditional,
	}
}

func visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n.IfCondition())
}

func visitWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n.WhileCondition())
}

func visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	check(ctx, n.ConditionalCondition())
}

func check(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	switch t.String() {
	case "true":
		ctx.Report(expr, "condition is always truthy")
	case "false":
		ctx.Report(expr, "condition is always falsy")
	}
}
