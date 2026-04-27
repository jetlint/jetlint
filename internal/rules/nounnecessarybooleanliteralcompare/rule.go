// Package nounnecessarybooleanliteralcompare implements the
// no-unnecessary-boolean-literal-compare rule: flag `x === true`,
// `x !== false`, etc. where x already has type boolean.
package nounnecessarybooleanliteralcompare

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-boolean-literal-compare"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	switch op {
	case wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken:
	default:
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	if isBooleanLiteralKeyword(left) && nonBoolUnacceptable(ctx, right) {
		return
	}
	if isBooleanLiteralKeyword(right) && nonBoolUnacceptable(ctx, left) {
		return
	}
	if !isBooleanLiteralKeyword(left) && !isBooleanLiteralKeyword(right) {
		return
	}
	// One side is a boolean literal — check the other side's type.
	other := left
	if isBooleanLiteralKeyword(left) {
		other = right
	}
	t := ctx.TypeOf(other)
	if t == nil {
		return
	}
	if !isStrictlyBoolean(t) {
		return
	}
	ctx.Report(n, "comparing a boolean to a boolean literal is redundant; use the value directly")
}

func isBooleanLiteralKeyword(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword:
		return true
	}
	return false
}

func nonBoolUnacceptable(ctx *engine.Context, other *wrapperchecker.Node) bool {
	t := ctx.TypeOf(other)
	return t == nil || !isStrictlyBoolean(t)
}

func isStrictlyBoolean(t *wrapperchecker.Type) bool {
	if t.IsBooleanLike() {
		return true
	}
	if !t.IsUnion() {
		return false
	}
	for _, m := range t.UnionMembers() {
		if !m.IsBooleanLike() {
			return false
		}
	}
	return true
}
