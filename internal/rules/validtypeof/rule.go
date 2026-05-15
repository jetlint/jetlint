// Package validtypeof implements the valid-typeof rule: flag
// `typeof x === "stirng"` and similar typos where the string operand
// is not one of the eight values JavaScript's typeof can actually
// return. The literal-equality framing covers `==`, `!=`, `===`, `!==`;
// relational operators against typeof are out of scope (they are
// nonsensical but produce a runtime error rather than a silent bug,
// so they belong to a different rule).
//
// String operands accept both regular string literals and template
// literals without expressions. When the non-typeof side is anything
// other than a constant string (a variable, a function call), the rule
// stays silent — the analysis is purely syntactic and refuses to guess.
package validtypeof

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "valid-typeof"

// New constructs a validtypeof rule instance ready for registration
// with the engine.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

// validTypeofResults is the closed set of strings `typeof` may return
// at runtime. Comparing against any other literal is a typo.
var validTypeofResults = map[string]bool{
	"undefined": true,
	"object":    true,
	"boolean":   true,
	"number":    true,
	"string":    true,
	"function":  true,
	"symbol":    true,
	"bigint":    true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isEqualityOperator(n.BinaryOperatorKind()) {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	// Check both sides: a typeof X on one side and a string literal on
	// the other. We emit independently for each side that matches —
	// `"strnig" === typeof x` and `typeof x === "strnig"` are both bugs,
	// and `"strnig" === typeof x === "boguss"` would be unusual but is
	// not reachable since this is a single BinaryExpression.
	checkSide(ctx, left, right)
	checkSide(ctx, right, left)
}

// checkSide reports a diagnostic when typeofSide is `typeof X` and
// literalSide is a string literal whose value is not a valid typeof
// result.
func checkSide(ctx *engine.Context, typeofSide, literalSide *wrapperchecker.Node) {
	if !isTypeofExpression(typeofSide) {
		return
	}
	lit, ok := stringLiteralValue(literalSide)
	if !ok {
		return
	}
	if validTypeofResults[lit] {
		return
	}
	ctx.Report(literalSide, "invalid typeof comparison value: "+lit+" is not a typeof result")
}

func isEqualityOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken:
		return true
	}
	return false
}

func isTypeofExpression(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	return n.Kind() == wrapperchecker.KindTypeOfExpression
}

// stringLiteralValue returns the value of n if n is a plain string
// literal or a template literal without substitutions. The second
// return is false when n is anything else (a variable, an expression,
// a template with `${}` interpolation).
func stringLiteralValue(n *wrapperchecker.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText(), true
	}
	return "", false
}
