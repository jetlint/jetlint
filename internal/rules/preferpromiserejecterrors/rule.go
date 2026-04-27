// Package preferpromiserejecterrors implements the prefer-promise-reject-errors
// rule: flag `Promise.reject(X)` where X is not an Error.
package preferpromiserejecterrors

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-promise-reject-errors"

// Options is the configurable surface of the rule.
type Options struct {
	AllowEmptyReject     bool
	AllowThrowingAny     bool
	AllowThrowingUnknown bool
}

func DefaultOptions() Options {
	// typescript-eslint defaults: any/unknown rejections are flagged
	// unless the user opts in via options.
	return Options{}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("prefer-promise-reject-errors options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowEmptyReject":
			if err := json.Unmarshal(val, &out.AllowEmptyReject); err != nil {
				return Options{}, err
			}
		case "allowThrowingAny":
			if err := json.Unmarshal(val, &out.AllowThrowingAny); err != nil {
				return Options{}, err
			}
		case "allowThrowingUnknown":
			if err := json.Unmarshal(val, &out.AllowThrowingUnknown); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule                          { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule   { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: r.visit,
		wrapperchecker.KindNewExpression:  r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	callee = unwrapParens(callee)
	args := n.CallArguments()

	if isPromiseRejectCallee(ctx, callee) {
		r.reportRejectCall(ctx, n, args)
		return
	}
	if callee.Kind() == wrapperchecker.KindIdentifier && isExecutorRejectParam(ctx, callee) {
		r.reportRejectCall(ctx, n, args)
	}
}

func (r *rule) reportRejectCall(ctx *engine.Context, n *wrapperchecker.Node, args []*wrapperchecker.Node) {
	if len(args) == 0 {
		if r.opts.AllowEmptyReject {
			return
		}
		ctx.Report(n, "Promise.reject() should be called with an Error instance")
		return
	}
	r.checkArg(ctx, n, args[0])
}

// isPromiseRejectCallee reports whether the callee expression is a
// recognizable form of `Promise.reject`: direct property access,
// optional-chained, computed `Promise['reject']`, or via an alias
// like `const foo = Promise; foo.reject(…)`.
func isPromiseRejectCallee(ctx *engine.Context, callee *wrapperchecker.Node) bool {
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		if callee.PropertyAccessName() != "reject" {
			return false
		}
		return receiverIsPromise(ctx, callee.PropertyAccessReceiver())
	case wrapperchecker.KindElementAccessExpression:
		idx := callee.ElementAccessIndex()
		if idx == nil {
			return false
		}
		// `Promise['reject']` — index is a string literal.
		if idx.Kind() != wrapperchecker.KindStringLiteral {
			return false
		}
		if idx.LiteralText() != "reject" {
			return false
		}
		return receiverIsPromise(ctx, callee.ElementAccessReceiver())
	}
	return false
}

func receiverIsPromise(ctx *engine.Context, recv *wrapperchecker.Node) bool {
	if recv == nil {
		return false
	}
	recv = unwrapParens(recv)
	if recv.Kind() == wrapperchecker.KindIdentifier && recv.LiteralText() == "Promise" {
		return true
	}
	// Aliased / inherited: `const foo = Promise; foo.reject(…)` and
	// `class Foo extends Promise<…> {}; Foo.reject(…)`. The receiver's
	// type carries the PromiseConstructor shape directly, in its symbol,
	// or via a base-type name.
	t := ctx.TypeOf(recv)
	if t == nil {
		return false
	}
	if typeIsPromiseConstructor(t) {
		return true
	}
	return false
}

func typeIsPromiseConstructor(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.SymbolName() == "PromiseConstructor" {
		return true
	}
	for _, base := range t.BaseTypeNames() {
		if base == "PromiseConstructor" || base == "Promise" {
			return true
		}
	}
	return false
}

func unwrapParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// isExecutorRejectParam reports whether the identifier at callee
// resolves to the second parameter of a Promise executor (the
// `reject` slot in `new Promise((resolve, reject) => …)`).
func isExecutorRejectParam(ctx *engine.Context, id *wrapperchecker.Node) bool {
	sym := ctx.Checker().SymbolOf(id)
	if sym == nil {
		return false
	}
	for _, decl := range sym.Declarations() {
		if isPromiseExecutorRejectParam(ctx, decl) {
			return true
		}
	}
	return false
}

func isPromiseExecutorRejectParam(ctx *engine.Context, decl *wrapperchecker.Node) bool {
	if decl == nil || decl.Kind() != wrapperchecker.KindParameter {
		return false
	}
	fn := decl.Parent()
	if fn == nil {
		return false
	}
	if fn.Kind() != wrapperchecker.KindArrowFunction && fn.Kind() != wrapperchecker.KindFunctionExpression {
		return false
	}
	idx := -1
	pos := 0
	declPos := decl.Pos()
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			if c.Pos() == declPos {
				idx = pos
				return true
			}
			pos++
		}
		return false
	})
	if idx != 1 {
		return false
	}
	call := fn.Parent()
	if call == nil || call.Kind() != wrapperchecker.KindNewExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil {
		return false
	}
	callee = unwrapParens(callee)
	if callee.Kind() == wrapperchecker.KindIdentifier && callee.LiteralText() == "Promise" {
		return true
	}
	// `new foo.bar(...)` where foo.bar's type is PromiseConstructor.
	if t := ctx.TypeOf(callee); typeIsPromiseConstructor(t) {
		return true
	}
	return false
}


func (r *rule) checkArg(ctx *engine.Context, call, arg *wrapperchecker.Node) {
	t := ctx.TypeOf(arg)
	if t == nil {
		return
	}
	if t.IsAny() {
		if r.opts.AllowThrowingAny {
			return
		}
		ctx.Report(call, "Promise.reject() should be called with an Error instance")
		return
	}
	if t.IsUnknown() {
		if r.opts.AllowThrowingUnknown {
			return
		}
		ctx.Report(call, "Promise.reject() should be called with an Error instance")
		return
	}
	errorT := ctx.Checker().GlobalErrorType()
	if isAcceptable(t, errorT, 0) {
		return
	}
	ctx.Report(call, "Promise.reject() should be called with an Error instance")
}

const recursionLimit = 16

func isAcceptable(t, errorT *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return true
	}
	if t.IsAny() || t.IsUnknown() {
		// Caller has decided whether to allow these — past this point
		// in the recursive walk we just ride along.
		return true
	}
	if t.IsNever() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAcceptable(m, errorT, depth+1) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isAcceptable(m, errorT, depth+1) {
				return true
			}
		}
		return false
	}
	if isErrorName(t.SymbolName()) {
		return true
	}
	for _, base := range t.BaseTypeNames() {
		if isErrorName(base) {
			return true
		}
	}
	// Structural assignability — catches Readonly<Error>, mapped
	// wrappers, and other shapes whose symbol isn't Error but whose
	// shape is.
	if errorT != nil && t.IsAssignableTo(errorT) {
		return true
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return isAcceptable(c, errorT, depth+1)
	}
	return false
}

func isErrorName(name string) bool {
	switch name {
	case "Error", "TypeError", "RangeError", "SyntaxError",
		"ReferenceError", "URIError", "EvalError", "AggregateError":
		return true
	}
	return false
}
