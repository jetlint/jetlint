// Package noprocessglobal implements no-process-global: flag bare
// `process` identifier references. `process` is a Node.js host
// global; under strict ESM (or Deno/Bun) modules it's not available
// unless explicitly imported via `import process from "node:process"`.
// Forcing the import documents the Node dependency and lets bundlers
// see it.
//
// Skipped when the file binds a local `process` (import, function
// declaration, variable, parameter) — the reference resolves to the
// user binding, not the global.
package noprocessglobal

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-process-global"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.LiteralText() != "process" {
		return
	}
	if isNonReferenceIdentifier(n) {
		return
	}
	if nameIsBoundInFile(n, "process") {
		return
	}
	ctx.Report(n, "import `process` from `node:process` instead of using the global")
}

func isNonReferenceIdentifier(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		recv := p.PropertyAccessReceiver()
		return recv == nil || !sameNode(recv, n)
	case wrapperchecker.KindPropertyAssignment:
		init := p.PropertyInitializer()
		return init == nil || !sameNode(init, n)
	case wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindExportSpecifier:
		return true
	}
	return false
}

func sameNode(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Pos() == b.Pos() && a.End() == b.End()
}

func nameIsBoundInFile(ref *wrapperchecker.Node, name string) bool {
	root := ref
	for {
		p := root.Parent()
		if p == nil {
			break
		}
		root = p
	}
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if found {
			return true
		}
		if isBindingWithName(c, name) {
			found = true
			return true
		}
		c.ForEachChild(walk)
		return found
	}
	root.ForEachChild(walk)
	return found
}

func isBindingWithName(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindNamespaceImport:
		return bindingName(n) == name
	}
	return false
}

func bindingName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
