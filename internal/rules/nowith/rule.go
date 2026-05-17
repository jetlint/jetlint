// Package nowith implements no-with: `with` makes scope unresolvable
// at parse time and was banned by strict mode. Forbid it everywhere.
package nowith

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-with"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, file *wrapperchecker.Node) {
	src := file.SourceText()
	// Look for the `with (` syntax. Skip occurrences inside strings/comments.
	off := 0
	for {
		i := strings.Index(src[off:], "with")
		if i < 0 {
			return
		}
		abs := off + i
		// Word boundary: previous char must not be a letter/digit/_.
		if abs > 0 {
			c := src[abs-1]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				off = abs + 4
				continue
			}
		}
		// Must be followed by whitespace + `(`.
		j := abs + 4
		for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n') {
			j++
		}
		if j >= len(src) || src[j] != '(' {
			off = abs + 4
			continue
		}
		if isInsideCommentOrString(src, abs) {
			off = abs + 4
			continue
		}
		ctx.Report(file, "`with` statement — banned in strict mode")
		return
	}
}

func isInsideCommentOrString(src string, pos int) bool {
	lineStart := strings.LastIndexByte(src[:pos], '\n') + 1
	if strings.Contains(src[lineStart:pos], "//") {
		return true
	}
	if i := strings.LastIndex(src[:pos], "/*"); i >= 0 {
		if j := strings.LastIndex(src[:pos], "*/"); j < i {
			return true
		}
	}
	return false
}
