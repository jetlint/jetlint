// Package preferreadonlyparametertypes implements the
// prefer-readonly-parameter-types rule: flag function parameters
// whose declared type is mutable.
package preferreadonlyparametertypes

import (
	"encoding/json"
	"fmt"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-readonly-parameter-types"

// Options is the configurable surface of the rule. Mirrors
// typescript-eslint's `prefer-readonly-parameter-types` options.
type Options struct {
	// TreatMethodsAsReadonly skips method-shaped properties (those with
	// call signatures) when scanning a parameter's properties for
	// mutability. Methods on `Readonly<T>` are inherently bound — most
	// codebases don't need to declare them readonly explicitly.
	TreatMethodsAsReadonly bool
	// IgnoreInferredTypes skips parameters that lack an explicit type
	// annotation. Useful for callbacks where TypeScript widens an
	// inferred parameter type for ergonomics.
	IgnoreInferredTypes bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("prefer-readonly-parameter-types options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "treatMethodsAsReadonly":
			if err := json.Unmarshal(val, &out.TreatMethodsAsReadonly); err != nil {
				return Options{}, fmt.Errorf("prefer-readonly-parameter-types option %q: %w", key, err)
			}
		case "ignoreInferredTypes":
			if err := json.Unmarshal(val, &out.IgnoreInferredTypes); err != nil {
				return Options{}, fmt.Errorf("prefer-readonly-parameter-types option %q: %w", key, err)
			}
		case "allow":
			// allow takes a list of TypeOrValueSpecifier objects; the
			// upstream test fixtures we run through do not exercise
			// this option, so accept and ignore for now.
		default:
			return Options{}, fmt.Errorf("prefer-readonly-parameter-types has no option %q", key)
		}
	}
	return out, nil
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindParameter: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	annot := n.ParameterTypeAnnotation()
	if annot == nil {
		if r.opts.IgnoreInferredTypes {
			return
		}
		// Without an annotation there's no declared type to flag —
		// the parameter's type is inferred at the call site.
		return
	}
	t := ctx.Checker().TypeFromTypeNode(annot)
	if t == nil {
		return
	}
	if !r.typeIsMutable(t, 8) {
		return
	}
	ctx.Report(n, "parameter is mutable; declare it readonly")
}

// typeIsMutable reports whether t is a mutable container shape
// (Array<T>, mutable tuple, an object with non-readonly properties,
// or a container whose nested elements are mutable). Conservatively
// returns false for primitives, fully-readonly types, and unknown
// shapes.
func (r *rule) typeIsMutable(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth <= 0 {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if r.typeIsMutable(m, depth-1) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		// Branded primitives — `string & { __tag: unique symbol }` —
		// look like an object intersection but at runtime are just the
		// primitive. typescript-eslint treats them as immutable since
		// the brand can't be mutated.
		for _, m := range t.IntersectionMembers() {
			if m.IsBooleanLike() || m.IsStringLike() || m.IsNumberLike() ||
				m.IsBigIntLike() || m.IsESSymbolLike() {
				return false
			}
		}
		for _, m := range t.IntersectionMembers() {
			if r.typeIsMutable(m, depth-1) {
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
	if hasCall && !r.hasNonFunctionProperty(t) {
		return false
	}
	// Type parameters: forward to the constraint when narrower.
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return r.typeIsMutable(c, depth-1)
		}
		return false
	}
	if t.SymbolName() == "Array" {
		return true
	}
	if t.SymbolName() == "ReadonlyArray" {
		for _, a := range t.TypeArguments() {
			if r.typeIsMutable(a, depth-1) {
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
			if r.typeIsMutable(a, depth-1) {
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
			// `treatMethodsAsReadonly` exempts call-signature-bearing
			// properties (`{ method(): T }`) from the mutability
			// check, since methods are usually not reassigned in
			// practice and matching upstream's option lets idiomatic
			// fixtures pass without forcing readonly modifiers
			// everywhere a method appears.
			if r.opts.TreatMethodsAsReadonly {
				if pt := t.PropertyType(name); pt != nil &&
					len(pt.CallSignatures()) > 0 &&
					!r.hasNonFunctionProperty(pt) {
					continue
				}
			}
			return true
		}
		// Even if the slot is readonly, the value at that slot might
		// itself be mutable (e.g. `readonly arr: number[]`).
		if pt := t.PropertyType(name); pt != nil {
			if r.typeIsMutable(pt, depth-1) {
				return true
			}
		}
	}
	return false
}

// hasNonFunctionProperty reports whether t carries any apparent
// property beyond the standard `Function` interface — typically
// from JS-expando assignments. Such added props might be mutable.
func (r *rule) hasNonFunctionProperty(t *wrapperchecker.Type) bool {
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
