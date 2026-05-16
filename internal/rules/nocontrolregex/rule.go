// Package nocontrolregex implements the no-control-regex rule. A
// regular-expression literal containing an ASCII control character
// (0x00–0x1f, expressed either literally or via `\xNN` / `\uNNNN` /
// `\u{...}` escapes referring to a value < 0x20) is almost always a
// mistake — the more readable `\t`, `\n`, `\r`, etc. escapes are
// available, and literal control characters in source text are
// invisible.
//
// The check also covers `new RegExp("...")` and `RegExp("...")`
// when the first argument is a string literal: identical content
// rules apply.
package nocontrolregex

import (
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-control-regex"

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
	text := n.SourceText()
	pattern := extractRegexPattern(text)
	if pattern == "" {
		return
	}
	if hit := findControlChar(pattern); hit != "" {
		ctx.Report(n, "Unexpected control character(s) in regular expression: "+hit+".")
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
	pattern := raw
	if len(raw) >= 2 {
		// Strip surrounding quotes / backticks when present in
		// the source text.
		first, last := raw[0], raw[len(raw)-1]
		if (first == '"' && last == '"') ||
			(first == '\'' && last == '\'') ||
			(first == '`' && last == '`') {
			pattern = raw[1 : len(raw)-1]
		}
	}
	// The string-literal source text still contains JS string
	// escapes (`\\`, `\n`, `\xNN`, …). The regex engine sees the
	// *interpreted* string, so undo those escapes before scanning.
	pattern = unescapeStringLiteral(pattern)
	if hit := findControlChar(pattern); hit != "" {
		ctx.Report(n, "Unexpected control character(s) in regular expression: "+hit+".")
	}
}

// unescapeStringLiteral converts a JavaScript string-literal body
// (without the surrounding quotes) into the runtime string the
// regex engine actually receives. Only the escapes relevant to
// detecting control characters are translated explicitly: `\\` →
// `\`, `\n`/`\t`/`\r`/`\0`/`\b`/`\f`/`\v` → the literal control
// byte, plus `\xNN` / `\uNNNN` / `\u{NN}` → the named code point.
// Other escapes pass through unchanged (so the downstream
// findControlChar still sees an unaltered `\d`, `\s`, etc.).
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
		case 'n':
			b.WriteByte('\n')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case '0':
			b.WriteByte(0)
			i++
		case 'b':
			b.WriteByte('\b')
			i++
		case 'f':
			b.WriteByte('\f')
			i++
		case 'v':
			b.WriteByte('\v')
			i++
		case 'x':
			if i+3 < len(s) {
				if v, err := strconv.ParseUint(s[i+2:i+4], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 3
					continue
				}
			}
			b.WriteByte(c)
		case 'u':
			if i+2 < len(s) && s[i+2] == '{' {
				end := strings.IndexByte(s[i+2:], '}')
				if end > 0 {
					if v, err := strconv.ParseUint(s[i+3:i+2+end], 16, 32); err == nil {
						b.WriteRune(rune(v))
						i += 2 + end
						continue
					}
				}
			}
			if i+5 < len(s) {
				if v, err := strconv.ParseUint(s[i+2:i+6], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 5
					continue
				}
			}
			b.WriteByte(c)
		default:
			// Unknown escape: preserve the backslash so the regex
			// engine can interpret it (e.g. `\d`, `\s`, `\w`).
			b.WriteByte(c)
		}
	}
	return b.String()
}

// extractRegexPattern returns the body of a regex literal (text
// between the leading and trailing `/`).
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

// findControlChar scans pattern for any literal control character or
// any `\xNN` / `\uNNNN` / `\u{...}` escape resolving to a code
// point in the 0x00–0x1f range. Returns a short description of the
// first hit, or "" when none.
func findControlChar(pattern string) string {
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' {
			if i+1 >= len(pattern) {
				continue
			}
			esc := pattern[i+1]
			switch esc {
			case 'x':
				if i+3 < len(pattern) {
					hex := pattern[i+2 : i+4]
					if v, err := strconv.ParseUint(hex, 16, 32); err == nil && v < 0x20 {
						return `\x` + hex
					}
					i += 3
					continue
				}
			case 'u':
				if i+2 < len(pattern) && pattern[i+2] == '{' {
					end := strings.IndexByte(pattern[i+2:], '}')
					if end > 0 {
						hex := pattern[i+3 : i+2+end]
						if v, err := strconv.ParseUint(hex, 16, 32); err == nil && v < 0x20 {
							return `\u{` + hex + `}`
						}
						i += 2 + end
						continue
					}
				}
				if i+5 < len(pattern) {
					hex := pattern[i+2 : i+6]
					if v, err := strconv.ParseUint(hex, 16, 32); err == nil && v < 0x20 {
						return `\u` + hex
					}
					i += 5
					continue
				}
			}
			// Skip whatever the escape produced.
			i++
			continue
		}
		if c < 0x20 {
			return controlName(c)
		}
	}
	return ""
}

func controlName(c byte) string {
	switch c {
	case '\t':
		return `\t`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	}
	return `\x` + zeroPad(strconv.FormatUint(uint64(c), 16))
}

func zeroPad(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}
