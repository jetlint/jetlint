// Package promisefunctionasync implements the promise-function-async
// rule: flag any function-like declaration whose declared (or inferred)
// return type is a Promise but isn't declared `async`.
package promisefunctionasync

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "promise-function-async"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if wrapperchecker.IsAsyncFunction(n) {
		return
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return
	}
	for _, sig := range t.CallSignatures() {
		rt := sig.ReturnType()
		if rt == nil {
			continue
		}
		if isPromiseLike(rt, 0) {
			ctx.Report(n, "function returns a Promise but is not declared `async`; mark async to make the return type explicit and enable await")
			return
		}
	}
}

const recursionLimit = 16

func isPromiseLike(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return false
	}
	if t.IsPromise() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isPromiseLike(m, depth+1) {
				return true
			}
		}
	}
	return false
}
