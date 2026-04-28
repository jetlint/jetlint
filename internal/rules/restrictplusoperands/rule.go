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
	AllowAny             bool
	AllowBoolean         bool
	AllowNullish         bool
	AllowNumberAndString bool
	AllowRegExp          bool
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
		if r.opts.AllowNumberAndString && (lk == "string" && rk == "number" || lk == "number" && rk == "string") {
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
	return classify(t, r.opts.AllowRegExp)
}

// classify reduces a type to one of the categories the `+` operator
// treats as safe to combine with itself: "string", "number",
// "bigint", or (with allowRegExp) "regexp".
func classify(t *wrapperchecker.Type, allowRegExp bool) string {
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
	if allowRegExp && t.SymbolName() == "RegExp" {
		return "regexp"
	}
	if t.IsUnion() {
		var seen string
		for _, m := range t.UnionMembers() {
			c := classify(m, allowRegExp)
			if c == "" {
				return ""
			}
			if seen == "" {
				seen = c
				continue
			}
			if seen != c {
				return ""
			}
		}
		return seen
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if c := classify(m, allowRegExp); c != "" {
				return c
			}
		}
		return ""
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return classify(c, allowRegExp)
		}
	}
	return ""
}
