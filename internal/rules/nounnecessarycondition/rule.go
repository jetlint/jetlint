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
		wrapperchecker.KindIfStatement:              visitIf,
		wrapperchecker.KindWhileStatement:           r.visitWhile,
		wrapperchecker.KindDoStatement:              r.visitWhile,
		wrapperchecker.KindForStatement:             r.visitFor,
		wrapperchecker.KindConditionalExpression:    visitConditional,
		wrapperchecker.KindBinaryExpression:         visitBinary,
		wrapperchecker.KindPrefixUnaryExpression:    visitPrefixUnary,
		wrapperchecker.KindCallExpression:           visitCall,
		wrapperchecker.KindPropertyAccessExpression: visitOptionalChain,
		wrapperchecker.KindElementAccessExpression:  visitOptionalChain,
	}
}

// visitOptionalChain flags `x?.y`, `x?.[i]`, `x?.()` when x's type is
// already non-nullable. Only the root link of each chain carries the
// `?.` token, so inner links are skipped; downstream optional links
// inherit their receiver's nullability and the static type of the
// inner expression won't be nullable even if the receiver is.
func visitOptionalChain(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.IsOptionalChainRoot() {
		return
	}
	recv := optionalChainReceiver(n)
	if recv == nil {
		return
	}
	// Element-access immediate receiver: TypeScript doesn't track
	// index validity without `noUncheckedIndexedAccess`, so `arr[i]?.`
	// is conventional guarding even when the element type is
	// non-nullable. Only skip the *direct* element access.
	if recv.Kind() == wrapperchecker.KindElementAccessExpression {
		return
	}
	t := effectiveReceiverType(ctx, recv)
	if t == nil {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		return
	}
	if t.IsNullOrUndefined() || typeContainsNullableForOptionalChain(t) {
		return
	}
	ctx.Report(n, "optional chain is unnecessary — the receiver is already non-nullable")
}

// effectiveReceiverType returns the type the chain link sees AFTER any
// preceding optional-chain shortcircuit has been considered. For a
// chained property access like the outer `?.baz` in `foo?.bar?.baz`,
// the immediate type of `foo?.bar` is `{baz: ...} | undefined` because
// of the `?.` shortcircuit. But by the time control reaches the second
// `?.`, we know foo wasn't nullish — so the type that matters is just
// the property's declared type on the source. Walk one access deeper
// to get that "would-be-reached" property type.
func effectiveReceiverType(ctx *engine.Context, recv *wrapperchecker.Node) *wrapperchecker.Type {
	if recv == nil {
		return nil
	}
	if recv.IsOptionalChain() {
		// recv is a chain link. The chain may have shortcircuited into
		// undefined by this point in the type system, but at the time
		// the next `?.` actually executes we know it didn't. Peel one
		// level using the property/call's own structure.
		switch recv.Kind() {
		case wrapperchecker.KindPropertyAccessExpression:
			src := recv.PropertyAccessReceiver()
			name := recv.PropertyAccessName()
			if src != nil && name != "" {
				if st := nonNullable(ctx.TypeOf(src)); st != nil {
					if pt := st.PropertyType(name); pt != nil {
						return pt
					}
				}
			}
		case wrapperchecker.KindCallExpression:
			// `foo?.bar()` — return type of the call signature on the
			// non-nullable callee gives the value seen at the next link.
			callee := recv.CalleeExpression()
			if calleeT := nonNullable(ctx.TypeOf(callee)); calleeT != nil {
				if sigs := calleeT.CallSignatures(); len(sigs) > 0 {
					if rt := sigs[0].ReturnType(); rt != nil {
						return rt
					}
				}
			}
		}
	}
	return ctx.TypeOf(recv)
}

// nonNullable returns t with the union members for `null`, `undefined`,
// and `void` removed. For non-union types, returns t unchanged unless
// it is itself nullish (in which case returns nil).
func nonNullable(t *wrapperchecker.Type) *wrapperchecker.Type {
	if t == nil {
		return nil
	}
	if !t.IsUnion() {
		if t.IsNullOrUndefined() || t.IsVoid() {
			return nil
		}
		return t
	}
	var remaining *wrapperchecker.Type
	for _, m := range t.UnionMembers() {
		if m == nil || m.IsNullOrUndefined() || m.IsVoid() {
			continue
		}
		// We can only meaningfully return *one* representative member
		// for property lookup; in the common chain case the receiver is
		// a single object type plus null/undefined so this is enough.
		if remaining == nil {
			remaining = m
		}
	}
	return remaining
}

func optionalChainReceiver(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return n.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		return n.ElementAccessReceiver()
	case wrapperchecker.KindCallExpression:
		return n.CalleeExpression()
	}
	return nil
}

func typeContainsNullableForOptionalChain(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsNullOrUndefined() || t.IsVoid() || t.IsUnknown() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContainsNullableForOptionalChain(m) {
				return true
			}
		}
	}
	if t.IsTypeParameter() {
		// Unconstrained `T` (or `T extends unknown`) admits null and
		// undefined at instantiation, so the optional chain is doing
		// real work. Forward to the constraint when there is one.
		c := t.BaseConstraint()
		if c == nil || c == t {
			return true
		}
		return typeContainsNullableForOptionalChain(c)
	}
	return false
}

// arrayPredicateMethods is the set of Array.prototype methods that
// take a boolean-returning predicate. A predicate that always returns
// the same truthy/falsy value makes the call vacuous.
var arrayPredicateMethods = map[string]bool{
	"filter":         true,
	"find":           true,
	"findIndex":      true,
	"findLast":       true,
	"findLastIndex":  true,
	"every":          true,
	"some":           true,
}

// visitCall flags array-predicate calls whose callback's return type
// is statically truthy or falsy. Only direct property-access callees
// like `arr.filter(cb)` are inspected — the receiver must be array
// or tuple typed.
func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	// Optional-chain call (`x?.()`): flag when callee is already
	// non-nullable.
	if n.IsOptionalChain() {
		visitOptionalChain(ctx, n)
	}
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	method := callee.PropertyAccessName()
	if !arrayPredicateMethods[method] {
		return
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || (!rt.IsArrayLikeType() && !rt.IsTupleType()) {
		return
	}
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	cb := args[0]
	cbT := ctx.TypeOf(cb)
	if cbT == nil {
		return
	}
	sigs := cbT.CallSignatures()
	if len(sigs) == 0 {
		return
	}
	retT := sigs[0].ReturnType()
	if retT == nil {
		return
	}
	// `any`/`unknown` returns can be anything at runtime — the
	// callback isn't constant. Type parameters narrow to their
	// constraint; if the constraint is statically truthy/falsy
	// (`<T extends true>`), the call is still vacuous.
	if retT.IsAny() || retT.IsUnknown() {
		return
	}
	if retT.IsTypeParameter() {
		if c := retT.BaseConstraint(); c != nil && c != retT {
			retT = c
		} else {
			return
		}
	}
	switch cb.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		// Inline callback — flag with the literal-style message.
		if isAlwaysTruthy(retT) {
			ctx.Report(cb, "callback always returns a truthy value — `"+method+"` is unnecessary")
		} else if isAlwaysFalsy(retT) {
			ctx.Report(cb, "callback always returns a falsy value — `"+method+"` returns no useful result")
		}
	case wrapperchecker.KindIdentifier:
		// Named function — distinct upstream message ids.
		if isAlwaysTruthy(retT) {
			ctx.Report(cb, "function `"+cb.LiteralText()+"` always returns truthy — `"+method+"` is unnecessary")
		} else if isAlwaysFalsy(retT) {
			ctx.Report(cb, "function `"+cb.LiteralText()+"` always returns falsy — `"+method+"` returns no useful result")
		}
	}
}

func visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	// `if (!x)`, `while (!x)`, etc. — the surrounding statement's
	// condition visitor already reports the redundant test, so don't
	// fire the unary visitor too. Same for `!!x` patterns: a single
	// report on the outermost is enough.
	if isInsideBooleanTest(n) {
		return
	}
	check(ctx, n.FirstChild())
}

// isInsideBooleanTest reports whether n sits as (or inside) the test
// of an if/while/do/for/ternary or as an operand of another logical
// negation that itself sits in such a position. The condition-level
// visitors (visitIf, visitWhile, etc.) report there; the unary
// visitor should defer to avoid duplicate reports.
func isInsideBooleanTest(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			cur = cur.Parent()
			continue
		case wrapperchecker.KindPrefixUnaryExpression:
			if cur.PrefixUnaryOperator() != "!" {
				return false
			}
			cur = cur.Parent()
			continue
		case wrapperchecker.KindIfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindForStatement,
			wrapperchecker.KindConditionalExpression:
			return true
		}
		return false
	}
	return false
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

// isAllowedLoopLiteral reports whether expr is one of the literals
// upstream's `only-allowed-literals` mode permits in a loop test —
// `true`, `false`, `0`, or `1`. Other truthy/falsy literals
// (`'truthy'`, `2`, `null`) are still flagged.
func isAllowedLoopLiteral(expr *wrapperchecker.Node) bool {
	expr = unwrapParen(expr)
	if expr == nil {
		return false
	}
	switch expr.Kind() {
	case wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword:
		return true
	case wrapperchecker.KindNumericLiteral:
		switch expr.LiteralText() {
		case "0", "1":
			return true
		}
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
	// Array index access (`arr[i]`) and non-literal tuple index access
	// (`tuple[n]`) read as the element type without modeling the
	// out-of-bounds-undefined possibility unless noUncheckedIndexedAccess
	// is on. typescript-eslint exempts these so safe runtime checks
	// don't get flagged. Restricted to direct array/tuple receivers —
	// `Record<string, T>` accesses are still flagged.
	if conditionFromArrayIndexAccess(ctx, unwrapParen(expr)) {
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

// conditionFromArrayIndexAccess reports whether expr derives its value
// from `arr[i]`, `!arr[i]`, or a property/optional chain rooted at one.
// Mirrors typescript-eslint's isArrayIndexExpression: only computed
// element access with array- or tuple-typed receivers, and tuple
// receivers only when the index is non-literal (literal-into-tuple is
// type-sound).
func conditionFromArrayIndexAccess(ctx *engine.Context, n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindPrefixUnaryExpression && n.PrefixUnaryOperator() == "!" {
		return conditionFromArrayIndexAccess(ctx, unwrapParen(n.FirstChild()))
	}
	switch n.Kind() {
	case wrapperchecker.KindElementAccessExpression:
		recv := n.ElementAccessReceiver()
		idx := n.ElementAccessIndex()
		if recv == nil {
			return false
		}
		rt := ctx.TypeOf(recv)
		if rt == nil {
			return false
		}
		if rt.IsArrayLikeType() && !rt.IsTupleType() {
			return true
		}
		if rt.IsTupleType() && idx != nil && !isLiteralIndex(idx) {
			return true
		}
		return false
	case wrapperchecker.KindPropertyAccessExpression:
		return conditionFromArrayIndexAccess(ctx, unwrapParen(n.FirstChild()))
	}
	return false
}

// isLiteralIndex reports whether idx is a numeric or string literal —
// indexing a tuple by such a literal yields a sound element type that
// shouldn't trigger the array-index carve-out.
func isLiteralIndex(idx *wrapperchecker.Node) bool {
	idx = unwrapParen(idx)
	if idx == nil {
		return false
	}
	switch idx.Kind() {
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return true
	}
	return false
}

// unwrapParen strips parentheses and non-null assertions to reach the
// underlying expression so callers can see through `(arr[i])` and
// `arr[i]!`.
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
	// Branded primitives (`'foo' & { __brand }` etc.): truthiness
	// follows the primitive member. Only consider literal-primitive
	// members — `string & {}` still admits the empty string, so the
	// intersection's truthiness is indeterminate.
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isLiteralPrimitive(m) && isAlwaysTruthy(m) {
				return true
			}
		}
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

// isLiteralPrimitive reports whether t is a literal type of a
// primitive — `'foo'`, `42`, `true`. Used by intersection-branded
// truthiness checks: only literal members pin the runtime truth.
func isLiteralPrimitive(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
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
	if t.IsIntersection() {
		// Branded primitives (`'' & { __brand }`): the brand is purely
		// nominal — runtime truthiness follows the primitive member.
		// Only literal-primitive members count: `string & {...}` still
		// admits any string, so the intersection's truth is not pinned.
		for _, m := range t.IntersectionMembers() {
			if isLiteralPrimitive(m) && isAlwaysFalsy(m) {
				return true
			}
		}
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isAlwaysFalsy(c)
		}
	}
	return false
}
