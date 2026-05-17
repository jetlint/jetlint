// Package nomisrefactoredshorthandassign implements
// no-misrefactored-shorthand-assign: `a += a + b` repeats `a` and is
// usually a refactor mistake — drop the second `a`.
package nomisrefactoredshorthandassign

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-misrefactored-shorthand-assign"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

// underlying maps the compound-assign operator to its arithmetic
// operator and whether that operator is commutative.
func underlying(op wrapperchecker.Kind) (string, bool, bool) {
	switch op {
	case wrapperchecker.KindPlusEqualsToken:
		// `+` doubles as string concat — only flag the LHS duplicate.
		return "+", false, true
	case wrapperchecker.KindMinusEqualsToken:
		return "-", false, true
	case wrapperchecker.KindAsteriskEqualsToken:
		return "*", true, true
	case wrapperchecker.KindSlashEqualsToken:
		return "/", false, true
	case wrapperchecker.KindPercentEqualsToken:
		return "%", false, true
	case wrapperchecker.KindAsteriskAsteriskEqualsToken:
		return "**", false, true
	case wrapperchecker.KindLessThanLessThanEqualsToken:
		return "<<", false, true
	case wrapperchecker.KindGreaterThanGreaterThanEqualsToken:
		return ">>", false, true
	case wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken:
		return ">>>", false, true
	case wrapperchecker.KindAmpersandEqualsToken:
		return "&", true, true
	case wrapperchecker.KindBarEqualsToken:
		return "|", true, true
	case wrapperchecker.KindCaretEqualsToken:
		return "^", true, true
	}
	return "", false, false
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op, commutative, ok := underlying(n.BinaryOperatorKind())
	if !ok {
		return
	}
	left := n.BinaryLeft()
	right := stripParens(n.BinaryRight())
	if left == nil || right == nil {
		return
	}
	if right.Kind() != wrapperchecker.KindBinaryExpression {
		return
	}
	// Confirm the right is using the underlying op via source text.
	rsrc := strings.TrimSpace(right.SourceText())
	leftSrc := strings.TrimSpace(left.SourceText())
	rl := stripParens(right.BinaryLeft())
	rr := stripParens(right.BinaryRight())
	if rl == nil || rr == nil {
		return
	}
	// Check op text between rl.End() and rr.Pos().
	mid := strings.TrimSpace(rsrc[rl.End()-right.Pos() : rr.Pos()-right.Pos()])
	if mid != op {
		return
	}
	rlSrc := strings.TrimSpace(rl.SourceText())
	rrSrc := strings.TrimSpace(rr.SourceText())
	if leftSrc == rlSrc {
		ctx.Report(n, "shorthand assignment repeats its left-hand side")
		return
	}
	if commutative && leftSrc == rrSrc {
		ctx.Report(n, "shorthand assignment repeats its left-hand side")
	}
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		n = inner
	}
	return n
}
