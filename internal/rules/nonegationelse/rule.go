// Package nonegationelse implements no-negation-else: an `if` with a
// negated test plus an `else` is easier to read with the branches
// swapped. Same for the ternary.
package nonegationelse

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-negation-else"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:            visitIf,
		wrapperchecker.KindConditionalExpression:  visitCond,
	}
}

func visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	var test, then, els *wrapperchecker.Node
	i := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch i {
		case 0:
			test = c
		case 1:
			then = c
		case 2:
			els = c
		}
		i++
		return false
	})
	_ = then
	if els == nil || test == nil {
		return
	}
	// Skip if else clause is another `if` (else-if chain).
	if els.Kind() == wrapperchecker.KindIfStatement {
		return
	}
	if isNegatedTest(test) {
		ctx.Report(n, "swap branches to remove the negation in the `if` test")
	}
}

func visitCond(ctx *engine.Context, n *wrapperchecker.Node) {
	var test *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if test == nil {
			test = c
		}
		return false
	})
	if test == nil {
		return
	}
	if isNegatedTest(test) {
		ctx.Report(n, "swap branches to remove the negation in the ternary test")
	}
}

func isNegatedTest(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindPrefixUnaryExpression:
		if n.PrefixUnaryOperator() != "!" {
			return false
		}
		// Skip double-negation `!!x`.
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner != nil && inner.Kind() == wrapperchecker.KindPrefixUnaryExpression && inner.PrefixUnaryOperator() == "!" {
			return false
		}
		return true
	case wrapperchecker.KindBinaryExpression:
		// Check for != or !==.
		src := n.SourceText()
		return containsOp(src, "!=") || containsOp(src, "!==")
	}
	return false
}

func containsOp(src, op string) bool {
	// Find op surrounded by non-identifier chars (rough).
	return indexOp(src, op) >= 0
}

func indexOp(src, op string) int {
	for i := 0; i+len(op) <= len(src); i++ {
		if src[i:i+len(op)] == op {
			return i
		}
	}
	return -1
}
