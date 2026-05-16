// Package noconstantcondition implements the no-constant-condition
// rule: a literal-only test in an `if`, `while`, `do-while`, `for`,
// or ternary either always passes or always fails, which is almost
// always a typo or unfinished development scaffolding.
//
// This is a deliberately conservative port — it flags the cases ESLint
// flags when the test is built only from values that the parser can
// recognise as constant without doing any reference / scope work:
// literals, function / class / object / array expressions, the
// `undefined` global, and parenthesised wrappers around them.
// Cases that need full constant-folding through logical operators,
// template substitutions, and `Boolean(...)` calls are deferred until
// a shared `is_constant` utility lands.
package noconstantcondition

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-constant-condition"

// CheckLoops controls whether constant conditions inside loops are
// flagged.
type CheckLoops int

const (
	// CheckLoopsAllExceptWhileTrue is the default: flag constant
	// conditions in loops except for the canonical `while(true)`
	// infinite-loop idiom.
	CheckLoopsAllExceptWhileTrue CheckLoops = iota
	// CheckLoopsAll flags every constant condition in a loop.
	CheckLoopsAll
	// CheckLoopsNone never flags constant conditions in loops.
	CheckLoopsNone
)

// Options configures the rule.
type Options struct {
	CheckLoops CheckLoops
}

// New constructs a rule with default options.
func New() engine.Rule { return &rule{} }

// NewWithOptions constructs a rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           r.visitIf,
		wrapperchecker.KindConditionalExpression: r.visitConditional,
		wrapperchecker.KindWhileStatement:        r.visitWhile,
		wrapperchecker.KindDoStatement:           r.visitDoWhile,
		wrapperchecker.KindForStatement:          r.visitFor,
	}
}

func (r *rule) visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	r.report(ctx, n.IfCondition())
}

func (r *rule) visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	r.report(ctx, n.ConditionalCondition())
}

func (r *rule) visitWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.CheckLoops == CheckLoopsNone {
		return
	}
	cond := n.WhileCondition()
	if r.opts.CheckLoops == CheckLoopsAllExceptWhileTrue && isLiteralTrue(cond) {
		return
	}
	r.report(ctx, cond)
}

func (r *rule) visitDoWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.CheckLoops == CheckLoopsNone {
		return
	}
	r.report(ctx, n.WhileCondition())
}

func (r *rule) visitFor(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.CheckLoops == CheckLoopsNone {
		return
	}
	r.report(ctx, n.ForStatementCondition())
}

func (r *rule) report(ctx *engine.Context, cond *wrapperchecker.Node) {
	if cond == nil {
		return
	}
	if isObviouslyConstant(cond) {
		ctx.Report(cond, "Unexpected constant condition")
	}
}

// isObviouslyConstant reports whether `n` is a value the parser can
// recognise as constant without scope / type information. The check
// is deliberately tight: any expression whose constancy depends on
// resolving an identifier or evaluating an operator returns false.
func isObviouslyConstant(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindClassExpression:
		return true
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isObviouslyConstant(inner)
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		if op == "void" || op == "typeof" {
			return true
		}
		if op == "!" {
			// `!expr` is constant when `expr`'s truthiness is
			// known — i.e. when it's any obvious constant.
			return isObviouslyConstant(n.PrefixUnaryOperand())
		}
		if op == "+" || op == "-" || op == "~" {
			// Arithmetic unary coerces to a number; only a
			// *primitive* constant yields a known number.
			// `+[a]` depends on `a` and is NOT constant.
			return isPrimitiveConstant(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		left := n.BinaryLeft()
		right := n.BinaryRight()
		if isAssignmentOperator(op) {
			if op == wrapperchecker.KindEqualsToken {
				return isObviouslyConstant(right)
			}
			switch op {
			case wrapperchecker.KindBarBarEqualsToken:
				// `a ||= c` is truthy when c is constant-truthy
				// (regardless of a's value).
				return isConstantTruthy(right)
			case wrapperchecker.KindAmpersandAmpersandEqualsToken:
				// `a &&= c` is falsy when c is constant-falsy.
				return isConstantFalsy(right)
			}
			// `??=` is not constant for truthiness: when a is
			// non-nullish, the result is a (truthiness unknown).
			return false
		}
		switch op {
		case wrapperchecker.KindCommaToken:
			// `a, 1` — the result is the RHS; constant iff RHS is.
			return isObviouslyConstant(right)
		case wrapperchecker.KindAmpersandAmpersandToken:
			// `&&`: result is constant when the LHS is a constant
			// FALSY (then result = LHS) or when both sides are
			// constant.
			if isConstantFalsy(left) {
				return true
			}
			return isObviouslyConstant(left) && isObviouslyConstant(right)
		case wrapperchecker.KindBarBarToken:
			// `||`: result is constant when the LHS is a constant
			// TRUTHY (then result = LHS) or when both sides are
			// constant.
			if isConstantTruthy(left) {
				return true
			}
			return isObviouslyConstant(left) && isObviouslyConstant(right)
		case wrapperchecker.KindQuestionQuestionToken:
			// `??`: result is constant when the LHS is a constant
			// NON-NULLISH (then result = LHS).
			if isConstantNonNullish(left) {
				return true
			}
			return isObviouslyConstant(left) && isObviouslyConstant(right)
		}
		// Arithmetic / comparison / bitwise / equality: result is
		// constant only when both operands are *primitive*
		// constants. Reference-type literals (arrays, objects,
		// functions, classes) are truthy when used as the entire
		// condition but their participation in arithmetic /
		// comparison depends on values that may not be constant
		// (e.g. `'' + [y]` depends on `y`).
		return isPrimitiveConstant(left) && isPrimitiveConstant(right)
	case wrapperchecker.KindTemplateExpression:
		// A template literal is constant truthy when at least one
		// of: a non-empty text part (head/middle/tail) OR a span
		// whose expression is constant truthy (concatenating any
		// non-empty string into the result guarantees a non-empty
		// final string).
		hasTruthyPart := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindTemplateHead,
				wrapperchecker.KindTemplateMiddle,
				wrapperchecker.KindTemplateTail:
				if c.LiteralText() != "" {
					hasTruthyPart = true
				}
			case wrapperchecker.KindTemplateSpan:
				c.ForEachChild(func(cc *wrapperchecker.Node) bool {
					switch cc.Kind() {
					case wrapperchecker.KindTemplateMiddle,
						wrapperchecker.KindTemplateTail:
						if cc.LiteralText() != "" {
							hasTruthyPart = true
						}
					default:
						// A span guarantees a non-empty string
						// only when its expression is a constant
						// PRIMITIVE that's truthy (string/number/
						// bigint with a non-zero/non-empty value).
						// Arrays/objects can stringify to empty.
						if isPrimitiveConstantTruthy(cc) {
							hasTruthyPart = true
						}
					}
					return false
				})
			}
			return false
		})
		return hasTruthyPart
	case wrapperchecker.KindNewExpression:
		// `new C(...)` always evaluates to a newly-allocated
		// object — truthy and never null. The condition can never
		// flip on the inputs.
		return true
	}
	return false
}

// isConstantTruthy reports whether n is a literal expression whose
// value is statically known to be truthy.
func isConstantTruthy(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isConstantTruthy(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindNewExpression:
		return true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText() != ""
	case wrapperchecker.KindNumericLiteral:
		text := n.LiteralText()
		return text != "" && text != "0" && text != "0.0" && text != ".0"
	case wrapperchecker.KindBigIntLiteral:
		text := n.LiteralText()
		return text != "" && text != "0n"
	}
	return false
}

// isPrimitiveConstantTruthy reports whether n is a primitive
// literal whose value (not just truthiness) is statically known to
// be truthy. Used inside template spans where we need the *string
// coercion* to be guaranteed non-empty.
func isPrimitiveConstantTruthy(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isPrimitiveConstantTruthy(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		// `${true}` → "true", `${false}` → "false", `${null}` →
		// "null" — all non-empty.
		return true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText() != ""
	case wrapperchecker.KindNumericLiteral:
		// Any numeric literal coerces to a non-empty string
		// ("0", "0.5", "NaN" — even zero is "0", non-empty).
		return true
	case wrapperchecker.KindBigIntLiteral:
		return true
	}
	return false
}

// isConstantFalsy reports whether n is a literal expression whose
// value is statically known to be falsy.
func isConstantFalsy(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isConstantFalsy(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindFalseKeyword, wrapperchecker.KindNullKeyword:
		return true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText() == ""
	case wrapperchecker.KindNumericLiteral:
		text := n.LiteralText()
		return text == "0" || text == "0.0" || text == ".0"
	case wrapperchecker.KindBigIntLiteral:
		return n.LiteralText() == "0n"
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindPrefixUnaryExpression:
		if n.PrefixUnaryOperator() == "void" {
			return true
		}
	}
	return false
}

// isConstantNonNullish reports whether n is a literal expression
// whose value is statically known to be neither null nor undefined.
func isConstantNonNullish(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isConstantNonNullish(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindNewExpression:
		return true
	}
	return false
}

// isPrimitiveConstant reports whether n is a constant whose *value*
// (not just truthiness) is statically determined. Reference-type
// literals (arrays/objects/functions/classes) and templates with
// embedded values are excluded: their string/number coercion
// depends on the embedded expressions.
func isPrimitiveConstant(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return true
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isPrimitiveConstant(inner)
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		if op == "void" {
			return true
		}
		if op == "+" || op == "-" || op == "~" || op == "!" {
			return isPrimitiveConstant(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if isAssignmentOperator(op) || op == wrapperchecker.KindCommaToken {
			return false
		}
		return isPrimitiveConstant(n.BinaryLeft()) && isPrimitiveConstant(n.BinaryRight())
	}
	return false
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

// isLiteralTrue reports whether the expression is the literal `true`
// (or `(true)` etc.) — the canonical infinite-loop idiom that the
// default `allExceptWhileTrue` config exempts.
func isLiteralTrue(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isLiteralTrue(inner)
	}
	return n.Kind() == wrapperchecker.KindTrueKeyword
}
