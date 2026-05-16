// Package main implements a lexical extractor for oxlint rule test
// fixtures. Each oxc rule file ends with a `fn test()` containing
// `let pass = vec![...]` and `let fail = vec![...]` blocks; each
// entry is either a plain Rust string literal or a tuple whose first
// element is a string literal (followed by options). The extractor
// pulls out each entry's code and writes it to a per-rule JSON file
// that the jetlint eslintcompat harness then loads.
//
// The extractor is deliberately lexical (token-aware but not
// AST-aware). The shape of oxc's test functions is regular enough
// that we never need full Rust parsing; we just need to skip
// comments and strings while we track bracket depth.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// tokKind labels each token of interest. We collapse anything that
// isn't a bracket, comma, string, or comment into tokOther — the
// extractor doesn't need to understand operators, keywords, or
// identifiers.
type tokKind int

const (
	tokOther         tokKind = iota
	tokString                // `"..."` or `r"..."` / `r#"..."#`
	tokOpenParen             // `(`
	tokCloseParen            // `)`
	tokOpenBracket           // `[`
	tokCloseBracket          // `]`
	tokOpenBrace             // `{`
	tokCloseBrace            // `}`
	tokComma                 // `,`
)

type tok struct {
	kind tokKind
	// For strings, value is the decoded contents (escapes processed
	// for normal strings, byte-for-byte for raw strings).
	value string
	pos   int // byte position in source, for error messages
}

// tokenize walks src and produces a token stream that skips
// whitespace and comments. It recognizes string literals (both
// regular and raw forms) so that punctuation inside strings does not
// affect bracket-depth tracking downstream.
func tokenize(src string) ([]tok, error) {
	var toks []tok
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '/' && i+1 < n && src[i+1] == '/':
			// Line comment runs to newline.
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			// Block comment. Rust block comments nest; track depth.
			i += 2
			depth := 1
			for i < n && depth > 0 {
				if i+1 < n && src[i] == '/' && src[i+1] == '*' {
					depth++
					i += 2
				} else if i+1 < n && src[i] == '*' && src[i+1] == '/' {
					depth--
					i += 2
				} else {
					i++
				}
			}
		case c == '"':
			s, next, err := readRegularString(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, tok{kind: tokString, value: s, pos: i})
			i = next
		case c == 'r' && i+1 < n && (src[i+1] == '"' || src[i+1] == '#'):
			s, next, err := readRawString(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, tok{kind: tokString, value: s, pos: i})
			i = next
		case c == 'b' && i+1 < n && (src[i+1] == '"' || (src[i+1] == 'r' && i+2 < n && (src[i+2] == '"' || src[i+2] == '#'))):
			// Byte strings — uncommon in test fixtures, but skip them
			// gracefully if encountered. We treat them as opaque.
			j := i
			if src[i+1] == 'r' {
				j++
			}
			j++ // past `b` (and possibly `r`)
			next := j
			if src[j] == '"' {
				_, end, err := readRegularString(src, j)
				if err != nil {
					return nil, err
				}
				next = end
			} else {
				_, end, err := readRawString(src, j)
				if err != nil {
					return nil, err
				}
				next = end
			}
			toks = append(toks, tok{kind: tokOther, value: src[i:next], pos: i})
			i = next
		case c == '\'':
			// Char literal or lifetime. Rust lifetimes look like `'a`,
			// `'static`. Char literals are `'x'`, `'\n'`, `'\u{1234}'`.
			// Both end before a comma/bracket so we don't need
			// precise distinction; just consume.
			i++
			// Crude scan: read until matching `'` or whitespace/punct.
			start := i
			for i < n && src[i] != '\'' && !isPunct(src[i]) && !unicode.IsSpace(rune(src[i])) {
				if src[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				i++
			}
			if i < n && src[i] == '\'' {
				i++
			}
			toks = append(toks, tok{kind: tokOther, value: src[start:i], pos: start})
		case c == '(':
			toks = append(toks, tok{kind: tokOpenParen, pos: i})
			i++
		case c == ')':
			toks = append(toks, tok{kind: tokCloseParen, pos: i})
			i++
		case c == '[':
			toks = append(toks, tok{kind: tokOpenBracket, pos: i})
			i++
		case c == ']':
			toks = append(toks, tok{kind: tokCloseBracket, pos: i})
			i++
		case c == '{':
			toks = append(toks, tok{kind: tokOpenBrace, pos: i})
			i++
		case c == '}':
			toks = append(toks, tok{kind: tokCloseBrace, pos: i})
			i++
		case c == ',':
			toks = append(toks, tok{kind: tokComma, pos: i})
			i++
		default:
			// Identifier, number, operator, etc. — collapse into
			// tokOther so the caller can still pattern-match on a
			// keyword sequence like `let pass = vec ! [`.
			start := i
			for i < n && !isStructural(src[i]) {
				i++
			}
			if i == start {
				// Unrecognized single char; advance so we don't loop.
				i++
			}
			toks = append(toks, tok{kind: tokOther, value: src[start:i], pos: start})
		}
	}
	return toks, nil
}

func isPunct(c byte) bool {
	switch c {
	case '(', ')', '[', ']', '{', '}', ',', ';', ':', '=', '+', '-', '*', '/', '!', '?', '<', '>', '&', '|':
		return true
	}
	return false
}

func isStructural(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '(', ')', '[', ']', '{', '}', ',', '"', '\'', '/',
		'!', '=', ':', ';', '<', '>':
		return true
	}
	return false
}

// readRegularString reads a `"..."` literal starting at src[i] (which
// must be `"`) and returns the decoded value plus the index just past
// the closing quote.
func readRegularString(src string, i int) (string, int, error) {
	if src[i] != '"' {
		return "", i, fmt.Errorf("readRegularString called on %q at %d", src[i], i)
	}
	var sb strings.Builder
	i++
	for i < len(src) {
		c := src[i]
		if c == '"' {
			return sb.String(), i + 1, nil
		}
		if c == '\\' && i+1 < len(src) {
			esc := src[i+1]
			switch esc {
			case 'n':
				sb.WriteByte('\n')
				i += 2
			case 't':
				sb.WriteByte('\t')
				i += 2
			case 'r':
				sb.WriteByte('\r')
				i += 2
			case '\\':
				sb.WriteByte('\\')
				i += 2
			case '"':
				sb.WriteByte('"')
				i += 2
			case '\'':
				sb.WriteByte('\'')
				i += 2
			case '0':
				sb.WriteByte(0)
				i += 2
			case 'u':
				// `\u{XXXX}` form.
				if i+2 < len(src) && src[i+2] == '{' {
					end := i + 3
					for end < len(src) && src[end] != '}' {
						end++
					}
					if end >= len(src) {
						return "", i, fmt.Errorf("unterminated \\u{...} escape at %d", i)
					}
					hex := src[i+3 : end]
					var r rune
					for _, h := range hex {
						r = r*16 + hexVal(byte(h))
					}
					sb.WriteRune(r)
					i = end + 1
				} else {
					sb.WriteByte('u')
					i += 2
				}
			case 'x':
				// `\xNN` byte escape.
				if i+3 < len(src) {
					r := hexVal(src[i+2])*16 + hexVal(src[i+3])
					sb.WriteRune(r)
					i += 4
				} else {
					sb.WriteByte(esc)
					i += 2
				}
			default:
				sb.WriteByte(esc)
				i += 2
			}
			continue
		}
		sb.WriteByte(c)
		i++
	}
	return "", i, fmt.Errorf("unterminated string literal")
}

func hexVal(b byte) rune {
	switch {
	case b >= '0' && b <= '9':
		return rune(b - '0')
	case b >= 'a' && b <= 'f':
		return rune(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return rune(b-'A') + 10
	}
	return 0
}

// readRawString reads a raw string literal starting at src[i] (which
// must be `r`). Returns the literal contents (no escape processing)
// and the index just past the closing delimiter.
func readRawString(src string, i int) (string, int, error) {
	if src[i] != 'r' {
		return "", i, fmt.Errorf("readRawString called on %q at %d", src[i], i)
	}
	j := i + 1
	hashes := 0
	for j < len(src) && src[j] == '#' {
		hashes++
		j++
	}
	if j >= len(src) || src[j] != '"' {
		return "", i, fmt.Errorf("malformed raw string at %d", i)
	}
	j++ // past opening `"`
	start := j
	for j < len(src) {
		if src[j] == '"' {
			// Need `hashes` `#`s after the quote.
			ok := true
			for k := 1; k <= hashes; k++ {
				if j+k >= len(src) || src[j+k] != '#' {
					ok = false
					break
				}
			}
			if ok {
				return src[start:j], j + 1 + hashes, nil
			}
		}
		j++
	}
	return "", i, fmt.Errorf("unterminated raw string at %d", i)
}

// findVec locates the contents of `let <name> = vec ! [ ... ]` in
// toks, returning the slice of tokens between the opening `[` and the
// matching `]`. Returns nil, nil when the named vec isn't present
// (some rules have only `pass` or only `fail`).
func findVec(toks []tok, name string) []tok {
	// Pattern: tokOther("let") ... tokOther(name) ... tokOther("=") ...
	// tokOther("vec") ... tokOther("!") ... tokOpenBracket ... tokCloseBracket
	for i := 0; i+5 < len(toks); i++ {
		if toks[i].kind == tokOther && toks[i].value == "let" {
			// Look for `name` next, allowing one or more `mut`/space tokens.
			j := i + 1
			// Skip optional `mut`
			if j < len(toks) && toks[j].kind == tokOther && toks[j].value == "mut" {
				j++
			}
			if j >= len(toks) || toks[j].kind != tokOther || toks[j].value != name {
				continue
			}
			j++
			// Expect `:` type? Skip optional type annotation up to `=`.
			for j < len(toks) && !(toks[j].kind == tokOther && toks[j].value == "=") {
				j++
			}
			if j >= len(toks) {
				continue
			}
			j++ // past `=`
			// Expect `vec`
			if j >= len(toks) || toks[j].kind != tokOther || toks[j].value != "vec" {
				continue
			}
			j++
			// Expect `!`
			if j >= len(toks) || toks[j].kind != tokOther || toks[j].value != "!" {
				continue
			}
			j++
			if j >= len(toks) || toks[j].kind != tokOpenBracket {
				continue
			}
			open := j
			// Find matching `]` with bracket-depth tracking.
			depth := 1
			for k := open + 1; k < len(toks); k++ {
				switch toks[k].kind {
				case tokOpenBracket:
					depth++
				case tokCloseBracket:
					depth--
					if depth == 0 {
						return toks[open+1 : k]
					}
				}
			}
			return nil
		}
	}
	return nil
}

// splitEntries divides a vec body's tokens into top-level entries by
// top-level commas. A "top-level" comma is one where the depth of
// parens, braces, and inner brackets is zero.
func splitEntries(body []tok) [][]tok {
	var entries [][]tok
	var cur []tok
	depth := 0
	for _, t := range body {
		switch t.kind {
		case tokOpenParen, tokOpenBracket, tokOpenBrace:
			depth++
			cur = append(cur, t)
		case tokCloseParen, tokCloseBracket, tokCloseBrace:
			depth--
			cur = append(cur, t)
		case tokComma:
			if depth == 0 {
				if hasContent(cur) {
					entries = append(entries, cur)
				}
				cur = nil
				continue
			}
			cur = append(cur, t)
		default:
			cur = append(cur, t)
		}
	}
	if hasContent(cur) {
		entries = append(entries, cur)
	}
	return entries
}

func hasContent(toks []tok) bool {
	for _, t := range toks {
		if t.kind != tokOther || strings.TrimSpace(t.value) != "" {
			return true
		}
	}
	return false
}

// firstString returns the first string-literal token's value in toks,
// or "", false if none. For a tuple entry like `("code", options)`,
// this naturally surfaces the code.
func firstString(toks []tok) (string, bool) {
	for _, t := range toks {
		if t.kind == tokString {
			return t.value, true
		}
	}
	return "", false
}

// extractOptions returns the parsed options array for a tuple entry,
// or nil when the entry has no options (a bare string or
// `("code", None)`). oxc fixtures wrap the array as
// `Some(serde_json::json!([...]))`; we slice the original source
// between the `[` and matching `]` after `json!(` and parse it as
// JSON, which works because the macro body is JSON-shaped.
func extractOptions(src string, entry []tok) ([]any, error) {
	after := tokensAfterCommaAtDepth(entry, 1)
	val, err := findJSONMacroValue(src, after, tokOpenBracket, tokCloseBracket)
	if err != nil || val == "" {
		return nil, err
	}
	cleaned := stripTrailingCommas(val)
	var opts []any
	if err := json.Unmarshal([]byte(cleaned), &opts); err != nil {
		return nil, fmt.Errorf("decode options %q: %w", cleaned, err)
	}
	return opts, nil
}

// extractSettings returns the parsed settings object for a 3-tuple
// entry of the form `(code, options, settings)`. oxc uses this slot to
// supply ESLint-style `globals` / `env` configuration via
// `Some(serde_json::json!({...}))`. Returns nil if the entry has no
// settings.
func extractSettings(src string, entry []tok) (map[string]any, error) {
	// Skip past the first comma (after code) and second comma (after
	// options) to reach the settings position.
	after := tokensAfterCommaAtDepth(entry, 2)
	val, err := findJSONMacroValue(src, after, tokOpenBrace, tokCloseBrace)
	if err != nil || val == "" {
		return nil, err
	}
	cleaned := stripTrailingCommas(val)
	var settings map[string]any
	if err := json.Unmarshal([]byte(cleaned), &settings); err != nil {
		return nil, fmt.Errorf("decode settings %q: %w", cleaned, err)
	}
	return settings, nil
}

// tokensAfterCommaAtDepth returns the tokens that appear after the
// n-th top-level comma (depth 1) in `entry`. Used to slice into the
// trailing positional arguments of a Rust tuple.
func tokensAfterCommaAtDepth(entry []tok, n int) []tok {
	depth := 0
	commasSeen := 0
	var out []tok
	for _, t := range entry {
		if commasSeen < n {
			switch t.kind {
			case tokOpenParen, tokOpenBracket, tokOpenBrace:
				depth++
			case tokCloseParen, tokCloseBracket, tokCloseBrace:
				depth--
			case tokComma:
				if depth == 1 {
					commasSeen++
					if commasSeen == n {
						continue
					}
				}
			}
			continue
		}
		out = append(out, t)
	}
	return out
}

// findJSONMacroValue scans `toks` for the first `json!(<open>...<close>)`
// invocation, returning the source slice of the macro argument.
func findJSONMacroValue(src string, toks []tok, openKind, closeKind tokKind) (string, error) {
	for i := 0; i+3 < len(toks); i++ {
		if toks[i].kind != tokOther || toks[i].value != "json" {
			continue
		}
		if toks[i+1].kind != tokOther || toks[i+1].value != "!" {
			continue
		}
		if toks[i+2].kind != tokOpenParen {
			continue
		}
		if toks[i+3].kind != openKind {
			continue
		}
		open := toks[i+3]
		d := 1
		for j := i + 4; j < len(toks); j++ {
			switch toks[j].kind {
			case openKind:
				d++
			case closeKind:
				d--
				if d == 0 {
					return src[open.pos : toks[j].pos+1], nil
				}
			}
		}
	}
	return "", nil
}

// stripTrailingCommas removes commas that appear immediately before
// a closing `]` or `}` (with any whitespace in between). Rust's
// serde_json::json! macro accepts these; Go's encoding/json does not.
// Strings inside the JSON aren't touched because we track quote state.
func stripTrailingCommas(s string) string {
	out := make([]byte, 0, len(s))
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' && (i == 0 || s[i-1] != '\\') {
			inStr = !inStr
			out = append(out, c)
			continue
		}
		if !inStr && c == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && (s[j] == ']' || s[j] == '}') {
				continue
			}
		}
		out = append(out, c)
	}
	return string(out)
}

// ExtractedCase is one fixture entry — code plus the parsed options
// array (nil when the upstream case had no options or used `None`)
// and optional ESLint-style settings (globals / env) captured from
// oxc's third tuple position.
type ExtractedCase struct {
	Code     string
	Options  []any
	Settings map[string]any
}

// Extract reads a single oxc rule source file and returns its pass
// and fail cases. Names map to the variable names in the `fn test()`
// body: `pass` for valid cases, `fail` for invalid.
func Extract(src string) (pass, fail []ExtractedCase, err error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range []string{"pass", "fail"} {
		body := findVec(toks, name)
		if body == nil {
			continue
		}
		entries := splitEntries(body)
		out := make([]ExtractedCase, 0, len(entries))
		for _, e := range entries {
			code, ok := firstString(e)
			if !ok {
				continue
			}
			opts, optErr := extractOptions(src, e)
			if optErr != nil {
				return nil, nil, fmt.Errorf("%s entry: %w", name, optErr)
			}
			settings, setErr := extractSettings(src, e)
			if setErr != nil {
				return nil, nil, fmt.Errorf("%s entry settings: %w", name, setErr)
			}
			out = append(out, ExtractedCase{
				Code:     code,
				Options:  opts,
				Settings: settings,
			})
		}
		if name == "pass" {
			pass = out
		} else {
			fail = out
		}
	}
	return pass, fail, nil
}
