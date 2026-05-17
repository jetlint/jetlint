// Package usesinglejsdocasterisk implements use-single-js-doc-asterisk:
// flag JSDoc block comments that use unnecessary extra asterisks.
// The common shapes biome's rule catches are:
//   - `**/` closer instead of ` */` — an extra leading asterisk on
//     the final line;
//   - a line inside the block that starts with `**` instead of the
//     conventional ` *` line prefix.
//
// Markdown-style bold (`**bold**`) on a content line is fine — it
// only matters when the asterisks are next to the comment's own
// framing.
//
// Detection is textual: JSDoc comments aren't surfaced as AST
// nodes here, so the rule walks the file source for `/** ... */`
// spans and inspects each line directly.
package usesinglejsdocasterisk

import (
	"os"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-single-js-doc-asterisk"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	// SourceText() on a SourceFile starts at the first token,
	// which skips leading comments and produces nothing for a
	// comment-only file. Read the underlying file directly so the
	// scan can see every JSDoc block.
	path, _, _, _, _ := src.SourceRange()
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	text := string(body)
	i := 0
	for {
		start := strings.Index(text[i:], "/**")
		if start < 0 {
			return
		}
		start += i
		end := strings.Index(text[start:], "*/")
		if end < 0 {
			return
		}
		end += start
		body := text[start : end+2]
		if jsdocHasExtraAsterisks(body) {
			ctx.Report(src, "JSDoc block has extra asterisks; use a single `* ` line prefix and ` */` closer")
		}
		i = end + 2
	}
}

// jsdocHasExtraAsterisks reports whether the comment text contains
// the unsafe-extra-asterisk patterns biome flags: a `**/` closer
// (instead of ` */`), or any content line that begins with `**`
// where the line prefix should be a single asterisk.
func jsdocHasExtraAsterisks(body string) bool {
	// Closer: ends with `**/`. The body always ends with `*/`, so
	// we look at the character immediately before that.
	if len(body) >= 3 && body[len(body)-3] == '*' &&
		body[len(body)-2] == '*' && body[len(body)-1] == '/' {
		// Distinguish `**/` from a one-line block `/** */` where
		// the leading `**` is part of the opener.
		if len(body) > 5 {
			return true
		}
	}
	// Content lines: skip the opening `/**` line and the closing
	// line (the one that holds `*/`); biome only flags inner-
	// content lines whose prefix is `**` instead of the
	// conventional `*`. Allowing the closing line means
	// `/**\n ** * Valid end */` stays valid.
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		if strings.Contains(line, "*/") {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "**") {
			return true
		}
	}
	return false
}
