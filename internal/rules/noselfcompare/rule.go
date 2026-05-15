// Package noselfcompare implements the no-self-compare rule: flag
// comparisons where both operands are textually identical, e.g.
// `a === a`. The pattern is almost always a typo or refactoring
// leftover.
//
// This is jetlint's first non-type-aware rule. It uses [wrapperchecker.Node.SourceText]
// to compare operands by their source span; equal source means same
// expression. The comparison is conservative — `obj.x` and `obj .x`
// differ in whitespace and will not be flagged, and a string literal
// containing whitespace can produce a false positive (`"a" === "a "`
// is not really a self-compare but the operands' source spans share a
// non-trivial prefix). Both edges are uncommon enough to accept while
// keeping the rule simple; structural AST equality is a future
// refinement.
package noselfcompare

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-self-compare"

// New constructs a noselfcompare rule instance ready for registration
// with the engine.
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
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftText := left.SourceText()
	if leftText == "" || leftText != right.SourceText() {
		return
	}
	ctx.Report(n, "comparing to itself is potentially pointless")
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
