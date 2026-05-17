// Package noreexportall implements no-re-export-all: flag
// `export * from "..."` (and `export * as X from "..."`).
// Wildcard re-exports defeat tree-shaking — bundlers can't tell
// which exports of the upstream module are actually consumed, so
// every symbol stays in the bundle. Prefer explicit
// `export { foo, bar } from "..."`.
//
// `export type * from "..."` (and `export type * as X from "..."`)
// is allowed: type-only re-exports are erased before bundling.
package noreexportall

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-re-export-all"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExportDeclaration: visit,
	}
}

func visit(ctx *engine.Context, decl *wrapperchecker.Node) {
	if decl.IsTypeOnlyExport() {
		return
	}
	// An ExportDeclaration is a wildcard re-export when it has no
	// named export list (NamedExports) but does have a module
	// specifier. NamespaceExport (`* as X`) still counts as a
	// wildcard re-export.
	var hasNamed, hasWildcard bool
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindNamedExports:
			hasNamed = true
		case wrapperchecker.KindNamespaceExport:
			hasWildcard = true
		}
		return false
	})
	if hasNamed {
		return
	}
	// Pure `export * from "..."` has neither NamedExports nor
	// NamespaceExport — detect via text since the wrapper doesn't
	// expose a dedicated flag.
	if !hasWildcard && !strings.Contains(decl.SourceText(), "*") {
		return
	}
	ctx.Report(decl, "wildcard re-export defeats tree-shaking — re-export specific names instead")
}
