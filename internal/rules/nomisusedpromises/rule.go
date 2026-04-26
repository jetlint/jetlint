// Package nomisusedpromises implements the no-misused-promises rule:
// flag any call expression argument that is an async function (returns
// Promise) where the parameter type expects a callback returning void.
// The classic case is `arr.forEach(async () => ...)`, where the
// returned Promise is silently dropped.
package nomisusedpromises

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-misused-promises"

// New constructs a fresh rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCallExpression,
	}
}

func visitCallExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	args := n.CallArguments()
	for i, arg := range args {
		if !isAsyncCallback(arg) {
			continue
		}
		expected := ctx.Checker().ContextualTypeForArgument(n, i)
		if expected == nil {
			continue
		}
		if !expectsVoidReturn(expected) {
			continue
		}
		ctx.Report(arg,
			"async callback returns a Promise that the parameter's void return type silently drops")
	}
}

// isAsyncCallback reports whether the argument node is a function
// literal declared with the `async` modifier.
func isAsyncCallback(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression:
		return wrapperchecker.IsAsyncFunction(n)
	}
	return false
}

// expectsVoidReturn returns true when the parameter type's call
// signature(s) all return void or void-like types.
func expectsVoidReturn(t *wrapperchecker.Type) bool {
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
