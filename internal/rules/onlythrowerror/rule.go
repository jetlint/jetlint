// Package onlythrowerror implements the only-throw-error rule: flag
// `throw X` where X is not an Error (or any/unknown).
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint.
package onlythrowerror

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "only-throw-error"

// Options is the configurable surface of the rule.
type Options struct {
	// AllowThrowingAny: when true (default), `throw anyValue` is fine.
	AllowThrowingAny bool
	// AllowThrowingUnknown: when true (default), `throw unknownValue` is fine.
	AllowThrowingUnknown bool
	// AllowRethrowing: when true (default), re-throwing a caught
	// value (e.g. inside a catch clause) is fine.
	AllowRethrowing bool
	// Allow: explicit type matchers (e.g. `[{from: "lib", name: "undefined"}]`)
	// whose values are accepted as throws.
	Allow []TypeMatcher
}

// TypeMatcher names a type by symbol name (and optionally source).
type TypeMatcher struct {
	From string
	Name string
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options {
	return Options{
		AllowThrowingAny:     true,
		AllowThrowingUnknown: true,
		AllowRethrowing:      true,
	}
}

// OptionsFromJSON parses raw JSON options.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("only-throw-error options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowThrowingAny":
			if err := json.Unmarshal(val, &out.AllowThrowingAny); err != nil {
				return Options{}, fmt.Errorf("only-throw-error option %q: %w", key, err)
			}
		case "allowThrowingUnknown":
			if err := json.Unmarshal(val, &out.AllowThrowingUnknown); err != nil {
				return Options{}, fmt.Errorf("only-throw-error option %q: %w", key, err)
			}
		case "allowRethrowing":
			if err := json.Unmarshal(val, &out.AllowRethrowing); err != nil {
				return Options{}, fmt.Errorf("only-throw-error option %q: %w", key, err)
			}
		case "allow":
			matchers, err := parseMatchers(val)
			if err != nil {
				return Options{}, fmt.Errorf("only-throw-error option %q: %w", key, err)
			}
			out.Allow = matchers
		default:
			return Options{}, fmt.Errorf("only-throw-error has no option %q", key)
		}
	}
	return out, nil
}

func parseMatchers(raw json.RawMessage) ([]TypeMatcher, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("expected an array of matchers")
	}
	out := make([]TypeMatcher, 0, len(entries))
	for _, entry := range entries {
		var s string
		if err := json.Unmarshal(entry, &s); err == nil {
			if s != "" {
				out = append(out, TypeMatcher{Name: s})
			}
			continue
		}
		var obj struct {
			From string `json:"from"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(entry, &obj); err != nil {
			return nil, fmt.Errorf("matcher must be a string or {from, name} object")
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
		wrapperchecker.KindThrowStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if r.isAcceptable(ctx, expr, t, 0) {
		return
	}
	ctx.Report(expr, "throw of a value whose type is not Error or a subclass")
}

const recursionLimit = 16

func (r *rule) isAcceptable(ctx *engine.Context, expr *wrapperchecker.Node, t *wrapperchecker.Type, depth int) bool {
	if t == nil || depth > recursionLimit {
		return true
	}
	// `throw e` where e is an identifier resolving to a catch binding:
	// allow rethrow regardless of the type. Checked first so the
	// allowThrowingAny / allowThrowingUnknown options don't override
	// this case.
	if r.opts.AllowRethrowing && depth == 0 && expr.Kind() == wrapperchecker.KindIdentifier {
		if expr.IdentifierResolvesToCatchBinding(ctx.Checker()) {
			return true
		}
	}
	if t.IsAny() {
		// `throw new <imported>()` is the user explicitly invoking a
		// constructor named after a global. If the import is
		// unresolvable, the constructor's identity is uncertain —
		// upstream flags this regardless of AllowThrowingAny.
		if expr.Kind() == wrapperchecker.KindNewExpression {
			if callee := expr.CalleeExpression(); callee != nil &&
				callee.Kind() == wrapperchecker.KindIdentifier &&
				callee.IsImportedIdentifier(ctx.Checker()) {
				return false
			}
		}
		// `throw <imported>()` — call of an imported function whose
		// type couldn't be resolved. When the user has provided an
		// allow-list (signaling intent to throw a specific package's
		// error type), trust the import.
		if expr.Kind() == wrapperchecker.KindCallExpression && len(r.opts.Allow) > 0 {
			if callee := expr.CalleeExpression(); callee != nil &&
				callee.Kind() == wrapperchecker.KindIdentifier &&
				callee.IsImportedIdentifier(ctx.Checker()) {
				return true
			}
		}
		return r.opts.AllowThrowingAny
	}
	// `undefined` keyword — match by name in the allow-list.
	if t.IsNullOrUndefined() && r.allowMatchesName("undefined") {
		return true
	}
	if t.IsUnknown() {
		return r.opts.AllowThrowingUnknown
	}
	// allow-list
	if r.matchesAllow(t) {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !r.isAcceptable(ctx, expr, m, depth+1) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if r.isAcceptable(ctx, expr, m, depth+1) {
				return true
			}
		}
		return false
	}
	if isErrorName(t.SymbolName()) && t.SymbolIsAmbient() && !t.SymbolIsUserDeclared() {
		return true
	}
	for _, base := range t.BaseTypeNames() {
		if isErrorName(base) {
			return true
		}
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return r.isAcceptable(ctx, expr, c, depth+1)
	}
	return false
}

// matchesAllow checks the OUTER type only — union/intersection
// walking is handled in isAcceptable so each constituent gets its
// own check (a union with Error and a non-Error member must still
// flag the non-Error member).
func (r *rule) matchesAllow(t *wrapperchecker.Type) bool {
	if len(r.opts.Allow) == 0 {
		return false
	}
	if matchByName(t.SymbolName(), r.opts.Allow) {
		return true
	}
	if matchByName(t.AliasSymbolName(), r.opts.Allow) {
		return true
	}
	return false
}

func (r *rule) allowMatchesName(name string) bool {
	for _, m := range r.opts.Allow {
		if m.Name == name {
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

func isErrorName(name string) bool {
	switch name {
	case "Error", "TypeError", "RangeError", "SyntaxError",
		"ReferenceError", "URIError", "EvalError", "AggregateError":
		return true
	}
	return false
}

