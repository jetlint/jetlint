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
	"strings"

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
	if !isObviouslyConstant(cond) {
		return
	}
	// A shadowed `undefined` or `Boolean` binding makes the constancy
	// premise wrong — the local symbol could hold anything. Skip the
	// diagnostic when scope resolution shows the relevant identifier
	// doesn't refer to the global.
	if hasLocallyShadowedReference(cond, ctx.Checker()) {
		return
	}
	ctx.Report(cond, "Unexpected constant condition")
}

// hasLocallyShadowedReference walks `n` and returns true if any
// `undefined` identifier or `Boolean(...)` call resolves to a
// non-global binding. The constancy check assumes those names refer
// to the global undefined value / Boolean constructor; when they're
// shadowed, the assumption breaks and we must abstain.
func hasLocallyShadowedReference(n *wrapperchecker.Node, checker *wrapperchecker.Checker) bool {
	if n == nil || checker == nil {
		return false
	}
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			if c.LiteralText() == "undefined" && isLocallyBound(c, checker) {
				found = true
				return
			}
		case wrapperchecker.KindCallExpression:
			callee := c.CalleeExpression()
			if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier &&
				callee.LiteralText() == "Boolean" && isLocallyBound(callee, checker) {
				found = true
				return
			}
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(n)
	return found
}

// isLocallyBound reports whether `ref` resolves to a symbol with a
// user-supplied declaration (i.e. NOT a global ambient binding).
// Declarations inside ambient `.d.ts` files (lib type definitions
// for the standard library) don't count — `Boolean` from
// `lib.es5.d.ts` is the global constructor we're meant to detect,
// not a shadow.
//
// TypeScript's symbol resolution prefers the global lib declaration
// when a user file at script scope redeclares the same name (an
// error, but the reference still resolves to the lib symbol). To
// cover that case, fall back to a lexical scan of the enclosing
// source file for a binding with the matching identifier name.
func isLocallyBound(ref *wrapperchecker.Node, checker *wrapperchecker.Checker) bool {
	sym := checker.SymbolOf(ref)
	if sym != nil {
		for _, decl := range sym.Declarations() {
			if decl == nil || isAmbientDeclaration(decl) {
				continue
			}
			switch decl.Kind() {
			case wrapperchecker.KindVariableDeclaration,
				wrapperchecker.KindParameter,
				wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindClassDeclaration,
				wrapperchecker.KindClassExpression,
				wrapperchecker.KindImportSpecifier,
				wrapperchecker.KindImportClause,
				wrapperchecker.KindNamespaceImport,
				wrapperchecker.KindBindingElement:
				return true
			}
		}
	}
	return enclosingSourceHasBinding(ref, ref.LiteralText())
}

// enclosingSourceHasBinding reports whether any binding visible at
// `ref`'s position declares the identifier `name`. We walk up the
// chain of scope-introducing ancestors (SourceFile, function bodies,
// blocks) and at each level scan the direct statement siblings —
// skipping the child containing the reference, so we don't re-enter
// our own subtree. This correctly distinguishes bindings that
// dominate the reference site (which shadow the global) from
// bindings nested inside the reference's enclosing statement (which
// don't).
func enclosingSourceHasBinding(ref *wrapperchecker.Node, name string) bool {
	if name == "" {
		return false
	}
	cur := ref
	for cur != nil {
		parent := cur.Parent()
		if parent == nil {
			break
		}
		if isScopeIntroducer(parent) {
			if scopeHasSiblingBinding(parent, cur, name) {
				return true
			}
		}
		cur = parent
	}
	return false
}

// isScopeIntroducer reports whether `n` introduces a lexical scope
// where direct child declarations are visible to its other
// descendants.
func isScopeIntroducer(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindSourceFile,
		wrapperchecker.KindBlock,
		wrapperchecker.KindModuleBlock:
		return true
	}
	return false
}

// scopeHasSiblingBinding checks the direct children of `scope` (a
// scope-introducer) for a declaration with `name`. The `skip` child
// — typically the path back to the reference — is excluded so we
// don't pick up declarations nested inside the reference's own
// subtree.
func scopeHasSiblingBinding(scope, skip *wrapperchecker.Node, name string) bool {
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if found || c == nil || c == skip {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindVariableStatement:
			if list := c.VariableStatementDeclarationList(); list != nil {
				list.ForEachChild(func(decl *wrapperchecker.Node) bool {
					if decl.Kind() == wrapperchecker.KindVariableDeclaration {
						if nm := decl.DeclarationName(); nm != nil && nm.LiteralText() == name {
							found = true
						}
					}
					return found
				})
			}
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration:
			if nm := c.DeclarationName(); nm != nil && nm.LiteralText() == name {
				found = true
			}
		}
		return found
	})
	return found
}

// isAmbientDeclaration reports whether `decl` lives in a `.d.ts`
// file (TypeScript's standard library / type-definition shape).
func isAmbientDeclaration(decl *wrapperchecker.Node) bool {
	name, _, _, _, _ := decl.SourceRange()
	return strings.HasSuffix(name, ".d.ts")
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
	case wrapperchecker.KindVoidExpression:
		// `void <anything>` always evaluates to undefined → falsy.
		return true
	case wrapperchecker.KindTypeOfExpression:
		// `typeof <anything>` always evaluates to a non-empty
		// string → truthy.
		return true
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
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
			// `&&`: result is constant when *either* operand is a
			// constant FALSY (the whole expression then evaluates
			// to a falsy value regardless of the other side) or
			// when both sides are constant.
			if isConstantFalsy(left) || isConstantFalsy(right) {
				return true
			}
			return isObviouslyConstant(left) && isObviouslyConstant(right)
		case wrapperchecker.KindBarBarToken:
			// `||`: result is constant when *either* operand is a
			// constant TRUTHY (the whole expression then evaluates
			// to a truthy value regardless of the other side) or
			// when both sides are constant.
			if isConstantTruthy(left) || isConstantTruthy(right) {
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
		// `+` with one operand a primitive constant and the other a
		// fully-coercible array literal produces a known string.
		// Handle this before the generic primitive-constant check so
		// `if ('' + [])` (always falsy) and `if ('' + ['a'])`
		// (always truthy) are flagged.
		if op == wrapperchecker.KindPlusToken {
			if (isPrimitiveConstant(left) && isCoercibleArrayLiteral(right)) ||
				(isPrimitiveConstant(right) && isCoercibleArrayLiteral(left)) {
				return true
			}
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
	case wrapperchecker.KindCallExpression:
		// `Boolean()` with no args is always `false` (constant
		// falsy). `Boolean(X)` with a value whose truthiness is
		// known is a constant. We deliberately don't try to
		// detect local shadowing of `Boolean` — ESLint's rule
		// matches the identifier text and flags these uniformly.
		callee := n.CalleeExpression()
		if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier && callee.SourceText() == "Boolean" {
			args := n.CallArguments()
			if len(args) == 0 {
				return true
			}
			first := args[0]
			if isConstantTruthy(first) || isConstantFalsy(first) {
				return true
			}
		}
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
		wrapperchecker.KindNewExpression,
		wrapperchecker.KindTypeOfExpression:
		return true
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText() != ""
	case wrapperchecker.KindNumericLiteral:
		text := n.LiteralText()
		return text != "" && text != "0" && text != "0.0" && text != ".0"
	case wrapperchecker.KindBigIntLiteral:
		text := n.LiteralText()
		return text != "" && text != "0n"
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		if op == "!" {
			return isConstantFalsy(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		left := n.BinaryLeft()
		right := n.BinaryRight()
		switch op {
		case wrapperchecker.KindAmpersandAmpersandToken:
			// `a && b` is truthy iff both operands are truthy.
			return isConstantTruthy(left) && isConstantTruthy(right)
		case wrapperchecker.KindBarBarToken:
			// `a || b` is truthy if either operand is truthy.
			return isConstantTruthy(left) || isConstantTruthy(right)
		case wrapperchecker.KindQuestionQuestionToken:
			// `a ?? b` is truthy if a is non-nullish and truthy,
			// or if a is nullish and b is truthy.
			if isConstantTruthy(left) {
				return true
			}
			// If LHS is provably nullish (null/undefined/void), result is RHS.
			if isConstantNullish(left) {
				return isConstantTruthy(right)
			}
		case wrapperchecker.KindCommaToken:
			return isConstantTruthy(right)
		case wrapperchecker.KindBarBarEqualsToken:
			// `a ||= b` evaluates to `a` if a is truthy, else `b`.
			// So the value is truthy iff b is truthy (both branches
			// then yield a truthy result).
			return isConstantTruthy(right)
		case wrapperchecker.KindAmpersandAmpersandEqualsToken:
			// `a &&= b` evaluates to `a` if a is falsy, else `b`.
			// Constant truthy only when both arms are truthy, but
			// the falsy arm yields `a` (unknown) — so not provable.
			return false
		case wrapperchecker.KindEqualsToken:
			return isConstantTruthy(right)
		}
	}
	return false
}

// isConstantNullish reports whether n is statically known to be
// null or undefined.
func isConstantNullish(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isConstantNullish(inner)
	}
	switch n.Kind() {
	case wrapperchecker.KindNullKeyword, wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
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
	case wrapperchecker.KindBinaryExpression:
		// `+` of two primitives that each stringify to a
		// non-empty string yields a non-empty string.
		op := n.BinaryOperatorKind()
		if op == wrapperchecker.KindPlusToken {
			return isPrimitiveConstantTruthy(n.BinaryLeft()) && isPrimitiveConstantTruthy(n.BinaryRight())
		}
	case wrapperchecker.KindTemplateExpression:
		// Reuse the rule's main detector — a template whose
		// truthiness is statically known is acceptable here.
		return isObviouslyConstant(n)
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
	case wrapperchecker.KindVoidExpression:
		return true
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		if op == "!" {
			return isConstantTruthy(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		left := n.BinaryLeft()
		right := n.BinaryRight()
		switch op {
		case wrapperchecker.KindAmpersandAmpersandToken:
			// `a && b` is falsy if either operand is falsy.
			return isConstantFalsy(left) || isConstantFalsy(right)
		case wrapperchecker.KindBarBarToken:
			// `a || b` is falsy iff both operands are falsy.
			return isConstantFalsy(left) && isConstantFalsy(right)
		case wrapperchecker.KindQuestionQuestionToken:
			// `a ?? b` is falsy when a is non-nullish and falsy,
			// or when a is nullish and b is falsy.
			if isConstantNullish(left) {
				return isConstantFalsy(right)
			}
			// Without known nullishness of a, can't prove falsy.
		case wrapperchecker.KindCommaToken:
			return isConstantFalsy(right)
		case wrapperchecker.KindAmpersandAmpersandEqualsToken:
			// `a &&= b` evaluates to `a` if a is falsy, else `b`.
			// The result is falsy iff b is falsy (both branches
			// then yield a falsy result).
			return isConstantFalsy(right)
		case wrapperchecker.KindBarBarEqualsToken:
			return false
		case wrapperchecker.KindEqualsToken:
			return isConstantFalsy(right)
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
	case wrapperchecker.KindVoidExpression:
		// `void <anything>` always yields the primitive `undefined`.
		return true
	case wrapperchecker.KindTypeOfExpression:
		// `typeof X` yields a string whose VALUE depends on X.
		// Only a known-primitive operand makes the result a known
		// constant; `typeof someVariable` is the canonical runtime
		// type-check idiom and must NOT be treated as constant.
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isPrimitiveConstant(inner)
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
		if op == "!" {
			// `!X` yields boolean true/false whenever X has a
			// statically known truthiness.
			operand := n.PrefixUnaryOperand()
			if isConstantTruthy(operand) || isConstantFalsy(operand) {
				return true
			}
			return isPrimitiveConstant(operand)
		}
		if op == "+" || op == "-" || op == "~" {
			return isPrimitiveConstant(n.PrefixUnaryOperand())
		}
	case wrapperchecker.KindBinaryExpression:
		op := n.BinaryOperatorKind()
		if isAssignmentOperator(op) || op == wrapperchecker.KindCommaToken {
			return false
		}
		left := n.BinaryLeft()
		right := n.BinaryRight()
		// `&&` / `||` with a short-circuiting LHS resolve to that
		// LHS — so the result is primitive iff the LHS is.
		switch op {
		case wrapperchecker.KindAmpersandAmpersandToken:
			if isConstantFalsy(left) {
				return isPrimitiveConstant(left)
			}
			if isConstantTruthy(left) {
				return isPrimitiveConstant(right)
			}
		case wrapperchecker.KindBarBarToken:
			if isConstantTruthy(left) {
				return isPrimitiveConstant(left)
			}
			if isConstantFalsy(left) {
				return isPrimitiveConstant(right)
			}
		}
		return isPrimitiveConstant(left) && isPrimitiveConstant(right)
	}
	return false
}

// isCoercibleArrayLiteral reports whether `n` is an array literal
// whose string coercion is statically known: zero elements
// (stringifies to `""`), or all elements are primitive constants /
// nested coercible arrays (each contributes a known piece, joined
// by `,`). Spread elements and identifiers disqualify it.
func isCoercibleArrayLiteral(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isCoercibleArrayLiteral(inner)
	}
	if n.Kind() != wrapperchecker.KindArrayLiteralExpression {
		return false
	}
	for _, e := range n.ArrayElements() {
		if e == nil {
			continue
		}
		if e.Kind() == wrapperchecker.KindSpreadElement {
			return false
		}
		// `OmittedExpression` (a hole) stringifies to `""`, which is
		// fine — keep iterating.
		if e.Kind() == wrapperchecker.KindOmittedExpression {
			continue
		}
		if !isPrimitiveConstant(e) && !isCoercibleArrayLiteral(e) {
			return false
		}
	}
	return true
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
