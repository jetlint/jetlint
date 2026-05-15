// Package noirregularwhitespace implements the no-irregular-whitespace
// rule: invisible / unusual whitespace characters (non-breaking space,
// zero-width space, ideographic space, etc.) survive copy-paste, hide
// behind line-noise in code review, and silently break tooling that
// expects ASCII spacing. This port scans the source text once, then
// trims the AST-derived ranges that the rule's options say to skip
// (strings, regex, templates, comments, JSX text by default).
package noirregularwhitespace

import (
	"unicode/utf8"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-irregular-whitespace"

// Options configures the rule.
type Options struct {
	SkipStrings   bool
	SkipComments  bool
	SkipRegExps   bool
	SkipTemplates bool
	SkipJSXText   bool
}

// DefaultOptions matches ESLint's defaults (skip = true everywhere).
func DefaultOptions() Options {
	return Options{
		SkipStrings:   true,
		SkipComments:  true,
		SkipRegExps:   true,
		SkipTemplates: true,
		SkipJSXText:   true,
	}
}

// New constructs a rule with default options.
func New() engine.Rule { return &rule{opts: DefaultOptions()} }

// NewWithOptions constructs a rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: r.visit,
	}
}

type span struct{ start, end int }

// Wrapper-internal kinds the public `checker` package omits but we
// still need to recognise. Values come from
// `internal/ast/kind_generated.go` and are stable across the tsgo
// fork's API releases.
const (
	kindJsxText               = wrapperchecker.Kind(11)
	kindJsxTextAllWhiteSpaces = wrapperchecker.Kind(12)
)

// tokenStart returns the offset of `n`'s first non-trivia byte. The
// wrapper's `Pos()` includes leading trivia (whitespace, comments) so
// using it as a skip-span start would mask irregular whitespace that
// sits *before* the literal token.
func tokenStart(n *wrapperchecker.Node) int {
	return n.End() - len(n.SourceText())
}

func (r *rule) visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	src := sf.LeadingTriviaText() + sf.SourceText()
	if src == "" {
		return
	}
	skips := r.collectSkipSpans(sf, src)
	for offset := 0; offset < len(src); {
		ru, size := utf8.DecodeRuneInString(src[offset:])
		if size == 0 {
			break
		}
		if isIrregular(ru) {
			// The BOM (U+FEFF) is permitted at the very start of a
			// file; flag it only when it appears elsewhere.
			if !(ru == '\uFEFF' && offset == 0) && !inAnySpan(offset, skips) {
				ctx.Report(sf, "Unexpected irregular whitespace")
			}
		}
		offset += size
	}
}

// collectSkipSpans returns the source ranges the rule should exclude
// from its irregular-whitespace scan. Comment ranges come from a
// manual scan of code-context regions; literal/template/regex spans
// come straight from the AST.
func (r *rule) collectSkipSpans(sf *wrapperchecker.Node, src string) []span {
	var spans []span
	if r.opts.SkipComments {
		spans = append(spans, commentSpans(sf, src)...)
	}
	collectAstSpans(sf, &spans, r.opts)
	return spans
}

// collectAstSpans walks the AST below `n` and records the span of
// each literal node whose contents the configured options say to
// skip.
func collectAstSpans(n *wrapperchecker.Node, out *[]span, opts Options) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral:
			if opts.SkipStrings {
				*out = append(*out, span{tokenStart(c), c.End()})
			}
		case wrapperchecker.KindNoSubstitutionTemplateLiteral:
			if opts.SkipTemplates {
				*out = append(*out, span{tokenStart(c), c.End()})
			}
		case wrapperchecker.KindTemplateExpression:
			if opts.SkipTemplates {
				collectTemplateQuasiSpans(c, out)
			}
		case wrapperchecker.KindRegularExpressionLiteral:
			if opts.SkipRegExps {
				*out = append(*out, span{tokenStart(c), c.End()})
			}
		case kindJsxText, kindJsxTextAllWhiteSpaces:
			// JsxText's SourceText() is empty in the wrapper so the
			// tokenStart heuristic collapses the span; use the raw
			// Pos()/End() range, which already excludes leading
			// trivia for JsxText specifically.
			if opts.SkipJSXText {
				*out = append(*out, span{c.Pos(), c.End()})
			}
		}
		collectAstSpans(c, out, opts)
		return false
	})
}

func collectTemplateQuasiSpans(tmpl *wrapperchecker.Node, out *[]span) {
	tmpl.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindTemplateHead,
			wrapperchecker.KindTemplateMiddle,
			wrapperchecker.KindTemplateTail:
			*out = append(*out, span{tokenStart(c), c.End()})
		case wrapperchecker.KindTemplateSpan:
			collectTemplateQuasiSpans(c, out)
		}
		return false
	})
}

// commentSpans scans `src` for `//` and `/*` comment runs. To avoid
// mistaking comment-prefix characters that appear inside a string or
// regex for the start of a comment we skip over the AST literal spans.
func commentSpans(sf *wrapperchecker.Node, src string) []span {
	var lits []span
	collectAstSpans(sf, &lits, Options{
		SkipStrings:   true,
		SkipTemplates: true,
		SkipRegExps:   true,
	})
	sortSpans(lits)
	merged := mergeSpans(lits)

	var spans []span
	i := 0
	for i < len(src) {
		if in := containingSpan(i, merged); in != nil {
			i = in.end
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			end := i + 2
			for end < len(src) && src[end] != '\n' && src[end] != '\r' {
				end++
			}
			spans = append(spans, span{i, end})
			i = end
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			end := i + 2
			for end+1 < len(src) && !(src[end] == '*' && src[end+1] == '/') {
				end++
			}
			if end+1 < len(src) {
				end += 2
			} else {
				end = len(src)
			}
			spans = append(spans, span{i, end})
			i = end
			continue
		}
		i++
	}
	return spans
}

func sortSpans(spans []span) {
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j-1].start > spans[j].start; j-- {
			spans[j-1], spans[j] = spans[j], spans[j-1]
		}
	}
}

func mergeSpans(spans []span) []span {
	if len(spans) == 0 {
		return spans
	}
	out := spans[:1]
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

func containingSpan(off int, spans []span) *span {
	lo, hi := 0, len(spans)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		s := &spans[mid]
		if off < s.start {
			hi = mid - 1
		} else if off >= s.end {
			lo = mid + 1
		} else {
			return s
		}
	}
	return nil
}

func inAnySpan(off int, spans []span) bool {
	for _, s := range spans {
		if off >= s.start && off < s.end {
			return true
		}
	}
	return false
}

// isIrregular reports whether `r` matches one of the Unicode
// characters ESLint flags. Mirrors oxc's `is_irregular_whitespace`
// plus the irregular line terminators U+2028 / U+2029, the
// zero-width-no-break-space U+FEFF, and the Mongolian Vowel Separator
// U+180E.
func isIrregular(r rune) bool {
	switch r {
	case '', // VT
		'',// FF
		'', // NEL
		' ',                // NBSP
		' ',                // OGHAM SPACE MARK
		'᠎',                // MONGOLIAN VOWEL SEPARATOR
		' ', ' ', ' ', ' ', // EN/EM quads
		' ', ' ', ' ', ' ', // three/four/six/figure
		' ', ' ', ' ', // punctuation/thin/hair
		'​',      // ZERO WIDTH SPACE
		' ',      // LINE SEPARATOR
		' ',      // PARAGRAPH SEPARATOR
		' ',      // NARROW NO-BREAK SPACE
		' ',      // MEDIUM MATHEMATICAL SPACE
		'　',      // IDEOGRAPHIC SPACE
		'\uFEFF': // ZERO WIDTH NO-BREAK SPACE / BOM
		return true
	}
	return false
}
