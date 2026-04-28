// Package restrictplusoperands implements the restrict-plus-operands
// rule: flag `a + b` where the operands aren't both string-like or
// both numeric.
package restrictplusoperands

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "restrict-plus-operands"

// Options is the configurable surface of the rule.
type Options struct {
	AllowAny                bool
	AllowBoolean            bool
	AllowNullish            bool
	AllowNumberAndString    bool
	AllowRegExp             bool
	SkipCompoundAssignments bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("restrict-plus-operands options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowAny":
			if err := json.Unmarshal(val, &out.AllowAny); err != nil {
				return Options{}, err
			}
		case "allowBoolean":
			if err := json.Unmarshal(val, &out.AllowBoolean); err != nil {
				return Options{}, err
			}
		case "allowNullish":
			if err := json.Unmarshal(val, &out.AllowNullish); err != nil {
				return Options{}, err
			}
		case "allowNumberAndString":
			if err := json.Unmarshal(val, &out.AllowNumberAndString); err != nil {
				return Options{}, err
			}
		case "allowRegExp":
			if err := json.Unmarshal(val, &out.AllowRegExp); err != nil {
				return Options{}, err
			}
		case "skipCompoundAssignments":
			if err := json.Unmarshal(val, &out.SkipCompoundAssignments); err != nil {
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
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	if op != wrapperchecker.KindPlusToken && op != wrapperchecker.KindPlusEqualsToken {
		return
	}
	if op == wrapperchecker.KindPlusEqualsToken && r.opts.SkipCompoundAssignments {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	lt := ctx.TypeOf(left)
	rt := ctx.TypeOf(right)
	if lt == nil || rt == nil {
		return
	}
	lk := r.classify(lt)
	rk := r.classify(rt)
	if lk == "" {
		ctx.Report(left, "operand of `+` has a type that doesn't safely compose under string concatenation or numeric addition")
	}
	if rk == "" {
		ctx.Report(right, "operand of `+` has a type that doesn't safely compose under string concatenation or numeric addition")
	}
	if lk != "" && rk != "" && lk != rk {
		// `any` is compatible with anything when allowAny is set.
		if lk == "any" || rk == "any" {
			return
		}
		// Nullish counts as compatible with the other side when allowNullish.
		if lk == "nullish" || rk == "nullish" {
			return
		}
		if r.opts.AllowNumberAndString && isStringableKind(lk) && isStringableKind(rk) {
			return
		}
		ctx.Report(n, "operands of `+` are different kinds: "+lk+" + "+rk)
	}
}

func (r *rule) classify(t *wrapperchecker.Type) string {
	if t.IsAny() {
		if r.opts.AllowAny {
			return "any"
		}
		return ""
	}
	if t.IsUnknown() {
		return ""
	}
	if t.IsNullOrUndefined() && r.opts.AllowNullish {
		return "nullish"
	}
	if t.IsBooleanLike() {
		if r.opts.AllowBoolean {
			return "boolean"
		}
		return ""
	}
	return classify(t, r.opts)
}

// isStringableKind reports whether a kind can be coerced into a
// string for `+` (relevant under allowNumberAndString).
func isStringableKind(k string) bool {
	return k == "string" || k == "number" || k == "bigint" || k == "stringable"
}

// classify reduces a type to one of the categories the `+` operator
// treats as safe to combine with itself: "string", "number",
// "bigint", or (with allowRegExp) "regexp". With allowRegExp the
// regex member is treated as string-coercible — typescript-eslint
// permits `regex + string`.
func classify(t *wrapperchecker.Type, opts Options) string {
	if t.IsAny() || t.IsUnknown() {
		return ""
	}
	if t.IsStringLike() {
		return "string"
	}
	if t.IsNumberLike() {
		return "number"
	}
	if t.IsBigIntLike() {
		return "bigint"
	}
	if opts.AllowRegExp && t.SymbolName() == "RegExp" {
		return "string"
	}
	if t.IsNullOrUndefined() && opts.AllowNullish {
		return "nullish"
	}
	if t.IsUnion() {
		var seen string
		mixed := false
		for _, m := range t.UnionMembers() {
			c := classify(m, opts)
			if c == "" {
				return ""
			}
			// `nullish` is treated as compatible with whatever else is
			// in the union — `string | undefined` is still string-like.
			if c == "nullish" {
				continue
			}
			if seen == "" || seen == "nullish" {
				seen = c
				continue
			}
			if seen != c {
				if opts.AllowNumberAndString && isStringableKind(seen) && isStringableKind(c) {
					mixed = true
					continue
				}
				return ""
			}
		}
		if seen == "" {
			seen = "nullish"
		}
		if mixed {
			return "stringable"
		}
		return seen
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if c := classify(m, opts); c != "" {
				return c
			}
		}
		return ""
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return classify(c, opts)
		}
	}
	return ""
}
