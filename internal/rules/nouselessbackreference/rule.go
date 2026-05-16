// Package nouselessbackreference implements the
// no-useless-backreference rule. A `\N` backreference inside a
// regex pattern is useless when the group it points to does not
// exist (number greater than the highest capturing group, or a
// named reference `\k<name>` with no matching `(?<name>...)`
// group). Such a backreference can never match, which makes the
// surrounding pattern impossible to satisfy.
//
// The scan walks the pattern character-by-character honoring
// escapes and character classes. Group counting includes only
// real `(`/`(?:`/`(?<name>`/`(?=`/`(?!`/`(?<=`/`(?<!`-style
// openers — escaped `\(` does not count.
package nouselessbackreference

import (
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-backreference"

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
	pattern := extractRegexPattern(n.SourceText())
	if pattern == "" {
		return
	}
	groups, names := countGroups(pattern)
	for _, ref := range collectBackrefs(pattern) {
		if ref.name != "" {
			if _, ok := names[ref.name]; !ok {
				ctx.Report(n, "Backreference '\\k<"+ref.name+">' refers to a non-existent group.")
			}
			continue
		}
		if ref.num > groups {
			ctx.Report(n, "Backreference '\\"+strconv.Itoa(ref.num)+"' refers to a non-existent group.")
		}
	}
}

type backref struct {
	num  int
	name string
}

// countGroups returns the number of capturing groups in the pattern
// and the set of named groups.
func countGroups(p string) (int, map[string]bool) {
	names := map[string]bool{}
	count := 0
	inClass := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			if i+1 < len(p) {
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
		if inClass {
			continue
		}
		if c == '(' {
			// Classify the opener.
			if i+1 < len(p) && p[i+1] == '?' {
				// Non-capturing / lookahead / lookbehind /
				// named group introducer.
				if i+2 < len(p) && p[i+2] == '<' {
					// Named: `(?<name>...)` — name ends at `>`.
					end := strings.IndexByte(p[i+3:], '>')
					if end > 0 {
						names[p[i+3:i+3+end]] = true
						count++
						i += 3 + end
						continue
					}
				}
				// Non-capturing or assertion — does not bump count.
				continue
			}
			count++
		}
	}
	return count, names
}

// collectBackrefs returns every backreference (`\N` or `\k<name>`)
// outside of a character class.
func collectBackrefs(p string) []backref {
	var refs []backref
	inClass := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '[' {
			inClass = true
			continue
		}
		if c == ']' {
			inClass = false
			continue
		}
		if c != '\\' || i+1 >= len(p) {
			continue
		}
		if inClass {
			i++
			continue
		}
		esc := p[i+1]
		if esc == 'k' && i+2 < len(p) && p[i+2] == '<' {
			end := strings.IndexByte(p[i+3:], '>')
			if end > 0 {
				refs = append(refs, backref{name: p[i+3 : i+3+end]})
				i += 3 + end
				continue
			}
		}
		if esc >= '1' && esc <= '9' {
			j := i + 1
			for j < len(p) && p[j] >= '0' && p[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(p[i+1 : j])
			refs = append(refs, backref{num: n})
			i = j - 1
			continue
		}
		i++
	}
	return refs
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
