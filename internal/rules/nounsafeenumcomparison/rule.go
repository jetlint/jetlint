// Package nounsafeenumcomparison implements the no-unsafe-enum-comparison
// rule: flag a comparison where one side is an enum and the other isn't
// the same enum (e.g. `Fruit.Apple === 0`).
package nounsafeenumcomparison

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-enum-comparison"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	if !isComparison(op) {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftT := ctx.TypeOf(left)
	rightT := ctx.TypeOf(right)
	if leftT == nil || rightT == nil {
		return
	}
	leftEnum := containsEnum(leftT)
	rightEnum := containsEnum(rightT)
	if !leftEnum && !rightEnum {
		return
	}
	// Both sides enum: assume comparison is intentional.
	// If only one side is an enum and the other isn't, flag — unless
	// the non-enum side is null/undefined/any/unknown (allowed).
	if leftEnum && rightEnum {
		return
	}
	other := rightT
	if rightEnum {
		other = leftT
	}
	if other.IsAny() || other.IsUnknown() || other.IsNullOrUndefined() {
		return
	}
	ctx.Report(n, "comparing an enum to a non-enum value; either use the enum's member or compare two enums of the same type")
}

func isComparison(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken,
		wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken:
		return true
	}
	return false
}

func containsEnum(t *wrapperchecker.Type) bool {
	if t.IsEnumLike() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsEnumLike() {
				return true
			}
		}
	}
	return false
}
