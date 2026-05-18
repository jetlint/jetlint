// Package nodeprecatedimports implements the no-deprecated-imports rule:
// flag imports whose source symbol carries a `@deprecated` JSDoc tag.
//
// Mirrors biome's noDeprecatedImports. Differs from no-deprecated in
// that it fires at the import site rather than every use site, so the
// author can mark and migrate the import in one place.
package nodeprecatedimports

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-deprecated-imports"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportClause:    r.visitClause,
		wrapperchecker.KindImportSpecifier: r.visitSpecifier,
	}
}

// visitClause handles the default-import slot: `import X from "y"` and
// `import X, { ... } from "y"`. Only the leading Identifier child of an
// ImportClause is the default binding; NamedBindings (NamedImports or
// NamespaceImport) live as a separate child and are handled elsewhere.
func (rule) visitClause(ctx *engine.Context, n *wrapperchecker.Node) {
	var name *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c
			return true
		}
		return false
	})
	if name == nil {
		return
	}
	sym := ctx.Checker().SymbolOf(name)
	if sym == nil || !sym.IsDeprecated() {
		return
	}
	report(ctx, name, sym.DeprecationReason())
}

// visitSpecifier handles named imports: `{ foo }` and `{ foo as bar }`.
// The local-binding symbol resolves through the alias chain to the
// source export, so IsDeprecated picks up a `@deprecated` annotation on
// the original declaration. Reports at the source-name slot — the
// first identifier in source order (the propertyName when aliased, the
// only identifier otherwise) — matching biome's diagnostic position.
func (rule) visitSpecifier(ctx *engine.Context, n *wrapperchecker.Node) {
	var first, last *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindStringLiteral:
			if first == nil {
				first = c
			}
			last = c
		}
		return false
	})
	if last == nil {
		return
	}
	sym := ctx.Checker().SymbolOf(last)
	if sym == nil || !sym.IsDeprecated() {
		return
	}
	target := first
	if target == nil {
		target = last
	}
	report(ctx, target, sym.DeprecationReason())
}

func report(ctx *engine.Context, at *wrapperchecker.Node, reason string) {
	if reason == "" {
		ctx.Report(at, "Deprecated import.")
		return
	}
	ctx.Report(at, fmt.Sprintf("Deprecated import: %s", reason))
}
