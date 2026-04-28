// Package nounsafeassignment implements the no-unsafe-assignment rule:
// flag where an `any` value is laundered into a more-specific typed
// slot — variable declarations, assignment expressions, object
// literal property values, and array elements.
package nounsafeassignment

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-assignment"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visitVariableDeclaration,
		wrapperchecker.KindBinaryExpression:    visitBinary,
		wrapperchecker.KindPropertyAssignment:  visitPropertyAssignment,
		wrapperchecker.KindParameter:           visitParameter,
		wrapperchecker.KindPropertyDeclaration: visitPropertyDeclaration,
		wrapperchecker.KindBindingElement:      visitBindingElement,
	}
}

func visitBindingElement(ctx *engine.Context, n *wrapperchecker.Node) {
	// Walk up to find the enclosing variable declaration / parameter.
	// If that declaration has an annotation, the user opted in; we
	// only flag when the inferred type at this binding position is
	// `any` and there's no annotation in the declaration chain.
	if hasAnnotationAncestor(n) {
		return
	}
	t := ctx.TypeOf(n)
	if t == nil {
		return
	}
	if t.IsAny() {
		ctx.Report(n, "unsafe assignment of an `any` value via destructuring")
	}
}

func hasAnnotationAncestor(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindVariableDeclaration:
			return cur.VariableDeclarationType() != nil
		case wrapperchecker.KindParameter:
			return cur.ParameterTypeAnnotation() != nil
		case wrapperchecker.KindPropertyDeclaration:
			return cur.PropertyDeclarationType() != nil
		}
	}
	return false
}

func visitParameter(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.ParameterInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	lhs := lhsFromParameterAnnotation(ctx, n)
	check(ctx, init, rhs, lhs)
}

func visitPropertyDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.PropertyDeclarationInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	lhs := lhsFromPropertyAnnotation(ctx, n)
	check(ctx, init, rhs, lhs)
}

// lhsFromParameterAnnotation returns the parameter's declared type
// only when there's an explicit type annotation; an inferred any from
// `(a = ...)` shouldn't suppress the rhs check.
func lhsFromParameterAnnotation(ctx *engine.Context, n *wrapperchecker.Node) *wrapperchecker.Type {
	annot := n.ParameterTypeAnnotation()
	if annot == nil {
		return nil
	}
	return ctx.Checker().TypeFromTypeNode(annot)
}

func lhsFromPropertyAnnotation(ctx *engine.Context, n *wrapperchecker.Node) *wrapperchecker.Type {
	annot := n.PropertyDeclarationType()
	if annot == nil {
		return nil
	}
	return ctx.Checker().TypeFromTypeNode(annot)
}

func visitVariableDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	if rhs == nil {
		return
	}
	annot := n.VariableDeclarationType()
	var lhs *wrapperchecker.Type
	if annot != nil {
		lhs = ctx.Checker().TypeFromTypeNode(annot)
	}
	check(ctx, init, rhs, lhs)
}

func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.BinaryOperatorKind() != wrapperchecker.KindEqualsToken {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	rhs := ctx.TypeOf(right)
	lhs := ctx.TypeOf(left)
	check(ctx, right, rhs, lhs)
}

func visitPropertyAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.PropertyInitializer()
	if init == nil {
		return
	}
	rhs := ctx.TypeOf(init)
	lhs := ctx.Checker().ContextualTypeOf(init)
	if lhs == nil {
		return
	}
	check(ctx, init, rhs, lhs)
}

func check(ctx *engine.Context, src *wrapperchecker.Node, rhs, lhs *wrapperchecker.Type) {
	if rhs == nil {
		return
	}
	// `unknown` on the lhs is the explicit "I'll narrow later" escape
	// hatch — assigning any to unknown isn't unsafe.
	if lhs != nil && lhs.IsUnknown() {
		return
	}
	if rhs.IsAny() {
		// Don't flag when lhs is also any (no information lost).
		if lhs != nil && lhs.IsAny() {
			return
		}
		ctx.Report(src, "unsafe assignment of an `any` value")
		return
	}
	if lhs == nil {
		return
	}
	if lhs.IsAny() {
		return
	}
	if assignmentIsUnsafe(rhs, lhs, 8) {
		ctx.Report(src, "unsafe assignment of an `any` value to a more specific declared type")
	}
}

// assignmentIsUnsafe reports whether the rhs smuggles `any` past the
// declared lhs type. Bare any → flag. Generic shapes that match the
// lhs in name and arity recurse into matching positions so
// `Set<any>` to `Set<string>` flags via positional argument compare.
func assignmentIsUnsafe(rhs, lhs *wrapperchecker.Type, depth int) bool {
	if rhs == nil || depth <= 0 {
		return false
	}
	if rhs.IsAny() {
		if lhs.IsAny() || lhs.IsUnknown() {
			return false
		}
		return true
	}
	if rhs.IsUnion() {
		for _, m := range rhs.UnionMembers() {
			if assignmentIsUnsafe(m, lhs, depth-1) {
				return true
			}
		}
		return false
	}
	rhsArgs := rhs.TypeArguments()
	lhsArgs := lhs.TypeArguments()
	if len(rhsArgs) > 0 && len(rhsArgs) == len(lhsArgs) &&
		rhs.SymbolName() == lhs.SymbolName() && rhs.SymbolName() != "" {
		for i := range rhsArgs {
			if assignmentIsUnsafe(rhsArgs[i], lhsArgs[i], depth-1) {
				return true
			}
		}
	}
	return false
}
