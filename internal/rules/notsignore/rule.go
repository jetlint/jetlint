// Package notsignore implements no-ts-ignore: `@ts-ignore` silences
// the type checker without saying why. `@ts-expect-error` does the
// same job but breaks loudly when the underlying issue is fixed,
// preventing the suppression from outliving the bug.
package notsignore

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-ts-ignore"

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
	// Scan once for occurrences of `@ts-ignore` inside comments.
	off := 0
	for {
		i := strings.Index(src[off:], "@ts-ignore")
		if i < 0 {
			return
		}
		abs := off + i
		// Verify the occurrence is inside a comment (// or /* */).
		if isInsideComment(src, abs) {
			ctx.Report(file, "@ts-ignore silently hides errors — use @ts-expect-error so it breaks when fixed")
		}
		off = abs + len("@ts-ignore")
	}
}

func isInsideComment(src string, pos int) bool {
	// Walk back to find either:
	//   - `//` on the same line before `pos` → line comment
	//   - `/*` before `pos` without a closing `*/` between → block comment
	lineStart := strings.LastIndexByte(src[:pos], '\n') + 1
	if i := strings.Index(src[lineStart:pos], "//"); i >= 0 {
		return true
	}
	if i := strings.LastIndex(src[:pos], "/*"); i >= 0 {
		if j := strings.LastIndex(src[:pos], "*/"); j < i {
			return true
		}
	}
	return false
}
