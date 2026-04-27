// Package nofloatingpromises implements the no-floating-promises rule:
// flag any expression statement whose value is a Promise that is not
// awaited, returned, voided, or otherwise handled by a chain handler.
//
// The rule's signature catch is the cross-file case — calling an async
// function from another module without awaiting it. That requires real
// type information across imports, which is exactly what biome cannot
// produce today and what makes this rule the headline differentiator
// for the linter.
//
// Behavioral spec: this is a Go reimplementation of the rule of the
// same name from typescript-eslint
// (https://typescript-eslint.io/rules/no-floating-promises/), MIT
// licensed. The option surface, recursion order, type-gate placement,
// chain-handler semantics, and allow-list semantics are all derived
// from that project's source. Code structure and the underlying
// type-checker API are ours; observable behavior aims to match
// upstream's so users can swap implementations without surprise. The
// upstream test fixtures live under testdata/typescript-eslint/ with
// the upstream LICENSE preserved alongside.
//
// The rule mirrors typescript-eslint's option set:
//   - IgnoreVoid (default true): treat `void promise` as suppression
//   - IgnoreIIFE (default false): allow `(async () => { ... })()`
//   - CheckThenables (default false): match any thenable, not just Promise
//   - AllowForKnownSafePromises: type names whose Promise instances are safe
//   - AllowForKnownSafeCalls: callee names whose call results are safe
package nofloatingpromises

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-floating-promises"

// Options is the configurable surface of the rule. Field defaults match
// typescript-eslint's documented defaults; callers should construct via
// DefaultOptions and override only what they need.
type Options struct {
	// IgnoreVoid: when true (default), `void promise` suppresses the
	// warning. Set false to require `await` even when the value is
	// explicitly discarded.
	IgnoreVoid bool
	// IgnoreIIFE: when true, an immediately-invoked async function
	// expression `(async () => { ... })()` at statement position is not
	// flagged. Default false.
	IgnoreIIFE bool
	// CheckThenables: when true, any structural thenable (object with a
	// callable `then`) is treated like a Promise. Default false (only
	// branded `Promise<T>` is matched).
	CheckThenables bool
	// AllowForKnownSafePromises: type matchers naming Promise types that
	// the user has marked safe to leave unhandled (e.g. an async logger
	// that swallows rejection internally).
	AllowForKnownSafePromises []TypeMatcher
	// AllowForKnownSafeCalls: callee matchers; when a call's resolved
	// callee matches one of these, the call is not flagged regardless
	// of its return type. Used for assertion-library chains and similar.
	AllowForKnownSafeCalls []TypeMatcher
}

// TypeMatcher names a TypeScript type by symbol name and origin.
// `From` is `"file"` (declared in the user's source), `"lib"` (one of
// TypeScript's built-in lib.*.d.ts files), or `"package"` (declared in
// an installed npm package). The current implementation matches by
// `Name` only — `From` is accepted for forward compatibility but not
// yet used to disambiguate.
type TypeMatcher struct {
	From string
	Name string
}

// DefaultOptions returns the option values that match typescript-eslint's
// documented defaults.
func DefaultOptions() Options {
	return Options{IgnoreVoid: true}
}

// OptionsFromJSON parses a raw JSON options object (the second element
// of a typescript-eslint-style `["error", {...}]` rule entry) into a
// typed Options. Defaults are applied for any field absent from the
// JSON. Unknown keys produce an error so typos and stale option names
// surface at config-load time. Empty/null input returns DefaultOptions.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-floating-promises options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "ignoreVoid":
			if err := json.Unmarshal(val, &out.IgnoreVoid); err != nil {
				return Options{}, fmt.Errorf("no-floating-promises option %q: %w", key, err)
			}
		case "ignoreIIFE":
			if err := json.Unmarshal(val, &out.IgnoreIIFE); err != nil {
				return Options{}, fmt.Errorf("no-floating-promises option %q: %w", key, err)
			}
		case "checkThenables":
			if err := json.Unmarshal(val, &out.CheckThenables); err != nil {
				return Options{}, fmt.Errorf("no-floating-promises option %q: %w", key, err)
			}
		case "allowForKnownSafePromises":
			matchers, err := parseMatchers(val)
			if err != nil {
				return Options{}, fmt.Errorf("no-floating-promises option %q: %w", key, err)
			}
			out.AllowForKnownSafePromises = matchers
		case "allowForKnownSafeCalls":
			matchers, err := parseMatchers(val)
			if err != nil {
				return Options{}, fmt.Errorf("no-floating-promises option %q: %w", key, err)
			}
			out.AllowForKnownSafeCalls = matchers
		default:
			return Options{}, fmt.Errorf("no-floating-promises has no option %q (expected ignoreVoid, ignoreIIFE, checkThenables, allowForKnownSafePromises, or allowForKnownSafeCalls)", key)
		}
	}
	return out, nil
}

// parseMatchers accepts either bare strings or `{from, name}` objects
// — the two shapes typescript-eslint allows for type matchers.
func parseMatchers(raw json.RawMessage) ([]TypeMatcher, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("expected an array of matchers")
	}
	out := make([]TypeMatcher, 0, len(entries))
	for i, entry := range entries {
		var s string
		if err := json.Unmarshal(entry, &s); err == nil {
			if s == "" {
				return nil, fmt.Errorf("matcher %d: empty string is not a valid name", i)
			}
			out = append(out, TypeMatcher{Name: s})
			continue
		}
		var obj struct {
			From string `json:"from"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(entry, &obj); err != nil {
			return nil, fmt.Errorf("matcher %d: must be a string or {from, name} object", i)
		}
		if obj.Name == "" {
			return nil, fmt.Errorf("matcher %d: missing required 'name' field", i)
		}
		out = append(out, TypeMatcher{From: obj.From, Name: obj.Name})
	}
	return out, nil
}

// New constructs a rule instance with default options.
func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

// NewWithOptions constructs a rule instance with the given options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExpressionStatement: r.visitExpressionStatement,
	}
}

// visitExpressionStatement is the rule's entry point. It applies the
// top-level escape hatches (IgnoreIIFE and AllowForKnownSafeCalls
// match the outer expression only, not anything inside the recursion)
// and then asks isUnhandled whether the statement floats a promise.
func (r *rule) visitExpressionStatement(ctx *engine.Context, stmt *wrapperchecker.Node) {
	expr := stmt.FirstChild()
	if expr == nil {
		return
	}
	outer := peelParens(expr)
	if r.opts.IgnoreIIFE && outer.Kind() == wrapperchecker.KindCallExpression && isIIFE(outer) {
		return
	}
	if r.calleeIsAllowedSafe(ctx, outer) {
		return
	}
	if r.isUnhandled(ctx, expr) {
		ctx.Report(stmt, "promise returned by this expression is not awaited or otherwise handled")
	}
}

// isUnhandled mirrors typescript-eslint's recursive check. Order
// matters and matches upstream:
//  1. Peel parens.
//  2. Comma walks subexpressions unconditionally.
//  3. Allow-listed types short-circuit.
//  4. `void X` with !IgnoreVoid recurses into X.
//  5. Type gate: if neither a Promise (or thenable when CheckThenables)
//     nor a Promise-array, the expression is not floating.
//  6. Past the gate: chain handlers (`.catch`, `.then(_,_)`,
//     `.finally`) and structural recursion (conditional, logical).
//  7. Anything else past the gate is floating.
func (r *rule) isUnhandled(ctx *engine.Context, expr *wrapperchecker.Node) bool {
	expr = peelParens(expr)
	if expr == nil {
		return false
	}
	// Comma walks before the type gate: any subexpression could float
	// even when the comma's last value isn't promise-like.
	if expr.Kind() == wrapperchecker.KindBinaryExpression &&
		expr.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
		if r.isUnhandled(ctx, expr.BinaryLeft()) {
			return true
		}
		return r.isUnhandled(ctx, expr.BinaryRight())
	}
	// Assignment captures the value into the LHS — not floating.
	if expr.Kind() == wrapperchecker.KindBinaryExpression &&
		expr.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
		return false
	}
	// Allow-listed Promise types are never floating.
	if t := ctx.TypeOf(expr); t != nil && r.isAllowedSafePromise(t) {
		return false
	}
	// `void X` with !IgnoreVoid recurses into the operand.
	if expr.Kind() == wrapperchecker.KindVoidExpression && !r.opts.IgnoreVoid {
		return r.isUnhandled(ctx, expr.FirstChild())
	}
	// `await X` handles a Promise (or thenable). The exception is
	// `await arrayOfPromises` — awaiting an array doesn't unwrap the
	// inner promises, so each element is still floating. The wrapper's
	// TypeOf for AwaitExpression doesn't always compute the awaited
	// type for intersections (e.g. `Promise<T> & U` stays as the
	// intersection rather than unwrapping to `T & U`), so we inspect
	// the operand's type rather than the await-result type.
	if expr.Kind() == wrapperchecker.KindAwaitExpression {
		operand := expr.FirstChild()
		if operand == nil {
			return false
		}
		ot := ctx.TypeOf(operand)
		if ot != nil && r.typeIsPromiseArray(ot) {
			return true
		}
		return false
	}
	// Type gate.
	t := ctx.TypeOf(expr)
	if t == nil {
		return false
	}
	if !r.typeIsPromiseLike(t) && !r.typeIsPromiseArray(t) {
		// Last-chance: signature return type for calls, base constraint
		// for generics. Either of these can carry a Promise even when
		// the contextual type doesn't.
		if !r.callReturnIsPromiseLike(ctx, expr) && !r.constraintIsFloating(t) {
			return false
		}
	}
	// Past the gate.
	if expr.Kind() == wrapperchecker.KindCallExpression {
		return r.isUnhandledChainCall(ctx, expr)
	}
	if expr.Kind() == wrapperchecker.KindConditionalExpression {
		whenTrue, whenFalse := expr.ConditionalBranches()
		return r.isUnhandled(ctx, whenTrue) || r.isUnhandled(ctx, whenFalse)
	}
	if expr.Kind() == wrapperchecker.KindBinaryExpression {
		switch expr.BinaryOperatorKind() {
		case wrapperchecker.KindBarBarToken,
			wrapperchecker.KindAmpersandAmpersandToken,
			wrapperchecker.KindQuestionQuestionToken:
			return r.isUnhandled(ctx, expr.BinaryLeft()) ||
				r.isUnhandled(ctx, expr.BinaryRight())
		}
	}
	return true
}

// isUnhandledChainCall analyzes a call expression that has already
// passed the type gate, deciding whether it represents a handled chain
// (catch/then with rejection / finally with handled receiver) or a
// bare unhandled promise call. Returns true when unhandled.
func (r *rule) isUnhandledChainCall(ctx *engine.Context, call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return true
	}
	args := call.CallArguments()
	// Any spread arg makes positional handler placement unreliable.
	for _, a := range args {
		if a.Kind() == wrapperchecker.KindSpreadElement {
			return true
		}
	}
	switch callee.PropertyAccessName() {
	case "catch":
		if len(args) >= 1 && isCallableArgType(ctx, args[0]) {
			return false
		}
		return true
	case "then":
		if len(args) >= 2 && isCallableArgType(ctx, args[1]) {
			return false
		}
		return true
	case "finally":
		// `.finally(...)` doesn't change rejection semantics; defer to
		// whether the receiver chain is handled. Args count doesn't
		// matter — `.finally()` with no args still chains.
		return r.isUnhandled(ctx, callee.PropertyAccessReceiver())
	}
	return true
}

// peelParens unwraps any nested ParenthesizedExpression layers.
func peelParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// isIIFE reports whether the call is an immediately-invoked function
// expression — a call whose callee, after peeling parens, is an arrow
// function or function expression.
func isIIFE(call *wrapperchecker.Node) bool {
	callee := peelParens(call.CalleeExpression())
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		return true
	}
	return false
}

// isCallableArgType reports whether the argument's type is unambiguously
// callable — every union member has at least one call signature. If
// any member is a non-callable (string, number, null, undefined,
// object without call sigs), the argument can't be relied on to handle
// rejection, and upstream flags the chain as not handled.
func isCallableArgType(ctx *engine.Context, arg *wrapperchecker.Node) bool {
	if arg == nil {
		return false
	}
	t := ctx.TypeOf(arg)
	if t == nil {
		return false
	}
	for _, m := range t.UnionMembers() {
		if len(m.CallSignatures()) == 0 {
			return false
		}
	}
	return true
}

// calleeIsAllowedSafe reports whether the call's callee — by symbol
// name or its type's symbol — matches one of the user-supplied
// AllowForKnownSafeCalls matchers. Applied at the top of the
// ExpressionStatement only, mirroring upstream's
// `isKnownSafePromiseCall` placement.
func (r *rule) calleeIsAllowedSafe(ctx *engine.Context, expr *wrapperchecker.Node) bool {
	if len(r.opts.AllowForKnownSafeCalls) == 0 ||
		expr.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := expr.CalleeExpression()
	if callee == nil {
		return false
	}
	if sym := ctx.Checker().SymbolOf(callee); sym != nil &&
		matchByName(sym.Name(), r.opts.AllowForKnownSafeCalls) {
		return true
	}
	if t := ctx.TypeOf(callee); t != nil &&
		typeMatchesAny(t, r.opts.AllowForKnownSafeCalls) {
		return true
	}
	return false
}

// callReturnIsPromiseLike returns true when expr is a CallExpression
// whose resolved signature returns a Promise. The contextually-narrowed
// type can hide a Promise return (e.g. when a callback's contextual
// signature returns void), so we check the signature directly.
func (r *rule) callReturnIsPromiseLike(ctx *engine.Context, expr *wrapperchecker.Node) bool {
	if expr.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	sig := ctx.Checker().ResolvedSignature(expr)
	if sig == nil {
		return false
	}
	rt := sig.ReturnType()
	if rt == nil {
		return false
	}
	return r.typeIsPromiseLike(rt) || r.typeIsPromiseArray(rt)
}

// constraintIsFloating handles generic type parameters by walking the
// declared constraint (`function f<T extends Array<Promise<...>>>(a: T)`
// — `a;` should flag because of the constraint).
func (r *rule) constraintIsFloating(t *wrapperchecker.Type) bool {
	c := t.BaseConstraint()
	if c == nil || c == t {
		return false
	}
	if r.isAllowedSafePromise(c) {
		return false
	}
	return r.typeIsPromiseLike(c) || r.typeIsPromiseArray(c)
}

// typeIsPromiseLike returns true when the type, or any union member of
// it, is a Promise (or any thenable when CheckThenables is enabled).
// Allow-listed types are excluded.
func (r *rule) typeIsPromiseLike(t *wrapperchecker.Type) bool {
	if r.isAllowedSafePromise(t) {
		return false
	}
	if r.matchesPromiseLike(t) {
		return true
	}
	for _, m := range t.UnionMembers() {
		if r.isAllowedSafePromise(m) {
			continue
		}
		if r.matchesPromiseLike(m) {
			return true
		}
	}
	return false
}

func (r *rule) matchesPromiseLike(t *wrapperchecker.Type) bool {
	if t.IsPromise() {
		return true
	}
	if r.opts.CheckThenables && t.IsThenable() {
		return true
	}
	return false
}

// typeIsPromiseArray returns true when the type, or any union member,
// is an array or tuple containing a Promise.
func (r *rule) typeIsPromiseArray(t *wrapperchecker.Type) bool {
	if r.isAllowedSafePromise(t) {
		return false
	}
	if r.isPromiseCollection(t) {
		return true
	}
	for _, m := range t.UnionMembers() {
		if r.isAllowedSafePromise(m) {
			continue
		}
		if r.isPromiseCollection(m) {
			return true
		}
	}
	return false
}

func (r *rule) isPromiseCollection(t *wrapperchecker.Type) bool {
	if t.IsTupleType() {
		for _, e := range t.TypeArguments() {
			if !r.isAllowedSafePromise(e) && r.matchesPromiseLikeAcrossUnion(e) {
				return true
			}
		}
		return false
	}
	if t.IsArrayLikeType() {
		elem := t.ArrayElementType()
		if elem != nil && !r.isAllowedSafePromise(elem) && r.matchesPromiseLikeAcrossUnion(elem) {
			return true
		}
	}
	return false
}

func (r *rule) matchesPromiseLikeAcrossUnion(t *wrapperchecker.Type) bool {
	if r.matchesPromiseLike(t) {
		return true
	}
	for _, m := range t.UnionMembers() {
		if r.matchesPromiseLike(m) {
			return true
		}
	}
	return false
}

// isAllowedSafePromise reports whether the type matches one of the
// user-supplied AllowForKnownSafePromises matchers. Walks union and
// intersection members so `Foo & { hey?: string }` matches when `Foo`
// is allow-listed.
func (r *rule) isAllowedSafePromise(t *wrapperchecker.Type) bool {
	if len(r.opts.AllowForKnownSafePromises) == 0 {
		return false
	}
	return typeMatchesAny(t, r.opts.AllowForKnownSafePromises)
}

// typeMatchesAny reports whether the type matches any of the matchers,
// considering both the type's symbol name and its alias-symbol name
// (so `type Foo = Promise<X> & {...}` matches by `Foo`). Walks union
// and intersection members so an intersection like `Foo & {hey?}`
// matches when `Foo` is allow-listed. The current implementation is
// name-only: From is accepted for forward compatibility but not used
// to disambiguate.
func typeMatchesAny(t *wrapperchecker.Type, matchers []TypeMatcher) bool {
	if t == nil || len(matchers) == 0 {
		return false
	}
	if matchByName(t.SymbolName(), matchers) || matchByName(t.AliasSymbolName(), matchers) {
		return true
	}
	for _, m := range t.UnionMembers() {
		if matchByName(m.SymbolName(), matchers) || matchByName(m.AliasSymbolName(), matchers) {
			return true
		}
	}
	for _, m := range t.IntersectionMembers() {
		if matchByName(m.SymbolName(), matchers) || matchByName(m.AliasSymbolName(), matchers) {
			return true
		}
	}
	return false
}

func matchByName(name string, matchers []TypeMatcher) bool {
	if name == "" {
		return false
	}
	for _, m := range matchers {
		if m.Name == name {
			return true
		}
	}
	return false
}
