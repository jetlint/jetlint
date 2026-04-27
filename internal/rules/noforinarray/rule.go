// Package noforinarray implements the no-for-in-array rule: flag
// `for (k in xs)` where xs is an array (use `for…of` or `forEach`).
package noforinarray

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-for-in-array"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindForInStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.ForInOrOfExpression()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isArrayLikeWalking(t, 0) {
		ctx.Report(n, "for-in over an array iterates string-keyed indices and inherited properties; use for-of or forEach instead")
	}
}

const recursionLimit = 16

func isArrayLikeWalking(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return false
	}
	if t.IsTupleType() {
		return true
	}
	if t.IsArrayLikeType() {
		return true
	}
	// HTMLCollection, NodeList, IArguments, and custom `{[k:number]:V; length}`
	// types are array-like by index-signature even when not assignable to
	// ReadonlyArray. Require BOTH numeric index AND `length` so plain
	// numeric-keyed records aren't flagged.
	if t.HasNumericIndex() && hasLengthProperty(t) {
		return true
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

func hasLengthProperty(t *wrapperchecker.Type) bool {
	for _, name := range t.PropertyNames() {
		if name == "length" {
			return true
		}
	}
	return false
}
