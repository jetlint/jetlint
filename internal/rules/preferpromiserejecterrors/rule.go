// Package preferpromiserejecterrors implements the prefer-promise-reject-errors
// rule: flag `Promise.reject(X)` where X is not an Error.
package preferpromiserejecterrors

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-promise-reject-errors"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
		wrapperchecker.KindNewExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	args := n.CallArguments()

	// Promise.reject(...) — propertyAccess on Promise (or alias).
	if isPromiseRejectCallee(ctx, callee) {
		reportRejectCall(ctx, n, args)
		return
	}
	// reject(X) inside `new Promise((resolve, reject) => …)` — the
	// callee resolves to the executor's reject parameter.
	if callee.Kind() == wrapperchecker.KindIdentifier && isExecutorRejectParam(ctx, callee) {
		reportRejectCall(ctx, n, args)
	}
}

func reportRejectCall(ctx *engine.Context, n *wrapperchecker.Node, args []*wrapperchecker.Node) {
	if len(args) == 0 {
		ctx.Report(n, "Promise.reject() should be called with an Error instance")
		return
	}
	checkArg(ctx, n, args[0])
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
	if recv.Kind() == wrapperchecker.KindIdentifier && recv.LiteralText() == "Promise" {
		return true
	}
	// Aliased: `const foo = Promise; foo.reject(…)` — type's symbol is
	// the PromiseConstructor.
	t := ctx.TypeOf(recv)
	if t == nil {
		return false
	}
	if t.SymbolName() == "PromiseConstructor" {
		return true
	}
	for _, base := range t.BaseTypeNames() {
		if base == "PromiseConstructor" {
			return true
		}
	}
	return false
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
		if !isPromiseExecutorRejectParam(decl) {
			continue
		}
		return true
	}
	return false
}

func isPromiseExecutorRejectParam(decl *wrapperchecker.Node) bool {
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
	// Find this parameter's index in the function's parameter list. A
	// reject is the second parameter (index 1).
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
	// Function must be the first argument of `new Promise(...)`.
	call := fn.Parent()
	if call == nil || call.Kind() != wrapperchecker.KindNewExpression {
		return false
	}
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return callee.LiteralText() == "Promise"
}

func checkArg(ctx *engine.Context, call, arg *wrapperchecker.Node) {
	t := ctx.TypeOf(arg)
	if t == nil {
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
	if t.IsAny() {
		return true
	}
	if t.IsUnknown() || t.IsNever() {
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
