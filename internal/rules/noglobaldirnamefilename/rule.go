// Package noglobaldirnamefilename implements no-global-dirname-filename:
// flag references to `__dirname` and `__filename`, the CommonJS
// path globals. ESM modules don't have them — code that mixes ESM
// syntax with these identifiers either bundles a polyfill or crashes
// at module load. The ESM-native replacement is `import.meta.dirname`
// / `import.meta.filename`.
//
// Only bare identifier reads count. An identifier appearing as a
// property name in an object literal (`{ __dirname: foo }`) is just
// a key string and is not a reference to the global.
package noglobaldirnamefilename

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-global-dirname-filename"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	name := n.LiteralText()
	if name != "__dirname" && name != "__filename" {
		return
	}
	if isNonReferenceIdentifier(n) {
		return
	}
	if nameIsBoundInFile(n, name) {
		return
	}
	ctx.Report(n, "use `import.meta.dirname`/`import.meta.filename` — `"+name+"` is a CommonJS-only global")
}

// nameIsBoundInFile walks the source file looking for any binding
// named `name`. If found, the references in this file are local,
// not the CommonJS global. Conservative — flags only when no
// binding exists anywhere.
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
		wrapperchecker.KindClassDeclaration:
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

// isNonReferenceIdentifier reports whether n is an identifier in a
// position that doesn't reference a value: a property name on an
// object literal, the right-hand side of a member access, an
// import/export specifier name, etc. In those spots the identifier
// text is metadata, not a read of the surrounding scope's binding.
func isNonReferenceIdentifier(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		// `foo.__dirname` — the name on the right is a member
		// key, not a global read. PropertyAccessReceiver gives the
		// LHS; anything else must be the name.
		recv := p.PropertyAccessReceiver()
		return recv == nil || !sameNode(recv, n)
	case wrapperchecker.KindPropertyAssignment:
		// `{ __dirname: ... }` — the key is the first child. The
		// initializer expression comes second.
		init := p.PropertyInitializer()
		// If n is the initializer (or inside it via a deeper
		// expression), it's a value reference; otherwise it's the
		// key.
		return init == nil || !sameNode(init, n)
	case wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindExportSpecifier:
		// `import { __filename as filename }` / `export { foo as __dirname }`
		// — the identifier is the property name on the imported
		// module's namespace, not a reference to the local global.
		return true
	}
	return false
}

// sameNode reports whether two wrapper Node pointers describe the
// same AST node. Pointer equality doesn't work because the wrapper
// allocates a new Node struct on every Parent/child access; we
// compare span positions instead, which uniquely identify a node
// inside a single source file.
func sameNode(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Pos() == b.Pos() && a.End() == b.End()
}
