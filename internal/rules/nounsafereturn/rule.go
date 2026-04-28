// Package nounsafereturn implements the no-unsafe-return rule: flag
// returning a value containing `any` from a function that's expected
// to return something more specific.
package nounsafereturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-return"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement: visitReturn,
		wrapperchecker.KindArrowFunction:   visitArrow,
	}
}

func visitReturn(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	fn := enclosingFunction(n)
	checkReturnedValue(ctx, fn, expr)
}

func visitArrow(ctx *engine.Context, n *wrapperchecker.Node) {
	body := n.FunctionBody()
	if body == nil {
		return
	}
	// Block-bodied arrows are handled by the ReturnStatement visitor.
	if body.Kind() == wrapperchecker.KindBlock {
		return
	}
	checkReturnedValue(ctx, n, body)
}

func checkReturnedValue(ctx *engine.Context, fn *wrapperchecker.Node, expr *wrapperchecker.Node) {
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	declared := declaredReturnTypeFor(ctx, fn)
	if fn != nil && wrapperchecker.HasAsyncModifier(fn) {
		// Async: compare awaited types — `return value` in an async fn
		// produces a Promise<awaited(value)>.
		t = unwrapPromise(t)
		if declared != nil {
			declared = unwrapPromise(declared)
		}
	}
	if declared != nil && (declared.IsAny() || declared.IsUnknown()) {
		return
	}
	if declared == nil {
		// No declared return.
		// For sync functions: passing a Promise<any> through is
		// inferred as the same Promise<any> return type; not unsafe.
		// For async functions: every Promise member's awaited type
		// becomes part of the returned promise's payload. If that
		// payload contains `any`, we're laundering any through the
		// async wrapper.
		if fn != nil && wrapperchecker.HasAsyncModifier(fn) {
			if awaitedContainsAny(t) {
				ctx.Report(expr, "returning an `any` value defeats the function's declared return type")
			}
			return
		}
		if t.IsPromise() {
			return
		}
		if typeContainsAnyDeep(t, 8) {
			ctx.Report(expr, "returning an `any` value defeats the function's declared return type")
		}
		return
	}
	if !returnIsUnsafe(t, declared, 8) {
		return
	}
	// Nested `any` inside generic args (e.g. Map<any, string>) only
	// counts as unsafe when the user wrote `any` in the expression. A
	// bare `new Map()` returns Map<any, any> in tsgo's checker but
	// should have been narrowed contextually to the declared shape.
	if !t.IsAny() && !exprHasExplicitAnyKeyword(expr) {
		return
	}
	ctx.Report(expr, "returning an `any` value defeats the function's declared return type")
}

// exprHasExplicitAnyKeyword reports whether the AST of the returned
// expression contains a literal `any` type-keyword node.
func exprHasExplicitAnyKeyword(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindAnyKeyword {
		return true
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if exprHasExplicitAnyKeyword(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

// awaitedContainsAny reports whether the awaited type of t contains
// `any` anywhere. Walks unions and intersections so a union of
// Promise<any> | Promise<object> resolves to (any | object), where
// the any leaks through.
func awaitedContainsAny(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	awaited := unwrapPromise(t)
	if awaited.IsAny() {
		return true
	}
	if awaited.IsUnion() {
		for _, m := range awaited.UnionMembers() {
			if awaitedContainsAny(m) {
				return true
			}
		}
		return false
	}
	if awaited.IsIntersection() {
		for _, m := range awaited.IntersectionMembers() {
			if awaitedContainsAny(m) {
				return true
			}
		}
		return false
	}
	return false
}

// unwrapPromise recurses through Promise<…<X>> wrappers and returns
// the innermost non-Promise type. Async returns flatten any number of
// promise layers, so `Promise<Promise<X>>` resolves to `X`.
func unwrapPromise(t *wrapperchecker.Type) *wrapperchecker.Type {
	for i := 0; i < 8 && t != nil; i++ {
		if !t.IsPromise() {
			return t
		}
		args := t.TypeArguments()
		if len(args) != 1 {
			return t
		}
		t = args[0]
	}
	return t
}

// typeContainsAnyDeep reports whether the type or any of its nested
// type arguments / union members is `any`.
func typeContainsAnyDeep(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsAny() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeContainsAnyDeep(m, depth-1) {
				return true
			}
		}
	}
	for _, a := range t.TypeArguments() {
		if typeContainsAnyDeep(a, depth-1) {
			return true
		}
	}
	return false
}

// returnIsUnsafe reports whether returning an actual value of type
// `actual` to a function expecting `declared` would smuggle an `any`
// past the type system. The check recurses into matching generic type
// arguments (Set<any> vs Set<string>) but doesn't unwrap when the
// declared and actual types have different shapes.
func returnIsUnsafe(actual, declared *wrapperchecker.Type, depth int) bool {
	if actual == nil || depth <= 0 {
		return false
	}
	if actual.IsAny() {
		if declared != nil && (declared.IsAny() || declared.IsUnknown()) {
			return false
		}
		return true
	}
	if actual.IsUnion() {
		for _, m := range actual.UnionMembers() {
			if returnIsUnsafe(m, declared, depth-1) {
				return true
			}
		}
		return false
	}
	if declared == nil {
		return false
	}
	// Same generic type — compare type arguments positionally.
	actualArgs := actual.TypeArguments()
	declaredArgs := declared.TypeArguments()
	if len(actualArgs) > 0 && len(actualArgs) == len(declaredArgs) &&
		actual.SymbolName() == declared.SymbolName() && actual.SymbolName() != "" {
		for i := range actualArgs {
			if returnIsUnsafe(actualArgs[i], declaredArgs[i], depth-1) {
				return true
			}
		}
	}
	return false
}

func enclosingFunction(n *wrapperchecker.Node) *wrapperchecker.Node {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration:
			return cur
		}
	}
	return nil
}

// declaredReturnTypeFor returns the function's declared return type
// (annotation or contextual type), or nil for inferred returns with
// no contextual constraint.
func declaredReturnTypeFor(ctx *engine.Context, fn *wrapperchecker.Node) *wrapperchecker.Type {
	if fn == nil {
		return nil
	}
	if ann := fn.FunctionReturnTypeAnnotation(); ann != nil {
		return ctx.Checker().TypeFromTypeNode(ann)
	}
	// Function expressions / arrows passed as args or assigned to
	// typed slots get a contextual return type from the surrounding
	// signature. Use the contextual type's call signatures.
	if t := ctx.Checker().ContextualTypeOf(fn); t != nil {
		for _, sig := range t.CallSignatures() {
			rt := sig.ReturnType()
			if rt != nil {
				return rt
			}
		}
	}
	return nil
}
