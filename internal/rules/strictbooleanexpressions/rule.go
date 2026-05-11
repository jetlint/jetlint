// Package strictbooleanexpressions implements the strict-boolean-expressions
// rule: flag any boolean-context expression whose type is not strictly
// boolean.
package strictbooleanexpressions

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "strict-boolean-expressions"

// Options is the configurable surface of the rule.
type Options struct {
	AllowString          bool
	AllowNumber          bool
	AllowNullableObject  bool
	AllowNullableBoolean bool
	AllowNullableString  bool
	AllowNullableNumber  bool
	AllowNullableEnum    bool
	AllowAny             bool
	AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing bool
}

func DefaultOptions() Options {
	return Options{
		AllowString:         true,
		AllowNumber:         true,
		AllowNullableObject: true,
	}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("strict-boolean-expressions options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowString":
			if err := json.Unmarshal(val, &out.AllowString); err != nil {
				return Options{}, err
			}
		case "allowNumber":
			if err := json.Unmarshal(val, &out.AllowNumber); err != nil {
				return Options{}, err
			}
		case "allowNullableObject":
			if err := json.Unmarshal(val, &out.AllowNullableObject); err != nil {
				return Options{}, err
			}
		case "allowNullableBoolean":
			if err := json.Unmarshal(val, &out.AllowNullableBoolean); err != nil {
				return Options{}, err
			}
		case "allowNullableString":
			if err := json.Unmarshal(val, &out.AllowNullableString); err != nil {
				return Options{}, err
			}
		case "allowNullableNumber":
			if err := json.Unmarshal(val, &out.AllowNullableNumber); err != nil {
				return Options{}, err
			}
		case "allowNullableEnum":
			if err := json.Unmarshal(val, &out.AllowNullableEnum); err != nil {
				return Options{}, err
			}
		case "allowAny":
			if err := json.Unmarshal(val, &out.AllowAny); err != nil {
				return Options{}, err
			}
		case "allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing":
			if err := json.Unmarshal(val, &out.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile:            r.visitSourceFile,
		wrapperchecker.KindIfStatement:           r.visitTestPosition,
		wrapperchecker.KindConditionalExpression: r.visitTestPosition,
		wrapperchecker.KindWhileStatement:        r.visitTestPosition,
		wrapperchecker.KindDoStatement:           r.visitTestPosition,
		wrapperchecker.KindForStatement:          r.visitForStatement,
		wrapperchecker.KindPrefixUnaryExpression: r.visitPrefixUnary,
		wrapperchecker.KindBinaryExpression:      r.visitBinary,
		wrapperchecker.KindCallExpression:        r.visitCall,
	}
}

// visitSourceFile emits a per-file `noStrictNullCheck` advisory when
// the program lacks strictNullChecks and the caller hasn't opted in
// via `allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing`. The
// classification of nullable vs plain types this rule performs relies
// on strictNullChecks; without it, many results are misleading.
func (r *rule) visitSourceFile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing {
		return
	}
	if ctx.Program().HasStrictNullChecks() {
		return
	}
	ctx.Report(n, "this rule requires `strictNullChecks` to be enabled; otherwise nullable types cannot be reliably distinguished")
}

// shouldSkipForOptIn reports whether the rule should stop emitting
// regular diagnostics for this program. When strictNullChecks is off
// and the user has explicitly opted in via
// `allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing: true`,
// they're declaring they understand the analysis is unreliable; we
// defer to inline suppressions rather than producing reports the rule
// would normally make.
func (r *rule) shouldSkipForOptIn(ctx *engine.Context) bool {
	return r.opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing &&
		!ctx.Program().HasStrictNullChecks()
}

// visitCall checks call expressions for positions that semantically
// coerce a value to boolean — assertion arguments, array-predicate
// callback returns.
func (r *rule) visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.shouldSkipForOptIn(ctx) {
		return
	}
	r.checkAssertionArgument(ctx, n)
	r.checkArrayPredicateCallback(ctx, n)
}

func (r *rule) checkAssertionArgument(ctx *engine.Context, n *wrapperchecker.Node) {
	sig := ctx.Checker().ResolvedSignature(n)
	idx := -1
	hasTypePredicate := false
	if sig != nil {
		idx = sig.AssertsParameterIndex()
		hasTypePredicate = sig.TypePredicateNarrowedType() != nil
	}
	if idx < 0 {
		// Union-of-assertion-functions: `(asserts1 | asserts2)(arg)`
		// has no single resolved signature, but each call signature on
		// the callee's type carries the same assertion-parameter index.
		// Inspect those directly.
		callee := n.CalleeExpression()
		if callee == nil {
			return
		}
		ct := ctx.TypeOf(callee)
		if ct == nil {
			return
		}
		var sigs []*wrapperchecker.Signature
		if ct.IsUnion() {
			// Union of function types: collect signatures from each
			// branch. The union itself often has no synthesized call
			// signature, but each branch can be its own asserter.
			for _, m := range ct.UnionMembers() {
				for _, s := range m.CallSignatures() {
					sigs = append(sigs, s)
				}
			}
		} else {
			sigs = ct.CallSignatures()
		}
		if len(sigs) == 0 {
			return
		}
		for _, s := range sigs {
			i := s.AssertsParameterIndex()
			if i < 0 {
				return
			}
			if s.TypePredicateNarrowedType() != nil {
				return
			}
			if idx == -1 {
				idx = i
			} else if idx != i {
				return
			}
		}
	}
	if idx < 0 || hasTypePredicate {
		return
	}
	args := n.CallArguments()
	if idx >= len(args) {
		return
	}
	// A spread argument at or before the asserts-parameter index makes
	// positional alignment ambiguous — we cannot know which call-site
	// argument the asserted parameter receives.
	for i := 0; i <= idx && i < len(args); i++ {
		if args[i].Kind() == wrapperchecker.KindSpreadElement {
			return
		}
	}
	r.checkBoolean(ctx, args[idx])
}

// arrayPredicateMethods is the set of Array.prototype methods whose
// callback is expected to return a boolean. The callback's return
// position coerces, so the rule applies the same shape check it would
// to an `if` test.
var arrayPredicateMethods = map[string]bool{
	"filter": true, "find": true, "findIndex": true,
	"findLast": true, "findLastIndex": true,
	"every": true, "some": true,
}

func (r *rule) checkArrayPredicateCallback(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if !arrayPredicateMethods[callee.PropertyAccessName()] {
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
	// When the callback carries an explicit return-type annotation,
	// trust that as the user's declared shape — and check it strictly
	// against boolean. The general `isAcceptable` is the right gate for
	// inferred return types (so `array => array` keeps working under
	// the default `allowString`), but an explicit `(x): boolean | T => ...`
	// is the exact mistake the predicate-callback check is designed to
	// flag: the author widened the return for no reason.
	var retT *wrapperchecker.Type
	var strictCheck bool
	if cb.Kind() == wrapperchecker.KindArrowFunction ||
		cb.Kind() == wrapperchecker.KindFunctionExpression {
		if annot := cb.FunctionReturnTypeAnnotation(); annot != nil {
			retT = ctx.Checker().TypeFromTypeNode(annot)
			strictCheck = retT != nil
		}
	}
	if retT == nil {
		cbT := ctx.TypeOf(cb)
		if cbT == nil {
			return
		}
		// Look at the callback symbol's declarations to walk every
		// declared overload — typescript-go's CallSignatures on the
		// type often returns contextually-narrowed signatures that
		// don't distinguish overloads with different return types.
		if r.flagIfDeclaredOverloadsReturnNonBoolean(ctx, cb) {
			return
		}
		sigs := cbT.CallSignatures()
		if len(sigs) == 0 {
			return
		}
		retT = sigs[0].ReturnType()
		if retT == nil {
			return
		}
	}
	acceptable := r.isAcceptable(retT)
	if strictCheck {
		acceptable = isStrictlyBooleanReturn(retT)
	}
	if acceptable {
		return
	}
	if retT.IsAny() || retT.IsUnknown() {
		ctx.Report(cb, "predicate callback returns a value of type any or unknown; narrow before returning")
		return
	}
	ctx.Report(cb, "predicate callback returns a value whose type is not strictly boolean; compare against the intended sentinel")
}

// flagIfDeclaredOverloadsReturnNonBoolean walks the callback symbol's
// declarations directly (each overload's FunctionDeclaration carries
// its own return type) and reports the predicate callback when any
// overload returns a value whose shape isn't acceptable for a boolean
// test. Returns true when a diagnostic was issued.
func (r *rule) flagIfDeclaredOverloadsReturnNonBoolean(ctx *engine.Context, cb *wrapperchecker.Node) bool {
	sym := ctx.Checker().SymbolOf(cb)
	if sym == nil {
		return false
	}
	decls := sym.Declarations()
	if len(decls) < 2 {
		return false
	}
	hasOverload := false
	for _, d := range decls {
		switch d.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindMethodSignature,
			wrapperchecker.KindCallSignature:
		default:
			continue
		}
		hasOverload = true
		annot := d.FunctionReturnTypeAnnotation()
		if annot == nil {
			continue
		}
		retT := ctx.Checker().TypeFromTypeNode(annot)
		if retT == nil {
			continue
		}
		// Predicate-callback positions require strictly boolean return
		// types — wider returns like `string` or `boolean | T` are the
		// shape mistake the rule wants to catch even when `allowString`
		// or similar would normally let those types pass.
		if isStrictlyBooleanReturn(retT) {
			continue
		}
		if retT.IsAny() || retT.IsUnknown() {
			ctx.Report(cb, "predicate callback returns a value of type any or unknown; narrow before returning")
		} else {
			ctx.Report(cb, "predicate callback returns a value whose type is not strictly boolean; compare against the intended sentinel")
		}
		return true
	}
	_ = hasOverload
	return false
}

// isStrictlyBooleanReturn reports whether a declared return-type
// annotation is acceptable on a predicate callback — i.e., the
// annotation already commits to boolean (or `boolean | never`).
// Wider annotations like `boolean | number` are exactly the
// predicate-return mistake the rule wants to surface.
func isStrictlyBooleanReturn(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsBooleanLike() {
		return true
	}
	if !t.IsUnion() {
		return false
	}
	for _, m := range t.UnionMembers() {
		if !m.IsBooleanLike() && !m.IsNever() {
			return false
		}
	}
	return true
}

// visitBinary checks the left operand of `&&` and `||` — the
// short-circuit operators evaluate their LHS as a boolean test, so
// the same constraints apply. The right operand is not in a boolean
// position (its value flows out as the expression result), so it
// isn't checked here.
func (r *rule) visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindAmpersandAmpersandToken, wrapperchecker.KindBarBarToken:
	default:
		return
	}
	if r.shouldSkipForOptIn(ctx) {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	tested := isBinaryTested(n)
	// LHS is always evaluated as boolean (for short-circuit). RHS is
	// evaluated as boolean only when the binary itself flows into a
	// boolean test — the result value is what's tested.
	if left != nil {
		r.checkBooleanOperand(ctx, left)
	}
	if tested && right != nil {
		r.checkBooleanOperand(ctx, right)
	}
}

// checkBooleanOperand is the per-operand entry point for visitBinary.
// When the operand is itself a logical &&/|| (possibly under parens),
// the engine will fire visitBinary on that inner node and handle its
// operands there; checking here would double-report.
func (r *rule) checkBooleanOperand(ctx *engine.Context, n *wrapperchecker.Node) {
	inner := n
	for inner != nil && inner.Kind() == wrapperchecker.KindParenthesizedExpression {
		inner = inner.FirstChild()
	}
	if inner != nil && inner.Kind() == wrapperchecker.KindBinaryExpression {
		switch inner.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindBarBarToken:
			return
		}
	}
	r.checkBoolean(ctx, n)
}

// isBinaryTested reports whether the boolean result of a `&&`/`||`
// expression flows into a position that interprets it as boolean. The
// LHS of a logical operator is the operator's test value; the RHS is
// the result value, which is only tested when the operator itself is.
func isBinaryTested(n *wrapperchecker.Node) bool {
	cur := n
	for {
		parent := cur.Parent()
		if parent == nil {
			return false
		}
		switch parent.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			cur = parent
			continue
		case wrapperchecker.KindIfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindForStatement,
			wrapperchecker.KindConditionalExpression:
			return true
		case wrapperchecker.KindPrefixUnaryExpression:
			return parent.PrefixUnaryOperator() == "!"
		case wrapperchecker.KindBinaryExpression:
			switch parent.BinaryOperatorKind() {
			case wrapperchecker.KindAmpersandAmpersandToken,
				wrapperchecker.KindBarBarToken:
				// LHS of a logical operator is always tested for the
				// short-circuit; RHS is tested only when the operator
				// itself is.
				if parent.BinaryLeft().Same(cur) {
					return true
				}
				cur = parent
				continue
			}
			return false
		}
		return false
	}
}

func (r *rule) visitForStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	cond := n.ForStatementCondition()
	if cond == nil {
		return
	}
	r.checkBoolean(ctx, cond)
}

func (r *rule) visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	r.checkBoolean(ctx, operand)
}

func (r *rule) checkBoolean(ctx *engine.Context, expr *wrapperchecker.Node) {
	if r.shouldSkipForOptIn(ctx) {
		return
	}
	// Logical-chain expressions are handled per-operand by visitBinary
	// so each branch reports against its real operand position; doing
	// type-checking on the whole chain here would attribute the report
	// to the wrong node.
	inner := expr
	for inner != nil && inner.Kind() == wrapperchecker.KindParenthesizedExpression {
		inner = inner.FirstChild()
	}
	if inner != nil && inner.Kind() == wrapperchecker.KindBinaryExpression {
		switch inner.BinaryOperatorKind() {
		case wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindBarBarToken:
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if r.isAcceptable(t) {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		ctx.Report(expr, "boolean test on a value of type any or unknown; narrow the value first")
		return
	}
	ctx.Report(expr, "boolean test on a value whose type is not strictly boolean; coerce explicitly or compare against the intended sentinel")
}

func (r *rule) visitTestPosition(ctx *engine.Context, n *wrapperchecker.Node) {
	test := testExpressionOf(n)
	if test == nil {
		return
	}
	r.checkBoolean(ctx, test)
}

func testExpressionOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindIfStatement:
		return n.IfCondition()
	case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
		return n.WhileCondition()
	case wrapperchecker.KindConditionalExpression:
		return n.ConditionalCondition()
	}
	return nil
}

// isAlwaysTruthyLiteral reports whether t is a literal type that can
// never be falsy at runtime — non-empty strings, non-zero numbers,
// the `true` literal, etc.
func isAlwaysTruthyLiteral(t *wrapperchecker.Type) bool {
	s := t.String()
	switch {
	case t.IsStringLike() && s != "string" && s != "\"\"" && s != "''":
		return true
	case t.IsNumberLike() && s != "number" && s != "0":
		return true
	case t.IsBigIntLike() && s != "bigint" && s != "0n":
		return true
	case t.IsBooleanLike() && s == "true":
		return true
	}
	return false
}

// isAcceptable reports whether t is OK in a boolean test under the
// configured options.
func (r *rule) isAcceptable(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return r.opts.AllowAny
	}
	if !t.IsUnion() {
		return r.scalarAcceptable(t)
	}
	hasNullable := false
	hasBool := false
	hasString := false
	hasNumber := false
	hasEnum := false
	hasOther := false
	hasAlwaysTruthy := false
	for _, m := range t.UnionMembers() {
		switch {
		case m.IsNullOrUndefined():
			hasNullable = true
		case m.IsEnumLike():
			// Check enum membership before the more general string/
			// number primitive checks — an enum literal is also
			// IsNumberLike/IsStringLike but the enum identity is
			// the load-bearing classification for nullable-enum
			// option handling.
			hasEnum = true
		case isAlwaysTruthyLiteral(m):
			// Non-empty string, non-zero numeric, `true`, etc. — never
			// confused with the null/undefined branch in a boolean test.
			hasAlwaysTruthy = true
		case m.IsBooleanLike() || intersectionContainsBoolean(m):
			hasBool = true
		case m.IsStringLike() || intersectionContainsStringLike(m):
			hasString = true
		case m.IsNumberLike() || m.IsBigIntLike() || intersectionContainsNumberLike(m):
			hasNumber = true
		case m.IsNever():
			// Unreachable.
		case m.IsTypeParameter():
			if c := m.BaseConstraint(); c != nil && c != m && r.isAcceptable(c) {
				continue
			}
			hasOther = true
		default:
			hasOther = true
		}
	}
	// If the only non-nullable members are always-truthy literals (or
	// always-truthy literals + an object), treat the whole union the
	// same way as `T | null` for an object T.
	if hasAlwaysTruthy && !hasBool && !hasString && !hasNumber && !hasEnum && !hasOther {
		if hasNullable {
			return r.opts.AllowNullableObject
		}
		return true
	}
	if hasAlwaysTruthy {
		// Mixed always-truthy with regular primitives — keep checking
		// the regular primitives below; the literal doesn't add risk.
	}
	if hasOther {
		// Object/function-typed members count as "nullable-object" only
		// when paired with a nullable.
		return hasNullable && r.opts.AllowNullableObject &&
			!hasBool && !hasString && !hasNumber && !hasEnum
	}
	if hasNullable {
		switch {
		case hasBool && !hasString && !hasNumber && !hasEnum:
			return r.opts.AllowNullableBoolean
		case hasString && !hasBool && !hasNumber && !hasEnum:
			return r.opts.AllowNullableString && r.opts.AllowString
		case hasNumber && !hasBool && !hasString && !hasEnum:
			return r.opts.AllowNullableNumber && r.opts.AllowNumber
		case hasEnum && !hasBool && !hasString && !hasNumber:
			return r.opts.AllowNullableEnum
		case !hasBool && !hasString && !hasNumber && !hasEnum:
			// Pure null/undefined union — the condition is always
			// false; flag regardless of AllowNullableObject (which is
			// about nullable object refs, not the no-object case).
			return false
		}
		return false
	}
	// No nullable.
	if hasBool && !hasString && !hasNumber && !hasEnum {
		return true
	}
	if hasString && !hasBool && !hasNumber && !hasEnum {
		return r.opts.AllowString
	}
	if hasNumber && !hasBool && !hasString && !hasEnum {
		return r.opts.AllowNumber
	}
	if hasBool || hasString || hasNumber || hasEnum {
		// Mixed primitive union — only OK if all components are.
		if hasBool {
			if hasString && !r.opts.AllowString {
				return false
			}
			if hasNumber && !r.opts.AllowNumber {
				return false
			}
			return true
		}
	}
	return false
}

func intersectionContainsBoolean(t *wrapperchecker.Type) bool {
	if !t.IsIntersection() {
		return false
	}
	for _, m := range t.IntersectionMembers() {
		if m.IsBooleanLike() {
			return true
		}
	}
	return false
}

func intersectionContainsStringLike(t *wrapperchecker.Type) bool {
	if !t.IsIntersection() {
		return false
	}
	for _, m := range t.IntersectionMembers() {
		if m.IsStringLike() {
			return true
		}
	}
	return false
}

func intersectionContainsNumberLike(t *wrapperchecker.Type) bool {
	if !t.IsIntersection() {
		return false
	}
	for _, m := range t.IntersectionMembers() {
		if m.IsNumberLike() || m.IsBigIntLike() {
			return true
		}
	}
	return false
}

func (r *rule) scalarAcceptable(t *wrapperchecker.Type) bool {
	if t.IsBooleanLike() {
		return true
	}
	if t.IsStringLike() {
		return r.opts.AllowString
	}
	if t.IsNumberLike() || t.IsBigIntLike() {
		return r.opts.AllowNumber
	}
	if t.IsNever() {
		return true
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return r.isAcceptable(c)
		}
	}
	// Branded intersections (`boolean & { __BRAND }`) keep the
	// flavor of their primitive member — accept the intersection if
	// any member is itself acceptable in a boolean position.
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if r.scalarAcceptable(m) {
				return true
			}
		}
	}
	return false
}
