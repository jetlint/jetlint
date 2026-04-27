// Package restricttemplateexpressions implements the restrict-template-expressions
// rule: flag template-literal interpolations of values whose type
// isn't string (or one of the explicitly allowed primitive types).
package restricttemplateexpressions

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "restrict-template-expressions"

// Options is the configurable surface of the rule.
type Options struct {
	AllowNumber  bool
	AllowBoolean bool
	AllowAny     bool
	AllowNullish bool
	AllowRegExp  bool
	AllowNever   bool
	AllowArray   bool
	Allow        []TypeMatcher
}

type TypeMatcher struct {
	From string
	Name string
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options {
	return Options{
		AllowNumber:  true,
		AllowBoolean: true,
		AllowAny:     true,
		AllowNullish: true,
		AllowRegExp:  true,
		AllowNever:   true,
		AllowArray:   false,
	}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("restrict-template-expressions options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowNumber":
			if err := json.Unmarshal(val, &out.AllowNumber); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowBoolean":
			if err := json.Unmarshal(val, &out.AllowBoolean); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowAny":
			if err := json.Unmarshal(val, &out.AllowAny); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowNullish":
			if err := json.Unmarshal(val, &out.AllowNullish); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowRegExp":
			if err := json.Unmarshal(val, &out.AllowRegExp); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowNever":
			if err := json.Unmarshal(val, &out.AllowNever); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowArray":
			if err := json.Unmarshal(val, &out.AllowArray); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allow":
			matchers, err := parseMatchers(val)
			if err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
			out.Allow = matchers
		default:
			return Options{}, fmt.Errorf("restrict-template-expressions has no option %q", key)
		}
	}
	return out, nil
}

func parseMatchers(raw json.RawMessage) ([]TypeMatcher, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("expected an array")
	}
	out := make([]TypeMatcher, 0, len(entries))
	for _, e := range entries {
		var s string
		if err := json.Unmarshal(e, &s); err == nil {
			if s != "" {
				out = append(out, TypeMatcher{Name: s})
			}
			continue
		}
		var obj struct {
			From string `json:"from"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(e, &obj); err != nil {
			return nil, err
		}
		if obj.Name != "" {
			out = append(out, TypeMatcher{From: obj.From, Name: obj.Name})
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTemplateSpan: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	// Skip tagged templates.
	if templateExpr := n.Parent(); templateExpr != nil {
		if outer := templateExpr.Parent(); outer != nil &&
			outer.Kind() == wrapperchecker.KindTaggedTemplateExpression {
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if r.acceptable(t, 0) {
		return
	}
	ctx.Report(expr, "template-literal interpolation of a value whose type is not string")
}

const recursionLimit = 16

func (r *rule) acceptable(t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return true
	}
	if t.IsStringLike() {
		return true
	}
	if t.IsAny() {
		return r.opts.AllowAny
	}
	if t.IsUnknown() {
		return r.opts.AllowAny
	}
	if t.IsNullOrUndefined() {
		return r.opts.AllowNullish
	}
	if t.IsNever() {
		return r.opts.AllowNever
	}
	if t.IsNumberLike() {
		return r.opts.AllowNumber
	}
	if t.IsBigIntLike() {
		return r.opts.AllowNumber
	}
	if t.IsBooleanLike() {
		return r.opts.AllowBoolean
	}
	// allow-list (e.g. RegExp)
	if r.matchesAllow(t) {
		return true
	}
	if t.SymbolName() == "RegExp" {
		return r.opts.AllowRegExp
	}
	if r.opts.AllowArray && (t.IsTupleType() || t.IsArrayLikeType() || t.ArrayElementType() != nil) {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !r.acceptable(m, depth+1) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		// Intersection: any acceptable member qualifies.
		for _, m := range t.IntersectionMembers() {
			if r.acceptable(m, depth+1) {
				return true
			}
		}
		return false
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return r.acceptable(c, depth+1)
	}
	return false
}

func (r *rule) matchesAllow(t *wrapperchecker.Type) bool {
	if len(r.opts.Allow) == 0 {
		return false
	}
	if matchByName(t.SymbolName(), r.opts.Allow) || matchByName(t.AliasSymbolName(), r.opts.Allow) {
		return true
	}
	// Walk inheritance — `class Derived extends Base` should match a
	// `Base` allow entry.
	for _, base := range t.BaseTypeNames() {
		if matchByName(base, r.opts.Allow) {
			return true
		}
	}
	return false
}

func matchByName(name string, matchers []TypeMatcher) bool {
	if name == "" {
		return false
	}
	for _, m := range matchers {
		if m.Name == name {
			return true
		}
	}
	return false
}
