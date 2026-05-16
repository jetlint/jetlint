// Package nounusedvars implements the no-unused-vars rule: flag
// declarations that are introduced and then never read. The rule
// fires once per source file, collecting every name introduced by a
// VariableDeclaration / FunctionDeclaration / ClassDeclaration /
// ImportClause / ImportSpecifier / NamespaceImport, plus every
// function-like Parameter — and then walks the rest of the file
// looking for identifier reads that reference back to one of those
// names.
//
// The port is intentionally conservative and string-equality based.
// It does not honor option modifiers (varsIgnorePattern,
// argsIgnorePattern, caughtErrors, ignoreRestSiblings); names that
// start with `_` are treated as intentionally-unused. Names whose
// declaration is exported (top-level `export` / `export default`)
// are excluded because the export itself is the consumer.
package nounusedvars

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-vars"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: r.visit,
	}
}

type binding struct {
	name string
	node *wrapperchecker.Node
	kind string // "var" | "function" | "class" | "import" | "param" | "type"
}

func (r *rule) visit(ctx *engine.Context, src *wrapperchecker.Node) {
	bindings := []binding{}
	references := map[string]bool{}
	exported := map[string]bool{}

	// First pass: collect all declarations and exported names. Use
	// a hand-rolled walker so we can distinguish between "this
	// identifier introduces a name" and "this identifier reads a
	// value".
	var collect func(n *wrapperchecker.Node, inExport bool)
	collect = func(n *wrapperchecker.Node, inExport bool) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindExportDeclaration,
			wrapperchecker.KindExportAssignment:
			// Anything inside an export statement is a consumer of
			// names — recurse with a flag so referenced identifiers
			// are recorded as exported.
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				collect(c, true)
				return false
			})
			return
		case wrapperchecker.KindVariableStatement:
			isExp := inExport || hasExportModifier(n)
			collectVariableStatement(n, isExp, &bindings, exported)
			return
		case wrapperchecker.KindFunctionDeclaration:
			isExp := inExport || hasExportModifier(n)
			if name := declarationIdentifier(n); name != nil {
				if isExp {
					exported[name.SourceText()] = true
				} else {
					bindings = append(bindings, binding{
						name: name.SourceText(),
						node: name,
						kind: "function",
					})
				}
			}
			// Don't recurse into the function body for declaration
			// collection — inner statements are referenced
			// reactively in the second pass.
		case wrapperchecker.KindClassDeclaration:
			isExp := inExport || hasExportModifier(n)
			if name := declarationIdentifier(n); name != nil {
				if isExp {
					exported[name.SourceText()] = true
				} else {
					bindings = append(bindings, binding{
						name: name.SourceText(),
						node: name,
						kind: "class",
					})
				}
			}
		case wrapperchecker.KindImportSpecifier,
			wrapperchecker.KindNamespaceImport:
			if name := declarationIdentifier(n); name != nil {
				bindings = append(bindings, binding{
					name: name.SourceText(),
					node: name,
					kind: "import",
				})
			}
			return
		case wrapperchecker.KindImportClause:
			// Default import: the clause's own name child is the
			// default binding; namespace/named imports are nested
			// further and handled by recursion.
			if name := declarationIdentifier(n); name != nil {
				bindings = append(bindings, binding{
					name: name.SourceText(),
					node: name,
					kind: "import",
				})
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			collect(c, inExport)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		collect(c, false)
		return false
	})

	// Second pass: walk every identifier in a *reference* position
	// (i.e. not the declaration site) and record its text. The
	// declaration-site set is keyed by position because the wrapper
	// creates fresh *Node values for each traversal — pointer
	// equality would never match.
	declSet := map[int]bool{}
	for _, b := range bindings {
		declSet[b.node.Pos()] = true
	}
	var visit func(n *wrapperchecker.Node)
	visit = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
			// Only the receiver is a value reference; the property
			// name on the RHS is not.
			visit(n.PropertyAccessReceiver())
			return
		}
		if n.Kind() == wrapperchecker.KindIdentifier && !declSet[n.Pos()] {
			references[n.SourceText()] = true
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			visit(c)
			return false
		})
	}
	src.ForEachChild(func(c *wrapperchecker.Node) bool {
		visit(c)
		return false
	})

	// Report bindings that are never referenced, never exported,
	// and don't start with an intentional underscore.
	for _, b := range bindings {
		if strings.HasPrefix(b.name, "_") {
			continue
		}
		if exported[b.name] {
			continue
		}
		if references[b.name] {
			continue
		}
		ctx.Report(b.node, "'"+b.name+"' is declared but never used.")
	}
}

// collectVariableStatement walks a VariableStatement, recording each
// declared name as an unused-vars binding (or an export, when the
// statement carries an `export` modifier).
func collectVariableStatement(stmt *wrapperchecker.Node, inExport bool, bindings *[]binding, exported map[string]bool) {
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		if n.Kind() == wrapperchecker.KindVariableDeclaration {
			if name := n.VariableDeclarationName(); name != nil {
				addBindingNames(name, "var", inExport, bindings, exported)
			}
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(stmt)
}

// addBindingNames walks the binding target of a VariableDeclaration
// (which may be an identifier or a destructuring pattern) and
// records each introduced identifier.
func addBindingNames(target *wrapperchecker.Node, kind string, inExport bool, bindings *[]binding, exported map[string]bool) {
	if target == nil {
		return
	}
	if target.Kind() == wrapperchecker.KindIdentifier {
		if inExport {
			exported[target.SourceText()] = true
		} else {
			*bindings = append(*bindings, binding{
				name: target.SourceText(),
				node: target,
				kind: kind,
			})
		}
		return
	}
	target.ForEachChild(func(c *wrapperchecker.Node) bool {
		addBindingNames(c, kind, inExport, bindings, exported)
		return false
	})
}

// declarationIdentifier returns the first identifier child of a
// declaration node. Used for function/class/import-clause names.
func declarationIdentifier(n *wrapperchecker.Node) *wrapperchecker.Node {
	var name *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c
			return true
		}
		return false
	})
	return name
}

// hasExportModifier reports whether n is a declaration carrying an
// `export` modifier (i.e. `export const x = ...`, `export function f`).
// We detect it via SourceText prefix, which is sufficient for the
// rule's purposes without needing wrapper-API modifier accessors.
func hasExportModifier(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindVariableStatement,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindEnumDeclaration:
		text := n.SourceText()
		return strings.HasPrefix(strings.TrimLeft(text, " \t"), "export")
	}
	return false
}
