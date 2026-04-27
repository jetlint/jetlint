// Package requireawait implements the require-await rule: flag any
// async function whose body never uses await / for-await-of / yield.
package requireawait

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "require-await"

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
	if !wrapperchecker.HasAsyncModifier(n) {
		return
	}
	body := functionBody(n)
	if body == nil {
		return
	}
	if bodyUsesAwait(ctx, body, n) {
		return
	}
	if bodyReturnsPromise(ctx, body, n) {
		return
	}
	ctx.Report(n, "async function has no await expression; remove the async modifier or return a Promise from the body")
}

// bodyReturnsPromise reports whether the function body produces a
// Promise — either a return statement returning a Promise-typed value
// or, for arrow expression bodies, the body itself being a Promise.
// Stops descending into nested function-likes (those have their own
// async/await analysis).
func bodyReturnsPromise(ctx *engine.Context, body *wrapperchecker.Node, fn *wrapperchecker.Node) bool {
	if body == nil {
		return false
	}
	// Arrow concise body — its type IS the function's return value.
	if body.Kind() != wrapperchecker.KindBlock {
		t := ctx.TypeOf(body)
		if t != nil && (t.IsPromise() || anyMemberPromise(t)) {
			return true
		}
		// Block-only walking below skips this case.
	}
	found := false
	var walk func(n *wrapperchecker.Node) bool
	walk = func(n *wrapperchecker.Node) bool {
		if found || n == nil {
			return false
		}
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return false
			}
		}
		if n.Kind() == wrapperchecker.KindReturnStatement {
			if expr := n.FirstChild(); expr != nil {
				t := ctx.TypeOf(expr)
				if t != nil && (t.IsPromise() || anyMemberPromise(t)) {
					found = true
					return false
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return found
		})
		return false
	}
	walk(body)
	return found
}

// isAsyncYield reports whether a YieldExpression introduces async
// iteration: either `yield* asyncIterable` (delegate over an async
// iterator) or `yield <promise>` (yields a Promise the consumer
// awaits). Both count as the function "using await semantics".
func isAsyncYield(ctx *engine.Context, n *wrapperchecker.Node) bool {
	op := n.YieldOperand()
	if op == nil {
		return false
	}
	t := ctx.TypeOf(op)
	if t == nil {
		return false
	}
	if anyMemberPromise(t) {
		return true
	}
	if n.IsYieldDelegate() {
		if hasAsyncIteratorMember(t) {
			return true
		}
		// Fallback: tsgo collapses some operand types to `any` at the
		// yield site. Try the operand's declared type (for identifier
		// references) or the call's signature return type (for direct
		// calls).
		if op.Kind() == wrapperchecker.KindIdentifier {
			if dt := op.DeclaredTypeOfIdentifier(ctx.Checker()); dt != nil &&
				hasAsyncIteratorMember(dt) {
				return true
			}
		}
		if op.Kind() == wrapperchecker.KindCallExpression {
			if sig := ctx.Checker().ResolvedSignature(op); sig != nil {
				if rt := sig.ReturnType(); rt != nil && hasAsyncIteratorMember(rt) {
					return true
				}
			}
		}
	}
	return false
}

// hasAsyncIteratorMember reports whether the type contains
// AsyncIterable / AsyncIterator anywhere — by Symbol.asyncIterator
// property, by symbol-name match, or by the type's printed shape.
// The string-fallback is a heuristic for cases where tsgo collapses
// the type to `any` at the yield site.
func hasAsyncIteratorMember(t *wrapperchecker.Type) bool {
	if matchAsync(t) {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if matchAsync(m) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if matchAsync(m) {
				return true
			}
		}
	}
	return false
}

func matchAsync(t *wrapperchecker.Type) bool {
	for _, name := range t.PropertyNames() {
		if containsSubstring(name, "@asyncIterator") {
			return true
		}
	}
	name := t.SymbolName()
	if containsSubstring(name, "AsyncIter") || containsSubstring(name, "AsyncGenerator") {
		return true
	}
	s := t.String()
	return containsSubstring(s, "AsyncIter") || containsSubstring(s, "AsyncGenerator")
}

func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func anyMemberPromise(t *wrapperchecker.Type) bool {
	if t.IsPromise() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				return true
			}
		}
	}
	return false
}

// functionBody returns the body node of a function-like. For arrow
// functions with a non-block body, returns the expression directly.
func functionBody(n *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

// bodyUsesAwait walks the function body looking for any AwaitExpression
// or for-await-of statement. Stops descending into nested function-like
// nodes (those have their own scopes for await purposes).
func bodyUsesAwait(ctx *engine.Context, body *wrapperchecker.Node, fn *wrapperchecker.Node) bool {
	found := false
	var walk func(n *wrapperchecker.Node) bool
	walk = func(n *wrapperchecker.Node) bool {
		if found || n == nil {
			return false
		}
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return false
			}
		}
		switch n.Kind() {
		case wrapperchecker.KindAwaitExpression:
			found = true
			return false
		case wrapperchecker.KindForOfStatement:
			if n.HasAwaitModifier() {
				found = true
				return false
			}
		case wrapperchecker.KindYieldExpression:
			if isAsyncYield(ctx, n) {
				found = true
				return false
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return found
		})
		return false
	}
	walk(body)
	return found
}
