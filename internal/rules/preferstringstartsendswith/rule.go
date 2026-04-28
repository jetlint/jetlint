// Package preferstringstartsendswith implements the
// prefer-string-starts-ends-with rule: flag patterns equivalent to
// startsWith/endsWith that should use the dedicated method.
package preferstringstartsendswith

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-string-starts-ends-with"

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
	if op != wrapperchecker.KindEqualsEqualsEqualsToken &&
		op != wrapperchecker.KindExclamationEqualsEqualsToken &&
		op != wrapperchecker.KindEqualsEqualsToken &&
		op != wrapperchecker.KindExclamationEqualsToken {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	if matchStartsWith(ctx, left, right) || matchStartsWith(ctx, right, left) {
		ctx.Report(n, "use String.startsWith instead of comparing the first character")
		return
	}
	if matchIndexOfZero(ctx, left, right) || matchIndexOfZero(ctx, right, left) {
		ctx.Report(n, "use String.startsWith instead of indexOf-against-zero")
		return
	}
	if matchEndsWith(ctx, left, right) || matchEndsWith(ctx, right, left) {
		ctx.Report(n, "use String.endsWith instead of comparing the last character")
	}
}

// matchEndsWith reports whether (access, literal) form
// `s[s.length - N] === '...'` where the literal is a string of length N.
// Currently only the N=1 case is detected.
func matchEndsWith(ctx *engine.Context, access, literal *wrapperchecker.Node) bool {
	if access.Kind() != wrapperchecker.KindElementAccessExpression {
		return false
	}
	if literal.Kind() != wrapperchecker.KindStringLiteral &&
		literal.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return false
	}
	if len([]rune(literal.LiteralText())) != 1 {
		return false
	}
	recv := access.ElementAccessReceiver()
	if recv == nil {
		return false
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !rt.IsStringLike() {
		return false
	}
	idx := access.ElementAccessIndex()
	if idx == nil || idx.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if idx.BinaryOperatorKind() != wrapperchecker.KindMinusToken {
		return false
	}
	idxLeft := idx.BinaryLeft()
	idxRight := idx.BinaryRight()
	if idxLeft == nil || idxRight == nil {
		return false
	}
	if idxRight.Kind() != wrapperchecker.KindNumericLiteral || idxRight.LiteralText() != "1" {
		return false
	}
	if idxLeft.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if idxLeft.PropertyAccessName() != "length" {
		return false
	}
	// Receiver of length must be the same identifier as the access
	// receiver.
	lengthRecv := idxLeft.PropertyAccessReceiver()
	if lengthRecv == nil {
		return false
	}
	return sameIdentifier(recv, lengthRecv)
}

func sameIdentifier(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Kind() != wrapperchecker.KindIdentifier || b.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return a.LiteralText() == b.LiteralText()
}

// matchIndexOfZero reports whether call/literal form `s.indexOf(x) === 0`
// where s is string-typed. The literal must be the number 0.
func matchIndexOfZero(ctx *engine.Context, call, zero *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	if zero.Kind() != wrapperchecker.KindNumericLiteral || zero.LiteralText() != "0" {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if callee.PropertyAccessName() != "indexOf" {
		return false
	}
	if len(call.CallArguments()) != 1 {
		return false
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	rt := ctx.TypeOf(recv)
	return rt != nil && rt.IsStringLike()
}

func matchStartsWith(ctx *engine.Context, indexAccess, literal *wrapperchecker.Node) bool {
	if indexAccess.Kind() != wrapperchecker.KindElementAccessExpression {
		return false
	}
	idx := indexAccess.ElementAccessIndex()
	if idx == nil {
		return false
	}
	if idx.Kind() != wrapperchecker.KindNumericLiteral || idx.LiteralText() != "0" {
		return false
	}
	recv := indexAccess.ElementAccessReceiver()
	if recv == nil {
		return false
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !rt.IsStringLike() {
		return false
	}
	if literal.Kind() != wrapperchecker.KindStringLiteral &&
		literal.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return false
	}
	// Must be exactly one character to map to startsWith.
	text := literal.LiteralText()
	if len([]rune(text)) != 1 {
		return false
	}
	return true
}
