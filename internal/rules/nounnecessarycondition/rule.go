// Package nounnecessarycondition implements the no-unnecessary-condition
// rule: flag conditional positions whose test type is provably
// constant (always-true or always-false).
package nounnecessarycondition

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unnecessary-condition"

// LoopConditionMode controls how strictly statically-constant
// `while/for/do` test expressions are treated. Mirrors upstream's
// tri-state `allowConstantLoopConditions` option.
type LoopConditionMode int

const (
	// LoopConditionNever flags any statically-constant loop test.
	LoopConditionNever LoopConditionMode = iota
	// LoopConditionAlways exempts all loops with constant tests.
	LoopConditionAlways
	// LoopConditionOnlyAllowedLiterals exempts loops whose test is a
	// literal `true`, `false`, `0`, `1`, `''`, etc. expressed inline in
	// the source — references to declared constants of literal type
	// (`declare const t: true; while (t) {}`) are still flagged.
	LoopConditionOnlyAllowedLiterals
)

// Options is the configurable surface of the rule.
type Options struct {
	// AllowConstantLoopConditions controls flagging of constant
	// `while/for/do` test expressions per LoopConditionMode.
	AllowConstantLoopConditions LoopConditionMode
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           visitIf,
		wrapperchecker.KindWhileStatement:        r.visitWhile,
		wrapperchecker.KindDoStatement:           r.visitWhile,
		wrapperchecker.KindForStatement:          r.visitFor,
		wrapperchecker.KindConditionalExpression: visitConditional,
		wrapperchecker.KindBinaryExpression:      visitBinary,
		wrapperchecker.KindPrefixUnaryExpression: visitPrefixUnary,
	}
}

func visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	check(ctx, n.FirstChild())
}

// visitBinary covers `a && b` and `a || b` outside an explicit test
// position — the operator's branching depends on `a`'s truthiness,
// so a constant `a` makes the whole expression redundant. `??` is
// excluded because TS doesn't always model index access as nullable
// (the `noUncheckedIndexedAccess` flag changes the type), so a
// trailing `?? default` is often a deliberate runtime guard.
func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	switch op {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken:
		if l := n.BinaryLeft(); l != nil {
			check(ctx, l)
		}
		return
	case wrapperchecker.KindQuestionQuestionEqualsToken:
		l := n.BinaryLeft()
		if l == nil {
			return
		}
		t := ctx.TypeOf(l)
		if t == nil || t.IsAny() || t.IsUnknown() {
			return
		}
		if !typeIncludesNullOrUndefined(t) {
			if isIndexLikeAccess(l) {
				return
			}
			ctx.Report(l, "left of `??=` is never null or undefined; the assignment never runs")
			return
		}
		if isAlwaysNullishOnly(t) {
			ctx.Report(l, "left of `??=` is always null or undefined; remove the conditional")
		}
		return
	case wrapperchecker.KindQuestionQuestionToken:
		// `a ?? def` is unnecessary if `a` can't be null/undefined,
		// or — symmetrically — if `a` is ALWAYS null/undefined (the
		// default branch always wins).
		l := n.BinaryLeft()
		if l == nil {
			return
		}
		t := ctx.TypeOf(l)
		if t == nil {
			return
		}
		if t.IsAny() || t.IsUnknown() {
			return
		}
		if !typeIncludesNullOrUndefined(t) {
			// Skip indexed/keyed access here — under
			// noUncheckedIndexedAccess the runtime value can be
			// undefined even though the static type is narrower.
			if isIndexLikeAccess(l) {
				return
			}
			ctx.Report(l, "left of `??` is never null or undefined")
			return
		}
		if isAlwaysNullishOnly(t) {
			ctx.Report(l, "left of `??` is always null or undefined; remove the `??`")
		}
		return
	case wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken,
		wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken:
		checkRelational(ctx, n)
	case wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken:
		checkEquality(ctx, n)
	}
}

// checkEquality flags `a === b` (and the other equality operators)
// when the types of both sides make the comparison statically
// determinable — equal literal types are always true; disjoint
// primitive literals are always false; one side is null/undefined
// and the other's type can't be that.
func checkEquality(ctx *engine.Context, n *wrapperchecker.Node) {
	left, right := n.BinaryLeft(), n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	lt, rt := ctx.TypeOf(left), ctx.TypeOf(right)
	if lt == nil || rt == nil {
		return
	}
	// Both single literal types — straightforward static comparison.
	if isSingleLiteral(lt) && isSingleLiteral(rt) {
		ls, rs := lt.String(), rt.String()
		if ls != "" && rs != "" {
			ctx.Report(n, "comparison is statically determinable from the literal types ("+ls+" vs "+rs+")")
			return
		}
	}
	// Mixed: one side is null/undefined. If the other side can't be
	// that value, the comparison is statically false (or always true
	// for `!==`/`!=`).
	op := n.BinaryOperatorKind()
	loose := op == wrapperchecker.KindEqualsEqualsToken || op == wrapperchecker.KindExclamationEqualsToken
	if comparisonIsAgainstExcludedNullish(lt, rt, right, loose) ||
		comparisonIsAgainstExcludedNullish(rt, lt, left, loose) {
		ctx.Report(n, "comparison with null/undefined against a type that excludes it is statically determinable")
	}
}

// comparisonIsAgainstExcludedNullish reports whether the LHS type is
// exactly the literal value (null or undefined) and the RHS type
// can never contain that exact value. The corresponding `other`
// expression is the syntactic node, used to skip element-access
// targets that may be unsoundly narrower than their declared type.
func comparisonIsAgainstExcludedNullish(thisSide, otherSide *wrapperchecker.Type, otherNode *wrapperchecker.Node, loose bool) bool {
	if !isNullOrUndefinedSide(thisSide) || isIndexLikeAccess(otherNode) {
		return false
	}
	// `==`/`!=` treat null and undefined as equivalent — only flag
	// when the other side excludes BOTH.
	if loose {
		return !typeIncludesNullOrUndefined(otherSide)
	}
	if thisSide.IsNull() {
		return !typeIncludesNull(otherSide)
	}
	if thisSide.IsUndefined() {
		return !typeIncludesUndefined(otherSide)
	}
	return false
}

func typeIncludesNull(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	if t.IsNull() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeIncludesNull(m) {
				return true
			}
		}
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeIncludesNull(c)
		}
		return true
	}
	return false
}

func typeIncludesUndefined(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	if t.IsUndefined() || t.IsVoid() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeIncludesUndefined(m) {
				return true
			}
		}
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeIncludesUndefined(c)
		}
		return true
	}
	return false
}

// isIndexLikeAccess reports whether n is — or whose receiver chain
// passes through — an element access (`a[x]`) expression. Under
// noUncheckedIndexedAccess the runtime value can be undefined even
// when TypeScript types it narrower, so checks for nullishness
// against such expressions are deliberately allowed. The walk handles
// `arr[42]?.foo`, `arr[42].foo`, `arr[42].foo.bar`, etc.
func isIndexLikeAccess(n *wrapperchecker.Node) bool {
	for n != nil {
		switch n.Kind() {
		case wrapperchecker.KindElementAccessExpression:
			return true
		case wrapperchecker.KindPropertyAccessExpression:
			n = n.PropertyAccessReceiver()
			continue
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindNonNullExpression:
			n = n.FirstChild()
			continue
		}
		return false
	}
	return false
}

// isNullOrUndefinedSide reports whether t is exactly null or
// undefined (the literal types). Excludes wider types that just
// happen to contain those.
func isNullOrUndefinedSide(t *wrapperchecker.Type) bool {
	if t == nil || t.IsUnion() {
		return false
	}
	return t.IsNullOrUndefined()
}

// typeIncludesNullOrUndefined reports whether t (or any union arm)
// is null/undefined. Type parameters forward through their constraints.
func typeIncludesNullOrUndefined(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	if t.IsNullOrUndefined() || t.IsVoid() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeIncludesNullOrUndefined(m) {
				return true
			}
		}
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeIncludesNullOrUndefined(c)
		}
		return true
	}
	return false
}

// isAlwaysNullishOnly reports whether t is exclusively null/undefined
// (every union arm is null or undefined). Used to flag a `??` whose
// left operand always resolves to the fallback.
func isAlwaysNullishOnly(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsNullOrUndefined() || t.IsVoid() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAlwaysNullishOnly(m) {
				return false
			}
		}
		return true
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isAlwaysNullishOnly(c)
		}
	}
	return false
}

// checkRelational flags `<`, `<=`, `>`, `>=` between literal types
// where the result is statically determinable.
func checkRelational(ctx *engine.Context, n *wrapperchecker.Node) {
	left, right := n.BinaryLeft(), n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	lt, rt := ctx.TypeOf(left), ctx.TypeOf(right)
	if lt == nil || rt == nil {
		return
	}
	if !isSingleLiteral(lt) || !isSingleLiteral(rt) {
		return
	}
	ctx.Report(n, "comparison between literal types is statically determinable")
}

// isSingleLiteral reports whether t is a non-union, non-generic
// type whose value is fully determined at compile time — string,
// number, boolean, or bigint literals plus null/undefined.
func isSingleLiteral(t *wrapperchecker.Type) bool {
	if t == nil || t.IsUnion() || t.IsIntersection() || t.IsAny() || t.IsUnknown() || t.IsTypeParameter() {
		return false
	}
	if t.IsNullOrUndefined() {
		return true
	}
	s := t.String()
	switch {
	case t.IsBooleanLike() && (s == "true" || s == "false"):
		return true
	case t.IsStringLike() && s != "string":
		return true
	case t.IsNumberLike() && s != "number":
		return true
	case t.IsBigIntLike() && s != "bigint":
		return true
	}
	return false
}

func visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	checkRecursive(ctx, n.IfCondition())
}

func (r *rule) visitWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	r.checkLoopCondition(ctx, n.WhileCondition())
}

func (r *rule) visitFor(ctx *engine.Context, n *wrapperchecker.Node) {
	r.checkLoopCondition(ctx, n.ForStatementCondition())
}

// checkLoopCondition applies the loop-condition mode to the test
// expression of a `while`/`do`/`for` statement. The recursive walk is
// skipped under `always`, taken under `never`, and conditionally taken
// under `only-allowed-literals` (test is an inline literal → skip).
func (r *rule) checkLoopCondition(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	switch r.opts.AllowConstantLoopConditions {
	case LoopConditionAlways:
		return
	case LoopConditionOnlyAllowedLiterals:
		if isAllowedLoopLiteral(expr) {
			return
		}
	}
	checkRecursive(ctx, expr)
}

// isAllowedLoopLiteral reports whether expr is a primitive literal
// directly written in source (`while (true)`, `while (1)`, etc.) —
// the inline-literal carve-out for `only-allowed-literals`. Reference
// to a declared constant of literal type is not allowed.
func isAllowedLoopLiteral(expr *wrapperchecker.Node) bool {
	expr = unwrapParen(expr)
	if expr == nil {
		return false
	}
	switch expr.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindNullKeyword:
		return true
	}
	return false
}

func visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	checkRecursive(ctx, n.ConditionalCondition())
}

// checkRecursive walks &&/||/?? chains at the test position so each
// operand is checked individually. `b1 && b2` where b1 is always
// truthy reports on b1, since the conjunction collapses to just b2.
func checkRecursive(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindBarBarToken:
			checkRecursive(ctx, expr.BinaryLeft())
			checkRecursive(ctx, expr.BinaryRight())
			return
		}
	}
	if expr.Kind() == wrapperchecker.KindParenthesizedExpression {
		checkRecursive(ctx, expr.FirstChild())
		return
	}
	check(ctx, expr)
}

func check(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if t.IsNever() {
		// `never` in a condition position is unreachable — the
		// surrounding branch can never run.
		ctx.Report(expr, "condition is unreachable (type never)")
		return
	}
	// `arr[42]` and `obj[key]` look statically truthy/falsy because
	// the indexed access narrows to T, but at runtime the value can
	// be undefined whenever noUncheckedIndexedAccess isn't enabled.
	// typescript-eslint's rule never flags conditions whose value
	// flows from an element access for that reason. The check covers
	// `arr[42]`, `arr[42].foo`, `arr[42]?.foo`, and `!arr[42]`.
	if conditionFromIndexAccess(unwrapParen(expr)) {
		return
	}
	if isAlwaysTruthy(t) {
		ctx.Report(expr, "condition is always truthy")
		return
	}
	if isAlwaysFalsy(t) {
		ctx.Report(expr, "condition is always falsy")
	}
}

// conditionFromIndexAccess reports whether the condition expression
// derives its value (directly or through `!` / nested prop access)
// from an element access expression. Index access carves out the
// noUncheckedIndexedAccess discrepancy upstream documents.
func conditionFromIndexAccess(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindPrefixUnaryExpression && n.PrefixUnaryOperator() == "!" {
		return conditionFromIndexAccess(unwrapParen(n.FirstChild()))
	}
	return isIndexLikeAccess(n)
}

// unwrapParen strips parentheses and non-null assertions to reach the
// underlying expression so isIndexLikeAccess can see through `(arr[i])`
// and `arr[i]!`.
func unwrapParen(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil {
		switch n.Kind() {
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindNonNullExpression:
			n = n.FirstChild()
			continue
		}
		return n
	}
	return n
}

// isAlwaysTruthy reports whether t is a type whose every inhabitant
// is truthy at runtime. Covers `true`, non-empty string literals,
// non-zero number literals, and unions of such.
func isAlwaysTruthy(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAlwaysTruthy(m) {
				return false
			}
		}
		return true
	}
	s := t.String()
	switch {
	case t.IsBooleanLike() && s == "true":
		return true
	case t.IsStringLike() && s != "string" && s != "\"\"" && s != "''":
		return true
	case t.IsNumberLike() && s != "number" && s != "0":
		return true
	case t.IsBigIntLike() && s != "bigint" && s != "0n":
		return true
	}
	// Non-primitive, non-nullable types (objects, arrays, functions,
	// classes) are always truthy in JS — only `null`/`undefined`/empty
	// strings/zero numbers are falsy. We've already excluded those.
	if isNonNullableNonPrimitive(t) {
		return true
	}
	// Type parameter: forward to the constraint when it's narrower
	// than `unknown`. `T extends object` is always truthy.
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isAlwaysTruthy(c)
		}
	}
	return false
}

func isNonNullableNonPrimitive(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() || t.IsNullOrUndefined() || t.IsNever() || t.IsVoid() {
		return false
	}
	if t.IsBooleanLike() || t.IsStringLike() || t.IsNumberLike() || t.IsBigIntLike() || t.IsEnumLike() {
		return false
	}
	if t.IsTypeParameter() {
		return false
	}
	if t.IsIntersection() {
		// Branded primitives like `boolean & { __brand: string }` look
		// like an intersection but their truth value is governed by
		// the underlying primitive, not the brand.
		for _, m := range t.IntersectionMembers() {
			if m.IsBooleanLike() || m.IsStringLike() || m.IsNumberLike() || m.IsBigIntLike() {
				return false
			}
		}
	}
	return true
}

// isAlwaysFalsy reports whether t can never be truthy at runtime.
func isAlwaysFalsy(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAlwaysFalsy(m) {
				return false
			}
		}
		return true
	}
	if t.IsNullOrUndefined() || t.IsVoid() {
		return true
	}
	s := t.String()
	switch s {
	case "false", "\"\"", "''", "0", "0n":
		return true
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isAlwaysFalsy(c)
		}
	}
	return false
}
