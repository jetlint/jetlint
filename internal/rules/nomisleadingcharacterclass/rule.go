// Package nomisleadingcharacterclass implements the
// no-misleading-character-class rule. A character class that
// contains a character requiring more than one UTF-16 code unit
// (i.e. an astral-plane code point that JS encodes as a surrogate
// pair) silently splits into its two halves unless the `u` flag is
// used. The resulting class matches each surrogate half
// individually rather than the intended grapheme, which almost
// always indicates a bug.
//
// The check focuses on the case the rule was designed to catch:
// the regex has no `u` flag AND a character class contains either a
// literal code point ≥ U+10000 OR a `\uNNNN` escape pair forming a
// surrogate pair. Grapheme cluster checks (combining marks, ZWJ,
// regional indicators, etc.) are out of scope for this conservative
// port — they require Unicode data we don't currently bundle.
package nomisleadingcharacterclass

import (
	"strconv"
	"strings"
	"unicode/utf8"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-misleading-character-class"

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
	pattern, flags := extractRegexParts(text)
	if pattern == "" {
		return
	}
	if strings.Contains(flags, "u") || strings.Contains(flags, "v") {
		return // `u`/`v` flag handles astral code points correctly.
	}
	if hasMisleadingClass(pattern) {
		ctx.Report(n, "Unexpected character in character class.")
	}
}

// extractRegexParts splits a regex literal's source text into the
// body and the trailing flag characters.
func extractRegexParts(text string) (string, string) {
	if !strings.HasPrefix(text, "/") {
		return "", ""
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
			return body[:i], body[i+1:]
		}
	}
	return body, ""
}

// hasMisleadingClass reports whether any character class in pattern
// contains an astral-plane code point (either as a literal or as a
// surrogate-pair `\uXXXX\uXXXX`).
func hasMisleadingClass(p string) bool {
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			if i+1 < len(p) {
				i++
			}
			continue
		}
		if c != '[' {
			continue
		}
		// Scan to matching `]`, honoring escapes.
		j := i + 1
		for j < len(p) {
			if p[j] == '\\' {
				if j+5 < len(p) && p[j+1] == 'u' {
					hi, ok := parseHex4(p[j+2 : j+6])
					if ok && hi >= 0xD800 && hi <= 0xDBFF {
						// High surrogate. Check for low surrogate
						// immediately after.
						if j+11 < len(p) && p[j+6] == '\\' && p[j+7] == 'u' {
							lo, ok2 := parseHex4(p[j+8 : j+12])
							if ok2 && lo >= 0xDC00 && lo <= 0xDFFF {
								return true
							}
						}
					}
					if ok && hi >= 0x10000 {
						return true
					}
					j += 6
					continue
				}
				if j+1 < len(p) && p[j+1] == 'u' && j+2 < len(p) && p[j+2] == '{' {
					end := strings.IndexByte(p[j+2:], '}')
					if end > 0 {
						if v, err := strconv.ParseUint(p[j+3:j+2+end], 16, 32); err == nil && v >= 0x10000 {
							return true
						}
						j += 2 + end + 1
						continue
					}
				}
				if j+1 < len(p) {
					j += 2
				} else {
					j++
				}
				continue
			}
			if p[j] == ']' {
				break
			}
			// Decode a UTF-8 rune; if it requires > 2 UTF-16 code
			// units (i.e. code point ≥ 0x10000), it's misleading.
			r, size := utf8.DecodeRuneInString(p[j:])
			if r >= 0x10000 {
				return true
			}
			j += size
		}
		i = j
	}
	return false
}

func parseHex4(s string) (uint32, bool) {
	if len(s) != 4 {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}
