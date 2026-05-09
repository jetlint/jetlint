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
	if atRecv == nil || atRecv.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	if !isFilterCall(atRecv) {
		return
	}
	filterRecv := memberReceiver(atRecv.CalleeExpression())
	if filterRecv == nil {
		return
	}
	rt := ctx.TypeOf(filterRecv)
	if !typeIsFilterable(rt) {
		return
	}
	ctx.Report(n, "use .find() instead of .filter().at(0)")
}

// typeIsFilterable reports whether t represents an array-like type,
// possibly hiding inside a nullable union (`T[] | undefined`) — the
// rule still suggests `.find()` for those.
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
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() {
			continue
		}
		if m.ArrayElementType() != nil || m.IsArrayLikeType() || m.IsTupleType() {
			return true
		}
	}
	return false
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

// commaRhs returns the rightmost operand of a chained comma expression
// — `(undefined, x)` is `x` for the purposes of further analysis.
func commaRhs(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindBinaryExpression && n.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
		n = n.BinaryRight()
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
	recv := commaRhs(unparenNode(n.ElementAccessReceiver()))
	if recv == nil || recv.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	if !isFilterCall(recv) {
		return
	}
	filterRecv := memberReceiver(recv.CalleeExpression())
	if filterRecv == nil {
		return
	}
	rt := ctx.TypeOf(filterRecv)
	if !typeIsFilterable(rt) {
		return
	}
	ctx.Report(n, "use .find() instead of .filter()[0]")
}

func isZero(ctx *engine.Context, n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral:
		if n.LiteralText() == "0" {
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
	case wrapperchecker.KindPrefixUnaryExpression:
		// `-0` and `-0n` are still zero.
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
	if v, ok := t.NumericLiteralValue(); ok && v == 0 {
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
