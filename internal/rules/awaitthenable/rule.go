// Package awaitthenable implements the await-thenable rule: flag any
// `await X` whose operand is not a thenable (Promise or object with a
// callable `then` property).
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint.
package awaitthenable

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "await-thenable"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAwaitExpression: visit,
		wrapperchecker.KindForOfStatement:  visitForOf,
	}
}

func visitForOf(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.HasAwaitModifier() {
		return
	}
	iter := n.ForOfIterable()
	if iter == nil {
		return
	}
	t := ctx.TypeOf(iter)
	if t == nil {
		return
	}
	if isAsyncIterable(t) || t.IsAny() || t.IsUnknown() {
		return
	}
	ctx.Report(iter, "for-await-of of a value that isn't an async iterable; the await has no effect")
}

// isAsyncIterable reports whether t exposes Symbol.asyncIterator
// (directly, on its symbol-name, or through a union/intersection).
func isAsyncIterable(t *wrapperchecker.Type) bool {
	if matchAsyncIter(t) {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if matchAsyncIter(m) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if matchAsyncIter(m) {
				return true
			}
		}
	}
	return false
}

func matchAsyncIter(t *wrapperchecker.Type) bool {
	for _, name := range t.PropertyNames() {
		if containsSubstring(name, "@asyncIterator") {
			return true
		}
	}
	if name := t.SymbolName(); containsSubstring(name, "AsyncIter") || containsSubstring(name, "AsyncGenerator") {
		return true
	}
	return false
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	t := ctx.TypeOf(operand)
	if t == nil {
		return
	}
	if isAcceptableForAwait(t) {
		return
	}
	ctx.Report(operand, "await of a value whose type is not thenable; the await has no effect")
}

// isAcceptableForAwait returns true for any/unknown, thenables, or
// generic types whose constraint resolves to one of the above.
func isAcceptableForAwait(t *wrapperchecker.Type) bool {
	return acceptable(t, 0)
}

const recursionLimit = 16

func acceptable(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return true
	}
	if t.IsAny() || t.IsUnknown() {
		return true
	}
	// Unconstrained type parameter: equivalent to unknown — could be
	// thenable.
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil {
			return acceptable(c, depth+1)
		}
		return true
	}
	if isAnyMemberThenable(t) {
		return true
	}
	return false
}

func isAnyMemberThenable(t *wrapperchecker.Type) bool {
	if t.IsThenable() || t.IsPromise() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsThenable() || m.IsPromise() {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if m.IsThenable() || m.IsPromise() {
				return true
			}
		}
	}
	return false
}
