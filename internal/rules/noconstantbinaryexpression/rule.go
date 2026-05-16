// Package noconstantbinaryexpression implements the
// no-constant-binary-expression rule. Logical and equality
// expressions whose result is statically known are flagged because
// they almost always indicate a bug: a programmer wrote a guard or
// comparison that cannot possibly behave the way they intended.
//
// The port covers the common patterns without a full constant-
// folding evaluator:
//
//   - `<truthy literal> && expr`  / `<falsy literal> && expr`
//   - `<truthy literal> || expr`  / `<falsy literal> || expr`
//   - `<nullish literal> ?? expr` / `<not-nullish> ?? expr`
//   - Strict and loose equality with a freshly constructed reference
//     literal (`{}`, `[]`, `() => {}`, `function(){}`, `class {}`,
//     `new Foo()`) or a numeric / boolean / null literal whose value
//     can't equal the other side.
//   - Assignment expressions (`x = ...`, `x += y`) used as a
//     `??` operand: assignment evaluates to the assigned value,
//     which is never nullish for arithmetic compound forms.
//
// More elaborate cases (Boolean(...), template literals, typeof
// chains) are out of scope; they need a richer "is this expression
// always X" utility that doesn't exist in jetlint yet.
package noconstantbinaryexpression

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-constant-binary-expression"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	left := unwrapParens(n.BinaryLeft())
	right := unwrapParens(n.BinaryRight())
	if left == nil || right == nil {
		return
	}
	switch op {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken:
		if hasConstantTruthiness(left) {
			ctx.Report(n, "Unexpected constant truthiness on the left-hand side of a logical operator.")
		}
	case wrapperchecker.KindQuestionQuestionToken:
		if hasConstantNullishness(left) {
			ctx.Report(n, "Unexpected constant nullishness on the left-hand side of a '??' expression.")
		}
	case wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken:
		strict := op == wrapperchecker.KindEqualsEqualsEqualsToken ||
			op == wrapperchecker.KindExclamationEqualsEqualsToken
		if isConstantEquality(left, right, strict) {
			ctx.Report(n, "Unexpected comparison to newly constructed value, which has a unique identity and never equals any reference.")
		}
	}
}

// unwrapParens strips ParenthesizedExpression wrappers so the analysis
// sees through `(...)`.
func unwrapParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		n = inner
	}
	return n
}

// hasConstantTruthiness reports whether n's truthiness can be
// determined statically.
func hasConstantTruthiness(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassExpression:
		return true
	case wrapperchecker.KindIdentifier:
		// `undefined` and `NaN` are global names without intrinsic
		// truthiness in the parser's eyes, but treating them is
		// risky (could be shadowed). Skip.
		return false
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		switch op {
		case "+", "-", "~":
			// `+x`, `-x`, `~x` coerce to a number; only a literal
			// operand yields a known result.
			return hasConstantTruthiness(n.PrefixUnaryOperand())
		case "!":
			// `!x` truthiness depends on x's truthiness — known only
			// when x's truthiness is known.
			return hasConstantTruthiness(n.PrefixUnaryOperand())
		case "void":
			return true // `void x` is always undefined => falsy.
		case "typeof":
			return true // `typeof x` is a non-empty string => truthy.
		}
	}
	return false
}

// hasConstantNullishness reports whether n's nullishness is statically
// determinable (always-nullish, or never-nullish).
func hasConstantNullishness(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTemplateExpression,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindNullKeyword:
		return true
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		switch op {
		case "+", "-", "~", "!", "typeof":
			return true
		case "void":
			return true
		case "++", "--":
			return true // prefix increment/decrement returns a number.
		}
	case wrapperchecker.KindPostfixUnaryExpression:
		return true // postfix ++/-- returns a number.
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		// Logical operators (&&, ||, ??) can produce a nullish
		// result; for the `??` form the comma operator passes
		// through; otherwise any binary operator produces a
		// primitive non-nullish value (number, string, boolean).
		if op == wrapperchecker.KindAmpersandAmpersandToken ||
			op == wrapperchecker.KindBarBarToken ||
			op == wrapperchecker.KindQuestionQuestionToken {
			return false
		}
		if op == wrapperchecker.KindCommaToken {
			return hasConstantNullishness(unwrapParens(n.BinaryRight()))
		}
		switch op {
		case wrapperchecker.KindEqualsToken,
			wrapperchecker.KindPlusEqualsToken,
			wrapperchecker.KindMinusEqualsToken,
			wrapperchecker.KindAsteriskEqualsToken,
			wrapperchecker.KindAsteriskAsteriskEqualsToken,
			wrapperchecker.KindSlashEqualsToken,
			wrapperchecker.KindPercentEqualsToken,
			wrapperchecker.KindLessThanLessThanEqualsToken,
			wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
			wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
			wrapperchecker.KindAmpersandEqualsToken,
			wrapperchecker.KindBarEqualsToken,
			wrapperchecker.KindCaretEqualsToken:
			// Compound arithmetic assignment evaluates to a number;
			// plain `=` to an always-nullishness-known RHS does too.
			if op == wrapperchecker.KindEqualsToken {
				return hasConstantNullishness(unwrapParens(n.BinaryRight()))
			}
			return true
		}
		// Any remaining binary operator (+, -, *, /, %, **, &, |,
		// ^, <<, >>, >>>, <, <=, >, >=, ==, !=, ===, !==, in,
		// instanceof) yields a non-nullish primitive.
		return true
	case wrapperchecker.KindDeleteExpression:
		return true // `delete x` is a boolean.
	}
	return false
}

// isConstantEquality reports whether a comparison of left and right
// (strict if `strict`, loose otherwise) is statically constant.
func isConstantEquality(left, right *wrapperchecker.Node, strict bool) bool {
	if isFreshReferenceLiteral(left) || isFreshReferenceLiteral(right) {
		return true
	}
	if strict {
		// Under ===, a numeric/boolean/null/undefined literal can be
		// compared to another with a known different type; covered
		// by hasConstantNullishness on each side combined with a
		// type-tag check.
		if hasIncompatibleTypeTag(left, right) {
			return true
		}
	}
	return false
}

// isFreshReferenceLiteral reports whether n is an expression that
// evaluates to a freshly constructed object/function/class/regex —
// these never strictly equal any value the user already has.
func isFreshReferenceLiteral(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindNewExpression:
		return true
	case wrapperchecker.KindBinaryExpression:
		if n.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
			return isFreshReferenceLiteral(unwrapParens(n.BinaryRight()))
		}
		if isAssignmentOperator(n.BinaryOperatorKind()) {
			return isFreshReferenceLiteral(unwrapParens(n.BinaryRight()))
		}
	case wrapperchecker.KindConditionalExpression:
		thn, els := conditionalBranches(n)
		return isFreshReferenceLiteral(unwrapParens(thn)) &&
			isFreshReferenceLiteral(unwrapParens(els))
	}
	return false
}

func conditionalBranches(cond *wrapperchecker.Node) (thn, els *wrapperchecker.Node) {
	// A ConditionalExpression has three children in order: condition,
	// whenTrue, whenFalse. ForEachChild walks them in syntactic order.
	var seen int
	cond.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch seen {
		case 1:
			thn = c
		case 2:
			els = c
		}
		seen++
		return false
	})
	return
}

func isAssignmentOperator(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

// hasIncompatibleTypeTag reports whether left and right have
// statically known primitive types that don't match — making `===`
// (or `!==`) constant.
func hasIncompatibleTypeTag(left, right *wrapperchecker.Node) bool {
	lt := primitiveTypeTag(left)
	rt := primitiveTypeTag(right)
	if lt == "" || rt == "" {
		return false
	}
	return lt != rt
}

// primitiveTypeTag returns a tag for n's primitive type when it is
// statically known: "number", "string", "boolean", "null",
// "undefined", "bigint", or "" when unknown.
func primitiveTypeTag(n *wrapperchecker.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case wrapperchecker.KindNullKeyword:
		return "null"
	case wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword:
		return "boolean"
	case wrapperchecker.KindNumericLiteral:
		return "number"
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return "string"
	case wrapperchecker.KindBigIntLiteral:
		return "bigint"
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		switch op {
		case "+", "-", "~", "++", "--":
			return "number"
		case "!":
			return "boolean"
		case "void":
			return "undefined"
		case "typeof":
			return "string"
		}
	case wrapperchecker.KindPostfixUnaryExpression:
		return "number"
	case wrapperchecker.KindIdentifier:
		if n.SourceText() == "undefined" {
			return "undefined"
		}
		if n.SourceText() == "NaN" {
			return "number"
		}
	}
	return ""
}
