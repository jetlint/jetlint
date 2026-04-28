// Package preferreadonlyparametertypes implements the
// prefer-readonly-parameter-types rule: flag function parameters
// whose declared type is mutable.
package preferreadonlyparametertypes

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-readonly-parameter-types"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindParameter: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	annot := n.ParameterTypeAnnotation()
	if annot == nil {
		return
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return
	}
	if !typeIsMutable(t, 8) {
		return
	}
	ctx.Report(n, "parameter is mutable; declare it readonly")
}

// typeIsMutable reports whether t is a mutable container shape
// (Array<T>, mutable tuple, or any container whose nested elements
// are mutable). Conservatively returns false for primitives,
// readonly arrays without mutable inner elements, and unknown shapes.
func typeIsMutable(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.SymbolName() == "Array" {
		return true
	}
	// ReadonlyArray<X> — outer is fine but inner X may be mutable.
	if t.SymbolName() == "ReadonlyArray" {
		for _, a := range t.TypeArguments() {
			if typeIsMutable(a, depth-1) {
				return true
			}
		}
		return false
	}
	if t.IsTupleType() {
		readonly := strings.HasPrefix(t.String(), "readonly")
		if !readonly {
			return true
		}
		// Readonly tuple — check element types.
		for _, a := range t.TypeArguments() {
			if typeIsMutable(a, depth-1) {
				return true
			}
		}
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeIsMutable(m, depth-1) {
				return true
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if typeIsMutable(m, depth-1) {
				return true
			}
		}
	}
	return false
}
