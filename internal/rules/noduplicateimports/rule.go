// Package noduplicateimports implements the no-duplicate-imports
// rule: every module should be imported in a single statement, since
// scattering imports of the same source across multiple statements
// adds churn without adding meaning. Two imports of the same module
// are flagged when their bindings could be combined into one
// statement; the only exception is a namespace + named pair, which
// cannot be expressed in a single `import` syntactically.
package noduplicateimports

import (
	"strings"

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
	// hasAnonymousNs is true for `export * from "mod"` (no alias).
	// This re-export form has a unique combinability profile: it
	// only matches another anonymous `export * from` of the same
	// source.
	hasAnonymousNs bool
	isTypeOnly     bool
	isExport       bool
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

// isSideEffectOnly reports whether the entry carries no bindings —
// i.e. `import "mod"` with no clause. Such a statement is fully
// covered by any other statement of the same source.
func isSideEffectOnly(e entry) bool {
	return !e.hasDefault && !e.hasNamed && !e.hasNs && !e.hasAnonymousNs
}

// canCombine reports whether two import (or import+export) statements
// of the same source are semantic duplicates that could be folded
// into a single statement. JavaScript syntax allows {} (side-effect),
// {default}, {named}, {namespace}, {default, named}, {default,
// namespace}; the forbidden combo is `named + namespace`.
//
// `export *` is a special case — it can only "combine" with another
// `export *`, since the wildcard re-export shape can't merge with a
// value import or a named export.
//
// Two type-only statements that mix default and named bindings are
// also kept separate: oxc's port treats `import type X` and
// `import type { Y }` as distinct shapes that aren't combined by
// the rule (matching ESLint behaviour even where syntax permits it).
func canCombine(a, b entry) bool {
	// A side-effect-only import (`import "mod"`) carries no bindings
	// and is fully subsumed by any other statement of the same
	// source.
	if isSideEffectOnly(a) || isSideEffectOnly(b) {
		return true
	}
	// Anonymous `export * from "mod"` only matches another anonymous
	// `export *` — it's a wildcard re-export with no shared shape
	// with imports or named exports.
	if a.hasAnonymousNs != b.hasAnonymousNs {
		return false
	}
	if a.hasAnonymousNs && b.hasAnonymousNs {
		return true
	}
	if a.isTypeOnly && b.isTypeOnly {
		if (a.hasDefault && b.hasNamed && !b.hasDefault) ||
			(a.hasNamed && !a.hasDefault && b.hasDefault) {
			return false
		}
	}
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
	e := entry{node: n, source: spec.LiteralText(), isTypeOnly: importIsTypeOnly(n)}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindImportClause {
			return false
		}
		categorize(c, &e)
		return true
	})
	return e, true
}

// importIsTypeOnly reports whether an ImportDeclaration carries the
// type-only marker (`import type ...` or `import type { ... }`).
// Detected via SourceText prefix because the wrapper does not
// expose ImportClause.IsTypeOnly as an accessor.
func importIsTypeOnly(n *wrapperchecker.Node) bool {
	text := strings.TrimLeft(n.SourceText(), " \t")
	return strings.HasPrefix(text, "import type")
}

// exportEntry inspects an ExportDeclaration with a `from "module"`
// re-export and returns its source-string + binding flags.
func exportEntry(n *wrapperchecker.Node) (entry, bool) {
	spec := n.ModuleSpecifier()
	if spec == nil || spec.Kind() != wrapperchecker.KindStringLiteral {
		return entry{}, false
	}
	e := entry{node: n, source: spec.LiteralText(), isTypeOnly: n.IsTypeOnlyExport(), isExport: true}
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
	// `export * from "mod"` produces an ExportDeclaration with no
	// ExportClause — neither NamedExports nor NamespaceExport
	// children. Tag it as an anonymous namespace re-export.
	if !e.hasNs && !e.hasNamed {
		e.hasAnonymousNs = true
	}
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
