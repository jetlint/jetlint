// Package nouselessbackreference implements the eslint
// `no-useless-backreference` rule and biome's stricter
// `no-useless-regex-backrefs` variant. A backreference inside a regex
// pattern is useless when:
//   - the referenced numbered group does not exist (number greater
//     than the highest capturing group), or
//   - the named reference `\k<name>` has no matching `(?<name>...)`
//     group, or
//   - the backreference sits inside the body of the very group it
//     refers to (a circular self-reference can never match — the
//     group has not finished matching yet).
//
// Such backreferences make the surrounding pattern impossible to
// satisfy. The walker honours escapes and character classes and
// classifies `(?:`, `(?=`, `(?!`, `(?<=`, `(?<!`, and `(?<name>`
// openers separately so only capturing groups bump the group count
// or appear on the open-group stack used for circularity checks.
package nouselessbackreference

import (
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

// IDEslint is the rule id used by eslint's `no-useless-backreference`.
const IDEslint = "no-useless-backreference"

// IDBiome is the rule id used by biome's `no-useless-regex-backrefs`.
const IDBiome = "no-useless-regex-backrefs"

// New constructs the rule under the eslint id. The eslint variant
// additionally flags backrefs to a group that does not exist
// (number greater than the highest capturing group, or a
// `\k<name>` whose name is not declared anywhere in the pattern).
func New() engine.Rule { return &rule{id: IDEslint, flagMissingGroup: true} }

// NewBiome constructs the rule under the biome id. The biome
// variant only flags circular self-references, because per the
// ECMAScript spec a `\N` digit escape past the group count is a
// regex octal escape — not a useless backref — and `\k<name>`
// without a matching named group is literal text when no named
// groups appear in the pattern.
func NewBiome() engine.Rule { return &rule{id: IDBiome, flagMissingGroup: false} }

type rule struct {
	id               string
	flagMissingGroup bool
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: r.id} }

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
	refs, total, names := analyzePattern(pattern)
	for _, ref := range refs {
		if ref.name != "" {
			if _, ok := names[ref.name]; !ok {
				if r.flagMissingGroup {
					ctx.Report(n, "Backreference '\\k<"+ref.name+">' refers to a non-existent group.")
				}
				continue
			}
			if ref.circular {
				ctx.Report(n, "Backreference '\\k<"+ref.name+">' refers to the group that contains it.")
			}
			continue
		}
		if ref.num > total {
			if r.flagMissingGroup {
				ctx.Report(n, "Backreference '\\"+strconv.Itoa(ref.num)+"' refers to a non-existent group.")
			}
			continue
		}
		if ref.circular {
			ctx.Report(n, "Backreference '\\"+strconv.Itoa(ref.num)+"' refers to the group that contains it.")
		}
	}
}

type backref struct {
	num      int
	name     string
	circular bool
}

type frame struct {
	num       int
	name      string
	capturing bool
}

// analyzePattern walks the regex pattern once, returning every
// backreference (with a circularity flag set when the backref's
// position lies inside the body of the group it points to), the
// total capturing-group count, and the set of named capturing
// groups.
func analyzePattern(p string) ([]backref, int, map[string]bool) {
	names := map[string]bool{}
	var refs []backref
	var stack []frame
	total := 0
	inClass := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			if i+1 >= len(p) {
				continue
			}
			esc := p[i+1]
			if inClass {
				i++
				continue
			}
			if esc == 'k' && i+2 < len(p) && p[i+2] == '<' {
				end := strings.IndexByte(p[i+3:], '>')
				if end > 0 {
					name := p[i+3 : i+3+end]
					circ := false
					for _, f := range stack {
						if f.capturing && f.name == name {
							circ = true
							break
						}
					}
					refs = append(refs, backref{name: name, circular: circ})
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
				circ := false
				for _, f := range stack {
					if f.capturing && f.num == n {
						circ = true
						break
					}
				}
				refs = append(refs, backref{num: n, circular: circ})
				i = j - 1
				continue
			}
			i++ // consume the escaped character
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
			if i+1 < len(p) && p[i+1] == '?' {
				if i+2 < len(p) {
					nxt := p[i+2]
					if nxt == ':' || nxt == '=' || nxt == '!' {
						stack = append(stack, frame{})
						i += 2
						continue
					}
					if nxt == '<' {
						if i+3 < len(p) && (p[i+3] == '=' || p[i+3] == '!') {
							stack = append(stack, frame{})
							i += 3
							continue
						}
						end := strings.IndexByte(p[i+3:], '>')
						if end > 0 {
							name := p[i+3 : i+3+end]
							names[name] = true
							total++
							stack = append(stack, frame{num: total, name: name, capturing: true})
							i += 3 + end
							continue
						}
					}
				}
				stack = append(stack, frame{})
				i++
				continue
			}
			total++
			stack = append(stack, frame{num: total, capturing: true})
			continue
		}
		if c == ')' {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
	}
	return refs, total, names
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
