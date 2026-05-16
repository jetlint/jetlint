// Package nounresolvedimports implements no-unresolved-imports: a
// relative or aliased import that doesn't actually resolve to a
// real file is dead syntax — at best it fails the build, at worst
// it silently confuses tooling. The rule asks the checker to
// resolve every import/export specifier and flags any that come
// back empty. Bare package imports (non-relative, non-aliased)
// are skipped — they depend on a `node_modules` tree the rule
// can't reliably model.
package nounresolvedimports

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/modresolve"
)

const id = "no-unresolved-imports"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visit,
		wrapperchecker.KindExportDeclaration: visit,
	}
}

func visit(ctx *engine.Context, decl *wrapperchecker.Node) {
	spec := decl.ModuleSpecifier()
	if spec == nil {
		return
	}
	if spec.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	text := spec.LiteralText()
	if !modresolve.IsRelativeSpecifier(text) {
		return
	}
	res := modresolve.Resolve(ctx.Checker(), spec)
	if res.Resolved {
		return
	}
	ctx.Report(spec, "could not resolve import path; check the path or restore the missing file")
}
