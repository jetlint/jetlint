// Package noarraydelete implements the no-array-delete rule: flag
// `delete arr[i]` where arr is an array (creates a hole instead of
// shifting elements).
package noarraydelete

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-array-delete"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindDeleteExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	// Only flag when operand is an element-access expression `obj[idx]`
	// whose object resolves to an array-like type. Delete of a property
	// access (`obj.foo`) is fine.
	if operand.Kind() != wrapperchecker.KindElementAccessExpression {
		return
	}
	obj := operand.FirstChild()
	if obj == nil {
		return
	}
	t := ctx.TypeOf(obj)
	if t == nil {
		return
	}
	if isArrayLikeWalking(t, 0) {
		ctx.Report(n, "delete on an array element creates a hole; use splice() to remove an element")
	}
}

const recursionLimit = 16

func isArrayLikeWalking(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsTupleType() {
		return true
	}
	if t.IsArrayLikeType() {
		// Exclude string (string is array-like with .length and char index).
		if t.IsStringLike() {
			return false
		}
		// Tuples are caught above; require a real Array.
		if t.SymbolName() == "Array" || t.SymbolName() == "ReadonlyArray" {
			return true
		}
		// Also any array literal type via element type detection.
		if t.ArrayElementType() != nil {
			return true
		}
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isArrayLikeWalking(m, depth+1) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isArrayLikeWalking(m, depth+1) {
				return true
			}
		}
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return isArrayLikeWalking(c, depth+1)
	}
	return false
}
