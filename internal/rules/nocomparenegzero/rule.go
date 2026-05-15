// Package nocomparenegzero implements the no-compare-neg-zero rule:
// comparing against `-0` with `===`/`==`/`<`/`>`/`<=`/`>=`/`!=`/`!==`
// does not distinguish `+0` from `-0`, so the author almost certainly
// meant `Object.is(x, -0)` (or `0` if they did not actually care about
// the sign).
package nocomparenegzero

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-compare-neg-zero"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op, ok := comparisonOperator(n.BinaryOperatorKind())
	if !ok {
		return
	}
	if !isNegativeZero(n.BinaryLeft()) && !isNegativeZero(n.BinaryRight()) {
		return
	}
	ctx.Report(n, fmt.Sprintf("Do not use the %s operator to compare against -0.", op))
}

func comparisonOperator(k wrapperchecker.Kind) (string, bool) {
	switch k {
	case wrapperchecker.KindEqualsEqualsEqualsToken:
		return "===", true
	case wrapperchecker.KindEqualsEqualsToken:
		return "==", true
	case wrapperchecker.KindExclamationEqualsEqualsToken:
		return "!==", true
	case wrapperchecker.KindExclamationEqualsToken:
		return "!=", true
	case wrapperchecker.KindLessThanToken:
		return "<", true
	case wrapperchecker.KindLessThanEqualsToken:
		return "<=", true
	case wrapperchecker.KindGreaterThanToken:
		return ">", true
	case wrapperchecker.KindGreaterThanEqualsToken:
		return ">=", true
	}
	return "", false
}

// isNegativeZero reports whether expr is the literal `-0` (or `-0n`),
// peeling through parentheses. Anything else — non-literal, `+0`,
// `-1`, a string — is not.
func isNegativeZero(expr *wrapperchecker.Node) bool {
	expr = stripParens(expr)
	if expr == nil || expr.Kind() != wrapperchecker.KindPrefixUnaryExpression {
		return false
	}
	if expr.PrefixUnaryOperator() != "-" {
		return false
	}
	operand := stripParens(expr.PrefixUnaryOperand())
	if operand == nil {
		return false
	}
	switch operand.Kind() {
	case wrapperchecker.KindNumericLiteral, wrapperchecker.KindBigIntLiteral:
		return literalIsZero(operand.LiteralText())
	}
	return false
}

// literalIsZero reports whether the source text of a numeric or BigInt
// literal denotes zero. Strips trailing `n` for BigInts; tolerates
// `0`, `0.0`, `0.0e10`, `0x0`, `0o0`, `0b0` — anything that parses to
// a magnitude of zero.
func literalIsZero(text string) bool {
	if text == "" {
		return false
	}
	if text[len(text)-1] == 'n' {
		text = text[:len(text)-1]
	}
	for _, ch := range text {
		switch ch {
		case '0', '.', 'x', 'X', 'o', 'O', 'b', 'B', 'e', 'E', '+', '-', '_':
			continue
		default:
			return false
		}
	}
	return true
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
