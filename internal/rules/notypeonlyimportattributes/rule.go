// Package notypeonlyimportattributes implements
// no-type-only-import-attributes: import-attributes (`with { ... }`)
// are erased only at the moment of runtime resolution, but a
// type-only import never reaches runtime, so attaching attributes
// to one is dead syntax that TypeScript silently strips. The rule
// flags type-only imports/exports paired with a `with` clause so
// the attributes are moved to a real runtime import where they
// actually do something.
package notypeonlyimportattributes

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-type-only-import-attributes"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindImportDeclaration: visitImport,
		wrapperchecker.KindExportDeclaration: visitExport,
	}
}

const importAttributesKind = wrapperchecker.Kind(301)

func visitImport(ctx *engine.Context, decl *wrapperchecker.Node) {
	attrs := findAttributes(decl)
	if attrs == nil || onlyResolutionMode(attrs) {
		return
	}
	if !importClauseHasTypeOnly(decl) {
		return
	}
	ctx.Report(attrs, "remove import attributes from this type-only import; they are stripped at compile time and have no runtime effect")
}

func visitExport(ctx *engine.Context, decl *wrapperchecker.Node) {
	attrs := findAttributes(decl)
	if attrs == nil || onlyResolutionMode(attrs) {
		return
	}
	if !exportHasTypeOnly(decl) {
		return
	}
	ctx.Report(attrs, "remove import attributes from this type-only export; they are stripped at compile time and have no runtime effect")
}

// onlyResolutionMode reports whether every attribute key in the
// `with { ... }` clause is `resolution-mode`. biome carves this out
// because TypeScript reads `resolution-mode` to pick between CJS and
// ESM type resolution — the attribute is *consumed* by the compiler,
// not erased like the others, so it's the one case where a type-only
// import legitimately needs a `with` clause.
func onlyResolutionMode(attrs *wrapperchecker.Node) bool {
	any := false
	allResolution := true
	attrs.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != importAttributeKind {
			return false
		}
		any = true
		if !attributeKeyIs(c, "resolution-mode") {
			allResolution = false
			return true
		}
		return false
	})
	return any && allResolution
}

func attributeKeyIs(attr *wrapperchecker.Node, want string) bool {
	var match bool
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindIdentifier:
			text := c.LiteralText()
			if text == want {
				match = true
			}
			return true
		}
		return false
	})
	return match
}

const importAttributeKind = wrapperchecker.Kind(302)

func findAttributes(decl *wrapperchecker.Node) *wrapperchecker.Node {
	var found *wrapperchecker.Node
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == importAttributesKind {
			found = c
			return true
		}
		return false
	})
	return found
}

// importClauseHasTypeOnly reports whether the import statement has
// `type` either at the clause level (`import type X`) or on any
// individual specifier (`import { type X }`).
func importClauseHasTypeOnly(decl *wrapperchecker.Node) bool {
	var hit bool
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		txt := c.SourceText()
		if strings.HasPrefix(txt, "type ") || strings.HasPrefix(txt, "type{") {
			hit = true
			return true
		}
		if hasInlineTypeSpecifier(c) {
			hit = true
			return true
		}
		return true
	})
	return hit
}

// hasInlineTypeSpecifier checks the named-imports child of an
// import clause for any `type X`-prefixed specifier.
func hasInlineTypeSpecifier(clause *wrapperchecker.Node) bool {
	var hit bool
	clause.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindNamedImports {
			return false
		}
		c.ForEachChild(func(spec *wrapperchecker.Node) bool {
			if spec.Kind() == wrapperchecker.KindImportSpecifier {
				if strings.HasPrefix(spec.SourceText(), "type ") {
					hit = true
					return true
				}
			}
			return false
		})
		return hit
	})
	return hit
}

// exportHasTypeOnly mirrors importClauseHasTypeOnly but uses the
// dedicated AST predicates exposed by the wrapper for exports.
func exportHasTypeOnly(decl *wrapperchecker.Node) bool {
	if decl.IsTypeOnlyExport() {
		return true
	}
	var hit bool
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindNamedExports {
			return false
		}
		c.ForEachChild(func(spec *wrapperchecker.Node) bool {
			if spec.IsTypeOnlyExportSpecifier() {
				hit = true
				return true
			}
			return false
		})
		return hit
	})
	return hit
}
