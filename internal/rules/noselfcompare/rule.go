// Package noselfcompare implements the no-self-compare rule: flag
// comparisons where both operands are structurally identical, e.g.
// `a === a`. The pattern is almost always a typo or refactoring
// leftover.
//
// Structural equality ignores parentheses and whitespace and compares
// AST shape: same node kinds in the same positions, with matching
// literal text at the leaves. This matches oxlint's
// `without_parentheses().content_eq()` approach, so `(x) == x` and
// `foo.bar ().baz .qux >= foo.bar().baz.qux` are both flagged while
// `"a" === "a "` (different string contents) is not.
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
	if !structurallyEqual(unwrap(left), unwrap(right)) {
		return
	}
	ctx.Report(n, "comparing to itself is potentially pointless")
}

// unwrap peels off parenthesized wrappers so `(x)` and `x` compare
// equal. Returns nil only when n itself is nil.
func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// structurallyEqual returns true when a and b are the same AST shape:
// identical Kind at every level, with matching LiteralText at the
// leaves. Parentheses inside the subtrees are skipped via unwrap on
// recursive descent.
func structurallyEqual(a, b *wrapperchecker.Node) bool {
	a = unwrap(a)
	b = unwrap(b)
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind() != b.Kind() {
		return false
	}
	if isTextLeaf(a.Kind()) && a.LiteralText() != b.LiteralText() {
		return false
	}
	var aKids, bKids []*wrapperchecker.Node
	a.ForEachChild(func(c *wrapperchecker.Node) bool { aKids = append(aKids, c); return false })
	b.ForEachChild(func(c *wrapperchecker.Node) bool { bKids = append(bKids, c); return false })
	if len(aKids) != len(bKids) {
		return false
	}
	for i := range aKids {
		if !structurallyEqual(aKids[i], bKids[i]) {
			return false
		}
	}
	return true
}

// isTextLeaf reports whether LiteralText() is safe to call on the
// given Kind — TypeScript-Go's Node.Text panics on non-leaf nodes.
func isTextLeaf(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindPrivateIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTemplateHead,
		wrapperchecker.KindTemplateMiddle,
		wrapperchecker.KindTemplateTail,
		wrapperchecker.KindRegularExpressionLiteral:
		return true
	}
	return false
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
