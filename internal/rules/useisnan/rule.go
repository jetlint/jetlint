// Package useisnan implements the use-isnan rule: flag comparisons
// against NaN, because such comparisons never produce the result the
// author expected. `NaN` is not equal to itself, and `<` / `>` against
// NaN is always false. The correct check is `Number.isNaN(x)`.
//
// The rule matches the bare `NaN` identifier and the `Number.NaN`
// property access on either side of any of the eight comparison
// operators. Parenthesized operands are unwrapped.
//
// Scope: this is the first non-type-aware rule that walks beyond a
// single expression node; it also serves as a pattern reference for
// operand-unwrapping helpers and PropertyAccessExpression matching.
//
// Not in scope for v1 (acceptable misses):
//   - Local variable shadowing the global `NaN` identifier. Users who
//     shadow `NaN` have bigger problems; the rule treats `NaN` as the
//     name regardless of binding.
//   - `enforceForSwitchCase` / `enforceForIndexOf` options from the
//     ESLint version. Both can be added later under rule options when
//     a real user asks for them.
package useisnan

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-isnan"

// New constructs a useisnan rule instance ready for registration with
// the engine.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isComparisonOperator(n.BinaryOperatorKind()) {
		return
	}
	if !isNaN(n.BinaryLeft()) && !isNaN(n.BinaryRight()) {
		return
	}
	ctx.Report(n, "use Number.isNaN() instead of comparing with NaN")
}

func isComparisonOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken,
		wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken:
		return true
	}
	return false
}

// isNaN reports whether n syntactically refers to the global NaN value.
// Recognizes the bare `NaN` identifier and the `Number.NaN` property
// access. Parentheses are unwrapped first.
func isNaN(n *wrapperchecker.Node) bool {
	n = stripParens(n)
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindIdentifier {
		return n.LiteralText() == "NaN"
	}
	if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
		recv := n.PropertyAccessReceiver()
		if recv == nil {
			return false
		}
		recv = stripParens(recv)
		if recv.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		return recv.LiteralText() == "Number" && n.PropertyAccessName() == "NaN"
	}
	return false
}

// stripParens unwraps a chain of parenthesized expressions, returning
// the first non-parenthesized inner node.
func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
