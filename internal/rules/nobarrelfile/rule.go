// Package nobarrelfile implements no-barrel-file: flag files whose
// only exports are re-exports from other modules. Barrel files
// import every named export from each referenced file just to
// re-export it, so importing one symbol drags in the full
// transitive graph — a known cause of dev-time and bundle bloat.
//
// Detection mirrors biome's check: every top-level export must be
// a re-export `export { ... } from "..."`, `export * from "..."`,
// or `export * as N from "..."`. Files that mix re-exports with
// local declarations (or files with no exports at all) are fine.
package nobarrelfile

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-barrel-file"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	var reexports []*wrapperchecker.Node
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindExportDeclaration {
			return false
		}
		if c.IsTypeOnlyExport() {
			return false
		}
		if c.ModuleSpecifier() == nil {
			return false
		}
		reexports = append(reexports, c)
		return false
	})
	for _, r := range reexports {
		ctx.Report(r, "re-export-only file acts as a barrel — import directly from the source module")
	}
}
