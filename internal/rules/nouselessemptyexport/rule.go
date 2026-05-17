// Package nouselessemptyexport implements no-useless-empty-export:
// `export {}` flips a file into "module" mode. Useful when nothing
// else is exported; redundant when the file already has another
// `export`.
package nouselessemptyexport

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-empty-export"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, file *wrapperchecker.Node) {
	// Walk top-level statements; record empty `export {}` and presence
	// of any other export.
	var emptyExports []*wrapperchecker.Node
	moduleMarkers := 0
	file.ForEachChild(func(c *wrapperchecker.Node) bool {
		src := strings.TrimSpace(c.SourceText())
		isEmpty := isEmptyExport(c, src)
		if isEmpty {
			emptyExports = append(emptyExports, c)
			return false
		}
		// Any import or any other export makes the file a module.
		if c.Kind() == wrapperchecker.KindImportDeclaration ||
			c.Kind() == wrapperchecker.KindExportDeclaration ||
			c.Kind() == wrapperchecker.KindExportAssignment ||
			startsWithExport(src) || hasExportModifier(c) {
			moduleMarkers++
		}
		return false
	})
	// Two-or-more `export {}` themselves are redundant after the first.
	if len(emptyExports) >= 2 {
		for _, e := range emptyExports[1:] {
			ctx.Report(e, "duplicate `export {}` — only one is needed")
		}
	}
	if moduleMarkers == 0 {
		return
	}
	// Report the first empty export (the others, if any, were reported as duplicates).
	if len(emptyExports) > 0 {
		ctx.Report(emptyExports[0], "`export {}` is redundant — the file already has other exports")
	}
}

func isEmptyExport(c *wrapperchecker.Node, src string) bool {
	if c.Kind() != wrapperchecker.KindExportDeclaration {
		return false
	}
	stripped := stripWhitespace(src)
	return stripped == "export{}" || stripped == "export{};"
}

func stripWhitespace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func startsWithExport(src string) bool {
	src = strings.TrimSpace(src)
	return strings.HasPrefix(src, "export ") || strings.HasPrefix(src, "export{") ||
		strings.HasPrefix(src, "export\t") || strings.HasPrefix(src, "export\n") ||
		strings.HasPrefix(src, "export*") || strings.HasPrefix(src, "export ")
}

func hasExportModifier(n *wrapperchecker.Node) bool {
	out := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindExportKeyword {
			out = true
		}
		return false
	})
	return out
}
