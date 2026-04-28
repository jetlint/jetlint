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
// (Array<T>, mutable tuple). Conservatively returns false for
// primitives, readonly arrays, and unknown shapes.
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
	if t.IsTupleType() {
		// `readonly [...]` prints with the readonly keyword; without
		// it the tuple is mutable.
		if !strings.HasPrefix(t.String(), "readonly") {
			return true
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
