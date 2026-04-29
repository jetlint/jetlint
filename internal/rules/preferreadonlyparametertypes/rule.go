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
// (Array<T>, mutable tuple, an object with non-readonly properties,
// or a container whose nested elements are mutable). Conservatively
// returns false for primitives, fully-readonly types, and unknown
// shapes.
func typeIsMutable(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if typeIsMutable(m, depth-1) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if typeIsMutable(m, depth-1) {
				return true
			}
		}
		return false
	}
	// Primitives are immutable.
	if t.IsBooleanLike() || t.IsStringLike() || t.IsNumberLike() ||
		t.IsBigIntLike() || t.IsNullOrUndefined() || t.IsVoid() ||
		t.IsESSymbolLike() || t.IsNever() {
		return false
	}
	// Function types: typescript-eslint treats callable types as
	// readonly — the call signature is immutable, and the standard
	// `Function` prototype members are effectively readonly even
	// though they're not declared with the modifier in lib.d.ts.
	hasCall := len(t.CallSignatures()) > 0
	if hasCall && !hasNonFunctionProperty(t) {
		return false
	}
	// Type parameters: forward to the constraint when narrower.
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeIsMutable(c, depth-1)
		}
		return false
	}
	if t.SymbolName() == "Array" {
		return true
	}
	if t.SymbolName() == "ReadonlyArray" {
		for _, a := range t.TypeArguments() {
			if typeIsMutable(a, depth-1) {
				return true
			}
		}
		return false
	}
	if t.IsTupleType() {
		if !strings.HasPrefix(t.String(), "readonly") {
			return true
		}
		for _, a := range t.TypeArguments() {
			if typeIsMutable(a, depth-1) {
				return true
			}
		}
		return false
	}
	// Object/interface/anonymous type: any non-readonly property
	// (including methods) makes the parameter mutable. For callable
	// types, skip the standard Function members — they're declared
	// without the modifier in lib.d.ts but are effectively readonly.
	for _, name := range t.PropertyNames() {
		if hasCall && isStandardFunctionProp(name) {
			continue
		}
		sym := t.PropertySymbol(name)
		if sym == nil {
			continue
		}
		if !sym.IsReadonly() {
			return true
		}
		// Even if the slot is readonly, the value at that slot might
		// itself be mutable (e.g. `readonly arr: number[]`).
		if pt := t.PropertyType(name); pt != nil {
			if typeIsMutable(pt, depth-1) {
				return true
			}
		}
	}
	return false
}

// hasNonFunctionProperty reports whether t carries any apparent
// property beyond the standard `Function` interface — typically
// from JS-expando assignments. Such added props might be mutable.
func hasNonFunctionProperty(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	for _, name := range t.PropertyNames() {
		if !isStandardFunctionProp(name) {
			return true
		}
	}
	return false
}

func isStandardFunctionProp(name string) bool {
	switch name {
	case "toString", "prototype", "length", "arguments", "caller",
		"apply", "call", "bind", "name":
		return true
	}
	if len(name) > 2 && name[0] == 0xfe {
		// Well-known symbol prefix in typescript-go internals.
		return true
	}
	return false
}
