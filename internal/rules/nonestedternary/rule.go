// Package nonestedternary implements no-nested-ternary: a ternary
// inside a ternary's branches is hard to read at a glance.
package nonestedternary

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-nested-ternary"

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
	if containsTernary(consequent) || containsTernary(alternate) {
		ctx.Report(n, "nested ternary — split into if/else or named variables")
	}
}

func containsTernary(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	// Unwrap parentheses.
	for n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner == nil {
			return false
		}
		n = inner
	}
	return n.Kind() == wrapperchecker.KindConditionalExpression
}
