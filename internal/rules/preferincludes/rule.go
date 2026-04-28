// Package preferincludes implements the prefer-includes rule: flag
// `x.indexOf(y) !== -1` (and equivalent shapes) in favor of
// `x.includes(y)`.
package preferincludes

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-includes"

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
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	// Match the comparison shape: `x.indexOf(y) <op> <constant>` where
	// the operator and constant together mean "found at least once".
	indexOfCall := left
	other := right
	if !isComparisonShape(op, indexOfCall, other) {
		// Try the other side: `<const> <op> x.indexOf(y)`. Only the
		// operators we treat as commutative go here.
		indexOfCall = right
		other = left
		if !isComparisonShape(reverseOp(op), indexOfCall, other) {
			return
		}
	}
	if !isFindIndexCall(indexOfCall) {
		return
	}
	recv := indexOfCall.CalleeExpression().PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !typeHasIncludesMethod(rt) {
		return
	}
	ctx.Report(n, "use .includes() instead of .indexOf() comparison")
}

func reverseOp(op wrapperchecker.Kind) wrapperchecker.Kind {
	switch op {
	case wrapperchecker.KindLessThanToken:
		return wrapperchecker.KindGreaterThanToken
	case wrapperchecker.KindGreaterThanToken:
		return wrapperchecker.KindLessThanToken
	case wrapperchecker.KindLessThanEqualsToken:
		return wrapperchecker.KindGreaterThanEqualsToken
	case wrapperchecker.KindGreaterThanEqualsToken:
		return wrapperchecker.KindLessThanEqualsToken
	}
	return op
}

// isComparisonShape reports whether call/other form one of the
// "found at least once" shapes:
//
//	indexOf !== -1 / != -1 / > -1 / >= 0
func isComparisonShape(op wrapperchecker.Kind, call, other *wrapperchecker.Node) bool {
	if !isFindIndexCall(call) {
		return false
	}
	switch op {
	case wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindGreaterThanToken:
		return isNumericLiteral(other, "-1")
	case wrapperchecker.KindGreaterThanEqualsToken:
		return isNumericLiteral(other, "0")
	}
	return false
}

func isFindIndexCall(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	return callee.PropertyAccessName() == "indexOf"
}

func isNumericLiteral(n *wrapperchecker.Node, want string) bool {
	if n == nil {
		return false
	}
	// `-1` parses as a prefix-unary minus on `1`.
	if n.Kind() == wrapperchecker.KindPrefixUnaryExpression && n.PrefixUnaryOperator() == "-" {
		inner := n.FirstChild()
		if inner != nil && inner.Kind() == wrapperchecker.KindNumericLiteral {
			return "-"+inner.LiteralText() == want
		}
		return false
	}
	if n.Kind() == wrapperchecker.KindNumericLiteral {
		return n.LiteralText() == want
	}
	return false
}

// typeHasIncludesMethod reports whether t carries a usable `includes`
// method. The rule's autofix uses it as the replacement, so the type
// must provide it.
func typeHasIncludesMethod(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	return t.PropertyType("includes") != nil
}
