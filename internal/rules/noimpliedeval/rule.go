// Package noimpliedeval implements the no-implied-eval rule: flag
// setTimeout / setInterval / setImmediate / execScript called with a
// string-typed first argument (which the runtime would eval).
//
// Also flags `new Function(...)` (always evals its body string).
package noimpliedeval

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-implied-eval"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
		wrapperchecker.KindNewExpression:  visitNew,
	}
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	// Function() called without `new` is also implied eval.
	if isFunctionConstructorCallee(ctx, callee) {
		ctx.Report(n, "constructing a function from a string is implied eval")
		return
	}
	if !isImpliedEvalCallee(callee) {
		return
	}
	if !isGlobalImpliedEvalCallee(ctx, callee) {
		return
	}
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	arg := args[0]
	t := ctx.TypeOf(arg)
	if t == nil {
		return
	}
	if !isCallableNotString(t) {
		ctx.Report(arg, "passing a non-function (string, undefined, any, etc.) to setTimeout/setInterval/setImmediate/execScript invokes the engine's eval; use a function instead")
	}
}

func isGlobalImpliedEvalCallee(ctx *engine.Context, callee *wrapperchecker.Node) bool {
	if callee.Kind() != wrapperchecker.KindIdentifier {
		return true
	}
	return !callee.FileHasTopLevelDeclaration(callee.LiteralText())
}

func isCallableNotString(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsStringLike() {
		return false
	}
	if t.IsNullOrUndefined() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isCallableNotString(m) {
				return false
			}
		}
		return true
	}
	if len(t.CallSignatures()) > 0 {
		return true
	}
	// `Function` (the prototype interface) — has callable apparent type.
	if t.SymbolName() == "Function" {
		return true
	}
	return false
}

func visitNew(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isFunctionConstructorCallee(ctx, n.CalleeExpression()) {
		return
	}
	ctx.Report(n, "constructing a function from a string is implied eval")
}

// isFunctionConstructorCallee reports whether the callee is the
// global Function constructor — `Function`, `window.Function`, or
// `window['Function']`.
func isFunctionConstructorCallee(ctx *engine.Context, callee *wrapperchecker.Node) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		if callee.LiteralText() != "Function" {
			return false
		}
		if callee.FileHasTopLevelDeclaration("Function") {
			return false
		}
		if callee.IsImportedIdentifier(ctx.Checker()) {
			return false
		}
		return true
	case wrapperchecker.KindPropertyAccessExpression:
		if callee.PropertyAccessName() != "Function" {
			return false
		}
		return isGlobalReceiver(callee.PropertyAccessReceiver())
	case wrapperchecker.KindElementAccessExpression:
		var children []*wrapperchecker.Node
		callee.ForEachChild(func(c *wrapperchecker.Node) bool {
			children = append(children, c)
			return false
		})
		if len(children) < 2 {
			return false
		}
		if !isGlobalReceiver(children[0]) {
			return false
		}
		idx := children[1]
		return idx.Kind() == wrapperchecker.KindStringLiteral && idx.LiteralText() == "Function"
	}
	return false
}

// isImpliedEvalCallee reports whether the callee names one of
// setTimeout / setInterval / setImmediate / execScript directly or via
// `window.<name>` / `globalThis.<name>` / `window['<name>']`. Property
// and element access only count when the receiver is the global object
// (window/globalThis/global/self).
func isImpliedEvalCallee(callee *wrapperchecker.Node) bool {
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return isImpliedEvalName(callee.LiteralText())
	case wrapperchecker.KindPropertyAccessExpression:
		recv := callee.PropertyAccessReceiver()
		if !isGlobalReceiver(recv) {
			return false
		}
		return isImpliedEvalName(callee.PropertyAccessName())
	case wrapperchecker.KindElementAccessExpression:
		var children []*wrapperchecker.Node
		callee.ForEachChild(func(c *wrapperchecker.Node) bool {
			children = append(children, c)
			return false
		})
		if len(children) < 2 {
			return false
		}
		recv := children[0]
		if !isGlobalReceiver(recv) {
			return false
		}
		idx := children[1]
		if idx.Kind() == wrapperchecker.KindStringLiteral {
			return isImpliedEvalName(idx.LiteralText())
		}
	}
	return false
}

func isGlobalReceiver(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	switch n.LiteralText() {
	case "window", "globalThis", "global", "self":
		return true
	}
	return false
}

func isImpliedEvalName(name string) bool {
	switch name {
	case "setTimeout", "setInterval", "setImmediate", "execScript":
		return true
	}
	return false
}
