// Package noemptycharacterclass implements the
// no-empty-character-class rule. A regular-expression literal that
// contains an empty character class — `[]` — cannot match any
// character, which makes the surrounding pattern impossible to
// satisfy and is almost always a typo.
//
// The check is a small hand-rolled scanner over the pattern body
// (i.e. the text between the leading and trailing `/`). It
// understands backslash escapes so it doesn't false-positive on
// `\[\]` or on a `[` that's escaped inside a character class.
package noemptycharacterclass

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-empty-character-class"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindRegularExpressionLiteral: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	text := n.SourceText()
	pattern := extractPattern(text)
	if pattern == "" {
		return
	}
	if containsEmptyClass(pattern) {
		ctx.Report(n, "Empty class.")
	}
}

// extractPattern returns the body of a regex literal (text between
// the leading and trailing `/`). Returns "" if the text doesn't
// look like a regex literal.
func extractPattern(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	// Find the closing slash, respecting backslash escapes and
	// character classes (`/` inside `[...]` doesn't terminate).
	body := text[1:]
	inClass := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\\' {
			if i+1 < len(body) {
				i++ // skip the escaped char
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

// containsEmptyClass reports whether pattern contains an empty
// character class `[]`. Honors backslash escapes so `\[\]` and `\[]`
// don't trigger.
func containsEmptyClass(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' {
			if i+1 < len(pattern) {
				i++
			}
			continue
		}
		if c == '[' {
			// Empty when the next non-escape char is `]`.
			j := i + 1
			if j < len(pattern) && pattern[j] == ']' {
				return true
			}
		}
	}
	return false
}
