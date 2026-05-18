// Package nouselessternary implements no-useless-ternary: ternaries
// whose branches are both boolean literals (or duplicate values) collapse
// to a simple expression.
package nouselessternary

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-ternary"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindConditionalExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	consequent, alternate := n.ConditionalBranches()
	if consequent == nil || alternate == nil {
		return
	}
	cKind, aKind := consequent.Kind(), alternate.Kind()
	if (cKind == wrapperchecker.KindTrueKeyword && aKind == wrapperchecker.KindFalseKeyword) ||
		(cKind == wrapperchecker.KindFalseKeyword && aKind == wrapperchecker.KindTrueKeyword) ||
		(cKind == wrapperchecker.KindTrueKeyword && aKind == wrapperchecker.KindTrueKeyword) ||
		(cKind == wrapperchecker.KindFalseKeyword && aKind == wrapperchecker.KindFalseKeyword) {
		ctx.Report(n, "ternary with boolean-literal branches collapses to a boolean expression")
	}
}
