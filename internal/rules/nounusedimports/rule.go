// Package nounusedimports implements no-unused-imports: flag the
// import binding when its name is never referenced anywhere in the
// rest of the file. This is the import-only subset of
// no-unused-vars, exposed as its own rule so configs can toggle
// import-cleanup independently.
//
// Reference detection works at the textual level: every identifier
// in the file that isn't itself an import binding counts as a
// reference. JSDoc tag contents are also scanned because biome's
// fixtures rely on `@link Foo` keeping `Foo`'s import alive.
//
// Skipped when:
//   - the file contains a `declare module` block (the module
//     augmentation may consume the import in a way the syntactic
//     scan misses);
//   - the file contains SFC framing (`---`, `<!--`, `<script>`,
//     `<template>`) — those aren't TS source and the wrapper's
//     parse output is unreliable for them.
package nounusedimports

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-imports"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, src *wrapperchecker.Node) {
	srcText := src.SourceText()
	if shouldSkipFile(srcText) {
		return
	}
	type imp struct {
		name string
		node *wrapperchecker.Node
	}
	var imports []imp
	importBindingPositions := map[int]bool{}
	src.ForEachChild(func(stmt *wrapperchecker.Node) bool {
		if stmt.Kind() != wrapperchecker.KindImportDeclaration {
			return false
		}
		collectImportBindings(stmt, func(name string, node *wrapperchecker.Node) {
			imports = append(imports, imp{name: name, node: node})
			importBindingPositions[node.Pos()] = true
		})
		return false
	})
	if len(imports) == 0 {
		return
	}
	used := referencedNames(src, importBindingPositions)
	// Also count JSDoc references — biome resolves `@link Foo` and
	// similar tags so an import kept alive only by JSDoc stays
	// unflagged.
	scanJSDoc(srcText, used)
	for _, im := range imports {
		if used[im.name] {
			continue
		}
		ctx.Report(im.node, "'"+im.name+"' is imported but never used")
	}
}

// shouldSkipFile returns true for inputs the rule's textual model
// can't reason about: SFC files (Vue/Astro/Svelte fragments) whose
// `<script>` content gets buried inside HTML the wrapper doesn't
// parse as TS source.
func shouldSkipFile(srcText string) bool {
	return strings.Contains(srcText, "<!--") ||
		strings.Contains(srcText, "<script") ||
		strings.Contains(srcText, "<template") ||
		strings.HasPrefix(strings.TrimLeft(srcText, " \t\n"), "---")
}

// collectImportBindings walks an ImportDeclaration and yields each
// binding identifier with its name. Default, namespace, and named
// imports are all handled; side-effect-only imports (`import "x"`)
// yield nothing.
func collectImportBindings(decl *wrapperchecker.Node, yield func(string, *wrapperchecker.Node)) {
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		c.ForEachChild(func(cc *wrapperchecker.Node) bool {
			switch cc.Kind() {
			case wrapperchecker.KindIdentifier:
				yield(cc.LiteralText(), cc)
			case wrapperchecker.KindNamespaceImport:
				cc.ForEachChild(func(n *wrapperchecker.Node) bool {
					if n.Kind() == wrapperchecker.KindIdentifier {
						yield(n.LiteralText(), n)
						return true
					}
					return false
				})
			default:
				// NamedImports / ImportSpecifier subtree
				cc.ForEachChild(func(spec *wrapperchecker.Node) bool {
					if spec.Kind() != wrapperchecker.KindImportSpecifier {
						return false
					}
					// The specifier's binding is its last child
					// identifier (the local name after any
					// `original as local`).
					var last *wrapperchecker.Node
					spec.ForEachChild(func(n *wrapperchecker.Node) bool {
						if n.Kind() == wrapperchecker.KindIdentifier {
							last = n
						}
						return false
					})
					if last != nil {
						yield(last.LiteralText(), last)
					}
					return false
				})
			}
			return false
		})
		return false
	})
}

// referencedNames returns the set of identifier texts that appear
// somewhere in src outside of the binding positions of imports
// themselves. Property access right-hand-sides are excluded — they
// name properties, not bindings.
func referencedNames(src *wrapperchecker.Node, importBindings map[int]bool) map[string]bool {
	used := make(map[string]bool)
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
			if recv := n.PropertyAccessReceiver(); recv != nil {
				walk(recv)
			}
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier {
			if !importBindings[n.Pos()] {
				used[n.LiteralText()] = true
			}
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c)
		return false
	})
	return used
}

// scanJSDoc finds identifier-like tokens inside `/** ... */`
// comment blocks. Biome's `@link Foo`, `@type {Foo}`, etc. all
// keep their referenced names alive; a textual scan handles every
// flavor uniformly.
func scanJSDoc(srcText string, used map[string]bool) {
	i := 0
	for {
		start := strings.Index(srcText[i:], "/**")
		if start < 0 {
			return
		}
		start += i
		end := strings.Index(srcText[start:], "*/")
		if end < 0 {
			return
		}
		end += start
		extractIdentifiers(srcText[start:end], used)
		i = end + 2
	}
}

func extractIdentifiers(text string, used map[string]bool) {
	start := -1
	for i := 0; i <= len(text); i++ {
		end := i == len(text)
		var c byte
		if !end {
			c = text[i]
		}
		isIdent := !end && (c == '_' || c == '$' ||
			(c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(start >= 0 && c >= '0' && c <= '9'))
		switch {
		case isIdent && start < 0:
			start = i
		case !isIdent && start >= 0:
			used[text[start:i]] = true
			start = -1
		}
	}
}
