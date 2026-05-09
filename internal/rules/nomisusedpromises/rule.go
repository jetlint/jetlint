// Package nomisusedpromises implements the no-misused-promises rule:
// flag any expression where a Promise is used in a position the
// language does not handle (conditionals, void-returning callback
// slots, spreads).
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint. The check shape, option set, and contextual
// type semantics derive from upstream's published test fixtures.
package nomisusedpromises

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-misused-promises"

// VoidReturnSubOptions selects which void-return positions the rule
// inspects. Mirrors typescript-eslint's `checksVoidReturn` sub-options.
type VoidReturnSubOptions struct {
	Arguments        bool
	Attributes       bool
	InheritedMethods bool
	Properties       bool
	Returns          bool
	Variables        bool
}

// Options is the configurable surface of the rule.
type Options struct {
	ChecksConditionals bool
	ChecksVoidReturn   bool
	ChecksSpreads      bool
	VoidReturn         VoidReturnSubOptions
}

func defaultVoidReturnSub() VoidReturnSubOptions {
	return VoidReturnSubOptions{
		Arguments:        true,
		Attributes:       true,
		InheritedMethods: true,
		Properties:       true,
		Returns:          true,
		Variables:        true,
	}
}

func DefaultOptions() Options {
	return Options{
		ChecksConditionals: true,
		ChecksVoidReturn:   true,
		ChecksSpreads:      true,
		VoidReturn:         defaultVoidReturnSub(),
	}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-misused-promises options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "checksConditionals":
			if err := json.Unmarshal(val, &out.ChecksConditionals); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q: %w", key, err)
			}
		case "checksSpreads":
			if err := json.Unmarshal(val, &out.ChecksSpreads); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q: %w", key, err)
			}
		case "checksVoidReturn":
			var b bool
			if err := json.Unmarshal(val, &b); err == nil {
				out.ChecksVoidReturn = b
				if !b {
					out.VoidReturn = VoidReturnSubOptions{}
				}
				continue
			}
			var sub map[string]bool
			if err := json.Unmarshal(val, &sub); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q must be boolean or sub-options object: %w", key, err)
			}
			out.ChecksVoidReturn = true
			out.VoidReturn = applyVoidReturnSub(defaultVoidReturnSub(), sub)
		default:
			return Options{}, fmt.Errorf("no-misused-promises has no option %q (expected checksConditionals, checksVoidReturn, or checksSpreads)", key)
		}
	}
	return out, nil
}

func applyVoidReturnSub(base VoidReturnSubOptions, sub map[string]bool) VoidReturnSubOptions {
	if v, ok := sub["arguments"]; ok {
		base.Arguments = v
	}
	if v, ok := sub["attributes"]; ok {
		base.Attributes = v
	}
	if v, ok := sub["inheritedMethods"]; ok {
		base.InheritedMethods = v
	}
	if v, ok := sub["properties"]; ok {
		base.Properties = v
	}
	if v, ok := sub["returns"]; ok {
		base.Returns = v
	}
	if v, ok := sub["variables"]; ok {
		base.Variables = v
	}
	return base
}

// ApplyVoidReturnSubOptions overrides selected sub-options. Used by
// the compatibility harness to thread sub-option fixtures through.
func ApplyVoidReturnSubOptions(opts Options, sub map[string]bool) Options {
	opts.VoidReturn = applyVoidReturnSub(opts.VoidReturn, sub)
	return opts
}

func New() engine.Rule                          { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule   { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:              r.visitCallExpression,
		wrapperchecker.KindNewExpression:               r.visitCallExpression,
		wrapperchecker.KindIfStatement:                 r.visitConditional,
		wrapperchecker.KindWhileStatement:              r.visitConditional,
		wrapperchecker.KindDoStatement:                 r.visitConditional,
		wrapperchecker.KindForStatement:                r.visitForStatement,
		wrapperchecker.KindConditionalExpression:       r.visitConditional,
		wrapperchecker.KindBinaryExpression:            r.visitBinaryExpression,
		wrapperchecker.KindPrefixUnaryExpression:       r.visitPrefixUnary,
		wrapperchecker.KindVariableDeclaration:         r.visitVariableDeclaration,
		wrapperchecker.KindPropertyAssignment:          r.visitPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment: r.visitShorthandProperty,
		wrapperchecker.KindMethodDeclaration:           r.visitMethodInObjectLiteral,
		wrapperchecker.KindReturnStatement:             r.visitReturnStatement,
		wrapperchecker.KindClassDeclaration:            r.visitClassLikeOrInterface,
		wrapperchecker.KindClassExpression:             r.visitClassLikeOrInterface,
		wrapperchecker.KindInterfaceDeclaration:        r.visitClassLikeOrInterface,
		wrapperchecker.KindSpreadElement:               r.visitSpreadElement,
		wrapperchecker.KindSpreadAssignment:            r.visitSpreadElement,
		wrapperchecker.KindJsxAttribute:                r.visitJsxAttribute,
	}
}

const (
	msgVoidCallback = "async callback returns a Promise that the parameter's void return type silently drops"
	msgConditional  = "promise used in a conditional position; the language tests truthiness, not promise resolution"
	msgSpread       = "promise spread does not unwrap into iterable elements"
)

// visitCallExpression implements upstream's checkArguments: walk every
// call signature on every union constituent of the callee type and
// collect the parameter indices that demand a void-returning function.
// A position with at least one thenable-returning candidate signature
// wins over a void-returning one (overload tolerance).
//
// Also handles upstream's checkArrayPredicates: an async predicate
// passed to `.filter`/`.find`/`.every`/`.some`/`.findIndex`/
// `.findLast`/`.findLastIndex` is always a misuse, since these methods
// don't await the predicate's result.
func (r *rule) visitCallExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	r.checkArrayPredicate(ctx, n)
	if !r.opts.VoidReturn.Arguments {
		return
	}
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	if isPromiseFinallyCall(ctx, n, callee) {
		return
	}
	fnType := ctx.TypeOf(callee)
	if fnType == nil {
		return
	}
	isNew := n.Kind() == wrapperchecker.KindNewExpression
	thenable := make(map[int]bool, len(args))
	void := make(map[int]bool, len(args))
	for _, sub := range fnType.UnionMembers() {
		var sigs []*wrapperchecker.Signature
		if isNew {
			sigs = sub.ConstructSignatures()
		} else {
			sigs = sub.CallSignatures()
		}
		for _, sig := range sigs {
			classifySignature(sig, len(args), thenable, void)
		}
	}
	// Generic calls with explicit type arguments resolve T's constraint
	// at the signature level but the contextual type at the argument
	// position carries the instantiated parameter type. Walk those
	// per-argument contextual types in addition to the signature-level
	// ones so we catch `useCallback<ReturnsVoid>(async () => {})` and
	// similar patterns.
	for idx := range args {
		ctxT := ctx.Checker().ContextualTypeForArgument(n, idx)
		if ctxT == nil {
			continue
		}
		classifyParameterType(ctxT, idx, thenable, void)
	}
	for idx := range thenable {
		delete(void, idx)
	}
	for idx, arg := range args {
		if !void[idx] {
			continue
		}
		if !returnsPromise(ctx, arg) {
			continue
		}
		ctx.Report(arg, msgVoidCallback)
	}
}

// isPromiseFinallyCall mirrors upstream's checkArguments early-out:
// `something.finally(callback)` is exempt from the void-return-argument
// check whenever the receiver is Promise-like, since `finally` awaits
// its callback's resolution before settling. The static method name
// can come from a `.finally` access or a string-keyed access whose
// literal value is "finally".
func isPromiseFinallyCall(ctx *engine.Context, call, callee *wrapperchecker.Node) bool {
	if call.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	receiver, name := staticMemberAccess(ctx, callee)
	if receiver == nil || name != "finally" {
		return false
	}
	rt := ctx.TypeOf(receiver)
	if rt == nil {
		return false
	}
	for _, sub := range rt.UnionMembers() {
		if sub.IsPromise() || sub.IsThenable() {
			return true
		}
	}
	return false
}

func staticMemberAccess(ctx *engine.Context, callee *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	if callee == nil {
		return nil, ""
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessReceiver(), callee.PropertyAccessName()
	case wrapperchecker.KindElementAccessExpression:
		recv := callee.ElementAccessReceiver()
		idx := callee.ElementAccessIndex()
		if recv == nil || idx == nil {
			return nil, ""
		}
		if idx.Kind() == wrapperchecker.KindStringLiteral {
			return recv, idx.LiteralText()
		}
		// Resolve identifier references via the index's type — a
		// `const f = 'finally'` const has a string-literal type whose
		// literal value we can read.
		if idxT := ctx.TypeOf(idx); idxT != nil {
			if v, ok := idxT.StringLiteralValue(); ok {
				return recv, v
			}
		}
		return nil, ""
	}
	return nil, ""
}

var arrayPredicateMethods = map[string]struct{}{
	"every": {}, "filter": {}, "find": {},
	"findIndex": {}, "findLast": {}, "findLastIndex": {}, "some": {},
}

func (r *rule) checkArrayPredicate(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	method := callee.PropertyAccessName()
	if _, ok := arrayPredicateMethods[method]; !ok {
		return
	}
	receiver := callee.PropertyAccessReceiver()
	if receiver == nil {
		return
	}
	t := ctx.TypeOf(receiver)
	if t == nil {
		return
	}
	if !typeIsArrayOrTuple(t) {
		return
	}
	args := call.CallArguments()
	if len(args) == 0 {
		return
	}
	cb := args[0]
	if !returnsPromise(ctx, cb) {
		return
	}
	ctx.Report(cb, msgVoidCallback)
}

func typeIsArrayOrTuple(t *wrapperchecker.Type) bool {
	for _, sub := range t.UnionMembers() {
		if sub.IsArrayLikeType() || sub.IsTupleType() {
			return true
		}
	}
	return false
}

// classifySignature walks a signature's parameters, expanding the
// trailing rest parameter against any extra argument slots. Rest types
// can be `Array<T>` (each trailing arg gets element type T) or a tuple
// (each trailing arg gets the corresponding tuple element).
func classifySignature(sig *wrapperchecker.Signature, nargs int, thenable, void map[int]bool) {
	params := sig.ParameterTypes()
	hasRest := sig.HasRestParameter()
	restIdx := -1
	if hasRest {
		restIdx = len(params) - 1
	}
	for idx, paramT := range params {
		if idx >= nargs {
			break
		}
		if hasRest && idx == restIdx {
			continue
		}
		if paramT == nil {
			continue
		}
		classifyParameterType(paramT, idx, thenable, void)
	}
	if !hasRest || restIdx < 0 {
		return
	}
	restT := params[restIdx]
	if restT == nil {
		return
	}
	if elem := restT.ArrayElementType(); elem != nil {
		for i := restIdx; i < nargs; i++ {
			classifyParameterType(elem, i, thenable, void)
		}
		return
	}
	if restT.IsTupleType() {
		tupleArgs := restT.TypeArguments()
		for i := restIdx; i < nargs; i++ {
			tupIdx := i - restIdx
			if tupIdx >= len(tupleArgs) {
				break
			}
			elem := tupleArgs[tupIdx]
			if elem == nil {
				continue
			}
			classifyParameterType(elem, i, thenable, void)
		}
	}
}

// classifyParameterType inspects one parameter type and adds the
// argument index to either the thenable-accepting or
// void-only-accepting set. Mirrors upstream's
// isThenableReturningFunctionType / isVoidReturningFunctionType pair.
func classifyParameterType(t *wrapperchecker.Type, idx int, thenable, void map[int]bool) {
	if anyMemberReturnsPromise(t) {
		thenable[idx] = true
		return
	}
	if allMemberSignaturesReturnVoid(t) {
		void[idx] = true
	}
}

func anyMemberReturnsPromise(t *wrapperchecker.Type) bool {
	for _, sub := range t.UnionMembers() {
		for _, sig := range sub.CallSignatures() {
			rt := sig.ReturnType()
			if rt == nil {
				continue
			}
			if rt.IsPromise() || rt.IsThenable() {
				return true
			}
		}
	}
	return false
}

// allMemberSignaturesReturnVoid mirrors isVoidReturningFunctionType:
// returns true if at least one signature (across all union members)
// returns void and none return a thenable.
func allMemberSignaturesReturnVoid(t *wrapperchecker.Type) bool {
	hadVoid := false
	for _, sub := range t.UnionMembers() {
		for _, sig := range sub.CallSignatures() {
			rt := sig.ReturnType()
			if rt == nil {
				continue
			}
			if rt.IsPromise() || rt.IsThenable() {
				return false
			}
			if rt.IsVoid() {
				hadVoid = true
			}
		}
	}
	return hadVoid
}

func returnsPromise(ctx *engine.Context, n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return false
	}
	return anyMemberReturnsPromise(t)
}

// allCallSignaturesExpectVoid is retained for callers that already have
// a contextual type representing a callable slot; it walks unions and
// requires every non-nullable callable member to return void.
func allCallSignaturesExpectVoid(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		anyVoidCallable := false
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if !allCallSignaturesExpectVoid(m) {
				return false
			}
			anyVoidCallable = true
		}
		return anyVoidCallable
	}
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		ret := s.ReturnType()
		if ret == nil {
			return false
		}
		if !ret.IsVoid() {
			return false
		}
	}
	return true
}

func (r *rule) visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	cond := conditionExpression(n)
	if cond == nil {
		return
	}
	checkPromiseInTest(ctx, cond)
}

func (r *rule) visitForStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	cond := n.ForStatementCondition()
	if cond == nil {
		return
	}
	checkPromiseInTest(ctx, cond)
}

func (r *rule) visitBinaryExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.ChecksVoidReturn && r.opts.VoidReturn.Variables &&
		n.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
		left := n.BinaryLeft()
		right := n.BinaryRight()
		if left != nil && right != nil && returnsPromise(ctx, right) {
			t := ctx.TypeOf(left)
			if t != nil && allCallSignaturesExpectVoid(t) {
				ctx.Report(right, msgVoidCallback)
			}
		}
	}
	// Logical operators always perform truthiness tests on their
	// operands: `Promise.resolve() || x` tests the promise's
	// truthiness regardless of whether the whole expression sits in a
	// test position. typescript-eslint's checkConditional walks
	// `||`/`&&` operands even outside a test position; only the LHS
	// is tested for truthiness by the operator itself. For `??`, the
	// LHS is null-checked (not truthy-checked), so we leave that
	// branch alone outside test positions.
	if !r.opts.ChecksConditionals {
		return
	}
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindBarBarToken,
		wrapperchecker.KindAmpersandAmpersandToken:
		if l := n.BinaryLeft(); l != nil {
			checkPromiseAtNonTest(ctx, l)
		}
	}
}

func checkPromiseAtNonTest(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	if expr.IsOptionalChain() {
		return
	}
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindAmpersandAmpersandToken:
			if l := expr.BinaryLeft(); l != nil {
				checkPromiseAtNonTest(ctx, l)
			}
			return
		case wrapperchecker.KindQuestionQuestionToken:
			// Outside a test position, `??` LHS is null-checked, not
			// truthy-checked, so no flag for promise on LHS here.
			return
		}
	}
	if expr.Kind() == wrapperchecker.KindParenthesizedExpression {
		checkPromiseAtNonTest(ctx, expr.FirstChild())
		return
	}
	t := ctx.TypeOf(expr)
	if t != nil && isTestPromise(t) {
		ctx.Report(expr, msgConditional)
	}
}

func (r *rule) visitVariableDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.Variables {
		return
	}
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	if !returnsPromise(ctx, init) {
		return
	}
	annot := n.VariableDeclarationType()
	if annot == nil {
		return
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return
	}
	if allCallSignaturesExpectVoid(t) {
		ctx.Report(init, msgVoidCallback)
	}
}

func (r *rule) visitShorthandProperty(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.Properties {
		return
	}
	id := n.FirstChild()
	if id == nil {
		return
	}
	t := ctx.TypeOf(id)
	if t == nil {
		return
	}
	if !returnTypeIsPromise(t) {
		return
	}
	expected := ctx.Checker().ContextualTypeOf(id)
	if expected == nil {
		return
	}
	if allCallSignaturesExpectVoid(expected) {
		ctx.Report(n, msgVoidCallback)
	}
}

func returnTypeIsPromiseUnion(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				return true
			}
		}
	}
	return false
}

// visitClassLikeOrInterface implements upstream's
// checkClassLikeOrInterfaceNode: for each member of a class or
// interface, look up the same-named member on every heritage type and
// report once per heritage type whose member returns void.
func (r *rule) visitClassLikeOrInterface(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.InheritedMethods {
		return
	}
	heritage := n.HeritageTypes(ctx.Checker())
	// Fall back to walking the class's instance type's base types.
	// For `class B extends MyClassExpression` where MyClassExpression
	// is a const holding a class expression, the heritage type node
	// resolves to a constructor type (or to `any`) rather than the
	// instance side; the symbol-rooted base walk recovers the
	// instance-side heritage in that case.
	if len(heritage) == 0 {
		if t := ctx.TypeOf(n); t != nil {
			heritage = t.BaseTypes()
			if len(heritage) == 0 {
				heritage = t.HeritageBaseTypes()
			}
		}
	} else if t := ctx.TypeOf(n); t != nil {
		if extra := t.BaseTypes(); len(extra) > 0 {
			heritage = appendUnique(heritage, extra)
		}
		if extra := t.HeritageBaseTypes(); len(extra) > 0 {
			heritage = appendUnique(heritage, extra)
		}
	}
	if len(heritage) == 0 {
		return
	}
	walkClassMembers(n, func(member *wrapperchecker.Node) {
		if member.HasStaticModifier() {
			return
		}
		if !memberReturnsPromise(ctx, member) {
			return
		}
		name := memberName(member)
		if name == "" {
			return
		}
		for _, base := range heritage {
			expected := lookupHeritageMember(base, name)
			if expected == nil {
				continue
			}
			if allMemberSignaturesReturnVoid(expected) {
				ctx.Report(member, msgVoidCallback)
			}
		}
	})
}

// lookupHeritageMember finds a same-named property on a heritage
// type. For `extends Foo` where Foo is a value (e.g. a class
// expression assigned to a const), the heritage type can resolve to
// the constructor side, whose apparent properties are static. Fall
// back to walking the construct signature's return type — that's the
// instance side, where the user's overriding method lives.
func lookupHeritageMember(base *wrapperchecker.Type, name string) *wrapperchecker.Type {
	if base == nil {
		return nil
	}
	if t := base.PropertyType(name); t != nil {
		return t
	}
	for _, sig := range base.ConstructSignatures() {
		rt := sig.ReturnType()
		if rt == nil {
			continue
		}
		if t := rt.PropertyType(name); t != nil {
			return t
		}
	}
	return nil
}

func appendUnique(out []*wrapperchecker.Type, more []*wrapperchecker.Type) []*wrapperchecker.Type {
	for _, t := range more {
		if t == nil {
			continue
		}
		dup := false
		for _, ex := range out {
			if ex.Equal(t) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, t)
		}
	}
	return out
}

func walkClassMembers(n *wrapperchecker.Node, fn func(*wrapperchecker.Node)) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindMethodSignature,
			wrapperchecker.KindPropertyDeclaration,
			wrapperchecker.KindPropertySignature,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			fn(c)
		}
		return false
	})
}

func memberReturnsPromise(ctx *engine.Context, n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindMethodSignature,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		t := ctx.TypeOf(n)
		if t != nil && returnTypeIsPromise(t) {
			return true
		}
		// Method signature declared via interface: check return-type
		// annotation when no concrete signature is available.
		ann := n.FunctionReturnTypeAnnotation()
		if ann != nil {
			rt := ctx.Checker().TypeFromTypeNode(ann)
			if rt != nil && (rt.IsPromise() || returnTypeIsPromiseUnion(rt)) {
				return true
			}
		}
		return false
	case wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertySignature:
		t := ctx.TypeOf(n)
		if t != nil && returnTypeIsPromise(t) {
			return true
		}
		return false
	}
	return false
}

func memberName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindPrivateIdentifier,
			wrapperchecker.KindStringLiteral:
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func returnTypeIsPromise(t *wrapperchecker.Type) bool {
	for _, sig := range t.CallSignatures() {
		rt := sig.ReturnType()
		if rt == nil {
			continue
		}
		if rt.IsPromise() {
			return true
		}
		if rt.IsUnion() {
			for _, m := range rt.UnionMembers() {
				if m.IsPromise() {
					return true
				}
			}
		}
	}
	return false
}

func (r *rule) visitPropertyAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.Properties {
		return
	}
	init := n.PropertyInitializer()
	if init == nil {
		return
	}
	if !returnsPromise(ctx, init) {
		return
	}
	expected := ctx.Checker().ContextualTypeOf(init)
	if expected == nil {
		return
	}
	if allCallSignaturesExpectVoid(expected) {
		ctx.Report(init, msgVoidCallback)
	}
}

// visitMethodInObjectLiteral handles `{ async f() {} }` — checks the
// surrounding object literal's contextual type for a same-named
// property whose call signatures expect void. Class members are
// handled separately via visitClassLikeOrInterface to support
// inheritedMethods sub-option gating.
func (r *rule) visitMethodInObjectLiteral(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.Properties {
		return
	}
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return
	}
	if !memberReturnsPromise(ctx, n) {
		return
	}
	objType := ctx.Checker().ContextualTypeOf(parent)
	if objType == nil {
		return
	}
	name := memberName(n)
	if name == "" {
		return
	}
	for _, sub := range objType.UnionMembers() {
		propType := sub.PropertyType(name)
		if propType == nil {
			continue
		}
		if allMemberSignaturesReturnVoid(propType) {
			ctx.Report(n, msgVoidCallback)
			return
		}
	}
}

// visitJsxAttribute handles `<Component func={async () => 0} />`:
// when the attribute's contextual type is a void-returning function
// and the value is a thenable-returning function expression, flag it.
func (r *rule) visitJsxAttribute(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.Attributes {
		return
	}
	var jsxExpr *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxExpression {
			jsxExpr = c
			return true
		}
		return false
	})
	if jsxExpr == nil {
		return
	}
	inner := jsxExpr.FirstChild()
	if inner == nil {
		return
	}
	if !returnsPromise(ctx, inner) {
		return
	}
	expected := ctx.Checker().ContextualTypeOf(jsxExpr)
	if expected == nil {
		expected = ctx.Checker().ContextualTypeOf(inner)
	}
	if expected == nil {
		return
	}
	if allCallSignaturesExpectVoid(expected) {
		ctx.Report(jsxExpr, msgVoidCallback)
	}
}

func (r *rule) visitReturnStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn || !r.opts.VoidReturn.Returns {
		return
	}
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	if !returnsPromise(ctx, expr) {
		return
	}
	expected := ctx.Checker().ContextualTypeOf(expr)
	if expected == nil {
		return
	}
	if allCallSignaturesExpectVoid(expected) {
		ctx.Report(expr, msgVoidCallback)
	}
}

func (r *rule) visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	checkPromiseInTest(ctx, operand)
}

func (r *rule) visitSpreadElement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksSpreads {
		return
	}
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if t.IsPromise() {
		ctx.Report(n, msgSpread)
		return
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				ctx.Report(n, msgSpread)
				return
			}
		}
	}
}

func checkPromiseInTest(ctx *engine.Context, expr *wrapperchecker.Node) {
	if expr.IsOptionalChain() {
		return
	}
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindAmpersandAmpersandToken:
			if l := expr.BinaryLeft(); l != nil {
				checkPromiseInTest(ctx, l)
			}
			if r := expr.BinaryRight(); r != nil {
				checkPromiseInTest(ctx, r)
			}
			return
		case wrapperchecker.KindQuestionQuestionToken:
			// In a test position, `a ?? b` evaluates to `a` when a is
			// not nullish, otherwise `b`. Either operand can therefore
			// be the value being truthy-tested by the enclosing
			// context, so descend into both.
			if l := expr.BinaryLeft(); l != nil {
				checkPromiseInTest(ctx, l)
			}
			if r := expr.BinaryRight(); r != nil {
				checkPromiseInTest(ctx, r)
			}
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isTestPromise(t) {
		ctx.Report(expr, msgConditional)
	}
}

// isTestPromise reports whether testing this type's truthiness is
// almost certainly a misuse: every union constituent must be a Promise
// (no stripping of null/undefined). A `Promise<T> | undefined` is not
// flagged because the test could be a definedness guard.
func isTestPromise(t *wrapperchecker.Type) bool {
	for _, m := range t.UnionMembers() {
		if !m.IsPromise() {
			return false
		}
	}
	return true
}

func conditionExpression(n *wrapperchecker.Node) *wrapperchecker.Node {
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
