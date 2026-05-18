// Package nounassignedvariables implements the no-unassigned-variables
// rule: a `let` or `var` declaration with no initializer that is read
// somewhere in scope but never written is almost certainly a bug — the
// reader sees `undefined`. Initialise the declaration, assign a value
// before the first read, or delete the binding entirely.
//
// Declarations skipped:
//   - `const` and `using`/`await using` bindings (already immutable).
//   - Declarations with an initializer (any expression, including
//     `undefined`).
//   - Destructuring patterns — the initializer covers them.
//   - For-in / for-of bindings — the iteration writes them.
//   - Ambient declarations: `declare let x`, plus anything nested
//     inside `declare module {}` or another ambient context.
//
// A binding is also accepted as soon as any reference in any nested
// scope writes to it — including via `x = ...`, compound assignment,
// `x++` / `x--`, and `for (x of ...)` / `for (x in ...)` where the
// LHS is the bare identifier.
//
// A binding declared but never read is left alone — `no-unused-vars`
// owns that case, matching biome's behaviour.
package nounassignedvariables

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unassigned-variables"

// New constructs the rule with default options. The rule has no
// configurable options today.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: r.visit,
	}
}

type trackedSym struct {
	declID   *wrapperchecker.Node
	hasRead  bool
	hasWrite bool
}

func (r *rule) visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	chk := ctx.Checker()
	candidates := map[uintptr]*trackedSym{}

	walkAll(sf, func(n *wrapperchecker.Node) {
		if n.Kind() != wrapperchecker.KindVariableDeclaration {
			return
		}
		if n.IsConstVariableDeclaration() {
			return
		}
		list := n.Parent()
		if list != nil && (list.IsUsingDeclaration() || list.IsAwaitUsingDeclaration()) {
			return
		}
		if n.VariableDeclarationInitializer() != nil {
			return
		}
		name := n.VariableDeclarationName()
		if name == nil || name.Kind() != wrapperchecker.KindIdentifier {
			return
		}
		if isForBinding(list) {
			return
		}
		if isInAmbientContext(n) {
			return
		}
		sym := chk.SymbolOf(name)
		if sym == nil {
			return
		}
		candidates[sym.ID()] = &trackedSym{declID: name}
	})

	if len(candidates) == 0 {
		return
	}

	walkAll(sf, func(n *wrapperchecker.Node) {
		if n.Kind() != wrapperchecker.KindIdentifier {
			return
		}
		sym := chk.SymbolOf(n)
		if sym == nil {
			return
		}
		ts, ok := candidates[sym.ID()]
		if !ok {
			return
		}
		if n.Pos() == ts.declID.Pos() && n.End() == ts.declID.End() {
			return
		}
		if n.IsAssignmentTarget() {
			ts.hasWrite = true
		} else {
			ts.hasRead = true
		}
	})

	for _, ts := range candidates {
		if ts.hasRead && !ts.hasWrite {
			ctx.Report(ts.declID, "This variable is declared but never assigned. Either assign a value, initialize it at the declaration, or remove the binding.")
		}
	}
}

func walkAll(n *wrapperchecker.Node, visit func(*wrapperchecker.Node)) {
	if n == nil {
		return
	}
	visit(n)
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walkAll(c, visit)
		return false
	})
}

func isForBinding(list *wrapperchecker.Node) bool {
	if list == nil {
		return false
	}
	parent := list.Parent()
	if parent == nil {
		return false
	}
	switch parent.Kind() {
	case wrapperchecker.KindForInStatement, wrapperchecker.KindForOfStatement:
		return true
	}
	return false
}

// isInAmbientContext reports whether any ancestor (up to the source
// file) carries the `declare` modifier. Catches both
// `declare let x;` (modifier on the VariableStatement) and
// `declare module "m" { let x; }` (modifier on the ModuleDeclaration).
func isInAmbientContext(n *wrapperchecker.Node) bool {
	for cur := n; cur != nil; cur = cur.Parent() {
		if cur.Kind() == wrapperchecker.KindSourceFile {
			return false
		}
		if cur.HasDeclareModifier() {
			return true
		}
	}
	return false
}
