// Package requirearraysortcompare implements the
// require-array-sort-compare rule: flag `arr.sort()` on a non-string
// array — JS's default sort is string-based, which surprises users
// of number arrays.
package requirearraysortcompare

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "require-array-sort-compare"

// Options is the configurable surface of the rule.
type Options struct {
	IgnoreStringArrays bool
}

func DefaultOptions() Options { return Options{IgnoreStringArrays: true} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("require-array-sort-compare options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "ignoreStringArrays":
			if err := json.Unmarshal(val, &out.IgnoreStringArrays); err != nil {
				return Options{}, err
			}
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
		wrapperchecker.KindCallExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	switch callee.PropertyAccessName() {
	case "sort", "toSorted":
	default:
		return
	}
	if len(n.CallArguments()) > 0 {
		// Any argument — even `undefined` — is treated as the user
		// having opted in. The rule fires only on bare `arr.sort()`.
		return
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil {
		return
	}
	if !shouldFlag(rt, r.opts.IgnoreStringArrays) {
		return
	}
	ctx.Report(n, "array.sort() on a non-string array uses string sorting; pass an explicit compare function")
}

// shouldFlag reports whether `arr.sort()` on an array of type t should
// be flagged. `t` itself isn't required to be a single array — for
// unions like `number[] | string[]` every member must be string-only
// for the call to be safe.
func shouldFlag(t *wrapperchecker.Type, ignoreStringArrays bool) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		// All members must be array-like for the call to type-check; if
		// every array member is string-only, leave alone — otherwise
		// flag.
		anyArray := false
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			elem := arrayElement(m)
			if elem == nil {
				return false
			}
			anyArray = true
			if !ignoreStringArrays || !isAllString(elem) {
				return true
			}
		}
		return false && anyArray // non-array members already returned false
	}
	elem := arrayElement(t)
	if elem == nil {
		return false
	}
	if ignoreStringArrays && isAllString(elem) {
		return false
	}
	return true
}

// arrayElement returns the element type of t when t is array-like,
// or nil. For unions, all members must be array-like; the returned
// element type is the first one encountered (the caller decides what
// to do with mixed element kinds — for `number[] | string[]` we want
// to flag because not every element is string).
func arrayElement(t *wrapperchecker.Type) *wrapperchecker.Type {
	if elem := t.ArrayElementType(); elem != nil {
		return elem
	}
	if t.IsTupleType() {
		if args := t.TypeArguments(); len(args) > 0 {
			return args[0]
		}
	}
	if t.IsUnion() {
		var combined *wrapperchecker.Type
		for _, m := range t.UnionMembers() {
			if elem := arrayElement(m); elem != nil {
				if combined == nil {
					combined = elem
				}
				continue
			}
			return nil
		}
		return combined
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return arrayElement(c)
		}
	}
	return nil
}

func isAllString(t *wrapperchecker.Type) bool {
	if t.IsStringLike() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAllString(m) {
				return false
			}
		}
		return true
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isAllString(c)
		}
	}
	return false
}
