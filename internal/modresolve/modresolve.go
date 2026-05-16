// Package modresolve wraps the typescript-go checker's module
// resolution so jetlint rules can answer "does this import path
// resolve?" and "what does the resolved module export?" without
// each rule re-implementing path normalization. The checker already
// performs the heavy lifting at type-check time; this package just
// surfaces the result through a small, rule-friendly API.
package modresolve

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
)

// Resolution describes the outcome of resolving a single module
// specifier (the string after `from` in an import/export).
type Resolution struct {
	// Resolved is true when the checker matched the specifier to a
	// real source file (relative or otherwise).
	Resolved bool
	// File is the absolute path of the resolved file, empty when the
	// specifier did not resolve.
	File string
	// HasDefault is true when the resolved module declares a default
	// export. Meaningless when Resolved is false.
	HasDefault bool
	// NamedExports holds the identifier names exported by the
	// resolved module. Nil when Resolved is false.
	NamedExports map[string]bool
}

// Resolve looks up the module the specifier refers to and reports
// what it found. `specifier` must be a StringLiteral node (the one
// returned by Node.ModuleSpecifier on an import/export declaration).
func Resolve(checker *wrapperchecker.Checker, specifier *wrapperchecker.Node) Resolution {
	if checker == nil || specifier == nil {
		return Resolution{}
	}
	sym := checker.ResolveExternalModule(specifier)
	if sym == nil {
		return Resolution{}
	}
	decls := sym.Declarations()
	for _, d := range decls {
		if d == nil {
			continue
		}
		if d.Kind() != wrapperchecker.KindSourceFile {
			continue
		}
		file, _, _, _, _ := d.SourceRange()
		named, hasDefault := scanExports(d)
		return Resolution{
			Resolved:     true,
			File:         file,
			HasDefault:   hasDefault,
			NamedExports: named,
		}
	}
	return Resolution{}
}

// scanExports walks a source file's top-level statements and
// collects the names it makes available as named exports plus
// whether it has a default export. The scan is deliberately
// shallow — it covers the export forms our rules actually care
// about (named function/class/variable declarations carrying an
// `export` modifier, `export { a, b }`, `export default ...`,
// `export * from ...`) without attempting to follow re-exports
// across files.
func scanExports(sourceFile *wrapperchecker.Node) (map[string]bool, bool) {
	named := map[string]bool{}
	var hasDefault bool
	sourceFile.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		switch stmt.Kind() {
		case wrapperchecker.KindExportAssignment:
			hasDefault = true
		case wrapperchecker.KindExportDeclaration:
			collectExportDeclaration(stmt, named, &hasDefault)
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindInterfaceDeclaration,
			wrapperchecker.KindTypeAliasDeclaration,
			wrapperchecker.KindEnumDeclaration:
			if hasExportModifier(stmt) {
				if hasDefaultModifier(stmt) {
					hasDefault = true
				} else if name := declarationIdentifier(stmt); name != "" {
					named[name] = true
				}
			}
		case wrapperchecker.KindVariableStatement:
			if hasExportModifier(stmt) {
				collectVariableNames(stmt, named)
			}
		}
		return false
	})
	return named, hasDefault
}

func collectExportDeclaration(stmt *wrapperchecker.Node, named map[string]bool, hasDefault *bool) {
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindNamedExports:
			c.ForEachChild(func(spec *wrapperchecker.Node) bool {
				name := exportSpecifierName(spec)
				if name == "default" {
					*hasDefault = true
				} else if name != "" {
					named[name] = true
				}
				return false
			})
		}
		return false
	})
}

// exportSpecifierName returns the *exported* name of an export
// specifier — the part after `as` when present, otherwise the
// only identifier. `export { a as b }` exports `b`, not `a`.
func exportSpecifierName(spec *wrapperchecker.Node) string {
	var names []string
	spec.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			names = append(names, c.LiteralText())
		}
		return false
	})
	if len(names) == 0 {
		return ""
	}
	return names[len(names)-1]
}

func hasExportModifier(stmt *wrapperchecker.Node) bool {
	var found bool
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindExportKeyword {
			found = true
			return true
		}
		return false
	})
	return found
}

func hasDefaultModifier(stmt *wrapperchecker.Node) bool {
	var found bool
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindDefaultKeyword {
			found = true
			return true
		}
		return false
	})
	return found
}

func declarationIdentifier(decl *wrapperchecker.Node) string {
	var name string
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func collectVariableNames(stmt *wrapperchecker.Node, named map[string]bool) {
	stmt.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindVariableDeclarationList {
			return false
		}
		c.ForEachChild(func(decl *wrapperchecker.Node) bool {
			if decl.Kind() != wrapperchecker.KindVariableDeclaration {
				return false
			}
			binding := decl.VariableDeclarationName()
			if binding == nil {
				return false
			}
			if binding.Kind() == wrapperchecker.KindIdentifier {
				named[binding.LiteralText()] = true
			}
			return false
		})
		return false
	})
}

// IsRelativeSpecifier reports whether the import specifier text
// looks like a relative path (`./x`, `../x`). Non-relative
// specifiers are package names or path aliases and need
// node_modules / tsconfig.paths to resolve.
func IsRelativeSpecifier(specifier string) bool {
	return strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") || specifier == "." || specifier == ".."
}
