// Package noyodaexpression implements no-yoda-expression: `if (5 ==
// foo)` reads backwards. Comparisons are easier to scan when the
// variable comes first, leaving the literal as a stable anchor on
// the right.
package noyodaexpression

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-yoda-expression"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

var comparisonOps = map[string]bool{
	"==": true, "===": true, "!=": true, "!==": true,
	"<": true, "<=": true, ">": true, ">=": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := operatorToken(n)
	if !comparisonOps[op] {
		return
	}
	left := leftOperand(n)
	right := rightOperand(n)
	if !isLiteralLike(left) || isLiteralLike(right) {
		return
	}
	if isPartOfRange(n) {
		return
	}
	ctx.Report(n, "Yoda condition — put the variable on the left")
}

func leftOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return unwrap(first)
}

func rightOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var third *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 2 {
			third = c
			return true
		}
		idx++
		return false
	})
	return unwrap(third)
}

func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		n = first
	}
	return n
}

func operatorToken(n *wrapperchecker.Node) string {
	var second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 1 {
			second = c
			return true
		}
		idx++
		return false
	})
	if second == nil {
		return ""
	}
	return second.SourceText()
}

func isLiteralLike(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral, wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral, wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		return true
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		if op == "-" || op == "+" {
			if c := n.PrefixUnaryOperand(); c != nil {
				return c.Kind() == wrapperchecker.KindNumericLiteral || c.Kind() == wrapperchecker.KindBigIntLiteral
			}
		}
	}
	return false
}

// isPartOfRange returns true if `n` is one side of a range
// expression like `0 <= x && x < 10` (or with `||`). Heuristic:
// parent is a logical `&&`/`||` and the sibling comparison uses the
// same variable.
func isPartOfRange(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	op := operatorToken(p)
	if op != "&&" && op != "||" {
		return false
	}
	left := leftOperand(p)
	right := rightOperand(p)
	var sibling *wrapperchecker.Node
	if left == n {
		sibling = right
	} else {
		sibling = left
	}
	if sibling == nil || sibling.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if !comparisonOps[operatorToken(sibling)] {
		return false
	}
	// If the non-literal side of `n` appears anywhere in the sibling,
	// they're constraining the same value.
	ourVar := nonLiteralSide(n)
	if ourVar == "" {
		return false
	}
	return strings.Contains(sibling.SourceText(), ourVar)
}

func nonLiteralSide(n *wrapperchecker.Node) string {
	l := leftOperand(n)
	r := rightOperand(n)
	if isLiteralLike(l) && !isLiteralLike(r) {
		return r.SourceText()
	}
	if isLiteralLike(r) && !isLiteralLike(l) {
		return l.SourceText()
	}
	return ""
}
