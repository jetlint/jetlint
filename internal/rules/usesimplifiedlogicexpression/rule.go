// Package usesimplifiedlogicexpression implements
// use-simplified-logic-expression: expressions like `true && X`,
// `X || true`, and `null ?? X` always reduce to one side — drop the
// trivial part.
package usesimplifiedlogicexpression

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-simplified-logic-expression"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindAmpersandAmpersandToken:
		// `true && X` or `X && true` simplifies.
		if isTrue(left) || isTrue(right) {
			ctx.Report(n, "logical expression has a constant operand — simplify")
		}
	case wrapperchecker.KindBarBarToken:
		if isTrue(left) || isTrue(right) || isFalse(left) || isFalse(right) {
			ctx.Report(n, "logical expression has a constant operand — simplify")
		}
	case wrapperchecker.KindQuestionQuestionToken:
		if isNull(left) || isUndefined(left) {
			ctx.Report(n, "left side of `??` is always nullish — drop it")
		}
	}
	// `!a || !b` → `!(a && b)` and `!a && !b` → `!(a || b)`.
	if op := n.BinaryOperatorKind(); op == wrapperchecker.KindAmpersandAmpersandToken || op == wrapperchecker.KindBarBarToken {
		if isNegated(left) && isNegated(right) {
			ctx.Report(n, "apply De Morgan's law to negate once instead of twice")
		}
	}
}

func isTrue(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindTrueKeyword
}

func isFalse(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindFalseKeyword
}

func isNull(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindNullKeyword
}

func isUndefined(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindIdentifier && n.SourceText() == "undefined"
}

func isNegated(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindPrefixUnaryExpression {
		return false
	}
	if n.PrefixUnaryOperator() != "!" {
		return false
	}
	// Skip double-negation `!!x` — that's a coercion idiom.
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
}
