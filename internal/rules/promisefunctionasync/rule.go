// Package promisefunctionasync implements the promise-function-async
// rule: flag any function-like declaration whose declared (or inferred)
// return type is a Promise but isn't declared `async`.
package promisefunctionasync

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "promise-function-async"

// Options mirrors the upstream config switches that gate which
// function-like kinds are inspected.
type Options struct {
	AllowAny                  bool
	AllowedPromiseNames       []string
	CheckArrowFunctions       bool
	CheckFunctionDeclarations bool
	CheckFunctionExpressions  bool
	CheckMethodDeclarations   bool
}

func DefaultOptions() Options {
	return Options{
		CheckArrowFunctions:       true,
		CheckFunctionDeclarations: true,
		CheckFunctionExpressions:  true,
		CheckMethodDeclarations:   true,
	}
}

func New() engine.Rule                        { return &rule{opts: DefaultOptions()} }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: r.visit,
		wrapperchecker.KindFunctionExpression:  r.visit,
		wrapperchecker.KindArrowFunction:       r.visit,
		wrapperchecker.KindMethodDeclaration:   r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if wrapperchecker.IsAsyncFunction(n) {
		return
	}
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration:
		if !r.opts.CheckFunctionDeclarations {
			return
		}
	case wrapperchecker.KindFunctionExpression:
		if !r.opts.CheckFunctionExpressions {
			return
		}
	case wrapperchecker.KindArrowFunction:
		if !r.opts.CheckArrowFunctions {
			return
		}
	case wrapperchecker.KindMethodDeclaration:
		if !r.opts.CheckMethodDeclarations {
			return
		}
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
	allowed := r.opts.AllowedPromiseNames
	hasExplicitAnnotation := n.FunctionReturnType() != nil
	allPromiseInOverloads := true
	anyPromiseInUnion := false
	for _, sig := range sigs {
		rt := sig.ReturnType()
		if rt == nil {
			return
		}
		if !r.opts.AllowAny && (rt.IsAny() || rt.IsUnknown()) {
			ctx.Report(n, "function declares `any`/`unknown` return — make async or annotate as Promise")
			return
		}
		if isAllPromiseLike(rt, allowed, 0) {
			continue
		}
		allPromiseInOverloads = false
		if hasPromiseLikeMember(rt, allowed, 0) {
			anyPromiseInUnion = true
		}
	}
	if allPromiseInOverloads {
		ctx.Report(n, "function returns a Promise but is not declared `async`; mark async to make the return type explicit and enable await")
		return
	}
	// An inferred mixed-Promise return is still flag-worthy — the user
	// can refactor to `async`. An explicit annotation that opted into a
	// Promise-bearing union is the user's stated intent, so leave it.
	if anyPromiseInUnion && !hasExplicitAnnotation {
		ctx.Report(n, "function returns a Promise but is not declared `async`; mark async to make the return type explicit and enable await")
	}
}

func isAllPromiseLike(t *wrapperchecker.Type, allowed []string, depth int) bool {
	if t == nil || depth > recursionLimit {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAllPromiseLike(m, allowed, depth+1) {
				return false
			}
		}
		return true
	}
	return isPromiseLikeType(t, allowed)
}

func hasPromiseLikeMember(t *wrapperchecker.Type, allowed []string, depth int) bool {
	if t == nil || depth > recursionLimit {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if hasPromiseLikeMember(m, allowed, depth+1) {
				return true
			}
		}
		return false
	}
	return isPromiseLikeType(t, allowed)
}

func isPromiseLikeType(t *wrapperchecker.Type, allowed []string) bool {
	if t.IsPromise() {
		return true
	}
	name := t.SymbolName()
	for _, p := range allowed {
		if name == p {
			return true
		}
	}
	return false
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
