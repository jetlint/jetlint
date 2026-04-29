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
		wrapperchecker.KindCallExpression:  visitPromiseAggregator,
	}
}

// visitPromiseAggregator flags `Promise.all`/`race`/`allSettled`/`any`
// calls whose argument is an iterable of values that aren't thenable
// — passing those would silently succeed but the aggregator wouldn't
// actually wait on anything.
func visitPromiseAggregator(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	switch callee.PropertyAccessName() {
	case "all", "race", "allSettled", "any":
	default:
		return
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if recv.LiteralText() != "Promise" {
		return
	}
	args := n.CallArguments()
	if len(args) != 1 {
		return
	}
	t := ctx.TypeOf(args[0])
	if t == nil {
		return
	}
	if t.IsAny() || t.IsUnknown() {
		return
	}
	if iterableHasNonThenableElement(t, 6) {
		ctx.Report(args[0], "Promise."+callee.PropertyAccessName()+" expects an iterable of thenables — non-thenable elements are not awaited")
	}
}

// iterableHasNonThenableElement reports whether any element of the
// iterable t is non-thenable. Walks unions (every member must be
// safe), tuples (each slot is an independent element type), and
// Iterable<T>/Array<T> (the type argument is the element type).
func iterableHasNonThenableElement(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if iterableHasNonThenableElement(m, depth-1) {
				return true
			}
		}
		return false
	}
	if t.IsTupleType() {
		for _, a := range t.TypeArguments() {
			if elementHasNonThenableMember(a) {
				return true
			}
		}
		return false
	}
	if elem := t.ArrayElementType(); elem != nil {
		return elementHasNonThenableMember(elem)
	}
	// Iterable<T>/IterableIterator<T>/AsyncIterable<T>: their first
	// type argument names the yielded element. Match by symbol name
	// to avoid descending into unrelated generics.
	switch t.SymbolName() {
	case "Iterable", "IterableIterator", "AsyncIterable",
		"Generator", "AsyncGenerator",
		"ReadonlyArray", "Array", "Set", "ReadonlySet":
		args := t.TypeArguments()
		if len(args) > 0 {
			return elementHasNonThenableMember(args[0])
		}
	}
	return false
}

// iterableElementType returns the element type of an array-like (or
// union of array-likes) — Array<T>, ReadonlyArray<T>, or a tuple.
// Returns nil when no array-like shape is involved.
func iterableElementType(t *wrapperchecker.Type) *wrapperchecker.Type {
	if t == nil {
		return nil
	}
	if t.IsUnion() {
		var combined []*wrapperchecker.Type
		for _, m := range t.UnionMembers() {
			e := iterableElementType(m)
			if e == nil {
				return nil
			}
			combined = append(combined, e)
		}
		if len(combined) == 0 {
			return nil
		}
		return combined[0]
	}
	if elem := t.ArrayElementType(); elem != nil {
		return elem
	}
	if t.IsTupleType() {
		args := t.TypeArguments()
		if len(args) > 0 {
			return args[0]
		}
	}
	return nil
}

// elementHasNonThenableMember reports whether ANY constituent of the
// element type is non-thenable. Promise.all etc. require every
// element to be thenable for the aggregation to be meaningful.
func elementHasNonThenableMember(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if elementHasNonThenableMember(m) {
				return true
			}
		}
		return false
	}
	if t.IsThenable() || t.IsPromise() {
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return elementHasNonThenableMember(c)
		}
		return false
	}
	return true
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
