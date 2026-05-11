// Package nounnecessarycondition implements the no-unnecessary-condition
// rule: flag conditional positions whose test type is provably
// constant (always-true or always-false).
package nounnecessarycondition

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
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
	// CheckTypePredicates enables flagging assertion/predicate
	// function calls whose argument already matches the predicate's
	// narrowed type (e.g. `assert(true)`, `isString(strVar)`).
	CheckTypePredicates bool
	// AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing
	// suppresses the per-file `noStrictNullCheck` advisory when the
	// program lacks `strictNullChecks`. The rule's nullability
	// reasoning is still unreliable; the option signals the user has
	// chosen to accept those false positives/negatives.
	AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing bool
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile:               r.visitSourceFile,
		wrapperchecker.KindCaseClause:               visitCaseClause,
		wrapperchecker.KindIfStatement:              visitIf,
		wrapperchecker.KindWhileStatement:           r.visitWhile,
		wrapperchecker.KindDoStatement:              r.visitWhile,
		wrapperchecker.KindForStatement:             r.visitFor,
		wrapperchecker.KindConditionalExpression:    visitConditional,
		wrapperchecker.KindBinaryExpression:         visitBinary,
		wrapperchecker.KindPrefixUnaryExpression:    visitPrefixUnary,
		wrapperchecker.KindCallExpression:           r.visitCall,
		wrapperchecker.KindPropertyAccessExpression: visitOptionalChain,
		wrapperchecker.KindElementAccessExpression:  visitOptionalChain,
	}
}

// visitSourceFile emits a per-file `noStrictNullCheck` advisory when
// the program lacks strictNullChecks. The rule's nullability reasoning
// is unreliable without it, so we surface that limitation up-front.
// The opt-in
// `allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing: true`
// suppresses the advisory for callers who've accepted the trade-off.
func (r *rule) visitSourceFile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing {
		return
	}
	if ctx.Program().HasStrictNullChecks() {
		return
	}
	ctx.Report(n, "this rule requires `strictNullChecks` to be enabled; otherwise nullable types cannot be reliably distinguished")
}

// checkAssertionCall flags a call to a function whose signature has
// an `asserts <param>` (or `<param> is T`) predicate when the argument
// already has a type that makes the predicate redundant — `assert(true)`
// over `asserts x`, `isString(strVar)` over `x is string`, etc.
func (r *rule) checkAssertionCall(ctx *engine.Context, n *wrapperchecker.Node) {
	sig := ctx.Checker().ResolvedSignature(n)
	if sig == nil {
		return
	}
	idx, isAsserts := predicateParamIndex(sig)
	if idx < 0 {
		return
	}
	args := n.CallArguments()
	if idx >= len(args) {
		return
	}
	arg := args[idx]
	argT := ctx.TypeOf(arg)
	if argT == nil {
		return
	}
	if argT.IsAny() || argT.IsUnknown() {
		return
	}
	narrowed := sig.TypePredicateNarrowedType()
	if narrowed != nil {
		// `<name> is T` (asserting or not): redundant when the argument
		// is already assignable to the target AND either the target is
		// mutually assignable back to the argument (i.e. equivalent) or
		// the target is a union the argument lives inside. Mirrors
		// upstream's gating so narrower literal arguments (like
		// `'falafel'` flowing into `is string`) aren't flagged.
		if !argT.IsAssignableTo(narrowed) {
			return
		}
		if !narrowed.IsAssignableTo(argT) && !narrowed.IsUnion() {
			return
		}
		ctx.Report(n, "predicate is unnecessary — the argument already has the predicate's target type")
		return
	}
	if !isAsserts {
		return
	}
	// Bare `asserts x`: redundant only when the argument's truthiness
	// is statically pinned — `assert(true)` or `assert(false)`.
	if isAlwaysTruthy(argT) || isAlwaysFalsy(argT) {
		ctx.Report(n, "assertion is unnecessary — the argument's truthiness is already pinned")
	}
}

// predicateParamIndex returns the parameter index covered by the
// signature's type predicate and whether the predicate is an asserting
// one. Returns -1 when the signature has no identifier-form predicate.
func predicateParamIndex(sig *wrapperchecker.Signature) (int, bool) {
	if idx := sig.AssertsParameterIndex(); idx >= 0 {
		return idx, true
	}
	if idx := sig.TypePredicateParameterIndex(); idx >= 0 {
		return idx, false
	}
	return -1, false
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
	// non-nullable. Tuples accessed with a numeric-literal index in
	// range differ — that slot is statically required, so its type
	// carries the same guarantees as a property access. Tuples
	// accessed with a non-literal index can still be out of range,
	// so the chain remains defensive.
	if recv.Kind() == wrapperchecker.KindElementAccessExpression {
		recvSrc := recv.ElementAccessReceiver()
		idx := recv.ElementAccessIndex()
		recvSrcT := ctx.TypeOf(recvSrc)
		// With noUncheckedIndexedAccess, TypeScript already widens the
		// element access to include `undefined` for out-of-bounds keys,
		// so flow narrowing handles the defensiveness — we don't need
		// to bail just because the receiver isn't a tuple.
		if !ctx.Program().NoUncheckedIndexedAccess() &&
			!elementAccessResolvesToDefiniteProperty(ctx, recvSrcT, idx) {
			if recvSrcT == nil || !recvSrcT.IsTupleType() {
				return
			}
			if idx == nil || idx.Kind() != wrapperchecker.KindNumericLiteral {
				return
			}
		}
	}
	// An element-access earlier in the chain "infects" downstream
	// links: `arr2[42]?.x?.y?.z` is defensive at every `?.` because the
	// runtime could have shortcircuited at `arr2[42]`. Skip flagging
	// whenever any preceding link is an element access on a non-tuple
	// receiver.
	if chainHasPriorElementAccess(ctx, recv) {
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

// unionPropertyType resolves `srcT[idxT]` when idxT is a string
// literal (or a union of string literals). For a single literal name
// it returns srcT.PropertyType(name); for a union it returns the
// union of each member's property type — provided every member
// resolves. Returns nil otherwise so the caller can fall back to the
// access expression's static type.
func unionPropertyType(srcT, idxT *wrapperchecker.Type) *wrapperchecker.Type {
	names := stringLiteralMembers(idxT)
	if len(names) == 0 {
		return nil
	}
	if len(names) == 1 {
		return srcT.PropertyType(names[0])
	}
	// Multiple literal names: any missing property means we can't
	// fully resolve. typescript-eslint widens to undefined in that
	// case; we conservatively bail.
	for _, n := range names {
		if srcT.PropertyType(n) == nil {
			return nil
		}
	}
	// Pick the first as a representative — the rule only needs to
	// know whether the type is nullable, not the exact union shape.
	return srcT.PropertyType(names[0])
}

// elementAccessResolvesToDefiniteProperty reports whether the
// indexed-access `recvSrcT[idx]` is guaranteed to land on a declared,
// non-optional property of the (non-nullable) source. When that holds,
// the access has the same nullability guarantees as a regular property
// access — so a subsequent `?.` in the chain is doing no real work,
// just like for `foo?.bar?.x`. Index signatures don't count: they
// could hit any string key and TypeScript reports the value type even
// when the runtime key is absent.
func elementAccessResolvesToDefiniteProperty(ctx *engine.Context, recvSrcT *wrapperchecker.Type, idx *wrapperchecker.Node) bool {
	if recvSrcT == nil || idx == nil {
		return false
	}
	srcT := nonNullable(recvSrcT)
	if srcT == nil {
		return false
	}
	idxT := ctx.TypeOf(idx)
	if idxT == nil {
		return false
	}
	for _, name := range stringLiteralMembers(idxT) {
		if !typeHasDefiniteOwnProperty(srcT, name) {
			return false
		}
	}
	// Require at least one literal — pure index-signature access stays
	// defensive.
	return len(stringLiteralMembers(idxT)) > 0
}

// stringLiteralMembers returns the literal string values present in
// t, walking unions. Returns nil for types whose values aren't
// statically known.
func stringLiteralMembers(t *wrapperchecker.Type) []string {
	if t == nil {
		return nil
	}
	if v, ok := t.StringLiteralValue(); ok {
		return []string{v}
	}
	if !t.IsUnion() {
		return nil
	}
	var out []string
	for _, m := range t.UnionMembers() {
		v, ok := m.StringLiteralValue()
		if !ok {
			return nil
		}
		out = append(out, v)
	}
	return out
}

// typeHasDefiniteOwnProperty reports whether name resolves on t to a
// declared, non-optional property whose static type carries no
// `undefined` member. Walks union members so the check is conservative
// for `A | B` — every branch must contribute the same guarantee.
func typeHasDefiniteOwnProperty(t *wrapperchecker.Type, name string) bool {
	if t == nil {
		return false
	}
	members := []*wrapperchecker.Type{t}
	if t.IsUnion() {
		members = t.UnionMembers()
	}
	for _, m := range members {
		sym := m.PropertySymbol(name)
		if sym == nil {
			return false
		}
		if sym.IsOptional() {
			return false
		}
		pt := m.PropertyType(name)
		if pt == nil {
			return false
		}
		if typeIncludesUndefined(pt) {
			return false
		}
	}
	return true
}

// chainHasPriorElementAccess walks the chain from recv backwards
// toward its root, returning true if any link is an element access on
// a non-tuple receiver AND TypeScript's compile-time type for that
// access is NOT nullable. Without `noUncheckedIndexedAccess`,
// TypeScript reports the array element as fully-typed, but at
// runtime the index could be out of range — so subsequent optional
// links in the chain remain defensive. When the option is on, the
// type already carries `| undefined` and TypeScript's flow narrowing
// handles the rest; we don't need a special "infect" exception there.
func chainHasPriorElementAccess(ctx *engine.Context, recv *wrapperchecker.Node) bool {
	for cur := recv; cur != nil; {
		if cur.Kind() == wrapperchecker.KindElementAccessExpression {
			recvSrcT := ctx.TypeOf(cur.ElementAccessReceiver())
			isUnchecked := false
			if elemT := ctx.TypeOf(cur); elemT != nil {
				isUnchecked = typeContainsNullableForOptionalChain(elemT)
			}
			if !isUnchecked && !ctx.Program().NoUncheckedIndexedAccess() {
				// Element type is statically non-nullable but could
				// still be out-of-bounds at runtime — defensive chain.
				// With `noUncheckedIndexedAccess`, TypeScript widens
				// out-of-bounds accesses to `| undefined`; control-flow
				// narrowing strips that when the index is safe, so a
				// non-nullable result here is genuinely safe.
				if recvSrcT == nil || !recvSrcT.IsTupleType() {
					return true
				}
				idx := cur.ElementAccessIndex()
				if idx == nil || idx.Kind() != wrapperchecker.KindNumericLiteral {
					return true
				}
			}
		}
		if !cur.IsOptionalChain() {
			return false
		}
		cur = optionalChainReceiver(cur)
	}
	return false
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
			// Mapped-type access doesn't surface a concrete property
			// symbol, so PropertyType returns nil even when the named
			// access is well-typed. The chain-added `| undefined` is
			// the only nullability on the receiver in that case —
			// strip it so the outer link reads as unnecessary.
			if propertyAccessHasNoIntrinsicUndefined(ctx, recv) {
				if et := nonNullable(ctx.TypeOf(recv)); et != nil {
					return et
				}
			}
		case wrapperchecker.KindElementAccessExpression:
			// `foo?.[key]` — at the point the next `?.` runs, foo was
			// non-nullish. If `key`'s type is a string literal (or a
			// union of them) naming definite properties of the
			// non-nullable receiver, the access has a precise type
			// that excludes the chain's `| undefined`. Otherwise,
			// return the access type as-is: any `| undefined` it
			// still carries (from the receiver's index signature or
			// `noUncheckedIndexedAccess`) is real, not chain-induced.
			src := recv.ElementAccessReceiver()
			idx := recv.ElementAccessIndex()
			if src != nil && idx != nil {
				if st := nonNullable(ctx.TypeOf(src)); st != nil {
					if it := ctx.TypeOf(idx); it != nil {
						if pt := unionPropertyType(st, it); pt != nil {
							return pt
						}
					}
				}
			}
		case wrapperchecker.KindCallExpression:
			// `foo?.bar()` — at the next chain link, foo wasn't nullish.
			// Inspect call signatures from every non-nullish union member
			// of the callee: if any returns nullable, the next `?.` is
			// doing real work and we want the caller to see that.
			callee := recv.CalleeExpression()
			if rt := callReturnTypeForChain(ctx, callee); rt != nil {
				return rt
			}
		}
	}
	return ctx.TypeOf(recv)
}

// callReturnTypeForChain reports the type that an optional-chained
// call expression's value carries forward to the next chain link, taking
// every callable union member of the callee into account. If any member
// can return nullable, prefer that signature's return type so the
// outer chain link reads as still doing work.
func callReturnTypeForChain(ctx *engine.Context, callee *wrapperchecker.Node) *wrapperchecker.Type {
	t := ctx.TypeOf(callee)
	if t == nil {
		return nil
	}
	members := []*wrapperchecker.Type{t}
	if t.IsUnion() {
		members = t.UnionMembers()
	}
	var firstNonNullable *wrapperchecker.Type
	for _, m := range members {
		if m == nil || m.IsNullOrUndefined() || m.IsVoid() {
			continue
		}
		sigs := m.CallSignatures()
		if len(sigs) == 0 {
			continue
		}
		for _, sig := range sigs {
			rt := sig.ReturnType()
			if rt == nil {
				continue
			}
			if typeContainsNullableForOptionalChain(rt) {
				return rt
			}
			if firstNonNullable == nil {
				firstNonNullable = rt
			}
		}
	}
	return firstNonNullable
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
func (r *rule) visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	// Optional-chain call (`x?.()`): flag when callee is already
	// non-nullable.
	if n.IsOptionalChain() {
		visitOptionalChain(ctx, n)
	}
	if r.opts.CheckTypePredicates {
		r.checkAssertionCall(ctx, n)
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

// inBooleanTestPosition reports whether n sits inside a position
// already handled by checkRecursive — the test of an if/while/for/do
// statement, the condition of a conditional expression, the operand
// of `!`, or a left-hand operand of `&&`/`||` whose own context is
// itself such a position.
func inBooleanTestPosition(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindIfStatement:
			return sameNodeIdentity(n, cur.IfCondition()) || isInsideTest(n, cur.IfCondition())
		case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
			return sameNodeIdentity(n, cur.WhileCondition()) || isInsideTest(n, cur.WhileCondition())
		case wrapperchecker.KindForStatement:
			return sameNodeIdentity(n, cur.ForStatementCondition()) || isInsideTest(n, cur.ForStatementCondition())
		case wrapperchecker.KindConditionalExpression:
			return sameNodeIdentity(n, cur.ConditionalCondition()) || isInsideTest(n, cur.ConditionalCondition())
		case wrapperchecker.KindPrefixUnaryExpression:
			if cur.PrefixUnaryOperator() == "!" {
				cur = cur.Parent()
				continue
			}
			return false
		case wrapperchecker.KindParenthesizedExpression:
			cur = cur.Parent()
			continue
		case wrapperchecker.KindBinaryExpression:
			switch cur.BinaryOperatorKind() {
			case wrapperchecker.KindAmpersandAmpersandToken,
				wrapperchecker.KindBarBarToken:
				cur = cur.Parent()
				continue
			}
			return false
		}
		return false
	}
	return false
}

// sameNodeIdentity reports whether two wrapper nodes refer to the
// same underlying AST node (compared via source-range tuples).
func sameNodeIdentity(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	af, asl, asc, ael, aec := a.SourceRange()
	bf, bsl, bsc, bel, bec := b.SourceRange()
	return af == bf && asl == bsl && asc == bsc && ael == bel && aec == bec
}

func isInsideTest(n, test *wrapperchecker.Node) bool {
	if test == nil {
		return false
	}
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		if sameNodeIdentity(cur, test) {
			return true
		}
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
		// Skip when this binary is itself a sub-expression of a
		// conditional position (if/while/for/do/conditional/!) — the
		// condition visitor's recursive walk already reports each
		// constant operand; firing here too would double-report.
		if inBooleanTestPosition(n) {
			return
		}
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
			// Skip indexed/keyed access here — without
			// noUncheckedIndexedAccess the static type doesn't include
			// the runtime "missing key" case, so the `??` is a
			// reasonable safety net. With the option on, the static
			// type already widens to include `undefined`, so a
			// non-nullable narrowing means the `??` is genuinely dead.
			if isIndexLikeAccess(l) && !ctx.Program().NoUncheckedIndexedAccess() {
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

// visitCaseClause flags `case X:` when the discriminant's static
// type and X's static type are both single literals that don't match
// — the case can never run — or that do match, since case fall-
// through over an always-matching value can hide bugs. Mirrors
// upstream's SwitchCase handler, which models the clause as
// `discriminant === test`.
func visitCaseClause(ctx *engine.Context, n *wrapperchecker.Node) {
	test := n.CaseExpression()
	if test == nil {
		return
	}
	parent := n.Parent()
	for parent != nil && parent.Kind() != wrapperchecker.KindSwitchStatement {
		parent = parent.Parent()
	}
	if parent == nil {
		return
	}
	disc := parent.SwitchExpression()
	if disc == nil {
		return
	}
	lt, rt := ctx.TypeOf(disc), ctx.TypeOf(test)
	if lt == nil || rt == nil {
		return
	}
	if !isSingleLiteral(lt) || !isSingleLiteral(rt) {
		return
	}
	ls, rs := lt.String(), rt.String()
	if ls == "" || rs == "" {
		return
	}
	ctx.Report(test, "case label comparison is statically determinable from the literal types ("+ls+" vs "+rs+")")
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
// against direct index access are deliberately allowed. A property
// access on top (e.g. `a[0].foo`) carries a real declared property
// type that the index-signature concern doesn't reach — upstream
// flags those `??` defaults. An optional-chain link anywhere in the
// expression is also considered index-like because the wrapper's
// TypeOf may not surface the chain-introduced undefined to callers.
func isIndexLikeAccess(n *wrapperchecker.Node) bool {
	root := n
	for n != nil {
		switch n.Kind() {
		case wrapperchecker.KindElementAccessExpression:
			if n == root {
				return true
			}
			if n.IsOptionalChain() {
				return true
			}
			return false
		case wrapperchecker.KindPropertyAccessExpression:
			if n.IsOptionalChain() {
				return true
			}
			rcv := n.PropertyAccessReceiver()
			if rcv == nil {
				return false
			}
			n = rcv
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
	if hasEslintDisableNextLine(n) {
		return
	}
	checkRecursive(ctx, n.IfCondition())
}

// hasEslintDisableNextLine reports whether the leading trivia of n
// includes an `// eslint-disable-next-line` directive. typescript-
// eslint suppresses diagnostics on the directly-following statement;
// we mirror that for the if/while/for handlers so cases that gate the
// rule on a disable comment exit cleanly.
func hasEslintDisableNextLine(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	return strings.Contains(n.LeadingTriviaText(), "eslint-disable-next-line")
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
			l, r := expr.BinaryLeft(), expr.BinaryRight()
			// `a && a` / `a || a` (or longer chains where the new
			// operand duplicates the running result) — the operator
			// has already decided based on the earlier occurrence, so
			// the duplicate doesn't change the outcome.
			if l != nil && r != nil && nodesTextEqual(l, r) {
				ctx.Report(r, "duplicate operand in logical expression has no effect")
			}
			checkRecursive(ctx, l)
			checkRecursive(ctx, r)
			return
		}
	}
	if expr.Kind() == wrapperchecker.KindParenthesizedExpression {
		checkRecursive(ctx, expr.FirstChild())
		return
	}
	// `!x` evaluates x in a boolean position — descend so a never-
	// typed inner operand surfaces as unreachable instead of being
	// hidden behind the negation's `true` result type.
	if expr.Kind() == wrapperchecker.KindPrefixUnaryExpression &&
		expr.PrefixUnaryOperator() == "!" {
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
	// Generic placeholders (type parameters, indexed access, etc.) are
	// always "conditionally necessary" — the runtime value depends on
	// the eventual substitution, so we can't say it's always truthy
	// or falsy. When the constraint resolves to a concrete shape,
	// substitute it so the downstream truthiness check can still fire.
	if conditionInvolvesTypeVariable(t) {
		return
	}
	if resolved := resolveTypeVariableConstraint(t); resolved != nil {
		t = resolved
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

// propertyAccessHasNoIntrinsicUndefined reports whether a property
// access `obj.prop` is guaranteed by the receiver's static shape to
// produce a value of non-`undefined` type — independent of any
// chain-induced `| undefined`. Holds when every union member of the
// non-nullable receiver supplies the property either through:
//   - a declared, non-optional, non-undefined property (regular
//     interface/object types);
//   - a string-index signature whose value type is non-undefined
//     (mapped types without `?`, `Record<...>`).
//
// Used to decide whether a subsequent `?.` chain link is doing real
// work when PropertyType returns nil (mapped types don't surface
// concrete symbols on demand).
func propertyAccessHasNoIntrinsicUndefined(ctx *engine.Context, recv *wrapperchecker.Node) bool {
	if recv == nil || recv.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	src := recv.PropertyAccessReceiver()
	name := recv.PropertyAccessName()
	if src == nil || name == "" {
		return false
	}
	srcT := nonNullable(ctx.TypeOf(src))
	if srcT == nil {
		return false
	}
	members := []*wrapperchecker.Type{srcT}
	if srcT.IsUnion() {
		members = srcT.UnionMembers()
	}
	unchecked := ctx.Program().NoUncheckedIndexedAccess()
	for _, m := range members {
		if !memberSuppliesDefiniteProperty(m, name, unchecked) {
			return false
		}
	}
	return true
}

// memberSuppliesDefiniteProperty reports whether type m's contribution
// to a `.name` access is a definite, non-`undefined` value — either
// via a declared non-optional property or a string-index signature
// whose value type doesn't include `undefined`. Under
// noUncheckedIndexedAccess, index-signature access widens to include
// undefined, so the index-signature path can't supply a definite
// value.
func memberSuppliesDefiniteProperty(m *wrapperchecker.Type, name string, noUncheckedIndexedAccess bool) bool {
	if m == nil {
		return false
	}
	if sym := m.PropertySymbol(name); sym != nil {
		if sym.IsOptional() {
			return false
		}
		pt := m.PropertyType(name)
		if pt == nil || typeIncludesUndefined(pt) {
			return false
		}
		return true
	}
	if noUncheckedIndexedAccess {
		return false
	}
	// Mapped types and `Record<...>`-style shapes don't surface a
	// concrete property symbol for an arbitrary key. Ask the checker
	// what `m[name]` resolves to — if the result is a real, non-
	// nullable type, the access lands on a definite value.
	if pt := m.IndexedAccessByLiteral(name); pt != nil &&
		!pt.IsAny() && !pt.IsUnknown() && !pt.IsNever() &&
		!typeIncludesNullOrUndefined(pt) {
		return true
	}
	return false
}

// nodesTextEqual reports whether a and b share identical source
// text after collapsing whitespace. Used to detect duplicate operands
// in a logical expression — purely syntactic since two textually
// identical expressions evaluated against the same scope produce the
// same value modulo side effects, and any side effects mean the
// duplicate is also a code-smell worth surfacing.
func nodesTextEqual(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return collapseWS(a.SourceText()) == collapseWS(b.SourceText())
}

// collapseWS replaces every run of whitespace in s with a single
// space and trims leading/trailing whitespace. Lets `arr [ 0 ]` match
// `arr[0]`.
func collapseWS(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inWS := true
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWS {
				b.WriteByte(' ')
				inWS = true
			}
			continue
		}
		b.WriteRune(r)
		inWS = false
	}
	out := b.String()
	if len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	return out
}

// resolveTypeVariableConstraint walks t replacing type variables
// (type parameters, indexed access) with their base constraint. Used
// before the truthiness check so `Obj[Key]` where Obj/Key resolve to
// concrete shapes (`Record<string, 1|2|3>['key']` → `1|2|3`) is still
// reportable as "always truthy". Returns nil if t carries no type
// variable and the caller can use the original value.
func resolveTypeVariableConstraint(t *wrapperchecker.Type) *wrapperchecker.Type {
	if t == nil {
		return nil
	}
	if t.IsTypeVariable() {
		c := t.BaseConstraint()
		if c == nil || c == t {
			return nil
		}
		if inner := resolveTypeVariableConstraint(c); inner != nil {
			return inner
		}
		return c
	}
	return nil
}

// conditionInvolvesTypeVariable reports whether any constituent of t
// (after walking through type-parameter and indexed-access
// constraints) remains a generic placeholder whose runtime value is
// unknown. Bare type parameters with a useful constraint (`T extends
// object`, `T extends 'a' | 'b'`) are *not* opaque — their truthiness
// follows the constraint, so the rule can still report them.
// Similarly, an indexed access `Obj[Key]` whose constraint resolves
// to a concrete shape is reportable. Mirrors upstream
// `isConditionalAlwaysNecessary` working on the constrained type.
func conditionInvolvesTypeVariable(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsTypeVariable() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return conditionInvolvesTypeVariable(c)
		}
		// No narrower constraint — value is effectively `unknown`.
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if conditionInvolvesTypeVariable(m) {
				return true
			}
		}
	}
	return false
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
