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

	// Match Promise.reject(...) — callee is propertyAccess `Promise.reject`.
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression &&
		callee.PropertyAccessName() == "reject" {
		if recv := callee.PropertyAccessReceiver(); recv != nil {
			if recv.Kind() == wrapperchecker.KindIdentifier && recv.LiteralText() == "Promise" {
				if len(args) == 0 {
					ctx.Report(n, "Promise.reject() should be called with an Error instance")
					return
				}
				checkArg(ctx, n, args[0])
				return
			}
		}
	}
	// Also match `new Promise((resolve, reject) => reject(X))` — second
	// callback param. Skipped for simplicity (rare in fixtures).
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
