// Package prefernullishcoalescing implements the
// prefer-nullish-coalescing rule: flag `x || y` where x is nullable —
// the `??` operator handles only null/undefined and avoids
// unintentionally treating other falsy values as the fallback trigger.
package prefernullishcoalescing

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-nullish-coalescing"

// Options configures the rule. See the upstream defaults at
// https://typescript-eslint.io/rules/prefer-nullish-coalescing.
type Options struct {
	IgnoreBooleanCoercion         bool
	IgnoreConditionalTests        bool
	IgnoreIfStatements            bool
	IgnoreMixedLogicalExpressions bool
	IgnorePrimitives              IgnorePrimitives
	IgnoreTernaryTests            bool
}

// IgnorePrimitives configures which primitive constituents disable the
// rule when they appear in the nullable union — set a flag to true to
// skip the report when that primitive is one of the union's members.
type IgnorePrimitives struct {
	Boolean bool
	BigInt  bool
	Number  bool
	String  bool
}

func DefaultOptions() Options {
	return Options{IgnoreConditionalTests: true}
}

func New() engine.Rule                        { return &rule{opts: DefaultOptions()} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression:      r.visit,
		wrapperchecker.KindConditionalExpression: r.visitTernary,
		wrapperchecker.KindIfStatement:           r.visitIfAssignment,
	}
}

// visitIfAssignment recognises lazy-initialization patterns equivalent
// to `subject ??= rhs`:
//
//	if (subject == null) subject = rhs;
//	if (!subject)        subject = rhs;
//	if (subject === null) subject = rhs;        // type adds only null
//	if (subject === undefined) subject = rhs;   // type adds only undef
//
// The body must be a single assignment / compound assignment to the
// same subject. `if`/`else` chains and bodies with side-effectful
// extras don't qualify.
func (r *rule) visitIfAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.IgnoreIfStatements {
		return
	}
	if n.IfElse() != nil {
		return
	}
	cond := n.IfCondition()
	if cond == nil {
		return
	}
	subject, ok := matchIfNullishCondition(unparen(cond), ctx)
	if !ok {
		return
	}
	body := n.IfThen()
	assign := singleAssignmentInBlock(body)
	if assign == nil {
		return
	}
	left := unparen(assign.BinaryLeft())
	if !sameSubject(subject, left) {
		return
	}
	ctx.Report(n, "use ??= for lazy initialisation against null/undefined")
}

func matchIfNullishCondition(cond *wrapperchecker.Node, ctx *engine.Context) (*wrapperchecker.Node, bool) {
	if cond == nil {
		return nil, false
	}
	// `!subject` form — subject must be nullable.
	if cond.Kind() == wrapperchecker.KindPrefixUnaryExpression && cond.PrefixUnaryOperator() == "!" {
		operand := unparen(cond.PrefixUnaryOperand())
		if operand == nil {
			return nil, false
		}
		t := ctx.TypeOf(operand)
		if t == nil || !typeIsNullable(t) {
			return nil, false
		}
		return operand, true
	}
	// Binary nullish comparison form.
	if cond.Kind() != wrapperchecker.KindBinaryExpression {
		return nil, false
	}
	// Disjunctive form: `foo === null || foo === undefined`.
	if cond.BinaryOperatorKind() == wrapperchecker.KindBarBarToken {
		if subject, isNeg, ok := analyseTernaryNullCheck(cond); ok && isNeg {
			return subject, true
		}
		return nil, false
	}
	subject, k, ok := matchNullishComparison(cond)
	if !ok {
		return nil, false
	}
	// We want the truthy branch to be the nullish case: condition true
	// → subject is nullish → assign rhs.
	if k.negative {
		return nil, false
	}
	if k.loose {
		return subject, true
	}
	t := ctx.TypeOf(subject)
	if t == nil || !t.IsUnion() {
		return nil, false
	}
	hasNull := false
	hasUndef := false
	hasOther := false
	for _, m := range t.UnionMembers() {
		switch {
		case m.IsNull():
			hasNull = true
		case m.IsUndefined():
			hasUndef = true
		default:
			hasOther = true
		}
	}
	if !hasOther {
		return nil, false
	}
	switch k.kind {
	case "null":
		if !hasNull || hasUndef {
			return nil, false
		}
	case "undefined":
		if !hasUndef || hasNull {
			return nil, false
		}
	}
	return subject, true
}

// singleAssignmentInBlock returns the assignment expression of a
// single-statement block or expression statement, or nil otherwise.
func singleAssignmentInBlock(body *wrapperchecker.Node) *wrapperchecker.Node {
	if body == nil {
		return nil
	}
	stmt := body
	if body.Kind() == wrapperchecker.KindBlock {
		stmts := body.BlockStatements()
		if len(stmts) != 1 {
			return nil
		}
		stmt = stmts[0]
	}
	if stmt == nil || stmt.Kind() != wrapperchecker.KindExpressionStatement {
		return nil
	}
	expr := stmt.ExpressionStatementExpression()
	if expr == nil || expr.Kind() != wrapperchecker.KindBinaryExpression {
		return nil
	}
	switch expr.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return expr
	}
	return nil
}

// visitTernary detects ternary patterns equivalent to `a ?? b`:
//   x !== null && x !== undefined ? x : y
//   x !== undefined && x !== null ? x : y
//   x === null || x === undefined ? y : x
//   x === undefined || x === null ? y : x
//   x !== null ? x : y / x === null ? y : x  (when x's type only adds null)
//   x !== undefined ? x : y                  (when x's type only adds undefined)
func (r *rule) visitTernary(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.IgnoreTernaryTests {
		return
	}
	if r.opts.IgnoreConditionalTests && isInConditionalTestPosition(n) {
		return
	}
	// `ignoreBooleanCoercion` skips ternaries only when they are arms of
	// a `||`/`&&` chain that is itself being boolean-coerced. A ternary
	// directly inside `Boolean(...)` is still flagged because the
	// suggested `??` form is exactly equivalent under coercion.
	if r.opts.IgnoreBooleanCoercion && ternaryInsideLogicalCoercion(n) {
		return
	}
	cond := n.ConditionalCondition()
	if cond == nil {
		return
	}
	thenBranch, elseBranch := n.ConditionalBranches()
	if thenBranch == nil || elseBranch == nil {
		return
	}
	subject, isNegated, ok := analyseTernaryNullCheck(cond)
	if !ok {
		subject, isNegated, ok = analyseSingleTernaryNullCheck(cond, ctx)
	}
	if !ok {
		subject, isNegated, ok = analyseImplicitTernaryNullCheck(cond, thenBranch, elseBranch, ctx)
		if !ok {
			return
		}
	}
	// When isNegated is false, the condition is truthy when subject
	// is non-nullish, so `cond ? subject : fallback`. When negated,
	// the layout is reversed: `cond ? fallback : subject`.
	wantSubject := thenBranch
	if isNegated {
		wantSubject = elseBranch
	}
	if !sameSubject(subject, wantSubject) {
		return
	}
	// Honor ignorePrimitives — when a primitive constituent is ignored,
	// any equivalent ternary on that union is also skipped.
	if t := ctx.TypeOf(subject); t != nil && r.shouldIgnoreByPrimitives(t) {
		return
	}
	ctx.Report(n, "use ?? instead of a manual null/undefined ternary check")
}

// analyseTernaryNullCheck recognises an explicit nullish guard and
// returns the subject expression plus a flag indicating whether the
// guard form is negated (`=== null/undefined` joined by `||`) versus
// non-negated (`!== null/undefined` joined by `&&`).
func analyseTernaryNullCheck(cond *wrapperchecker.Node) (*wrapperchecker.Node, bool, bool) {
	cond = unparen(cond)
	if cond == nil || cond.Kind() != wrapperchecker.KindBinaryExpression {
		return nil, false, false
	}
	op := cond.BinaryOperatorKind()
	switch op {
	case wrapperchecker.KindAmpersandAmpersandToken:
		// Each side: `subject !== null` / `subject !== undefined` /
		// `typeof subject !== "undefined"`.
		left := unparen(cond.BinaryLeft())
		right := unparen(cond.BinaryRight())
		ls, lk, lok := matchNullishComparison(left)
		rs, rk, rok := matchNullishComparison(right)
		if !lok || !rok || !sameSubject(ls, rs) {
			return nil, false, false
		}
		if !lk.negative || !rk.negative {
			return nil, false, false
		}
		if lk.kind == rk.kind {
			return nil, false, false
		}
		return ls, false, true
	case wrapperchecker.KindBarBarToken:
		left := unparen(cond.BinaryLeft())
		right := unparen(cond.BinaryRight())
		ls, lk, lok := matchNullishComparison(left)
		rs, rk, rok := matchNullishComparison(right)
		if !lok || !rok || !sameSubject(ls, rs) {
			return nil, false, false
		}
		if lk.negative || rk.negative {
			return nil, false, false
		}
		if lk.kind == rk.kind {
			return nil, false, false
		}
		return ls, true, true
	}
	return nil, false, false
}

type nullishKind struct {
	negative bool   // `!==` vs `===`
	kind     string // "null" or "undefined"
	loose    bool   // `==` / `!=` (loose) vs `===` / `!==` (strict)
}

// analyseImplicitTernaryNullCheck recognises the implicit truthy/falsy
// form of nullish guarding:
//
//	subject ? subject : fallback     → subject ?? fallback
//	!subject ? fallback : subject    → subject ?? fallback
//
// Applies when subject's type contains null or undefined.
func analyseImplicitTernaryNullCheck(cond, thenB, elseB *wrapperchecker.Node, ctx *engine.Context) (*wrapperchecker.Node, bool, bool) {
	cond = unparen(cond)
	if cond == nil {
		return nil, false, false
	}
	negated := false
	if cond.Kind() == wrapperchecker.KindPrefixUnaryExpression && cond.PrefixUnaryOperator() == "!" {
		cond = unparen(cond.PrefixUnaryOperand())
		negated = true
	}
	if cond == nil {
		return nil, false, false
	}
	subject := cond
	wantBranch := thenB
	if negated {
		wantBranch = elseB
	}
	if !sameSubject(subject, wantBranch) {
		return nil, false, false
	}
	// Reject side-effectful subjects: `f() ? f() : y` evaluates `f`
	// twice but `f() ?? y` evaluates it once, so they aren't equivalent.
	if containsCallOrNew(subject) {
		return nil, false, false
	}
	t := ctx.TypeOf(subject)
	if t == nil || !typeIsNullable(t) {
		return nil, false, false
	}
	return subject, negated, true
}

func containsCallOrNew(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindCallExpression, wrapperchecker.KindNewExpression:
		return true
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if containsCallOrNew(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// analyseSingleTernaryNullCheck recognises a single nullish guard:
//
//	x != null          → loose, covers both nullish constituents
//	x != undefined     → loose, covers both nullish constituents
//	x !== null         → strict, only when subject's type's nullish is null
//	x !== undefined    → strict, only when subject's type's nullish is undefined
//
// The companion `===` / `==` forms negate the branch direction.
func analyseSingleTernaryNullCheck(cond *wrapperchecker.Node, ctx *engine.Context) (*wrapperchecker.Node, bool, bool) {
	subject, k, ok := matchNullishComparison(cond)
	if !ok {
		return nil, false, false
	}
	// Loose comparison covers both nullish kinds — always reportable.
	if k.loose {
		return subject, !k.negative, true
	}
	// Strict comparison: only reportable when subject's nullish set is
	// exactly the matched kind. We need the subject's type for that.
	t := ctx.TypeOf(subject)
	if t == nil || !t.IsUnion() {
		return nil, false, false
	}
	hasNull := false
	hasUndef := false
	hasOther := false
	for _, m := range t.UnionMembers() {
		switch {
		case m.IsNull():
			hasNull = true
		case m.IsUndefined():
			hasUndef = true
		default:
			hasOther = true
		}
	}
	if !hasOther {
		return nil, false, false
	}
	switch k.kind {
	case "null":
		if !hasNull || hasUndef {
			return nil, false, false
		}
	case "undefined":
		if !hasUndef || hasNull {
			return nil, false, false
		}
	}
	return subject, !k.negative, true
}

// matchNullishComparison parses `subject !== null`, `subject !== undefined`,
// `typeof subject !== 'undefined'`, and the `===` counterparts.
func matchNullishComparison(n *wrapperchecker.Node) (*wrapperchecker.Node, nullishKind, bool) {
	if n == nil || n.Kind() != wrapperchecker.KindBinaryExpression {
		return nil, nullishKind{}, false
	}
	var negative, loose bool
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsEqualsEqualsToken:
		negative, loose = false, false
	case wrapperchecker.KindEqualsEqualsToken:
		negative, loose = false, true
	case wrapperchecker.KindExclamationEqualsEqualsToken:
		negative, loose = true, false
	case wrapperchecker.KindExclamationEqualsToken:
		negative, loose = true, true
	default:
		return nil, nullishKind{}, false
	}
	left := unparen(n.BinaryLeft())
	right := unparen(n.BinaryRight())
	subject, kindLit, ok := pickSubjectAndLiteral(left, right)
	if !ok {
		return nil, nullishKind{}, false
	}
	return subject, nullishKind{negative: negative, kind: kindLit, loose: loose}, true
}

func pickSubjectAndLiteral(a, b *wrapperchecker.Node) (*wrapperchecker.Node, string, bool) {
	if k, ok := identifyNullishLiteral(a); ok {
		return b, k, true
	}
	if k, ok := identifyNullishLiteral(b); ok {
		return a, k, true
	}
	return nil, "", false
}

func identifyNullishLiteral(n *wrapperchecker.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case wrapperchecker.KindNullKeyword:
		return "null", true
	case wrapperchecker.KindIdentifier:
		if n.LiteralText() == "undefined" {
			return "undefined", true
		}
	}
	return "", false
}

func unparen(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// sameSubject reports whether two expressions textually denote the
// same subject. We compare source text so member chains like
// `x.y[0].z` match across the three positions they occupy in the
// upstream `subject !== null && subject !== undefined ? subject : y`
// pattern. Optional-chain markers (`?.`) are normalised away so
// `x.n?.a`, `x?.n?.a`, and `x.n.a` compare equal — the ternary nullish
// guard treats them as the same subject.
func sameSubject(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	a = unparen(a)
	b = unparen(b)
	at := normalizeChain(a.SourceText())
	bt := normalizeChain(b.SourceText())
	if at != "" && at == bt {
		return true
	}
	if a.Kind() == wrapperchecker.KindIdentifier &&
		b.Kind() == wrapperchecker.KindIdentifier {
		return a.LiteralText() == b.LiteralText()
	}
	return false
}

func normalizeChain(s string) string {
	if s == "" {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', ')', ' ', '\t', '\n':
			// Drop parens and whitespace so `(x?.n).a` matches `x.n?.a`
			// after the `?` normalisation below.
			continue
		case '?':
			// `?.` is the optional-chain marker. Followed by `[` or `(`
			// it denotes optional element/call and the `.` itself isn't
			// part of the non-optional form (`x?.[k]` ↔ `x[k]`);
			// followed by an identifier it denotes optional member
			// access (`x?.b` ↔ `x.b`).
			if i+1 < len(s) && s[i+1] == '.' {
				if i+2 < len(s) && (s[i+2] == '[' || s[i+2] == '(') {
					i++ // skip `?.` together
					continue
				}
				continue // drop `?` only, leave the `.`
			}
		case '[':
			// Convert `['identname']` to `.identname` so bracket access
			// with a string-literal identifier matches the dot form.
			if end, name, ok := stringLiteralIdentBracket(s, i); ok {
				out = append(out, '.')
				out = append(out, name...)
				i = end
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}

// stringLiteralIdentBracket recognises `['identifier']` or
// `["identifier"]` starting at i (the `[`). Returns the index of the
// closing `]` and the unquoted identifier text. Only matches when the
// quoted contents form a valid JS identifier so we don't accidentally
// merge `x['a-b']` into `x.a-b`.
func stringLiteralIdentBracket(s string, i int) (int, string, bool) {
	if i+2 >= len(s) {
		return 0, "", false
	}
	q := s[i+1]
	if q != '\'' && q != '"' {
		return 0, "", false
	}
	j := i + 2
	for j < len(s) && s[j] != q {
		c := s[j]
		valid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c == '_') || (c == '$') ||
			(j > i+2 && c >= '0' && c <= '9')
		if !valid {
			return 0, "", false
		}
		j++
	}
	if j >= len(s) || j+1 >= len(s) || s[j+1] != ']' {
		return 0, "", false
	}
	if j == i+2 {
		return 0, "", false // empty literal — not an identifier
	}
	return j + 1, s[i+2 : j], true
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	if op != wrapperchecker.KindBarBarToken && op != wrapperchecker.KindBarBarEqualsToken {
		return
	}
	if r.opts.IgnoreConditionalTests && isInConditionalTestPosition(n) {
		return
	}
	if r.opts.IgnoreMixedLogicalExpressions && isMixedLogicalExpression(n) {
		return
	}
	if r.opts.IgnoreBooleanCoercion && isInBooleanCoercion(n) {
		return
	}
	left := n.BinaryLeft()
	if left == nil {
		return
	}
	t := ctx.TypeOf(left)
	if t == nil {
		return
	}
	if !typeIsNullable(t) {
		return
	}
	if r.shouldIgnoreByPrimitives(t) {
		return
	}
	if op == wrapperchecker.KindBarBarEqualsToken {
		ctx.Report(n, "use ??= for nullable values; ||= treats other falsy values as missing too")
		return
	}
	ctx.Report(n, "use ?? for nullable values; || treats other falsy values as missing too")
}

func (r *rule) shouldIgnoreByPrimitives(t *wrapperchecker.Type) bool {
	// `any` and `unknown` could hold any primitive — match the
	// ignore-primitives carve-out when any of its switches is on.
	if t.IsAny() || t.IsUnknown() {
		ip := r.opts.IgnorePrimitives
		return ip.Boolean || ip.BigInt || ip.Number || ip.String
	}
	if !t.IsUnion() {
		return false
	}
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() {
			continue
		}
		if r.matchesIgnoredPrimitive(m) {
			return true
		}
	}
	return false
}

// matchesIgnoredPrimitive walks intersections so branded primitives
// (`string & {brand}`) trip the same ignore-primitives carve-out as
// the bare primitive.
func (r *rule) matchesIgnoredPrimitive(m *wrapperchecker.Type) bool {
	switch {
	case r.opts.IgnorePrimitives.Boolean && m.IsBooleanLike():
		return true
	case r.opts.IgnorePrimitives.BigInt && m.IsBigIntLike():
		return true
	case r.opts.IgnorePrimitives.Number && m.IsNumberLike():
		return true
	case r.opts.IgnorePrimitives.String && m.IsStringLike():
		return true
	}
	if m.IsIntersection() {
		for _, c := range m.IntersectionMembers() {
			if r.matchesIgnoredPrimitive(c) {
				return true
			}
		}
	}
	return false
}

// isMixedLogicalExpression returns true when the `||` is part of a
// chain whose connected `||`/`&&` subtree contains at least one `&&`.
// We walk up to the topmost connected `||`/`&&` operator and then walk
// down looking for `&&` anywhere in the tree.
func isMixedLogicalExpression(n *wrapperchecker.Node) bool {
	root := n
	for {
		p := root.Parent()
		if p == nil || p.Kind() != wrapperchecker.KindBinaryExpression {
			break
		}
		op := p.BinaryOperatorKind()
		if op != wrapperchecker.KindBarBarToken &&
			op != wrapperchecker.KindAmpersandAmpersandToken {
			break
		}
		root = p
	}
	return subtreeContainsAnd(root)
}

func subtreeContainsAnd(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		return subtreeContainsAnd(n.FirstChild())
	}
	if n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	op := n.BinaryOperatorKind()
	if op == wrapperchecker.KindAmpersandAmpersandToken {
		return true
	}
	if op != wrapperchecker.KindBarBarToken {
		return false
	}
	return subtreeContainsAnd(n.BinaryLeft()) || subtreeContainsAnd(n.BinaryRight())
}

// ternaryInsideLogicalCoercion returns true when n's nearest non-paren
// parent is a `||`/`&&` chain that ultimately reaches a boolean
// coercion site. Distinguishes `Boolean(a ? a : b)` (always flag) from
// `Boolean((a ? a : b) || c)` (skip ternary along with the `||`).
func ternaryInsideLogicalCoercion(n *wrapperchecker.Node) bool {
	cur := n.Parent()
	for cur != nil && cur.Kind() == wrapperchecker.KindParenthesizedExpression {
		cur = cur.Parent()
	}
	if cur == nil || cur.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	op := cur.BinaryOperatorKind()
	if op != wrapperchecker.KindBarBarToken && op != wrapperchecker.KindAmpersandAmpersandToken {
		return false
	}
	return isInBooleanCoercion(cur)
}

func isInBooleanCoercion(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			continue
		case wrapperchecker.KindBinaryExpression:
			// Walk through enclosing `||`/`&&`/`??` chains and comma
			// expressions — `Boolean(a || b)`, `Boolean((x, a || b))`,
			// `Boolean(a ?? b)` all keep the inner `||` reachable from
			// the call.
			op := cur.BinaryOperatorKind()
			if op == wrapperchecker.KindBarBarToken ||
				op == wrapperchecker.KindAmpersandAmpersandToken ||
				op == wrapperchecker.KindQuestionQuestionToken ||
				op == wrapperchecker.KindCommaToken {
				continue
			}
			return false
		case wrapperchecker.KindCallExpression:
			callee := cur.CalleeExpression()
			if callee != nil && callee.Kind() == wrapperchecker.KindIdentifier &&
				callee.LiteralText() == "Boolean" {
				return true
			}
			return false
		case wrapperchecker.KindPrefixUnaryExpression:
			return true
		case wrapperchecker.KindIfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindForStatement,
			wrapperchecker.KindConditionalExpression:
			// Conditional tests are governed by ignoreConditionalTests,
			// not ignoreBooleanCoercion.
			return false
		}
		return false
	}
	return false
}

// isInConditionalTestPosition reports whether n is the test
// expression of a conditional/if/while/do/for, possibly nested
// through `||`/`&&` chains and parenthesized expressions.
func isInConditionalTestPosition(n *wrapperchecker.Node) bool {
	cur := n
	for {
		parent := cur.Parent()
		if parent == nil {
			return false
		}
		switch parent.Kind() {
		case wrapperchecker.KindIfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindConditionalExpression,
			wrapperchecker.KindForStatement:
			// Reached a parent that uses one of its children as a
			// test position; we walked here only through `||`/`&&`/
			// parens, so the original node is in a test context.
			return true
		case wrapperchecker.KindParenthesizedExpression:
			cur = parent
			continue
		case wrapperchecker.KindPrefixUnaryExpression:
			// Only `!` keeps us in test context; `+` / `-` switch to
			// numeric semantics so the inner `||` is not in test
			// position.
			if parent.PrefixUnaryOperator() == "!" {
				cur = parent
				continue
			}
			return false
		case wrapperchecker.KindBinaryExpression:
			switch parent.BinaryOperatorKind() {
			case wrapperchecker.KindAmpersandAmpersandToken,
				wrapperchecker.KindBarBarToken,
				wrapperchecker.KindQuestionQuestionToken,
				wrapperchecker.KindCommaToken:
				cur = parent
				continue
			}
		}
		return false
	}
}

// typeIsNullable reports whether t can hold null or undefined. A bare
// `null` / `undefined` type still qualifies — `null || y` and
// `null ?? y` are both `y`, so the rule still suggests the safer form.
// `unknown` and `any` also qualify because they include nullish.
func typeIsNullable(t *wrapperchecker.Type) bool {
	if t.IsNullOrUndefined() || t.IsUnknown() || t.IsAny() {
		return true
	}
	if !t.IsUnion() {
		return false
	}
	for _, m := range t.UnionMembers() {
		if m.IsNullOrUndefined() || m.IsUnknown() || m.IsAny() {
			return true
		}
	}
	return false
}
