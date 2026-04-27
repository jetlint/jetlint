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
	// Abstract methods don't have a body, so they can't be marked async.
	if n.HasAbstractModifier() {
		return
	}
	// Method declarations on interfaces/abstract classes have no body
	// (signature only) — skip those too.
	if n.FunctionBody() == nil && n.Kind() != wrapperchecker.KindArrowFunction {
		return
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return
	}
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return
	}
	for _, sig := range sigs {
		rt := sig.ReturnType()
		if rt == nil {
			return
		}
		// Every overload must be purely Promise-returning. A union
		// member that isn't a Promise (or any non-promise overload)
		// means the function legitimately returns a non-promise on
		// some path, so async would be wrong.
		if !isAllPromise(rt, 0) {
			return
		}
	}
	ctx.Report(n, "function returns a Promise but is not declared `async`; mark async to make the return type explicit and enable await")
}

const recursionLimit = 16

func isAllPromise(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAllPromise(m, depth+1) {
				return false
			}
		}
		return true
	}
	return t.IsPromise()
}
