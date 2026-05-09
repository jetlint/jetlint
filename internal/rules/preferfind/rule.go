// Package preferfind implements the prefer-find rule: flag
// `arr.filter(p)[0]` in favor of `arr.find(p)`.
package preferfind

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-find"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindElementAccessExpression: visit,
		wrapperchecker.KindCallExpression:          visitCall,
	}
}

// visitCall handles `arr.filter(...).at(0)`.
func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.IsOptionalChainRoot() {
		return // .at?.(0) — don't flag.
	}
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	if callee.IsOptionalChainRoot() {
		return // ?.at(0) — don't flag.
	}
	if !isAtAccess(callee) {
		return
	}
	args := n.CallArguments()
	if len(args) != 1 {
		return
	}
	if !isZero(ctx, args[0]) {
		return
	}
	atRecv := commaRhs(unparenNode(memberReceiver(callee)))
	if !receiverIsAllFilterCalls(ctx, atRecv) {
		return
	}
	ctx.Report(n, "use .find() instead of .filter().at(0)")
}

// typeIsFilterable reports whether t represents an array-like type,
// possibly hiding inside a nullable union (`T[] | undefined`) — the
// rule still suggests `.find()` for those. Non-nullish constituents
// must all be array-like; a stray string or object member means the
// `.filter` call could mean something else, and we shouldn't suggest
// converting it.
func typeIsFilterable(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.ArrayElementType() != nil || t.IsArrayLikeType() || t.IsTupleType() {
		return true
	}
	if !t.IsUnion() {
		return false
	}
	any := false
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() {
			continue
		}
		if m.ArrayElementType() == nil && !m.IsArrayLikeType() && !m.IsTupleType() {
			return false
		}
		any = true
	}
	return any
}

// isAtAccess returns true when accessor is `.at` or `['at']` or
// `[`at`]` (template).
func isAtAccess(accessor *wrapperchecker.Node) bool {
	switch accessor.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return accessor.PropertyAccessName() == "at"
	case wrapperchecker.KindElementAccessExpression:
		idx := accessor.ElementAccessIndex()
		if idx == nil {
			return false
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText() == "at"
		}
	}
	return false
}

func isFilterCall(call *wrapperchecker.Node) bool {
	if call == nil {
		return false
	}
	c := call.CalleeExpression()
	if c == nil {
		return false
	}
	switch c.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return c.PropertyAccessName() == "filter"
	case wrapperchecker.KindElementAccessExpression:
		idx := c.ElementAccessIndex()
		if idx == nil {
			return false
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText() == "filter"
		}
	}
	return false
}

// memberReceiver returns the receiver of a property/element access,
// regardless of which form is used.
func memberReceiver(member *wrapperchecker.Node) *wrapperchecker.Node {
	if member == nil {
		return nil
	}
	switch member.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return member.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		return member.ElementAccessReceiver()
	}
	return nil
}

// commaRhs returns the rightmost operand of a comma chain, walking
// through any interleaved parentheses so deeply nested sequences like
// `(1, (2, x))` resolve to `x`.
func commaRhs(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil {
		if n.Kind() == wrapperchecker.KindParenthesizedExpression {
			n = n.FirstChild()
			continue
		}
		if n.Kind() == wrapperchecker.KindBinaryExpression && n.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
			n = n.BinaryRight()
			continue
		}
		return n
	}
	return n
}

// unparenNode strips ParenthesizedExpression wrappers so member-chain
// analysis can see through grouping parentheses.
func unparenNode(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.IsOptionalChainRoot() {
		return // ?.[0] — don't flag.
	}
	idx := n.ElementAccessIndex()
	if idx == nil {
		return
	}
	if !isZero(ctx, idx) {
		return
	}
	recv := n.ElementAccessReceiver()
	if !receiverIsAllFilterCalls(ctx, recv) {
		return
	}
	ctx.Report(n, "use .find() instead of .filter()[0]")
}

// receiverIsAllFilterCalls walks comma/parens/conditional wrappers and
// returns true when every reachable subject is a `.filter(...)` call
// (or `['filter'](...)`) on a filterable receiver. Conditional and
// nested expressions in the upstream fixtures often wrap a filter call
// behind ternaries — both arms have to qualify for the rewrite to be
// equivalent. Optional-chain roots inside the receiver (e.g.
// `arr.filter?.(...)[0]`) disqualify because converting them to
// `find()` changes when the call short-circuits.
func receiverIsAllFilterCalls(ctx *engine.Context, n *wrapperchecker.Node) bool {
	n = commaRhs(unparenNode(n))
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindConditionalExpression {
		whenTrue, whenFalse := n.ConditionalBranches()
		return receiverIsAllFilterCalls(ctx, whenTrue) && receiverIsAllFilterCalls(ctx, whenFalse)
	}
	if n.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	if n.IsOptionalChainRoot() {
		return false // arr.filter?.(...) — optional call, don't convert.
	}
	if !isFilterCall(n) {
		return false
	}
	callee := n.CalleeExpression()
	filterRecv := memberReceiver(callee)
	if filterRecv == nil {
		return false
	}
	rt := ctx.TypeOf(filterRecv)
	return typeIsFilterable(rt)
}

func isZero(ctx *engine.Context, n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral:
		if numericLiteralCoercesToZero(n.LiteralText()) {
			return true
		}
	case wrapperchecker.KindBigIntLiteral:
		t := n.LiteralText()
		if t == "0n" || t == "0" {
			return true
		}
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		// `arr.filter(...)['0']` indexes the same slot.
		if n.LiteralText() == "0" {
			return true
		}
	case wrapperchecker.KindIdentifier:
		// `NaN` coerces to 0 inside Array.prototype.at via ToInteger.
		if n.LiteralText() == "NaN" {
			return true
		}
	case wrapperchecker.KindPrefixUnaryExpression:
		// `-0`, `-0n`, and small negative fractionals (truncated to 0).
		if n.PrefixUnaryOperator() == "-" {
			if inner := n.FirstChild(); inner != nil {
				return isZero(ctx, inner)
			}
		}
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return false
	}
	if v, ok := t.NumericLiteralValue(); ok && coercedZero(v) {
		return true
	}
	if t.IsBigIntLike() {
		// 0n typed via const reference: `const zero = 0n; arr[zero]`.
		s := t.String()
		if s == "0n" || s == "-0n" {
			return true
		}
	}
	return false
}

// numericLiteralCoercesToZero reports whether a numeric literal's text
// would be turned into 0 by `Array.prototype.at`'s ToInteger coercion.
// Includes plain "0", "0.0", "0e0", and small fractionals strictly
// between -1 and 1 (which truncate to 0).
func numericLiteralCoercesToZero(s string) bool {
	if s == "" {
		return false
	}
	// Plain digit-zero, possibly with decimals all zero, or 0e+x.
	if s == "0" {
		return true
	}
	// Try parsing as float. Small fractionals that truncate to 0
	// satisfy `at` semantics.
	v, ok := parseFloat(s)
	return ok && coercedZero(v)
}

func coercedZero(v float64) bool {
	// NaN → 0 in ToInteger.
	if v != v {
		return true
	}
	// Truncate toward zero — anything in (-1, 1) becomes 0.
	if v > -1 && v < 1 {
		return true
	}
	return false
}

func parseFloat(s string) (float64, bool) {
	// Hand-rolled minimal parse to avoid pulling strconv into a hot
	// path; fall back to a slow zero-check for the common case.
	var v float64
	var seenDigit bool
	var dec int
	var sign float64 = 1
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		v = v*10 + float64(s[i]-'0')
		seenDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			v = v*10 + float64(s[i]-'0')
			dec++
			seenDigit = true
			i++
		}
	}
	if !seenDigit {
		return 0, false
	}
	for ; dec > 0; dec-- {
		v /= 10
	}
	return v * sign, true
}
