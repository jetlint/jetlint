// Package useexportslast implements use-exports-last: keeping `export`
// statements at the bottom of a module makes the public surface easy
// to scan.
package useexportslast

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-exports-last"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	seenExport := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if isExportStatement(c) {
			seenExport = true
		} else if seenExport && isRelevantStatement(c) {
			ctx.Report(c, "move this statement above the exports")
		}
		return false
	})
}

func isExportStatement(c *wrapperchecker.Node) bool {
	switch c.Kind() {
	case wrapperchecker.KindExportDeclaration, wrapperchecker.KindExportAssignment:
		return true
	}
	// `export const` / `export function` / `export class` etc. — these are
	// regular declarations with an `export` modifier.
	if hasExportModifier(c) {
		return true
	}
	return false
}

func hasExportModifier(c *wrapperchecker.Node) bool {
	src := c.SourceText()
	for i := 0; i+6 <= len(src); i++ {
		switch src[i] {
		case ' ', '\t', '\n':
			continue
		default:
			if src[i:i+6] == "export" {
				return true
			}
			return false
		}
	}
	return false
}

func isRelevantStatement(c *wrapperchecker.Node) bool {
	switch c.Kind() {
	case wrapperchecker.KindVariableStatement, wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration, wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindInterfaceDeclaration, wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindModuleDeclaration, wrapperchecker.KindImportDeclaration,
		wrapperchecker.KindExpressionStatement, wrapperchecker.KindIfStatement,
		wrapperchecker.KindForStatement, wrapperchecker.KindWhileStatement:
		return true
	}
	return false
}
