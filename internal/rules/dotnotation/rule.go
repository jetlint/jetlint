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
	ctx.Report(n, "use dot notation: ."+key)
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
