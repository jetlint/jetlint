// Package preferreducetypeparameter implements the
// prefer-reduce-type-parameter rule: flag `arr.reduce(fn, init as T)`
// in favor of `arr.reduce<T>(fn, init)`.
package preferreducetypeparameter

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-reduce-type-parameter"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "reduce" {
		return
	}
	args := n.CallArguments()
	if len(args) != 2 {
		return
	}
	// Second argument must be an `as`-cast — that's the form the rule
	// rewrites into a type parameter on `reduce`.
	if args[1].Kind() != wrapperchecker.KindAsExpression {
		return
	}
	// Receiver must be an array-like (or tuple), since reduce on a
	// custom Reducer interface may genuinely need the cast.
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil || !isArrayLike(rt) {
		return
	}
	ctx.Report(args[1], "use `reduce<T>(...)` to declare the accumulator type instead of casting the initial value")
}

func isArrayLike(t *wrapperchecker.Type) bool {
	if t.IsTupleType() || t.IsArrayLikeType() || t.ArrayElementType() != nil {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isArrayLike(m) {
				return false
			}
		}
		return true
	}
	// Intersection: don't flag — the non-array side may add a custom
	// reduce signature where the cast is meaningful.
	return false
}
