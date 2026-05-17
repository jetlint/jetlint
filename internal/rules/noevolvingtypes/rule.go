// Package noevolvingtypes implements the no-evolving-types rule:
// reject `let`/`var`/`const` declarations whose type would evolve from
// `any` through later assignments because the declaration carries
// neither a type annotation nor an unambiguous initializer.
//
// A declaration evolves when TypeScript cannot fix a type at the
// declaration site:
//   - no initializer at all (`let a;`, `var a;`)
//   - the `null` literal as initializer (`let a = null;`)
//   - the empty array literal as initializer (`const a = [];`)
//
// An explicit type annotation always rescues the declaration, even when
// the initializer is otherwise ambiguous (`let a: number | null = null;`).
// For-in and for-of binding declarations are out of scope — the
// iterable supplies the binding type.
package noevolvingtypes

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-evolving-types"

// New constructs the rule with default options. The rule has no
// configurable options today.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if isForBinding(n.Parent()) {
		return
	}
	if isBindingPattern(n.VariableDeclarationName()) {
		return
	}
	if n.VariableDeclarationType() != nil {
		return
	}
	init := n.VariableDeclarationInitializer()
	if !isAmbiguousInitializer(init) {
		return
	}
	ctx.Report(n, "Variable has no explicit type and no initializer of a fixable type; its inferred type evolves through assignments.")
}

// isForBinding reports whether the variable declaration's containing
// list is the iteration variable of a for-in or for-of statement. The
// iterable narrows the binding's type, so these declarations cannot
// evolve.
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

// isBindingPattern reports whether the declaration's binding target is
// an object or array destructuring pattern. Destructuring assigns
// element-by-element from the initializer, so the inferred per-binding
// type does not evolve.
func isBindingPattern(name *wrapperchecker.Node) bool {
	if name == nil {
		return false
	}
	switch name.Kind() {
	case wrapperchecker.KindObjectBindingPattern,
		wrapperchecker.KindArrayBindingPattern:
		return true
	}
	return false
}

// isAmbiguousInitializer reports whether the initializer (if any) is
// one of the shapes that leaves TypeScript inferring an evolving type:
// missing, `null`, or an empty array literal.
func isAmbiguousInitializer(init *wrapperchecker.Node) bool {
	if init == nil {
		return true
	}
	switch init.Kind() {
	case wrapperchecker.KindNullKeyword:
		return true
	case wrapperchecker.KindArrayLiteralExpression:
		return len(init.ArrayElements()) == 0
	}
	return false
}
