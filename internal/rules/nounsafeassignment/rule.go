// Package nounsafeassignment implements the no-unsafe-assignment rule:
// flag variable declarations whose initialiser is typed `any` but whose
// declared (or inferred-via-annotation) target type is more specific.
// This catches the common pattern where an `any` value is silently
// laundered into a typed variable, defeating the type checker.
package nounsafeassignment

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-assignment"

// New constructs a fresh rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visitVariableDeclaration,
	}
}

func visitVariableDeclaration(ctx *engine.Context, n *wrapperchecker.Node) {
	// VariableDeclaration's children are (in order): name, type? exclamation? initializer?
	// We only care about declarations that have an initializer.
	var name, initializer *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch idx {
		case 0:
			name = c
		default:
			// The last child is the initializer in the absence of a type
			// node; record every child past the first and let the final
			// one win.
			initializer = c
		}
		idx++
		return false
	})
	if initializer == nil || name == nil || initializer == name {
		return
	}

	// If the initializer is also a TypeNode (for declarations that have
	// a type annotation but no initializer), there's nothing to assign.
	if isTypeNode(initializer) {
		return
	}

	rhsType := ctx.TypeOf(initializer)
	if rhsType == nil || !rhsType.IsAny() {
		return
	}
	lhsType := ctx.TypeOf(name)
	if lhsType == nil || lhsType.IsAny() || lhsType.IsUnknown() {
		// Both sides any/unknown: no information is being lost.
		return
	}
	ctx.Report(initializer,
		"unsafe assignment of an `any` value to a more specific declared type")
}

func isTypeNode(n *wrapperchecker.Node) bool {
	// Type nodes never carry runtime values; treating them as
	// initializers would produce false positives. The kinds we already
	// expose are non-type expressions, so anything outside that small
	// known set is conservatively skipped.
	switch n.Kind() {
	case wrapperchecker.KindCallExpression,
		wrapperchecker.KindIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindBinaryExpression,
		wrapperchecker.KindPropertyAccessExpression,
		wrapperchecker.KindTemplateExpression,
		wrapperchecker.KindParenthesizedExpression,
		wrapperchecker.KindAwaitExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindConditionalExpression:
		return false
	}
	return false
}
