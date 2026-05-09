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
		wrapperchecker.KindCallExpression:   r.visitCall,
	}
}

// visitCall flags `/regex/.test(s)` and friends — a regex anchored at
// `^` (or `$`) is equivalent to `s.startsWith(literal)` /
// `s.endsWith(literal)` when the pattern contains no metacharacters
// other than the anchor.
func (r *rule) visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "test" {
		return
	}
	args := n.CallArguments()
	if len(args) != 1 {
		return
	}
	// `.test(arg)` coerces arg to string at runtime. We accept any
	// argument shape — the arg is always conceptually a string.
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	if regexNodeIsAnchored(ctx, recv) {
		ctx.Report(n, "use String.startsWith/endsWith instead of regex .test on an anchored pattern")
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	switch op {
	case wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken:
	default:
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	allowSingle := r.opts.AllowSingleElementEquality == "always"

	// Detect each known anti-pattern in either L/R order. The string-
	// receiving call (the receiver of `.charAt`, `.slice`, `.match`,
	// etc.) typically appears on one side and a needle on the other.
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
		return
	}
	if matchEndsWithCharAt(ctx, left, right) || matchEndsWithCharAt(ctx, right, left) {
		ctx.Report(n, "use String.endsWith instead of charAt(s.length-1) comparison")
		return
	}
	if matchEndsWithLastIndexOf(ctx, left, right) || matchEndsWithLastIndexOf(ctx, right, left) {
		ctx.Report(n, "use String.endsWith instead of lastIndexOf comparison")
		return
	}
	if matchSliceStartsWith(ctx, left, right) || matchSliceStartsWith(ctx, right, left) {
		ctx.Report(n, "use String.startsWith instead of slice/substring comparison")
		return
	}
	if matchSliceEndsWith(ctx, left, right) || matchSliceEndsWith(ctx, right, left) {
		ctx.Report(n, "use String.endsWith instead of slice/substring comparison")
		return
	}
	if matchMatchAnchored(ctx, left, right) || matchMatchAnchored(ctx, right, left) {
		ctx.Report(n, "use String.startsWith/endsWith instead of regex anchor with .match")
		return
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
	if rt == nil || !isStringLikeType(rt) {
		return false
	}
	// Other side: string literal, or string-typed expression.
	if other.Kind() == wrapperchecker.KindStringLiteral ||
		other.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return true
	}
	ot := ctx.TypeOf(other)
	return isStringLikeType(ot)
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
	if rt == nil || !isStringLikeType(rt) {
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
	return isStringLikeType(ctx.TypeOf(recv))
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
	if rt == nil || !isStringLikeType(rt) {
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
	return isStringLikeType(ot)
}

// matchEndsWithCharAt detects `s.charAt(s.length - 1) === 'a'`.
func matchEndsWithCharAt(ctx *engine.Context, call, other *wrapperchecker.Node) bool {
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
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	if rt := ctx.TypeOf(recv); rt == nil || !isStringLikeType(rt) {
		return false
	}
	if !isLengthMinusOne(args[0], recv) {
		return false
	}
	return isStringLikeOrLiteral(ctx, other)
}

// matchEndsWithLastIndexOf detects `s.lastIndexOf(needle) === s.length - needle.length`.
// `needle` may be a string literal or a string-typed expression.
func matchEndsWithLastIndexOf(ctx *engine.Context, call, other *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if callee.PropertyAccessName() != "lastIndexOf" {
		return false
	}
	args := call.CallArguments()
	if len(args) != 1 {
		return false
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	if rt := ctx.TypeOf(recv); rt == nil || !isStringLikeType(rt) {
		return false
	}
	needle := args[0]
	return isLengthMinusNeedleLength(other, recv, needle)
}

// matchSliceStartsWith detects `s.slice(0, N) === 'literal'` or
// `s.slice(0, needle.length) === needle`. Same for substring.
func matchSliceStartsWith(ctx *engine.Context, call, other *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	name := callee.PropertyAccessName()
	if name != "slice" && name != "substring" && name != "substr" {
		return false
	}
	args := call.CallArguments()
	if len(args) != 2 {
		return false
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	if rt := ctx.TypeOf(recv); rt == nil || !isStringLikeType(rt) {
		return false
	}
	if !isZeroLiteral(args[0]) {
		return false
	}
	// Length matches: literal-length-of-other or needle-length where
	// needle == other, or any value if `other` is the literal needle
	// whose length is the second argument.
	if isLiteralStringWithMatchingLength(other, args[1]) {
		return true
	}
	if isLengthOfNeedle(args[1], other) {
		return true
	}
	return false
}

// matchSliceEndsWith detects forms of `s.slice(...) === needle` that
// equal `s.endsWith(needle)`:
//   `s.slice(-N) === 'lit'` (N == 'lit'.length)
//   `s.slice(-needle.length) === needle`
//   `s.slice(s.length - needle.length) === needle`
//   `s.slice(s.length - N) === 'lit'` (N == 'lit'.length)
func matchSliceEndsWith(ctx *engine.Context, call, other *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	name := callee.PropertyAccessName()
	if name != "slice" && name != "substring" && name != "substr" {
		return false
	}
	args := call.CallArguments()
	if len(args) < 1 {
		return false
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	if rt := ctx.TypeOf(recv); rt == nil || !isStringLikeType(rt) {
		return false
	}
	first := args[0]
	if len(args) == 1 {
		if isNegatedLiteral(first, other) || isNegatedLengthOfNeedle(first, other) {
			return true
		}
		// `s.slice(s.length - N)` form
		if isReceiverLengthMinusOther(first, recv, other) {
			return true
		}
		return false
	}
	// Two-arg form: `s.slice(s.length - N, s.length) === 'lit'`.
	if !isLengthOfReceiver(args[1], recv) {
		return false
	}
	return isReceiverLengthMinusOther(first, recv, other)
}

// isReceiverLengthMinusOther reports whether `node` is `recv.length - X`
// where X is either a numeric literal equal to other.length (when
// other is a string literal) or `other.length` (when other is an
// identifier / a string literal whose .length matches).
func isReceiverLengthMinusOther(node, recv, other *wrapperchecker.Node) bool {
	if node == nil || node.Kind() != wrapperchecker.KindBinaryExpression ||
		node.BinaryOperatorKind() != wrapperchecker.KindMinusToken {
		return false
	}
	left := node.BinaryLeft()
	right := node.BinaryRight()
	if !isLengthOfReceiver(left, recv) {
		return false
	}
	if other == nil {
		return false
	}
	if other.Kind() == wrapperchecker.KindStringLiteral ||
		other.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		want := len([]rune(other.LiteralText()))
		if right.Kind() == wrapperchecker.KindNumericLiteral &&
			right.LiteralText() == itoa(want) {
			return true
		}
	}
	if right.Kind() == wrapperchecker.KindPropertyAccessExpression &&
		right.PropertyAccessName() == "length" {
		nr := right.PropertyAccessReceiver()
		if sameIdentifier(nr, other) {
			return true
		}
	}
	return false
}

// matchMatchAnchored detects `s.match(/^bar/) !== null` (startsWith)
// and `s.match(/bar$/) !== null` (endsWith). Also accepts a const-bound
// regex literal: `const p = /^bar/; s.match(p) != null`.
func matchMatchAnchored(ctx *engine.Context, call, other *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if callee.PropertyAccessName() != "match" {
		return false
	}
	args := call.CallArguments()
	if len(args) != 1 {
		return false
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return false
	}
	if rt := ctx.TypeOf(recv); rt == nil || !isStringLikeType(rt) {
		return false
	}
	if !isNullKeyword(other) {
		return false
	}
	return regexNodeIsAnchored(ctx, args[0])
}

// --- low-level pattern helpers ---

func isLengthMinusOne(node, recv *wrapperchecker.Node) bool {
	if node == nil || node.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if node.BinaryOperatorKind() != wrapperchecker.KindMinusToken {
		return false
	}
	left := node.BinaryLeft()
	right := node.BinaryRight()
	if left == nil || right == nil {
		return false
	}
	if right.Kind() != wrapperchecker.KindNumericLiteral || right.LiteralText() != "1" {
		return false
	}
	return isLengthOfReceiver(left, recv)
}

func isLengthOfReceiver(node, recv *wrapperchecker.Node) bool {
	if node == nil || node.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if node.PropertyAccessName() != "length" {
		return false
	}
	r := node.PropertyAccessReceiver()
	return sameIdentifier(r, recv)
}

func isLengthMinusNeedleLength(other, recv, needle *wrapperchecker.Node) bool {
	if other == nil || other.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if other.BinaryOperatorKind() != wrapperchecker.KindMinusToken {
		return false
	}
	left := other.BinaryLeft()
	right := other.BinaryRight()
	if !isLengthOfReceiver(left, recv) {
		return false
	}
	if needle.Kind() == wrapperchecker.KindStringLiteral ||
		needle.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		want := len([]rune(needle.LiteralText()))
		if right.Kind() == wrapperchecker.KindNumericLiteral &&
			right.LiteralText() == itoa(want) {
			return true
		}
	}
	// Or: right is `needle.length` (Identifier receiver) or
	// `'lit'.length` (StringLiteral receiver matching the same literal).
	if right.Kind() == wrapperchecker.KindPropertyAccessExpression &&
		right.PropertyAccessName() == "length" {
		nr := right.PropertyAccessReceiver()
		if sameIdentifier(nr, needle) {
			return true
		}
		if nr != nil && (nr.Kind() == wrapperchecker.KindStringLiteral ||
			nr.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral) &&
			(needle.Kind() == wrapperchecker.KindStringLiteral ||
				needle.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral) &&
			nr.LiteralText() == needle.LiteralText() {
			return true
		}
	}
	return false
}

func isZeroLiteral(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindNumericLiteral && n.LiteralText() == "0"
}

func isLiteralStringWithMatchingLength(other, lengthArg *wrapperchecker.Node) bool {
	if other == nil {
		return false
	}
	if other.Kind() != wrapperchecker.KindStringLiteral &&
		other.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return false
	}
	want := len([]rune(other.LiteralText()))
	return lengthArg.Kind() == wrapperchecker.KindNumericLiteral &&
		lengthArg.LiteralText() == itoa(want)
}

// isLengthOfNeedle reports whether `lengthArg` is `needle.length` AND
// `needle` is the `other` operand (so the whole expression compares
// `s.slice(0, needle.length) === needle`).
func isLengthOfNeedle(lengthArg, needle *wrapperchecker.Node) bool {
	if lengthArg == nil || lengthArg.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	if lengthArg.PropertyAccessName() != "length" {
		return false
	}
	r := lengthArg.PropertyAccessReceiver()
	return sameIdentifier(r, needle)
}

func isNegatedLiteral(arg, other *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindPrefixUnaryExpression {
		return false
	}
	// Need access to operand — use FirstChild (operator + operand).
	op := arg.FirstChild()
	if op == nil {
		return false
	}
	// The operator is the first token; we want the operand. Walk
	// children: operator child first, then operand.
	var operand *wrapperchecker.Node
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		operand = c
		return false
	})
	if operand == nil {
		return false
	}
	if operand.Kind() != wrapperchecker.KindNumericLiteral {
		return false
	}
	if other == nil {
		return false
	}
	if other.Kind() != wrapperchecker.KindStringLiteral &&
		other.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return false
	}
	want := len([]rune(other.LiteralText()))
	return operand.LiteralText() == itoa(want)
}

func isNegatedLengthOfNeedle(arg, other *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindPrefixUnaryExpression {
		return false
	}
	var operand *wrapperchecker.Node
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		operand = c
		return false
	})
	if operand == nil {
		return false
	}
	return isLengthOfNeedle(operand, other)
}

func isStringLikeOrLiteral(ctx *engine.Context, n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindStringLiteral ||
		n.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return true
	}
	return isStringLikeType(ctx.TypeOf(n))
}

// isStringLikeType returns true for string, string-literal, template-
// literal types, and unions/intersections whose every constituent is
// string-like (so `'a' | 'b'`, `T extends 'a' | 'b'` after constraint
// resolution, and `string & {__brand}` all qualify).
func isStringLikeType(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsStringLike() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isStringLikeType(m) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isStringLikeType(m) {
				return true
			}
		}
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isStringLikeType(c)
		}
	}
	return false
}

func isNullKeyword(n *wrapperchecker.Node) bool {
	return n != nil && n.Kind() == wrapperchecker.KindNullKeyword
}

// regexNodeIsAnchored returns true when the node is a regex literal
// or const-bound regex whose pattern is a single anchored literal —
// `^foo` or `bar$` — with no metacharacters that change the match
// surface. A full-string anchor (`^foo$`) isn't equivalent to either
// startsWith or endsWith and is rejected.
func regexNodeIsAnchored(ctx *engine.Context, n *wrapperchecker.Node) bool {
	src, ok := regexSourceOf(ctx, n)
	if !ok || src == "" {
		return false
	}
	startsAnchored := src[0] == '^'
	endsAnchored := false
	if last := len(src) - 1; last >= 0 && src[last] == '$' {
		bs := 0
		for i := last - 1; i >= 0 && src[i] == '\\'; i-- {
			bs++
		}
		if bs%2 == 0 {
			endsAnchored = true
		}
	}
	if startsAnchored == endsAnchored {
		return false
	}
	// Strip the anchor and verify the remainder is a literal sub-
	// string with no regex metacharacters.
	body := src
	if startsAnchored {
		body = body[1:]
	}
	if endsAnchored {
		body = body[:len(body)-1]
	}
	return isLiteralRegexBody(body)
}

// isLiteralRegexBody reports whether a regex pattern body has no
// metacharacters (other than `\` escapes for already-literal chars
// like `\.`). Returns false on alternation, character classes,
// quantifiers, groups, anchors, or any escape that introduces a
// non-literal match (`\d`, `\w`, etc.).
func isLiteralRegexBody(src string) bool {
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '\\' {
			if i+1 >= len(src) {
				return false
			}
			next := src[i+1]
			// Allow common literal escapes.
			switch next {
			case '.', '\\', '/', '"', '\'', '?', '*', '+', '|',
				'(', ')', '[', ']', '{', '}', '^', '$':
			default:
				return false
			}
			i++ // skip the escaped char
			continue
		}
		switch c {
		case '.', '|', '?', '*', '+', '(', ')', '[', ']', '{', '}', '^', '$':
			return false
		}
	}
	return true
}

// regexSourceOf returns the regex source string when the node refers
// to a regex literal or a const variable bound to one.
func regexSourceOf(ctx *engine.Context, n *wrapperchecker.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	if n.Kind() == wrapperchecker.KindRegularExpressionLiteral {
		txt := n.LiteralText()
		// Trim leading `/` and trailing `/<flags>`.
		if len(txt) < 2 || txt[0] != '/' {
			return "", false
		}
		closeAt := -1
		for i := len(txt) - 1; i > 0; i-- {
			if txt[i] == '/' {
				closeAt = i
				break
			}
		}
		if closeAt <= 0 {
			return "", false
		}
		return txt[1:closeAt], true
	}
	// Identifier referring to a const regex / new RegExp(...).
	if n.Kind() == wrapperchecker.KindIdentifier {
		sym := ctx.Checker().SymbolOf(n)
		if sym == nil {
			return "", false
		}
		val := sym.SymbolValueDeclaration()
		if val == nil || val.Kind() != wrapperchecker.KindVariableDeclaration {
			return "", false
		}
		init := val.VariableDeclarationInitializer()
		if init == nil {
			return "", false
		}
		return regexSourceOf(ctx, init)
	}
	if n.Kind() == wrapperchecker.KindNewExpression {
		callee := n.CalleeExpression()
		if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier ||
			callee.LiteralText() != "RegExp" {
			return "", false
		}
		args := n.CallArguments()
		if len(args) == 0 {
			return "", false
		}
		first := args[0]
		if first.Kind() != wrapperchecker.KindStringLiteral &&
			first.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
			return "", false
		}
		return first.LiteralText(), true
	}
	return "", false
}

// containsTopLevelAlternation reports whether the regex pattern has a
// `|` at top level (not inside `[]` or `()`). When alternation crosses
// the anchor, the `^`/`$` semantics aren't equivalent to startsWith/
// endsWith.
func containsTopLevelAlternation(src string) bool {
	depthGroup := 0
	depthClass := 0
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch c {
		case '\\':
			i++ // skip escape
		case '[':
			depthClass++
		case ']':
			if depthClass > 0 {
				depthClass--
			}
		case '(':
			if depthClass == 0 {
				depthGroup++
			}
		case ')':
			if depthClass == 0 && depthGroup > 0 {
				depthGroup--
			}
		case '|':
			if depthGroup == 0 && depthClass == 0 {
				return true
			}
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
