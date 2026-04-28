// Package preferoptionalchain implements the prefer-optional-chain
// rule: flag short-circuit chains that should use the `?.` operator.
package preferoptionalchain

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-optional-chain"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
		wrapperchecker.KindElementAccessExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.IsOptionalChain() {
		return
	}
	var recv *wrapperchecker.Node
	if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
		recv = n.PropertyAccessReceiver()
	} else {
		recv = n.ElementAccessReceiver()
	}
	if recv == nil {
		return
	}
	// Unwrap parens.
	for recv.Kind() == wrapperchecker.KindParenthesizedExpression {
		recv = recv.FirstChild()
		if recv == nil {
			return
		}
	}
	// Receiver must be `X || {}`, `X || ({})` — the empty-object
	// fallback is the pattern this rule rewrites to `X?.bar`.
	if recv.Kind() != wrapperchecker.KindBinaryExpression {
		return
	}
	op := recv.BinaryOperatorKind()
	if op != wrapperchecker.KindBarBarToken && op != wrapperchecker.KindQuestionQuestionToken {
		return
	}
	right := recv.BinaryRight()
	if right == nil {
		return
	}
	for right.Kind() == wrapperchecker.KindParenthesizedExpression {
		right = right.FirstChild()
		if right == nil {
			return
		}
	}
	if !isEmptyObjectLiteral(right) {
		return
	}
	ctx.Report(n, "use optional chaining instead of `(X || {}).y`")
}

func isEmptyObjectLiteral(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	return len(n.ObjectProperties()) == 0
}
