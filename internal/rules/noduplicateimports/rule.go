// Package noduplicateimports implements the no-duplicate-imports
// rule: every module should be imported in a single statement, since
// scattering imports of the same source across multiple statements
// adds churn without adding meaning. Two imports of the same module
// are flagged when their bindings could be combined into one
// statement; the only exception is a namespace + named pair, which
// cannot be expressed in a single `import` syntactically.
package noduplicateimports

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-duplicate-imports"

// Options configures the rule.
type Options struct {
	// IncludeExports also checks `export ... from "module"` statements.
	IncludeExports bool
	// AllowSeparateTypeImports treats `import type` and value imports
	// as separate.
	AllowSeparateTypeImports bool
}

// New constructs a rule with default options.
func New() engine.Rule { return &rule{} }

// NewWithOptions constructs a rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: r.visit,
	}
}

type entry struct {
	node       *wrapperchecker.Node
	source     string
	hasDefault bool
	hasNamed   bool
	hasNs      bool
	isTypeOnly bool
}

func (r *rule) visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	var entries []entry
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindImportDeclaration:
			e, ok := importEntry(c)
			if ok {
				entries = append(entries, e)
			}
		case wrapperchecker.KindExportDeclaration:
			if r.opts.IncludeExports {
				e, ok := exportEntry(c)
				if ok {
					entries = append(entries, e)
				}
			}
		}
		return false
	})
	for i := 1; i < len(entries); i++ {
		cur := entries[i]
		for j := 0; j < i; j++ {
			prev := entries[j]
			if prev.source != cur.source {
				continue
			}
			if r.opts.AllowSeparateTypeImports && prev.isTypeOnly != cur.isTypeOnly {
				continue
			}
			if canCombine(prev, cur) {
				ctx.Report(cur.node, "'"+cur.source+"' import is duplicated.")
				break
			}
		}
	}
}

// canCombine reports whether two import (or import+export) statements
// of the same source could be expressed as a single statement.
// JavaScript syntax allows {} (side-effect), {default}, {named},
// {namespace}, {default, named}, {default, namespace}; the only
// forbidden combo is `named + namespace` in the same statement.
func canCombine(a, b entry) bool {
	hasNamed := a.hasNamed || b.hasNamed
	hasNs := a.hasNs || b.hasNs
	if hasNamed && hasNs {
		return false
	}
	return true
}

// importEntry inspects an ImportDeclaration and returns its
// source-string + binding flags.
func importEntry(n *wrapperchecker.Node) (entry, bool) {
	spec := n.ModuleSpecifier()
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return entry{}, false
	}
	e := entry{node: n, source: spec.LiteralText()}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		categorize(c, &e)
		return true
	})
	return e, true
}

// exportEntry inspects an ExportDeclaration with a `from "module"`
// re-export and returns its source-string + binding flags.
func exportEntry(n *wrapperchecker.Node) (entry, bool) {
	spec := n.ModuleSpecifier()
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return entry{}, false
	}
	e := entry{node: n, source: spec.LiteralText()}
	// For exports, classify by the export form: `export *` is
	// namespace-style; `export { … }` is named-style.
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindNamespaceExport:
			e.hasNs = true
		case wrapperchecker.KindNamedExports:
			e.hasNamed = true
		}
		return false
	})
	return e, true
}

// categorize fills the binding-flag fields of `e` by walking the
// ImportClause's children.
func categorize(clause *wrapperchecker.Node, e *entry) {
	clause.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			e.hasDefault = true
		case wrapperchecker.KindNamespaceImport:
			e.hasNs = true
		case wrapperchecker.KindNamedImports:
			e.hasNamed = true
		}
		return false
	})
}
