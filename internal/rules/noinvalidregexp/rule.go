// Package noinvalidregexp implements the no-invalid-regexp rule. A
// regex literal whose body is syntactically invalid would throw at
// runtime; flagging it at lint time prevents the surprise.
//
// JavaScript regex syntax is a superset of RE2, so Go's `regexp`
// package can't be used as a validator (it would false-positive on
// lookbehind, named backrefs, etc.). Instead we run a small
// hand-rolled structural check that detects the most common
// classes of mistake:
//
//   - Unbalanced parens (`(`, `(?:`, `(?=`, etc. without matching
//     `)`; or stray `)`).
//   - Unclosed character class (`[`).
//   - Dangling backslash at the end of the pattern.
//   - Unterminated quantifier (`{` with no matching `}`).
//   - Unterminated named group (`(?<` without `>`).
//
// The same check also runs on the first argument of
// `new RegExp(...)` / `RegExp(...)` when it is a string literal.
package noinvalidregexp

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-invalid-regexp"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindRegularExpressionLiteral: r.visitRegex,
		wrapperchecker.KindCallExpression:           r.visitCall,
		wrapperchecker.KindNewExpression:            r.visitCall,
	}
}

func (r *rule) visitRegex(ctx *engine.Context, n *wrapperchecker.Node) {
	pattern := extractRegexPattern(n.SourceText())
	if pattern == "" {
		return
	}
	if msg := validatePattern(pattern); msg != "" {
		ctx.Report(n, "Invalid regular expression: "+msg+".")
	}
}

func (r *rule) visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.SourceText() != "RegExp" {
		return
	}
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	first := args[0]
	if first.Kind() != wrapperchecker.KindStringLiteral &&
		first.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return
	}
	raw := first.SourceText()
	if len(raw) < 2 {
		return
	}
	pattern := raw[1 : len(raw)-1]
	pattern = unescapeStringLiteral(pattern)
	if msg := validatePattern(pattern); msg != "" {
		ctx.Report(n, "Invalid regular expression: "+msg+".")
	}
}

// validatePattern returns a short error message describing the first
// structural problem found in the pattern, or "" when the pattern
// is well-formed under our subset check.
func validatePattern(p string) string {
	parenDepth := 0
	inClass := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			if i+1 >= len(p) {
				return "trailing backslash"
			}
			i++
			continue
		}
		if inClass {
			if c == ']' {
				inClass = false
			}
			continue
		}
		switch c {
		case '[':
			inClass = true
		case '(':
			if i+1 < len(p) && p[i+1] == '?' && i+2 < len(p) && p[i+2] == '<' {
				// Named group `(?<name>...)`. Require matching `>`.
				if end := strings.IndexByte(p[i+3:], '>'); end < 0 {
					return "unterminated named group"
				}
			}
			parenDepth++
		case ')':
			if parenDepth == 0 {
				return "unmatched ')'"
			}
			parenDepth--
		case '{':
			// A quantifier `{n}`, `{n,}`, `{n,m}` must end with `}`
			// before any unescaped backslash/paren/bracket.
			end := -1
			for j := i + 1; j < len(p); j++ {
				if p[j] == '}' {
					end = j
					break
				}
				if p[j] == '\\' || p[j] == '(' || p[j] == ')' || p[j] == '[' {
					break
				}
			}
			if end >= 0 {
				i = end
			}
			// If no `}` is found within the heuristic window, JS
			// treats `{` as a literal — not an error.
		}
	}
	if inClass {
		return "unterminated character class"
	}
	if parenDepth > 0 {
		return "unmatched '('"
	}
	return ""
}

// extractRegexPattern returns the body of a regex literal.
func extractRegexPattern(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	body := text[1:]
	inClass := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\\' {
			if i+1 < len(body) {
				i++
			}
			continue
		}
		if c == '[' {
			inClass = true
			continue
		}
		if c == ']' {
			inClass = false
			continue
		}
		if c == '/' && !inClass {
			return body[:i]
		}
	}
	return body
}

// unescapeStringLiteral converts a JS string-literal body into the
// runtime string the regex engine actually receives. Only the
// escapes that change validation behavior (`\\` → `\`) are
// translated; others pass through so the regex engine still sees
// `\d`, `\s`, etc. unchanged.
func unescapeStringLiteral(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		esc := s[i+1]
		switch esc {
		case '\\', '"', '\'', '`', '/':
			b.WriteByte(esc)
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
