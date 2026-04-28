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
	if callee.PropertyAccessName() != "sort" {
		return
	}
	args := n.CallArguments()
	if len(args) > 0 {
		// `sort(undefined)` is fine; anything else is a compare fn.
		if len(args) == 1 && args[0].Kind() == wrapperchecker.KindIdentifier && args[0].LiteralText() == "undefined" {
			// fallthrough — no compare fn
		} else {
			return
		}
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil {
		return
	}
	rt := ctx.TypeOf(recv)
	if rt == nil {
		return
	}
	elem := arrayElement(rt)
	if elem == nil {
		// Not array-like — could be a custom .sort(); leave alone.
		return
	}
	if r.opts.IgnoreStringArrays && isAllString(elem) {
		return
	}
	ctx.Report(n, "array.sort() on a non-string array uses string sorting; pass an explicit compare function")
}

// arrayElement returns the element type of t when t is array-like,
// or nil. For unions, all members must be array-like.
func arrayElement(t *wrapperchecker.Type) *wrapperchecker.Type {
	if elem := t.ArrayElementType(); elem != nil {
		return elem
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
	return false
}
