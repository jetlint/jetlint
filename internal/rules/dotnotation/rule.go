// Package dotnotation implements the dot-notation rule: prefer
// `a.b` over `a['b']` when the property name is a valid identifier.
package dotnotation

import (
	"encoding/json"
	"fmt"
	"regexp"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "dot-notation"

// Options is the configurable surface of the rule.
type Options struct {
	AllowKeywords                     bool
	AllowPattern                      string
	AllowPrivateClassPropertyAccess   bool
	AllowProtectedClassPropertyAccess bool
	AllowIndexSignaturePropertyAccess bool
}

func DefaultOptions() Options {
	return Options{
		AllowKeywords: true,
	}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("dot-notation options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowKeywords":
			if err := json.Unmarshal(val, &out.AllowKeywords); err != nil {
				return Options{}, err
			}
		case "allowPattern":
			if err := json.Unmarshal(val, &out.AllowPattern); err != nil {
				return Options{}, err
			}
		case "allowPrivateClassPropertyAccess":
			if err := json.Unmarshal(val, &out.AllowPrivateClassPropertyAccess); err != nil {
				return Options{}, err
			}
		case "allowProtectedClassPropertyAccess":
			if err := json.Unmarshal(val, &out.AllowProtectedClassPropertyAccess); err != nil {
				return Options{}, err
			}
		case "allowIndexSignaturePropertyAccess":
			if err := json.Unmarshal(val, &out.AllowIndexSignaturePropertyAccess); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

func NewWithOptions(opts Options) engine.Rule {
	r := &rule{opts: opts}
	if opts.AllowPattern != "" {
		re, err := regexp.Compile(opts.AllowPattern)
		if err == nil {
			r.allowRe = re
		}
	}
	return r
}

type rule struct {
	opts    Options
	allowRe *regexp.Regexp
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindElementAccessExpression:  r.visit,
		wrapperchecker.KindPropertyAccessExpression: r.visitPropertyAccess,
	}
}

func (r *rule) visitPropertyAccess(ctx *engine.Context, n *wrapperchecker.Node) {
	// `obj.while` is fine when allowKeywords is true (the default).
	// With allowKeywords:false, the property MUST be in brackets.
	if r.opts.AllowKeywords {
		return
	}
	name := n.PropertyAccessName()
	if !isReservedWord(name) {
		return
	}
	ctx.Report(n, "use bracket notation for reserved word: ['"+name+"']")
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	idx := n.ElementAccessIndex()
	if idx == nil {
		return
	}
	key, ok := constantStringIndex(idx)
	if !ok {
		return
	}
	if !isValidIdentifier(key) {
		return
	}
	if r.allowRe != nil && r.allowRe.MatchString(key) {
		return
	}
	if isReservedWord(key) && !r.opts.AllowKeywords {
		// `a['while']` with allowKeywords:false: must stay in brackets.
		return
	}
	if r.shouldAllowByType(ctx, n, key) {
		return
	}
	ctx.Report(n, "use dot notation: ."+key)
}

// shouldAllowByType reports whether the receiver type's shape (private/
// protected member, or index signature) means bracket access is the
// appropriate form.
func (r *rule) shouldAllowByType(ctx *engine.Context, n *wrapperchecker.Node, key string) bool {
	recv := n.ElementAccessReceiver()
	if recv == nil {
		return false
	}
	rt := ctx.TypeOf(recv)
	if rt == nil {
		return false
	}
	if (r.opts.AllowPrivateClassPropertyAccess || r.opts.AllowProtectedClassPropertyAccess) && propertyHasAccessModifier(rt, key, r.opts.AllowPrivateClassPropertyAccess, r.opts.AllowProtectedClassPropertyAccess) {
		return true
	}
	if r.opts.AllowIndexSignaturePropertyAccess && typeHasIndexSignature(rt) && !typeHasOwnProperty(rt, key) {
		return true
	}
	// Pattern-keyed index signatures (`[key: \`prefix_${string}\`]:
	// T`, `[key: Lowercase<string>]: T`) only match at runtime
	// through bracket access — there's no way to phrase
	// `foo.\`prefix_xyz\`` as a property name. Treat the bracket
	// form as the only legal access for any key not declared as a
	// concrete property.
	if typeHasNonPlainStringIndexSignature(rt) && !typeHasOwnProperty(rt, key) {
		return true
	}
	return false
}

// typeHasNonPlainStringIndexSignature reports whether t (or any
// non-nullish union member) has a string-keyed index signature whose
// key type is narrower than plain `string`. Walks type-parameter
// constraints so `T extends Foo` inherits Foo's pattern signature.
func typeHasNonPlainStringIndexSignature(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if typeHasNonPlainStringIndexSignature(m) {
				return true
			}
		}
		return false
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return typeHasNonPlainStringIndexSignature(c)
		}
	}
	return t.HasNonPlainStringIndexSignature()
}

// propertyHasAccessModifier reports whether the property `key` on t
// is declared private (when allowPrivate is set) or protected (when
// allowProtected is set). Walks union members so `T | undefined`
// inherits the access modifiers of T.
func propertyHasAccessModifier(t *wrapperchecker.Type, key string, allowPrivate, allowProtected bool) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if propertyHasAccessModifier(m, key, allowPrivate, allowProtected) {
				return true
			}
		}
		return false
	}
	sym := t.PropertySymbol(key)
	if sym == nil {
		return false
	}
	for _, decl := range sym.Declarations() {
		if allowPrivate && decl.HasPrivateModifier() {
			return true
		}
		if allowProtected && decl.HasProtectedModifier() {
			return true
		}
	}
	return false
}

// typeHasIndexSignature reports whether t (or any union member) has
// any index signature (string-like, number-like, or template-literal-
// typed key).
func typeHasIndexSignature(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if typeHasIndexSignature(m) {
				return true
			}
		}
		return false
	}
	return t.HasIndexSignature("")
}

// typeHasOwnProperty reports whether the property `key` exists as a
// non-index-signature member of t. The index-signature exemption
// shouldn't apply when the key names a real declared property.
func typeHasOwnProperty(t *wrapperchecker.Type, key string) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if typeHasOwnProperty(m, key) {
				return true
			}
		}
		return false
	}
	sym := t.PropertySymbol(key)
	if sym == nil {
		return false
	}
	for _, decl := range sym.Declarations() {
		if decl.Kind() != wrapperchecker.KindIndexSignature {
			return true
		}
	}
	return false
}

// constantStringIndex extracts the property name from a string-like
// or boolean/null literal index, returning (name, true) when the
// index is something we can convert to dot access.
func constantStringIndex(n *wrapperchecker.Node) (string, bool) {
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral:
		return n.LiteralText(), true
	case wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.LiteralText(), true
	case wrapperchecker.KindTrueKeyword:
		return "true", true
	case wrapperchecker.KindFalseKeyword:
		return "false", true
	case wrapperchecker.KindNullKeyword:
		return "null", true
	}
	return "", false
}

// isValidIdentifier reports whether s is a syntactically valid ES
// identifier (start char + continue chars).
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isIdStart(r) {
				return false
			}
			continue
		}
		if !isIdContinue(r) {
			return false
		}
	}
	return true
}

func isIdStart(r rune) bool {
	return r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isIdContinue(r rune) bool {
	return isIdStart(r) || (r >= '0' && r <= '9')
}

func isReservedWord(s string) bool {
	// Strict ES5+ reserved words. Contextual identifiers like
	// `arguments`, `let`, `yield`, `eval` are intentionally excluded —
	// they're valid property names without bracket access.
	switch s {
	case "break", "case", "catch", "class", "const", "continue",
		"debugger", "default", "delete", "do", "else", "enum", "export",
		"extends", "false", "finally", "for", "function", "if", "import",
		"in", "instanceof", "new", "null", "return", "super", "switch",
		"this", "throw", "true", "try", "typeof", "var", "void", "while",
		"with":
		return true
	}
	return false
}
