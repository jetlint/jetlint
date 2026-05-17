// Package noexportedimports implements no-exported-imports: when you
// `import { X }` and then `export { X }`, it's clearer to write
// `export { X } from "mod"` directly.
package noexportedimports

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-exported-imports"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	imports := map[string]bool{}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindImportDeclaration {
			collectImportedNames(c, imports)
		}
		return false
	})
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindExportDeclaration {
			return false
		}
		// Skip re-exports with `from`.
		if hasFromClause(c) {
			return false
		}
		walkExportSpecifiers(c, func(name string, spec *wrapperchecker.Node) {
			if imports[name] {
				ctx.Report(spec, "re-export imported name directly with `export { "+name+" } from \"…\"`")
			}
		})
		return false
	})
}

func collectImportedNames(decl *wrapperchecker.Node, out map[string]bool) {
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			switch d.Kind() {
			case wrapperchecker.KindIdentifier:
				out[d.SourceText()] = true
			case wrapperchecker.KindNamespaceImport:
				d.ForEachChild(func(x *wrapperchecker.Node) bool {
					if x.Kind() == wrapperchecker.KindIdentifier {
						out[x.SourceText()] = true
					}
					return false
				})
			case wrapperchecker.KindNamedImports:
				d.ForEachChild(func(spec *wrapperchecker.Node) bool {
					if spec.Kind() == wrapperchecker.KindImportSpecifier {
						var last *wrapperchecker.Node
						spec.ForEachChild(func(x *wrapperchecker.Node) bool {
							if x.Kind() == wrapperchecker.KindIdentifier {
								last = x
							}
							return false
						})
						if last != nil {
							out[last.SourceText()] = true
						}
					}
					return false
				})
			}
			return false
		})
		return false
	})
}

func hasFromClause(decl *wrapperchecker.Node) bool {
	has := false
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStringLiteral {
			has = true
			return true
		}
		return false
	})
	return has
}

func walkExportSpecifiers(decl *wrapperchecker.Node, fn func(name string, spec *wrapperchecker.Node)) {
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindNamedExports {
			c.ForEachChild(func(spec *wrapperchecker.Node) bool {
				if spec.Kind() == wrapperchecker.KindExportSpecifier {
					var first *wrapperchecker.Node
					spec.ForEachChild(func(x *wrapperchecker.Node) bool {
						if first == nil {
							first = x
						}
						return false
					})
					if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
						fn(first.SourceText(), spec)
					}
				}
				return false
			})
		}
		return false
	})
}
