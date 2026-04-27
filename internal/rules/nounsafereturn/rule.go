// Package nounsafereturn implements the no-unsafe-return rule: flag a
// `return X` where X has type any but the function's declared return
// type is more specific.
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
		wrapperchecker.KindReturnStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if !typeContainsAny(t) {
		return
	}
	// If the function has an explicit return-type annotation, check
	// whether returning any narrows from it. Otherwise (inferred),
	// flag when the expression's type is `any` — upstream considers
	// inferred-any returns unsafe even without an annotation.
	fn := enclosingFunction(n)
	if fn != nil {
		if declared := declaredReturnType(ctx, fn); declared != nil {
			if declared.IsAny() || declared.IsUnknown() {
				return
			}
			// Async: declared is Promise<T>; check inner T.
			if wrapperchecker.HasAsyncModifier(fn) {
				if args := declared.TypeArguments(); len(args) == 1 {
					if args[0].IsAny() || args[0].IsUnknown() {
						return
					}
				}
			}
		}
	}
	ctx.Report(expr, "returning an `any` value defeats the function's declared return type")
}

// typeContainsAny reports whether the type is `any` or any union
// member is `any`.
func typeContainsAny(t *wrapperchecker.Type) bool {
	if t.IsAny() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsAny() {
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

// declaredReturnType returns the function's explicit return-type
// annotation as a Type, or nil for inferred returns.
func declaredReturnType(ctx *engine.Context, fn *wrapperchecker.Node) *wrapperchecker.Type {
	ann := fn.FunctionReturnTypeAnnotation()
	if ann == nil {
		return nil
	}
	return ctx.Checker().TypeFromTypeNode(ann)
}
