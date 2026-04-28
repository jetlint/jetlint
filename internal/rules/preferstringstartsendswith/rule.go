// Package preferstringstartsendswith implements the
// prefer-string-starts-ends-with rule: flag patterns equivalent to
// startsWith/endsWith that should use the dedicated method.
package preferstringstartsendswith

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-string-starts-ends-with"

// Options is the configurable surface of the rule.
type Options struct {
	// AllowSingleElementEquality, when "always", suppresses flags for
	// `s[0] === 'a'` and `s[s.length - 1] === 'a'` — single-character
	// equality is sometimes preferable for readability or perf.
	AllowSingleElementEquality string
}

func DefaultOptions() Options                 { return Options{AllowSingleElementEquality: "never"} }
func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
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
	allowSingle := r.opts.AllowSingleElementEquality == "always"
	if !allowSingle && (matchStartsWith(ctx, left, right) || matchStartsWith(ctx, right, left)) {
		ctx.Report(n, "use String.startsWith instead of comparing the first character")
		return
	}
	if matchCharAt(ctx, left, right) || matchCharAt(ctx, right, left) {
		ctx.Report(n, "use String.startsWith instead of charAt(0) comparison")
		return
	}
	if matchIndexOfZero(ctx, left, right) || matchIndexOfZero(ctx, right, left) {
		ctx.Report(n, "use String.startsWith instead of indexOf-against-zero")
		return
	}
	if !allowSingle && (matchEndsWith(ctx, left, right) || matchEndsWith(ctx, right, left)) {
		ctx.Report(n, "use String.endsWith instead of comparing the last character")
	}
}

// matchCharAt reports whether call/literal form `s.charAt(0) === 'x'`
// (or any equality op) where s is string-typed.
func matchCharAt(ctx *engine.Context, call, other *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if callee.PropertyAccessName() != "charAt" {
		return false
	}
	args := call.CallArguments()
	if len(args) != 1 {
		return false
	}
	if args[0].Kind() != wrapperchecker.KindNumericLiteral || args[0].LiteralText() != "0" {
		return false
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !rt.IsStringLike() {
		return false
	}
	// Other side: string literal, or string-typed expression.
	if other.Kind() == wrapperchecker.KindStringLiteral ||
		other.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return true
	}
	ot := ctx.TypeOf(other)
	return ot != nil && ot.IsStringLike()
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
	switch literal.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		// Must be exactly one character to map to startsWith.
		return len([]rune(literal.LiteralText())) == 1
	}
	// Non-literal RHS is allowed when its type is string-like — `s[0] === t`
	// where t: string is still equivalent to `s.startsWith(t)` when t
	// has length 1, and the rule prefers the explicit method either way.
	ot := ctx.TypeOf(literal)
	return ot != nil && ot.IsStringLike()
}
