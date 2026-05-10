// Package nounnecessarytypeassertion implements the
// no-unnecessary-type-assertion rule: flag `x as T` when x is
// already exactly T, and `x!` when x's type is already non-nullable.
package nounnecessarytypeassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
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
	// expressions in non-widening positions where the `const` keyword
	// adds nothing the const-context wouldn't already give.
	if isAsConst(annot) {
		if !r.opts.CheckLiteralConstAssertions {
			return
		}
		if !isLiteralExpression(src) || isInWideningPosition(n) {
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
	// Contextually unnecessary: when the surrounding position only
	// requires the source's narrower type, the cast adds nothing the
	// type system would otherwise reject. Mirrors typescript-eslint's
	// `contextuallyUnnecessary` branch.
	if isInsideCastChain(n) || shouldSkipContextualCheck(n, target) {
		return
	}
	if containsAny(effectiveSrcT) {
		// `any`-tainted source narrows usefully through any cast — the
		// cast is the only thing surfacing a real shape to the reader.
		// Use the unwrapped source so chained `as any as T` doesn't
		// look any-tainted just because the intermediate hop is `any`.
		return
	}
	// Property value in a problematic position: the property's contextual
	// type may not pin the source enough to skip the cast.
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
// value of an object-literal property where the property's contextual
// type is itself a union (or absent), or where the source isn't
// assignable to the property's non-nullable contextual type. Mirrors
// upstream's gating: in those positions the cast may be selecting a
// union member's shape, so we skip the contextual check.
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
	if objCtx == nil || !objCtx.IsUnion() {
		return false
	}
	propCtx := ctx.Checker().ContextualTypeOf(n)
	if propCtx == nil {
		return true
	}
	if propCtx.IsUnion() {
		// Union property type — the cast can be narrowing to a member.
		return true
	}
	// Single-shape property contextual: only problematic when the
	// source doesn't already fit, so the cast is doing real work.
	return srcT == nil || !srcT.IsAssignableTo(propCtx)
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
func containsAny(t *wrapperchecker.Type) bool {
	return typeContains(t, func(t *wrapperchecker.Type) bool {
		return t != nil && t.IsAny()
	}, map[*wrapperchecker.Type]struct{}{})
}

func typeContains(t *wrapperchecker.Type, pred func(*wrapperchecker.Type) bool, seen map[*wrapperchecker.Type]struct{}) bool {
	if t == nil {
		return false
	}
	if _, ok := seen[t]; ok {
		return false
	}
	seen[t] = struct{}{}
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
