// Package consistenttypeexports implements the consistent-type-exports
// rule: flag `export { X }` declarations whose specifiers refer to
// type-only symbols. The user should use `export type { X }` instead.
//
// The rule has two messages:
//   - typeOverValue: every specifier in the declaration is a type →
//     write `export type { ... }` to keep the JS output free of dead
//     imports.
//   - singleExportIsType / multipleExportsAreTypes: the declaration
//     mixes types and values → mark each type specifier inline with
//     `export { type X, value }`.
package consistenttypeexports

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "consistent-type-exports"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExportDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// `export type { ... }` / `export type *` is already correct; only
	// flag value-flavored declarations whose contents turn out to be
	// type-only.
	if n.IsTypeOnlyExport() {
		return
	}
	// `export * from '...'` and `export * as ns from '...'` have no
	// NamedExports clause. Handle separately: resolve the source module
	// and inspect whether any export is a runtime value.
	if isExportStar(n) {
		visitExportStar(ctx, n)
		return
	}
	specifiers := exportSpecifiers(n)
	if len(specifiers) == 0 {
		return
	}
	var typeSpecs, valueSpecs []*wrapperchecker.Node
	hasInlineTypeMarker := false
	for _, spec := range specifiers {
		if spec.IsTypeOnlyExportSpecifier() {
			hasInlineTypeMarker = true
			continue
		}
		local := exportSpecifierLocalName(spec)
		if local == nil {
			continue
		}
		sym := ctx.Checker().SymbolOf(local)
		if sym == nil {
			valueSpecs = append(valueSpecs, spec)
			continue
		}
		if sym.IsTypeOnly() {
			typeSpecs = append(typeSpecs, spec)
		} else {
			valueSpecs = append(valueSpecs, spec)
		}
	}
	// All specifiers are type-only and there's no inline `type` marker
	// — suggest `export type { ... }`.
	if len(typeSpecs) > 0 && len(valueSpecs) == 0 && !hasInlineTypeMarker {
		ctx.Report(n, "all exports are type-only — use `export type { ... }`")
		return
	}
	// Mixed: one report per declaration. The message lists every
	// type-only specifier so the suggestion is to mark each inline
	// (`export { type X, value }`). Mirrors upstream's
	// singleExportIsType / multipleExportsAreTypes.
	if len(typeSpecs) > 0 {
		ctx.Report(n, "type exports should be marked inline with `type`")
	}
}

// isExportStar reports whether n is `export * from '...'` or
// `export * as ns from '...'` — declarations with no NamedExports
// child. Both shapes are ExportDeclaration nodes; the named form
// instead has a NamedExports clause that exportSpecifiers picks up.
func isExportStar(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindExportDeclaration {
		return false
	}
	if n.ModuleSpecifier() == nil {
		return false
	}
	// `export { X }` / `export { X } from '...'` carries a NamedExports
	// child via the ExportClause slot; bare `export *` and namespace
	// `export * as ns` do not.
	hasNamed := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindNamedExports {
			hasNamed = true
			return true
		}
		return false
	})
	return !hasNamed
}

// visitExportStar handles `export * from '...'` / `export * as ns from '...'`.
// Resolves the source module and flags the declaration when the module
// has no runtime value exports — mirrors upstream's ExportAllDeclaration
// handler which uses the `getPropertiesOfType` / `getPropertyOfType`
// pair to filter out type-only-star-exported names.
func visitExportStar(ctx *engine.Context, n *wrapperchecker.Node) {
	spec := n.ModuleSpecifier()
	if spec == nil {
		return
	}
	mod := ctx.Checker().ResolveExternalModule(spec)
	if mod == nil {
		return
	}
	modType := ctx.Checker().TypeOfSymbol(mod)
	if modType == nil {
		return
	}
	// `PropertyNames` returns the property set including type-only-star
	// re-exports; `PropertySymbol(name)` returns nil for those (mirrors
	// the typeOnlyExportStarMap filtering in TS). A value export survives
	// both — short-circuit on the first one we find.
	for _, name := range modType.PropertyNames() {
		if modType.PropertySymbol(name) != nil {
			return
		}
	}
	ctx.Report(n, "all exports are type-only — use `export type *`")
}

// exportSpecifiers returns the named-export specifiers of an
// `export { ... }` (or `export { ... } from '...'`) declaration. Empty
// for `export *`, `export default`, or other shapes.
func exportSpecifiers(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var specs []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindNamedExports {
			c.ForEachChild(func(s *wrapperchecker.Node) bool {
				if s.Kind() == wrapperchecker.KindExportSpecifier {
					specs = append(specs, s)
				}
				return false
			})
			return true
		}
		return false
	})
	return specs
}

// exportSpecifierLocalName returns the identifier referring to the
// local symbol being re-exported. For `export { foo as bar }` this is
// the `foo` node; for `export { foo }` it's the same `foo`.
func exportSpecifierLocalName(spec *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	spec.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindStringLiteral:
			if first == nil {
				first = c
				return true
			}
		}
		return false
	})
	return first
}
