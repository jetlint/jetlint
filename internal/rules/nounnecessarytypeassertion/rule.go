// Package nounnecessarytypeassertion implements the
// no-unnecessary-type-assertion rule: flag `x as T` when x is
// already exactly T, and `x!` when x's type is already non-nullable.
package nounnecessarytypeassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unnecessary-type-assertion"

// Options is the configurable surface of the rule. Mirrors
// typescript-eslint's `no-unnecessary-type-assertion` options.
type Options struct {
	// TypesToIgnore lists target type names whose assertions should be
	// silently accepted regardless of source-type identity. Useful
	// when a project applies `as Foo` for documentation-style intent
	// even though the source is already `Foo`.
	TypesToIgnore []string
	// CheckLiteralConstAssertions enables flagging `as const` on
	// literal expressions in non-widening positions (e.g.
	// `const a = 'a' as const`). Off by default — most projects
	// adopt `as const` even where it has no runtime effect.
	CheckLiteralConstAssertions bool
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAsExpression:             r.visitAs,
		wrapperchecker.KindTypeAssertionExpression: r.visitAs,
		wrapperchecker.KindNonNullExpression:        visitNonNull,
	}
}

func (r *rule) visitAs(ctx *engine.Context, n *wrapperchecker.Node) {
	var src, annot *wrapperchecker.Node
	switch n.Kind() {
	case wrapperchecker.KindAsExpression:
		src = n.AsExpressionSource()
		annot = n.AsExpressionTarget()
	case wrapperchecker.KindTypeAssertionExpression:
		src = n.TypeAssertionSource()
		annot = n.TypeAssertionTarget()
	}
	if src == nil || annot == nil {
		return
	}
	// `as const` is meaningful in many positions. By default,
	// typescript-eslint doesn't flag it; opting into
	// `checkLiteralConstAssertions` flips on flagging for literal
	// expressions / values in non-widening positions where the `const`
	// keyword adds nothing the const-context wouldn't already give.
	if isAsConst(annot) {
		if !r.opts.CheckLiteralConstAssertions {
			return
		}
		// Upstream flags `as const` when the cast-result type is itself
		// already a literal AND the surrounding declaration would
		// implicitly preserve that literal (const variable / readonly
		// property). Cover both syntactic literals (`3 as const`) and
		// already-literal-typed values (enum members, named literal
		// types).
		castT := ctx.TypeOf(n)
		if castT == nil {
			return
		}
		isLiteralValue := isLiteralExpression(src) || isLiteralLikeType(castT)
		if !isLiteralValue || isInWideningPosition(n) {
			return
		}
		ctx.Report(n, "type assertion is unnecessary — the literal already has this exact type")
		return
	}
	srcT := ctx.TypeOf(src)
	target := ctx.Checker().TypeFromTypeNode(annot)
	if srcT == nil || target == nil {
		return
	}
	if r.targetIsIgnored(annot) {
		return
	}
	// `(() => {})() as undefined` — TypeScript infers the IIFE's
	// return as `void` (or sometimes `undefined`) when the body has no
	// explicit annotation. The cast is conventional documentation that
	// the call produces no value, so leave it alone.
	if target.String() == "undefined" && isIIFEWithoutReturnAnnotation(src) {
		return
	}
	// `expr as any as T` / `expr as unknown as T` — peel widening hops
	// off the source so subsequent checks see the underlying expression.
	effectiveSrc, effectiveSrcT := src, srcT
	if base := unwrapWideningCasts(src); base != nil {
		if bt := ctx.TypeOf(base); bt != nil {
			effectiveSrc, effectiveSrcT = base, bt
		}
	}
	// Asserting to a literal/tuple type often preserves narrower
	// inference at a widening boundary. Always skip non-literal source
	// (`let z = c as 1`); literal source is only flaggable when not in
	// a widening position.
	if isLiteralOrTupleAssertion(target) {
		if !isLiteralExpression(effectiveSrc) || isInWideningPosition(n) {
			return
		}
	}
	if effectiveSrcT.String() == target.String() {
		if effectiveSrc == src {
			ctx.Report(n, "type assertion is unnecessary — the source already has this type")
		} else {
			ctx.Report(n, "type assertion is unnecessary — the chain through `any`/`unknown` doesn't change the source's type")
		}
		return
	}
	// Special case: `T as NonNullable<T>` where T extends a non-null
	// type. TypeScript desugars `NonNullable<T>` to `T & {}`, so the
	// cast target is an intersection of the same type parameter with
	// an empty object — semantically a no-op when T's constraint is
	// already non-null. Mirrors upstream's intersection-with-type-
	// parameter-and-empty-object branch of `isTypeUnchanged`.
	if isTypeParamIntersectedWithEmptyObject(effectiveSrcT, target) {
		ctx.Report(n, "type assertion is unnecessary — `NonNullable<T>` collapses to `T` when T's constraint is non-null")
		return
	}
	// Object-literal source with the same property names as the cast
	// target and a widened-mutually-assignable shape: the implicit type
	// of the literal already structurally matches the named target, so
	// the cast adds nothing the type system would otherwise reject.
	// Restricted to object-literal sources to avoid recursive-type
	// IsAssignableTo calls (e.g. `type T = string | T[]`). Skip empty
	// source objects and targets with literal-typed properties — the
	// cast there preserves narrower inference that the implicit type
	// wouldn't.
	if effectiveSrc.Kind() == wrapperchecker.KindObjectLiteralExpression &&
		objectLiteralHasProperties(effectiveSrc) &&
		!effectiveSrcT.IsAny() && !effectiveSrcT.IsUnknown() &&
		!target.IsAny() && !target.IsUnknown() &&
		!effectiveSrcT.IsUnion() && !target.IsUnion() &&
		!hasLiteralProperty(target) &&
		hasSameProperties(effectiveSrcT, target) &&
		effectiveSrcT.IsAssignableTo(target) {
		if effectiveSrc == src {
			ctx.Report(n, "type assertion is unnecessary — the source already has this structural shape")
		} else {
			ctx.Report(n, "type assertion is unnecessary — the chain through `any`/`unknown` doesn't change the source's shape")
		}
		return
	}
	// Contextually unnecessary: when the surrounding position only
	// requires the source's narrower type, the cast adds nothing the
	// type system would otherwise reject. Mirrors typescript-eslint's
	// `contextuallyUnnecessary` branch.
	if isInsideCastChain(n) || shouldSkipContextualCheck(n, target) {
		return
	}
	if isInDestructuringDeclaration(n) {
		return
	}
	// Argument to overloaded function whose overloads disagree on
	// this argument's parameter type — the cast might be selecting an
	// overload, so skip the contextual check. Mirrors upstream's
	// `isArgumentToOverloadedFunction`.
	if isArgumentToOverloadedFunction(ctx, n, effectiveSrcT) {
		return
	}
	// In a generic call context, a non-literal cast's target shape is
	// what TypeScript will infer the generic parameter from — removing
	// the cast would change inference. Mirrors upstream's gating via
	// `isInGenericContext`. Property values are still checked because
	// upstream's gate explicitly preserves those.
	// Use effectiveSrc (post-widening-unwrap) so chained
	// `'literal' as unknown as T` patterns still see the literal at the
	// bottom.
	inGeneric := isInGenericCallContext(ctx, n)
	if inGeneric && !isLiteralExpression(effectiveSrc) {
		if p := n.Parent(); p == nil || p.Kind() != wrapperchecker.KindPropertyAssignment {
			return
		}
	}
	// `as any` in a generic call context — upstream gates the
	// contextual check under `castIsAny && isInGenericContext`. The
	// cast's any-widening is what's pinning inference, so leave it.
	if inGeneric && target.IsAny() {
		return
	}
	// In a generic call context, casting to a function type shapes the
	// generic parameter's inferred call signature. Removing the cast
	// would let TypeScript infer a different (often broader) shape
	// from the source's static type — keep it.
	if inGeneric {
		targetT := ctx.Checker().TypeFromTypeNode(annot)
		if targetT != nil && len(targetT.CallSignatures()) > 0 {
			return
		}
	}
	if containsAny(effectiveSrcT) {
		return
	}
	if isPropertyInProblematicContext(ctx, n, effectiveSrcT) {
		return
	}
	ctxT := ctx.Checker().ContextualTypeOf(n)
	if ctxT == nil {
		return
	}
	if ctxT.IsAny() {
		// Cast into a position that accepts `any` — flag only when
		// this is a call argument and the cast itself doesn't widen
		// into `any` (so the cast is actually narrowing).
		if !isCallArgument(n) || containsAny(target) {
			return
		}
	} else if !target.IsAny() && !target.IsUnknown() && containsAny(ctxT) {
		// Narrow cast into a position whose contextual type contains
		// `any` somewhere else — keep the cast as documentation.
		return
	}
	if !effectiveSrcT.IsAssignableTo(ctxT) {
		return
	}
	// Generics mismatch: when the contextual type has a property whose
	// call signatures are generic but the source's corresponding
	// property's signatures aren't, the cast is selecting between
	// generic and non-generic shapes — keep it.
	if genericsMismatch(effectiveSrcT, ctxT) {
		return
	}
	// `null as string | null` / `undefined as T | undefined` —
	// asserting a nullish literal to a union doesn't fire upstream.
	// The cast is conventional documentation that the initializer is
	// intentionally nullish.
	if target.IsUnion() && isNullishLiteralExpression(src) {
		return
	}
	ctx.Report(n, "type assertion is unnecessary — the value already satisfies the contextual type")
}

// unwrapWideningCasts peels off `as any` / `as unknown` /
// `<any>x` / `<unknown>x` assertions from expr, returning the
// innermost expression with a "real" type. Returns nil when expr
// isn't such a chain.
func unwrapWideningCasts(expr *wrapperchecker.Node) *wrapperchecker.Node {
	cur := expr
	stripped := false
	for cur != nil {
		var inner, annot *wrapperchecker.Node
		switch cur.Kind() {
		case wrapperchecker.KindAsExpression:
			inner = cur.AsExpressionSource()
			annot = cur.AsExpressionTarget()
		case wrapperchecker.KindTypeAssertionExpression:
			inner = cur.TypeAssertionSource()
			annot = cur.TypeAssertionTarget()
		case wrapperchecker.KindParenthesizedExpression:
			cur = cur.FirstChild()
			continue
		default:
			if stripped {
				return cur
			}
			return nil
		}
		if !isAnyOrUnknownAnnotation(annot) {
			if stripped {
				return cur
			}
			return nil
		}
		stripped = true
		cur = inner
	}
	return nil
}

// shouldSkipContextualCheck reports whether the cast at n sits in a
// position where typescript-eslint declines to fire the contextual
// fallback — destructuring inits, logical-assignment RHS, non-statement
// assignment, spreads, satisfies, etc. These positions either change
// flow typing in ways the cast can legitimately preserve or rely on
// the cast for type-system reasons even when assignability holds.
func shouldSkipContextualCheck(n *wrapperchecker.Node, target *wrapperchecker.Type) bool {
	p := n.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindSpreadElement,
		wrapperchecker.KindSpreadAssignment,
		wrapperchecker.KindSatisfiesExpression:
		return true
	case wrapperchecker.KindBinaryExpression:
		// Logical-assignment RHS (`a ??= rhs`, `a ||= rhs`, `a &&= rhs`)
		// can intentionally widen to add nullable members the source
		// type doesn't surface.
		switch p.BinaryOperatorKind() {
		case wrapperchecker.KindQuestionQuestionEqualsToken,
			wrapperchecker.KindBarBarEqualsToken,
			wrapperchecker.KindAmpersandAmpersandEqualsToken:
			return true
		}
		// `x ?? (rhs as any)` / `x || (rhs as any)`: the `any` widens
		// the union of the logical expression. Upstream skips
		// `as any`/`as unknown` in logical-expression operand position.
		if target != nil && (target.IsAny() || target.IsUnknown()) {
			switch p.BinaryOperatorKind() {
			case wrapperchecker.KindQuestionQuestionToken,
				wrapperchecker.KindBarBarToken,
				wrapperchecker.KindAmpersandAmpersandToken:
				return true
			}
		}
		// `a = b as T` where the assignment expression is itself nested
		// (used as a value, not a statement) — the cast preserves type
		// flow for the outer expression.
		if p.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
			if gp := p.Parent(); gp != nil &&
				gp.Kind() != wrapperchecker.KindExpressionStatement {
				return true
			}
		}
	}
	// Array literal source: `[x] as T[]` style casts of fresh arrays
	// help inference and shouldn't be flagged via contextual fallback,
	// unless the cast widens to `any` — `fn(['hello'] as any)` still
	// fires upstream because the call argument constrains it.
	if src := castSource(n); src != nil &&
		src.Kind() == wrapperchecker.KindArrayLiteralExpression {
		if target == nil || !target.IsAny() {
			return true
		}
	}
	return false
}

// isTypeParamIntersectedWithEmptyObject reports whether `cast` is
// shaped as `T & {}` where uncast is the type parameter T whose
// constraint is non-nullable. TypeScript represents `NonNullable<T>`
// exactly this way.
func isTypeParamIntersectedWithEmptyObject(uncast, cast *wrapperchecker.Type) bool {
	if uncast == nil || cast == nil {
		return false
	}
	if !uncast.IsTypeParameter() {
		return false
	}
	if !cast.IsIntersection() {
		return false
	}
	parts := cast.IntersectionMembers()
	if len(parts) != 2 {
		return false
	}
	hasUncast := false
	var other *wrapperchecker.Type
	for _, p := range parts {
		if p.String() == uncast.String() {
			hasUncast = true
			continue
		}
		other = p
	}
	if !hasUncast || other == nil {
		return false
	}
	// `{}` (the empty object type) has no property names and no call/
	// construct signatures. Generic mapped types like `Record<K, V>`
	// or `{ [P in K]: V }` would also expose no properties but they
	// reference type variables — exclude them.
	if len(other.PropertyNames()) != 0 ||
		len(other.CallSignatures()) != 0 ||
		len(other.ConstructSignatures()) != 0 ||
		other.HasIndexSignature("") {
		return false
	}
	if containsTypeVariable(other) {
		return false
	}
	// Reject types that carry a user-visible alias name (`Record<K, V>`,
	// `Foo<T>` etc.) — `{}` from `T & {}` (NonNullable<T>) is alias-
	// less and anonymous. tsgo's symbol name for anonymous structural
	// types is the internal "\xfetype" marker, not a user-facing name.
	if other.AliasSymbolName() != "" {
		return false
	}
	// Constraint of uncast must exist and be non-nullable.
	c := uncast.BaseConstraint()
	if c == nil {
		return false
	}
	return !typeContainsNullable(c)
}

// containsTypeVariable reports whether t (or any nested member/
// argument) is a type parameter. Mirrors upstream's check used to
// distinguish concrete empty objects (`{}`) from generic mapped types
// (`Record<K, V>`, `{ [P in K]: V }`).
func containsTypeVariable(t *wrapperchecker.Type) (result bool) {
	defer func() {
		if r := recover(); r != nil {
			result = false
		}
	}()
	return typeContains(t, func(t *wrapperchecker.Type) bool {
		return t != nil && t.IsTypeParameter()
	}, map[any]struct{}{})
}

// genericsMismatch reports whether the contextual type has at least
// one property whose call signatures are generic (carry their own
// type parameters) while the source's corresponding property either
// is absent or has only non-generic signatures. The cast there is
// load-bearing — the source can't substitute for a generic API.
func genericsMismatch(uncast, contextual *wrapperchecker.Type) bool {
	if uncast == nil || contextual == nil {
		return false
	}
	defer func() {
		_ = recover()
	}()
	for _, name := range contextual.PropertyNames() {
		ctxPropT := contextual.PropertyType(name)
		if ctxPropT == nil {
			continue
		}
		ctxSigs := ctxPropT.CallSignatures()
		anyCtxGeneric := false
		for _, sig := range ctxSigs {
			if decl := sig.SignatureDeclaration(); decl != nil && signatureHasTypeParameters(decl) {
				anyCtxGeneric = true
				break
			}
		}
		if !anyCtxGeneric {
			continue
		}
		uncastPropT := uncast.PropertyType(name)
		if uncastPropT == nil {
			return true
		}
		anyUncastGeneric := false
		for _, sig := range uncastPropT.CallSignatures() {
			if decl := sig.SignatureDeclaration(); decl != nil && signatureHasTypeParameters(decl) {
				anyUncastGeneric = true
				break
			}
		}
		if !anyUncastGeneric {
			return true
		}
	}
	return false
}

// isArgumentToOverloadedFunction reports whether n is an argument to a
// call whose callee has multiple call signatures and at least one
// signature either lacks this argument position or types it
// incompatibly with the uncasted value. Mirrors upstream.
func isArgumentToOverloadedFunction(ctx *engine.Context, n *wrapperchecker.Node, srcT *wrapperchecker.Type) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	if p.Kind() != wrapperchecker.KindCallExpression &&
		p.Kind() != wrapperchecker.KindNewExpression {
		return false
	}
	args := p.CallArguments()
	argIdx := -1
	for i, a := range args {
		if sameNodeIdentity(a, n) {
			argIdx = i
			break
		}
	}
	if argIdx < 0 {
		return false
	}
	callee := p.CalleeExpression()
	if callee == nil {
		return false
	}
	calleeT := ctx.TypeOf(callee)
	if calleeT == nil {
		return false
	}
	sigs := calleeT.CallSignatures()
	if len(sigs) <= 1 {
		return false
	}
	// Collect this argument's parameter type for each overload. For
	// rest parameters (`...items: T[]`), unwrap to the element type
	// so the comparison reflects what the argument is checked against.
	paramTypes := make([]*wrapperchecker.Type, 0, len(sigs))
	for _, sig := range sigs {
		params := sig.ParameterTypes()
		if argIdx >= len(params) {
			return true
		}
		pt := params[argIdx]
		// If the signature has a rest parameter at the last position
		// and we're at or past it, the actual checked type is the
		// element type of the rest array.
		if sig.HasRestParameter() && argIdx >= len(params)-1 && pt != nil {
			if elem := pt.ArrayElementType(); elem != nil {
				pt = elem
			}
		}
		paramTypes = append(paramTypes, pt)
	}
	// All param types equal → not an overload-discriminating arg.
	first := paramTypes[0]
	allEqual := true
	for _, pt := range paramTypes[1:] {
		if pt == nil || first == nil || pt.String() != first.String() {
			allEqual = false
			break
		}
	}
	if allEqual {
		return false
	}
	if srcT == nil {
		return true
	}
	// Overloads differ — if the uncasted source isn't assignable to
	// every overload's param, the cast is selecting overloads.
	for _, pt := range paramTypes {
		if pt == nil || !srcT.IsAssignableTo(pt) {
			return true
		}
	}
	return false
}

// isInGenericCallContext reports whether n sits inside an
// argument-passing position for a CallExpression/NewExpression whose
// callee has at least one generic call signature. Mirrors upstream's
// `isInGenericContext` heuristic without needing scope analysis: walk
// ancestors, halt at function-with-block-body, and ask the checker
// whether the call's callee is generic.
func isInGenericCallContext(ctx *engine.Context, n *wrapperchecker.Node) bool {
	seenFunction := false
	cur := n.Parent()
	for cur != nil {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration:
			return false
		case wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction:
			if body := cur.FunctionBody(); body != nil && body.Kind() == wrapperchecker.KindBlock {
				return false
			}
			if seenFunction {
				return false
			}
			seenFunction = true
		case wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression:
			callee := cur.CalleeExpression()
			if callee == nil {
				return false
			}
			calleeT := ctx.TypeOf(callee)
			if calleeT != nil && hasGenericCallSignature(calleeT) {
				return true
			}
		}
		cur = cur.Parent()
	}
	return false
}

func hasGenericCallSignature(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	for _, sig := range t.CallSignatures() {
		decl := sig.SignatureDeclaration()
		if decl == nil {
			continue
		}
		if signatureHasTypeParameters(decl) {
			return true
		}
	}
	return false
}

// signatureHasTypeParameters reports whether a function-like
// declaration node carries one or more type parameter declarations
// (e.g. `function f<T>()`). Detects them by walking the immediate
// children for KindTypeParameter nodes.
func signatureHasTypeParameters(decl *wrapperchecker.Node) bool {
	found := false
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindTypeParameter {
			found = true
			return true
		}
		return false
	})
	return found
}

// isInDestructuringDeclaration reports whether n is the initializer
// of a variable declaration that destructures: `const { x } = expr as T`
// or `const [x] = expr as T`. typescript-eslint skips contextual
// checks here because the destructuring pattern doesn't carry an
// annotation that pins the source's full shape.
func isInDestructuringDeclaration(n *wrapperchecker.Node) bool {
	p := n.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil || p.Kind() != wrapperchecker.KindVariableDeclaration {
		return false
	}
	if init := p.VariableDeclarationInitializer(); init == nil ||
		!sameNodeIdentity(init, n) {
		return false
	}
	name := p.VariableDeclarationName()
	if name == nil {
		return false
	}
	switch name.Kind() {
	case wrapperchecker.KindObjectBindingPattern,
		wrapperchecker.KindArrayBindingPattern:
		return true
	}
	return false
}

// isCallArgument reports whether n sits as a direct argument of a
// call or new expression (parens transparent).
func isCallArgument(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil && cur.Kind() == wrapperchecker.KindParenthesizedExpression {
		cur = cur.Parent()
	}
	if cur == nil {
		return false
	}
	return cur.Kind() == wrapperchecker.KindCallExpression ||
		cur.Kind() == wrapperchecker.KindNewExpression
}

// isPropertyInProblematicContext reports whether the cast at n is the
// value of an object-literal property in a position where the cast is
// load-bearing and the contextual check should be skipped. Mirrors
// upstream's gating: union object context narrows the property
// contextual (skip if the non-null contextual is itself a union or
// the source isn't assignable); object literals supplied to a
// `satisfies` operator always skip.
func isPropertyInProblematicContext(ctx *engine.Context, n *wrapperchecker.Node, srcT *wrapperchecker.Type) bool {
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindPropertyAssignment {
		return false
	}
	obj := p.Parent()
	if obj == nil || obj.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	objCtx := ctx.Checker().ContextualTypeOf(obj)
	if objCtx != nil && objCtx.IsUnion() {
		propCtx := ctx.Checker().ContextualTypeOf(n)
		if propCtx == nil {
			return true
		}
		// Approximate `getNonNullableType(propCtx)` by counting
		// non-nullable union members. A propCtx whose non-null part is
		// a true union (≥ 2 members) means the cast may be selecting a
		// member.
		if nonNullableUnionCount(propCtx) >= 2 {
			return true
		}
		return srcT == nil || !srcT.IsAssignableTo(propCtx)
	}
	// Object literal supplied to `satisfies T` (directly or via a
	// single call wrapper like `identity({...}) satisfies T`) — the
	// cast inside the property is load-bearing for the satisfies
	// shape check, so skip.
	parent := obj.Parent()
	if parent == nil {
		return false
	}
	if parent.Kind() == wrapperchecker.KindSatisfiesExpression {
		return true
	}
	if parent.Kind() == wrapperchecker.KindCallExpression {
		if gp := parent.Parent(); gp != nil &&
			gp.Kind() == wrapperchecker.KindSatisfiesExpression {
			return true
		}
	}
	return false
}

// nonNullableUnionCount returns the number of non-nullable members
// of a (possibly union) type. Single non-null types count as 1.
func nonNullableUnionCount(t *wrapperchecker.Type) int {
	if t == nil {
		return 0
	}
	if !t.IsUnion() {
		if t.IsNullOrUndefined() || t.IsVoid() {
			return 0
		}
		return 1
	}
	count := 0
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() || m.IsVoid() {
			continue
		}
		count++
	}
	return count
}

// castSource returns the source-expression of n if n is a cast.
func castSource(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindAsExpression:
		return n.AsExpressionSource()
	case wrapperchecker.KindTypeAssertionExpression:
		return n.TypeAssertionSource()
	}
	return nil
}

// containsAny reports whether t (or any nested member/argument) is
// the `any` type. Mirrors upstream's `containsAny` — `any` anywhere in
// the source type means the cast is doing meaningful narrowing.
// Defensively recovers from panics deep in the type-walk and returns
// false; an over-conservative answer here only suppresses one branch
// of the contextual check.
func containsAny(t *wrapperchecker.Type) (result bool) {
	defer func() {
		if r := recover(); r != nil {
			result = false
		}
	}()
	return typeContains(t, func(t *wrapperchecker.Type) bool {
		return t != nil && t.IsAny()
	}, map[any]struct{}{})
}

// typeContains walks t's structural shape (unions, intersections, type
// arguments) looking for any sub-type that satisfies pred. The seen map
// is keyed on the underlying checker type via t.Inner(): the wrapper's
// UnionMembers / TypeArguments / IntersectionMembers each return fresh
// wrapper instances around the same underlying types, so keying on the
// wrapper pointer fails to detect cycles and lets recursion blow the
// goroutine stack on legitimate recursive shapes (jetlint#624).
func typeContains(t *wrapperchecker.Type, pred func(*wrapperchecker.Type) bool, seen map[any]struct{}) bool {
	if t == nil {
		return false
	}
	key := any(t.Inner())
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	if pred(t) {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContains(m, pred, seen) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if typeContains(m, pred, seen) {
				return true
			}
		}
	}
	for _, a := range t.TypeArguments() {
		if typeContains(a, pred, seen) {
			return true
		}
	}
	return false
}

// isInsideCastChain reports whether n is the (possibly parenthesized)
// source of an enclosing AsExpression or TypeAssertionExpression. In
// chained casts like `(5 as any) as 5`, the inner cast should not be
// flagged independently — the outer cast already captures the issue.
func isInsideCastChain(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil && cur.Kind() == wrapperchecker.KindParenthesizedExpression {
		cur = cur.Parent()
	}
	if cur == nil {
		return false
	}
	switch cur.Kind() {
	case wrapperchecker.KindAsExpression, wrapperchecker.KindTypeAssertionExpression:
		return true
	}
	return false
}

// isAnyOrUnknownAnnotation reports whether the type-node names the
// `any` or `unknown` widening keyword.
func isAnyOrUnknownAnnotation(annot *wrapperchecker.Node) bool {
	if annot == nil {
		return false
	}
	switch annot.Kind() {
	case wrapperchecker.KindAnyKeyword,
		wrapperchecker.KindUnknownKeyword:
		return true
	}
	return false
}

// targetIsIgnored reports whether the assertion's target type-node
// names a type listed in TypesToIgnore. The match is by source text
// of the type-node so users can write `Foo`, `Foo<T>`, or qualified
// names and have the option apply consistently with upstream's
// regex-style match.
func (r *rule) targetIsIgnored(annot *wrapperchecker.Node) bool {
	if len(r.opts.TypesToIgnore) == 0 || annot == nil {
		return false
	}
	text := annot.SourceText()
	for _, ignored := range r.opts.TypesToIgnore {
		if text == ignored {
			return true
		}
	}
	return false
}

// isLiteralLikeType reports whether t is a literal type — boolean/
// string/number/bigint literal, enum member, or tuple. Mirrors
// upstream's `isTypeLiteral` which uses TypeScript's `type.isLiteral()`
// (which includes enum-literal). Used by the `as const` check to fire
// when the cast preserves an already-literal value.
func isLiteralLikeType(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsTupleType() {
		return true
	}
	if t.IsEnumLike() {
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

// isLiteralOrTupleAssertion reports whether the assertion target is
// a literal type, tuple, or readonly tuple. These can preserve
// narrower typing at widening boundaries even when the source's
// inferred type currently matches.
func isLiteralOrTupleAssertion(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsTupleType() {
		return true
	}
	// Enum members are number/string-like but their literal-ness comes
	// from the enum declaration, not from a source-position literal —
	// the cast `a as T.Value1` is a real assignment, not a widening
	// boundary. Treat enum members as non-literal so the regular
	// identity check runs.
	if t.IsEnumLike() {
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

func isAsConst(annot *wrapperchecker.Node) bool {
	// `as const` has TypeReference shape with the `const` keyword.
	if annot.Kind() != wrapperchecker.KindTypeReference {
		return false
	}
	var name string
	annot.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && name == "" {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name == "const"
}

// isInWideningPosition reports whether n's parent context would widen
// a literal type if the assertion were removed. Object/array literal
// elements widen unless the containing literal has an explicit
// contextual type. `let`/`var` initializers widen too. Conservative
// heuristic: walk through parens to the structural parent and check
// for these positions; assume "yes, widens" for ambiguous cases so we
// don't false-flag.
func isInWideningPosition(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil && cur.Kind() == wrapperchecker.KindParenthesizedExpression {
		cur = cur.Parent()
	}
	if cur == nil {
		return false
	}
	switch cur.Kind() {
	case wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment,
		wrapperchecker.KindArrayLiteralExpression:
		return true
	case wrapperchecker.KindPropertyDeclaration:
		// `readonly a = 3 as 3` doesn't widen — the readonly modifier
		// pins the literal at its narrowest type, so the cast adds
		// nothing.
		if cur.HasReadonlyModifier() {
			return false
		}
		return true
	case wrapperchecker.KindVariableDeclaration:
		// `let x = 'foo' as 'foo'` — widens unless x is `const` or has
		// an explicit annotation. `const x = 'foo' as 'foo'` doesn't
		// widen, so the assertion is redundant — return false there.
		// We conservatively return true (don't flag) when we can't
		// distinguish — gopls' parent traversal of VariableDeclaration
		// is ambiguous without checking the parent VariableDeclarationList.
		if list := cur.Parent(); list != nil {
			if isConstVariableDeclarationList(list) {
				return false
			}
			return true
		}
		return true
	}
	return false
}

func isConstVariableDeclarationList(list *wrapperchecker.Node) bool {
	if list == nil || list.Kind() != wrapperchecker.KindVariableDeclarationList {
		return false
	}
	// `const` flag isn't directly exposed; approximate by source text.
	t := list.SourceText()
	for i := 0; i+5 <= len(t); i++ {
		if t[i] == 'c' && t[i:i+5] == "const" {
			return true
		}
	}
	return false
}

// isIIFEWithoutReturnAnnotation reports whether n is a call expression
// whose callee is an inline arrow or function expression with no
// return type annotation. TypeScript widens such IIFE return types in
// ways the cast-removal check can't always undo, so we leave casts on
// these expressions alone.
func isIIFEWithoutReturnAnnotation(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := n.CalleeExpression()
	for callee != nil && callee.Kind() == wrapperchecker.KindParenthesizedExpression {
		callee = callee.FirstChild()
	}
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		return callee.FunctionReturnType() == nil
	}
	return false
}

// objectLiteralHasProperties reports whether n is a non-empty object
// literal — i.e. has at least one property assignment or shorthand
// entry. Empty `{}` typed against a structurally-equivalent target
// usually serves a documentation purpose.
func objectLiteralHasProperties(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindSpreadAssignment,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			found = true
			return true
		}
		return false
	})
	return found
}

// hasLiteralProperty reports whether t has any property whose type is
// a literal (`'foo'`, `42`, `true`). Casts that *preserve* literal
// inference at a widening boundary are meaningful — the implicit type
// at the source position would widen the property away.
func hasLiteralProperty(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	for _, name := range t.PropertyNames() {
		pt := t.PropertyType(name)
		if pt == nil {
			continue
		}
		if isLiteralOrTupleAssertion(pt) {
			return true
		}
	}
	return false
}

// hasSameProperties reports whether a and b share the same set of
// property names. Used to gate the structural identity check at
// widening boundaries (object literal cast to a named type) to avoid
// false positives when properties differ.
func hasSameProperties(a, b *wrapperchecker.Type) bool {
	if a == nil || b == nil {
		return false
	}
	aNames := a.PropertyNames()
	bNames := b.PropertyNames()
	if len(aNames) != len(bNames) {
		return false
	}
	bSet := make(map[string]struct{}, len(bNames))
	for _, n := range bNames {
		bSet[n] = struct{}{}
	}
	for _, n := range aNames {
		if _, ok := bSet[n]; !ok {
			return false
		}
	}
	return true
}

// isNullishLiteralExpression reports whether n is the bare `null`
// keyword or the `undefined` identifier. Such literals cast to a union
// preserve readable intent at variable initializers.
func isNullishLiteralExpression(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindNullKeyword {
		return true
	}
	if n.Kind() == wrapperchecker.KindIdentifier && n.LiteralText() == "undefined" {
		return true
	}
	return false
}

func isLiteralExpression(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		return true
	}
	return false
}

func visitNonNull(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	// `=`-assignment special case (mirrors upstream): on the LHS,
	// always report ("contextuallyUnnecessary"). On the RHS, skip
	// entirely — the assertion changes type-flow for code that runs
	// after the assignment so it isn't strictly redundant.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindBinaryExpression &&
		p.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
		if l := p.BinaryLeft(); l != nil && sameNodeIdentity(l, n) {
			ctx.Report(n, "non-null assertion is unnecessary — the assignment target already accepts the value's nullable members")
		}
		return
	}
	// The non-null assertion is redundant when the surrounding context
	// already accepts the source's full type (`let y: T | null = x!`
	// where `x: T | null` discards the assertion's effect; `nonNull(x!)`
	// when `nonNull` takes the same `T | null`). Use assignability —
	// if the unnarrowed source already fits the contextual type, the
	// `!` was a no-op.
	if ctxT := contextualNullableType(ctx, n); ctxT != nil &&
		typeContainsNullable(ctxT) && t.IsAssignableTo(ctxT) {
		ctx.Report(n, "non-null assertion is unnecessary — the contextual type already accepts the value's nullable members")
		return
	}
	// `unknown` includes null/undefined but the assertion still
	// narrows callers' usage — typescript-eslint doesn't flag.
	if t.IsUnknown() || t.IsAny() {
		return
	}
	// `void` carries undefined-as-the-only-inhabitant semantics —
	// `foo()!` over a `void`-returning call is treated as legitimate.
	if t.IsVoid() {
		return
	}
	if !t.IsNullOrUndefined() && !typeContainsNullable(t) {
		if isPossiblyUsedBeforeAssigned(ctx, expr, t) {
			// `let bar: number; bar!;` — bar's static type is number
			// but TS's definite-assignment analysis would error
			// without the `!`. The cast is suppressing a real
			// diagnostic, so don't flag it.
			return
		}
		ctx.Report(n, "non-null assertion is unnecessary — the value is already non-nullable")
	}
}

// isVarDeclaredInNestedBlock reports whether a `var` declaration node
// sits inside a nested block (deeper than the enclosing function /
// source file) that does NOT contain useSite. var-declarations hoist
// to the containing function/global scope, so the binding is visible
// outside the declaring block — but TypeScript treats the variable as
// possibly-undefined at uses that don't lexically follow the
// assignment. The `!` there suppresses a real "used before assigned"
// diagnostic, so don't flag it.
func isVarDeclaredInNestedBlock(decl, useSite *wrapperchecker.Node) bool {
	if decl == nil || useSite == nil {
		return false
	}
	list := decl.Parent()
	if list == nil || list.Kind() != wrapperchecker.KindVariableDeclarationList {
		return false
	}
	stmt := list.Parent()
	if stmt == nil {
		return false
	}
	// Walk up from the VariableStatement looking for a Block whose
	// parent is NOT another Block / function-like / source-file. A
	// nesting Block means the declaration is inside a scoped chunk.
	for cur := stmt.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindSourceFile,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			// Reached function-/file-scope without hitting a Block —
			// the declaration is at the same scope as the use.
			return false
		case wrapperchecker.KindBlock:
			// Nested block. Is useSite inside this block? If yes, the
			// declaration is in scope at the use point. If no, the use
			// is outside the declarator's block — flow analysis would
			// treat the binding as possibly-undefined.
			if !nodeIsDescendantOf(useSite, cur) {
				return true
			}
		}
	}
	return false
}

func nodeIsDescendantOf(n, anc *wrapperchecker.Node) bool {
	for cur := n; cur != nil; cur = cur.Parent() {
		if sameNodeIdentity(cur, anc) {
			return true
		}
	}
	return false
}

// isPossiblyUsedBeforeAssigned reports whether expr is a bare
// identifier that references a `let`/`var` declared without an
// initializer (and without `declare`). At a use site reached before
// the variable is definitely assigned, TypeScript would error without
// a non-null assertion. We can't run flow analysis here, so be
// conservative: treat any such identifier-of-uninitialized-let/var
// as possibly-used-before-assigned.
func isPossiblyUsedBeforeAssigned(ctx *engine.Context, expr *wrapperchecker.Node, currentT *wrapperchecker.Type) bool {
	if expr == nil || expr.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	sym := ctx.Checker().SymbolOf(expr)
	if sym == nil {
		return false
	}
	decls := sym.Declarations()
	if len(decls) == 0 {
		return false
	}
	for _, decl := range decls {
		if decl == nil {
			continue
		}
		if decl.Kind() != wrapperchecker.KindVariableDeclaration {
			return false
		}
		// `var x = 1` inside a block that doesn't contain the use site
		// hoists the binding but not the assignment — the use is
		// possibly-undefined and the `!` suppresses a real diagnostic.
		if isVarDeclaredInNestedBlock(decl, expr) {
			return true
		}
		if decl.VariableDeclarationInitializer() != nil {
			return false
		}
		annot := decl.VariableDeclarationType()
		if annot == nil {
			return false
		}
		// If the declared type still equals the type at the use site,
		// no flow narrowing has happened and the `!` is suppressing a
		// real "used before assigned" diagnostic. If flow has narrowed
		// the type (e.g. an assignment between decl and use), the `!`
		// adds nothing.
		if currentT != nil {
			if declT := ctx.Checker().TypeFromTypeNode(annot); declT != nil &&
				declT.String() != currentT.String() {
				return false
			}
		}
		list := decl.Parent()
		if list == nil || list.Kind() != wrapperchecker.KindVariableDeclarationList {
			return false
		}
		// `const x: T` initializers are required, so this branch can
		// only come up for `let`/`var`. The declaration list's source
		// text is the simplest discriminator the wrapper exposes.
		if isConstVariableDeclarationList(list) {
			return false
		}
		// `declare const`/`declare let` is ambient — the runtime
		// guarantees a value at the use site.
		if stmt := list.Parent(); stmt != nil && stmt.HasDeclareModifier() {
			return false
		}
		// `let x!: T;` carries a definite-assignment assertion; TS
		// treats x as assigned. The wrapper doesn't expose the `!:`
		// token directly, so detect it from the source text between
		// the identifier and the type annotation.
		if hasDefiniteAssignmentAssertion(decl) {
			return false
		}
	}
	return true
}

// hasDefiniteAssignmentAssertion reports whether a VariableDeclaration
// node was written with `!:` between the identifier and the type. The
// wrapper doesn't expose the AST exclamation-token field, so detect it
// from the declaration's source text.
func hasDefiniteAssignmentAssertion(decl *wrapperchecker.Node) bool {
	t := decl.SourceText()
	for i := 0; i+1 < len(t); i++ {
		if t[i] == '!' && t[i+1] == ':' {
			return true
		}
	}
	return false
}

// contextualNullableType returns the type the assertion's value will be
// assigned/passed into, where defined. For arguments this is the
// parameter's type; for variable declarations the annotation; etc.
func contextualNullableType(ctx *engine.Context, n *wrapperchecker.Node) *wrapperchecker.Type {
	p := n.Parent()
	if p == nil {
		return nil
	}
	// JsxExpression wrapper (`{expr}` inside JSX) — the contextual is
	// the JSX attribute's declared type.
	if p.Kind() == wrapperchecker.KindJsxExpression {
		return ctx.Checker().ContextualTypeOf(p)
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration:
		// `const y: T = x!` — annotation type, if present.
		if t := p.VariableDeclarationType(); t != nil {
			return ctx.Checker().TypeFromTypeNode(t)
		}
	case wrapperchecker.KindPropertyDeclaration:
		if t := p.PropertyDeclarationType(); t != nil {
			return ctx.Checker().TypeFromTypeNode(t)
		}
	case wrapperchecker.KindParameter:
		if t := p.ParameterTypeAnnotation(); t != nil {
			return ctx.Checker().TypeFromTypeNode(t)
		}
	case wrapperchecker.KindCallExpression, wrapperchecker.KindNewExpression:
		// Find the argument index of `n` and ask the checker.
		args := p.CallArguments()
		for i, a := range args {
			if sameNodeIdentity(a, n) {
				return ctx.Checker().ContextualTypeForArgument(p, i)
			}
		}
	case wrapperchecker.KindBinaryExpression:
		if p.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
			if l := p.BinaryLeft(); l != nil && !sameNodeIdentity(l, n) {
				return ctx.TypeOf(l)
			}
		}
	case wrapperchecker.KindReturnStatement:
		// Return type — best-effort: get the contextual type of the
		// return expression itself.
		return ctx.Checker().ContextualTypeOf(n)
	}
	return ctx.Checker().ContextualTypeOf(n)
}

func sameNodeIdentity(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	af, asl, asc, ael, aec := a.SourceRange()
	bf, bsl, bsc, bel, bec := b.SourceRange()
	return af == bf && asl == bsl && asc == bsc && ael == bel && aec == bec
}

func typeContainsNullable(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsNullOrUndefined() || t.IsVoid() || t.IsUnknown() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContainsNullable(m) {
				return true
			}
		}
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeContainsNullable(c)
		}
	}
	return false
}
