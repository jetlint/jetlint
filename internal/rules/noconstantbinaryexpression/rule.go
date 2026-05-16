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

// Operator-token Kinds the wrapper doesn't surface as named
// constants. Values come from the underlying AST kind enum order
// in typescript-go's internal/ast/kind_generated.go and travel
// through BinaryOperatorKind() unchanged. Mirroring is unfortunate
// but stable: the enum order is locked by upstream TypeScript.
const (
	kindAsteriskToken                       wrapperchecker.Kind = 41
	kindAsteriskAsteriskToken               wrapperchecker.Kind = 42
	kindSlashToken                          wrapperchecker.Kind = 43
	kindPercentToken                        wrapperchecker.Kind = 44
	kindLessThanLessThanToken               wrapperchecker.Kind = 47
	kindGreaterThanGreaterThanToken         wrapperchecker.Kind = 48
	kindGreaterThanGreaterThanGreaterThanToken wrapperchecker.Kind = 49
	kindAmpersandToken                      wrapperchecker.Kind = 50
	kindBarToken                            wrapperchecker.Kind = 51
	kindCaretToken                          wrapperchecker.Kind = 52
	kindInKeyword                           wrapperchecker.Kind = 120
	kindInstanceOfKeyword                   wrapperchecker.Kind = 121
)

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
		// `undefined` resolves to the undefined value (falsy) in
		// any normal scope; `NaN` is a non-zero number (truthy).
		// Risking the shadowed-binding edge case is acceptable —
		// that's almost always a deliberately confusing test.
		switch n.LiteralText() {
		case "undefined", "NaN":
			return true
		}
		return false
	case wrapperchecker.KindVoidExpression, wrapperchecker.KindTypeOfExpression:
		// `void x` is always undefined (falsy); `typeof x` is
		// always a non-empty string (truthy).
		return true
	case wrapperchecker.KindNewExpression:
		// `new C(...)` always yields a fresh object (truthy).
		return true
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return hasConstantTruthiness(inner)
	case wrapperchecker.KindTemplateExpression:
		// A template with at least one non-empty text part is a
		// non-empty string (truthy).
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindTemplateHead,
				wrapperchecker.KindTemplateMiddle,
				wrapperchecker.KindTemplateTail:
				if c.LiteralText() != "" {
					hit = true
				}
			case wrapperchecker.KindTemplateSpan:
				c.ForEachChild(func(cc *wrapperchecker.Node) bool {
					if cc.Kind() == wrapperchecker.KindTemplateMiddle ||
						cc.Kind() == wrapperchecker.KindTemplateTail {
						if cc.LiteralText() != "" {
							hit = true
						}
					}
					return false
				})
			}
			return false
		})
		return hit
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if op == wrapperchecker.KindCommaToken {
			// `(a, b, c)` — truthiness of the comma expression is
			// the truthiness of the rightmost operand.
			return hasConstantTruthiness(unwrapParens(n.BinaryRight()))
		}
		if isAssignmentOperator(op) && op == wrapperchecker.KindEqualsToken {
			return hasConstantTruthiness(unwrapParens(n.BinaryRight()))
		}
		left := unwrapParens(n.BinaryLeft())
		right := unwrapParens(n.BinaryRight())
		if op == wrapperchecker.KindAmpersandAmpersandToken {
			// `&&`: result truthiness is known when either operand
			// is provably falsy, or both have known truthiness.
			if isAlwaysFalsy(left) || isAlwaysFalsy(right) {
				return true
			}
			if hasConstantTruthiness(left) && hasConstantTruthiness(right) {
				return true
			}
		}
		if op == wrapperchecker.KindBarBarToken {
			// `||`: result truthiness is known when either operand
			// is provably truthy, or both have known truthiness.
			if isAlwaysTruthy(left) || isAlwaysTruthy(right) {
				return true
			}
			if hasConstantTruthiness(left) && hasConstantTruthiness(right) {
				return true
			}
		}
		if op == wrapperchecker.KindQuestionQuestionToken {
			// `??`: when LHS is provably non-nullish, result is
			// LHS; when provably nullish, result is RHS.
			if hasConstantNullishness(left) && hasConstantTruthiness(left) && hasConstantTruthiness(right) {
				return true
			}
		}
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
	case wrapperchecker.KindCallExpression:
		// `Boolean(const)` / `Boolean()` is a known boolean; can
		// be classified at the rule's logical-operator visit.
		return isAlwaysTruthy(n) || isAlwaysFalsy(n)
	}
	return false
}

// isAlwaysTruthy reports whether n is statically known to be truthy.
func isAlwaysTruthy(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isAlwaysTruthy(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindNewExpression,
		wrapperchecker.KindTypeOfExpression:
		return true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText() != ""
	case wrapperchecker.KindNumericLiteral:
		t := n.LiteralText()
		return t != "" && t != "0" && t != "0.0" && t != ".0"
	case wrapperchecker.KindBigIntLiteral:
		return n.LiteralText() != "0n"
	case wrapperchecker.KindPrefixUnaryExpression:
		if n.PrefixUnaryOperator() == "!" {
			return isAlwaysFalsy(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindCallExpression:
		callee := n.CalleeExpression()
		if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		if callee.SourceText() != "Boolean" {
			return false
		}
		args := n.CallArguments()
		if len(args) == 0 {
			return false
		}
		return isAlwaysTruthy(args[0])
	}
	return false
}

// isAlwaysFalsy reports whether n is statically known to be falsy.
func isAlwaysFalsy(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isAlwaysFalsy(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText() == ""
	case wrapperchecker.KindNumericLiteral:
		t := n.LiteralText()
		return t == "0" || t == "0.0" || t == ".0"
	case wrapperchecker.KindBigIntLiteral:
		return n.LiteralText() == "0n"
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindPrefixUnaryExpression:
		if n.PrefixUnaryOperator() == "!" {
			return isAlwaysTruthy(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindCallExpression:
		callee := n.CalleeExpression()
		if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		if callee.SourceText() != "Boolean" {
			return false
		}
		args := n.CallArguments()
		if len(args) == 0 {
			// `Boolean()` evaluates to `false`.
			return true
		}
		return isAlwaysFalsy(args[0])
	}
	return false
}

// isStaticallyNullish reports whether n is statically known to
// evaluate to null or undefined.
func isStaticallyNullish(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isStaticallyNullish(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindNullKeyword, wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		// `(a, b)` and `a = b` evaluate to the right-hand side —
		// nullishness traces through.
		if op == wrapperchecker.KindCommaToken || op == wrapperchecker.KindEqualsToken {
			return isStaticallyNullish(unwrapParens(n.BinaryRight()))
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
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindNewExpression:
		return true
	case wrapperchecker.KindIdentifier:
		// `undefined` is statically nullish; `NaN` is a number
		// (non-nullish).
		text := n.LiteralText()
		return text == "undefined" || text == "NaN"
	case wrapperchecker.KindVoidExpression:
		return true // `void x` is undefined => nullish.
	case wrapperchecker.KindTypeOfExpression:
		return true // `typeof x` is a string => non-nullish.
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return hasConstantNullishness(inner)
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
		// For `??`, the result is non-nullish whenever the RHS is
		// non-nullish: even if the LHS is nullish, the RHS takes
		// over. `&&` / `||` short-circuit semantics make their
		// result depend on operand values too dynamically to call
		// constant here.
		if op == wrapperchecker.KindQuestionQuestionToken {
			right := unwrapParens(n.BinaryRight())
			if hasConstantNullishness(right) && !isStaticallyNullish(right) {
				return true
			}
			return false
		}
		if op == wrapperchecker.KindAmpersandAmpersandToken ||
			op == wrapperchecker.KindBarBarToken {
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
	case wrapperchecker.KindCallExpression:
		// `Boolean(...)` / `String(...)` / `Number(...)` always
		// return a primitive of a known type — never nullish.
		callee := n.CalleeExpression()
		if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier {
			switch callee.SourceText() {
			case "Boolean", "String", "Number":
				return true
			}
		}
	}
	return false
}

// isConstantEquality reports whether a comparison of left and right
// (strict if `strict`, loose otherwise) is statically constant.
func isConstantEquality(left, right *wrapperchecker.Node, strict bool) bool {
	if strict {
		// Under === / !==, a fresh-reference literal can never
		// equal anything else (unique identity).
		if isFreshReferenceLiteral(left) || isFreshReferenceLiteral(right) {
			return true
		}
		// `new C()` always yields a freshly-allocated object — it
		// can't strictly-equal any primitive value.
		if (left.Kind() == wrapperchecker.KindNewExpression && primitiveTypeTag(right) != "") ||
			(right.Kind() == wrapperchecker.KindNewExpression && primitiveTypeTag(left) != "") {
			return true
		}
		// `new <well-known-builtin>()` is guaranteed to allocate a
		// new object instance; comparing strictly to any prior
		// reference always yields false.
		if isWellKnownNewExpression(left) || isWellKnownNewExpression(right) {
			return true
		}
		if hasIncompatibleTypeTag(left, right) {
			return true
		}
		return false
	}
	// Loose `==` / `!=`: an object / function / class / regex
	// literal has a fixed primitive coercion that never matches
	// `true` / `false` / `null` / `undefined`. Arrays with zero or
	// 2+ elements are included: zero stringifies to `""` (→ 0),
	// 2+ elements introduce a `,` so the string can't parse to a
	// finite number — neither can match boolean / null / undefined.
	if (isStablyCoercedReference(left) || isCoercionFixedArray(left)) && isLooseEqualityImmuneLiteral(right) {
		return true
	}
	if (isStablyCoercedReference(right) || isCoercionFixedArray(right)) && isLooseEqualityImmuneLiteral(left) {
		return true
	}
	// Two fresh-reference literals always allocate distinct objects;
	// loose equality between two references collapses to identity
	// comparison, which is always false.
	if isFreshReferenceLiteral(left) && isFreshReferenceLiteral(right) {
		return true
	}
	// Two singleton-valued expressions (true / false / null /
	// undefined / void X / !knownTruthiness / Boolean(const)) have a
	// statically determinable equality result under both `==` and
	// `===`.
	if hasSingletonSelfEquality(left, right) {
		return true
	}
	if hasIncompatibleTypeTag(left, right) && !looseEqualityCanCoerce(left, right) {
		return true
	}
	return false
}

// isCoercionFixedArray reports whether n is an array literal whose
// loose-equality coercion is fixed regardless of the elements'
// values: an empty array (`[]` → `""` → 0) or one with two-or-more
// elements (joined with `,`, never parseable as a finite number).
// Sparse arrays and spreads are excluded because ESLint's matching
// rule treats them as dynamic.
func isCoercionFixedArray(n *wrapperchecker.Node) bool {
	n = unwrapParens(n)
	if n == nil || n.Kind() != wrapperchecker.KindArrayLiteralExpression {
		return false
	}
	elems := n.ArrayElements()
	if len(elems) == 0 {
		return true
	}
	if len(elems) == 1 {
		return false
	}
	for _, e := range elems {
		if e == nil ||
			e.Kind() == wrapperchecker.KindOmittedExpression ||
			e.Kind() == wrapperchecker.KindSpreadElement {
			return false
		}
	}
	return true
}

// isStablyCoercedReference reports whether n is a fresh reference
// whose primitive coercion is a fixed string regardless of any
// embedded sub-expression: object / arrow / function / class / regex
// literal — but NOT array literals (their toString depends on
// elements).
func isStablyCoercedReference(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isStablyCoercedReference(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindRegularExpressionLiteral:
		return true
	}
	return false
}

// isLooseEqualityImmuneLiteral reports whether n is a primitive whose
// loose-equality coercion never matches a fresh reference literal:
// `true`, `false`, `null`, `undefined`. Numbers and strings are
// excluded because they can match an array's `toString()` output.
func isLooseEqualityImmuneLiteral(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isLooseEqualityImmuneLiteral(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		return true
	case wrapperchecker.KindIdentifier:
		return n.SourceText() == "undefined"
	case wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindCallExpression:
		// `Boolean(...)` always returns a boolean, which can't
		// loose-equal a fresh reference.
		callee := n.CalleeExpression()
		if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier {
			return callee.SourceText() == "Boolean"
		}
	case wrapperchecker.KindPrefixUnaryExpression:
		// `!X` always evaluates to a boolean primitive, immune to
		// loose-coercion against a fresh reference.
		if n.PrefixUnaryOperator() == "!" {
			return true
		}
	}
	return false
}

// looseEqualityCanCoerce reports whether the loose-equality coercion
// rules might collapse two values with different primitive type tags
// to a runtime-dependent result (e.g. `'5' == 5` depends on the
// string's content). Returning false means the comparison's outcome
// is statically determinable.
func looseEqualityCanCoerce(left, right *wrapperchecker.Node) bool {
	lt := primitiveTypeTag(left)
	rt := primitiveTypeTag(right)
	// `typeof X` returns a value from a fixed set of identifier-like
	// strings ("string", "number", ...) — none of which coerce to a
	// numeric value matching `true` (1) or `false` (0), and none of
	// which can equal an empty string under `==`. Treat any
	// typeof-vs-non-string comparison as statically determinable.
	lUnwrapped := unwrapParens(left)
	rUnwrapped := unwrapParens(right)
	if lUnwrapped != nil && lUnwrapped.Kind() == wrapperchecker.KindTypeOfExpression && rt != "string" && rt != "" {
		return false
	}
	if rUnwrapped != nil && rUnwrapped.Kind() == wrapperchecker.KindTypeOfExpression && lt != "string" && lt != "" {
		return false
	}
	pair := lt + "|" + rt
	switch pair {
	case "number|string", "string|number",
		"number|boolean", "boolean|number",
		"string|boolean", "boolean|string",
		"bigint|number", "number|bigint",
		"bigint|string", "string|bigint":
		return true
	}
	return false
}

// isFreshReferenceLiteral reports whether n is an expression that
// evaluates to a freshly constructed object/function/class/regex —
// these never strictly equal any value the user already has.
//
// Deliberately omits `new Expr()`: a constructor may return an
// existing object via an explicit `return`, so `new Foo() === x`
// is not provably constant without resolving `Foo`'s body.
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
		wrapperchecker.KindRegularExpressionLiteral:
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
	return cond.ConditionalWhenTrue(), cond.ConditionalWhenFalse()
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
	if hasSingletonSelfEquality(left, right) {
		return true
	}
	if isStringOrNumberBinary(left) && isNonStringOrNumberTag(primitiveTypeTag(right)) {
		return true
	}
	if isStringOrNumberBinary(right) && isNonStringOrNumberTag(primitiveTypeTag(left)) {
		return true
	}
	lt := primitiveTypeTag(left)
	rt := primitiveTypeTag(right)
	if lt == "" || rt == "" {
		return false
	}
	return lt != rt
}

// hasSingletonSelfEquality reports whether both operands are nodes
// whose value is a single statically-known constant (e.g. `null`,
// `true`, `undefined`, `void X`). Comparing two such values strictly
// yields a known result either way (equal or not).
func hasSingletonSelfEquality(left, right *wrapperchecker.Node) bool {
	l := unwrapParens(left)
	r := unwrapParens(right)
	if isSingletonValue(l) && isSingletonValue(r) {
		return true
	}
	return false
}

// isSingletonValue reports whether n always evaluates to one
// specific runtime value (and therefore comparison against another
// singleton is statically determinable). Includes the keywords
// true/false/null, the `undefined` identifier, `void X`, `!X` when
// X has known truthiness, and `Boolean()` / `Boolean(const)`.
func isSingletonValue(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isSingletonValue(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindPrefixUnaryExpression:
		if n.PrefixUnaryOperator() == "!" {
			operand := n.PrefixUnaryOperand()
			return isAlwaysTruthy(operand) || isAlwaysFalsy(operand)
		}
	case wrapperchecker.KindCallExpression:
		callee := n.CalleeExpression()
		if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		if callee.SourceText() != "Boolean" {
			return false
		}
		args := n.CallArguments()
		if len(args) == 0 {
			return true
		}
		return isAlwaysTruthy(args[0]) || isAlwaysFalsy(args[0])
	}
	return false
}

// isWellKnownNewExpression reports whether n is `new C(...)` where
// C is a built-in constructor name known to always allocate a fresh
// instance (so `x === new C(...)` is always false for any prior x).
func isWellKnownNewExpression(n *wrapperchecker.Node) bool {
	n = unwrapParens(n)
	if n == nil || n.Kind() != wrapperchecker.KindNewExpression {
		return false
	}
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	switch callee.SourceText() {
	case "Array", "ArrayBuffer", "Boolean", "DataView",
		"Date", "Error", "EvalError", "Map", "Number",
		"Object", "Promise", "RangeError", "ReferenceError",
		"RegExp", "Set", "String", "SyntaxError", "TypeError",
		"URIError", "WeakMap", "WeakSet":
		return true
	}
	return false
}

// isStringOrNumberBinary reports whether n is a binary `+` (or
// compound `+=`) expression — both always evaluate to either a
// number or a string.
func isStringOrNumberBinary(n *wrapperchecker.Node) bool {
	n = unwrapParens(n)
	if n == nil || n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	op := n.BinaryOperatorKind()
	return op == wrapperchecker.KindPlusToken || op == wrapperchecker.KindPlusEqualsToken
}

// isNonStringOrNumberTag reports whether a primitive-type tag refers
// to a type that `+` could never produce: boolean, null, undefined,
// or bigint (`+` of a bigint coerces, never yields bigint).
func isNonStringOrNumberTag(tag string) bool {
	switch tag {
	case "boolean", "null", "undefined", "bigint":
		return true
	}
	return false
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
	case wrapperchecker.KindVoidExpression:
		return "undefined"
	case wrapperchecker.KindTypeOfExpression:
		return "string"
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return primitiveTypeTag(inner)
	case wrapperchecker.KindIdentifier:
		if n.SourceText() == "undefined" {
			return "undefined"
		}
		if n.SourceText() == "NaN" {
			return "number"
		}
	case wrapperchecker.KindDeleteExpression:
		// `delete x` always evaluates to a boolean.
		return "boolean"
	case wrapperchecker.KindCallExpression:
		callee := n.CalleeExpression()
		if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier {
			switch callee.SourceText() {
			case "Boolean":
				return "boolean"
			case "String":
				return "string"
			case "Number":
				return "number"
			}
		}
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		switch op {
		case wrapperchecker.KindMinusToken,
			kindAsteriskToken,
			kindSlashToken,
			kindPercentToken,
			kindAsteriskAsteriskToken,
			kindLessThanLessThanToken,
			kindGreaterThanGreaterThanToken,
			kindGreaterThanGreaterThanGreaterThanToken,
			kindAmpersandToken,
			kindBarToken,
			kindCaretToken:
			// Arithmetic / bitwise binary always yields a number.
			return "number"
		case wrapperchecker.KindLessThanToken,
			wrapperchecker.KindLessThanEqualsToken,
			wrapperchecker.KindGreaterThanToken,
			wrapperchecker.KindGreaterThanEqualsToken,
			kindInKeyword,
			kindInstanceOfKeyword,
			wrapperchecker.KindEqualsEqualsToken,
			wrapperchecker.KindEqualsEqualsEqualsToken,
			wrapperchecker.KindExclamationEqualsToken,
			wrapperchecker.KindExclamationEqualsEqualsToken:
			return "boolean"
		case wrapperchecker.KindMinusEqualsToken,
			wrapperchecker.KindAsteriskEqualsToken,
			wrapperchecker.KindSlashEqualsToken,
			wrapperchecker.KindPercentEqualsToken,
			wrapperchecker.KindAsteriskAsteriskEqualsToken,
			wrapperchecker.KindLessThanLessThanEqualsToken,
			wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
			wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
			wrapperchecker.KindAmpersandEqualsToken,
			wrapperchecker.KindBarEqualsToken,
			wrapperchecker.KindCaretEqualsToken:
			return "number"
		}
	}
	return ""
}
