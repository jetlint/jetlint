// Package noclassassign implements the no-class-assign rule: a class
// declaration introduces a binding that JavaScript permits re-assigning
// even though the binding is meant to name the class. Re-assigning it
// almost always indicates a mistake — either accidental shadowing or a
// variable named after a class that should have been declared separately.
package noclassassign

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-class-assign"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.IsAssignmentTarget() {
		return
	}
	// Shorthand destructure targets (`{ A } = …`) need the value-binding
	// helper: the identifier itself looks up the property symbol, not the
	// outer binding the assignment ultimately writes to.
	var sym *wrapperchecker.Symbol
	if parent := n.Parent(); parent != nil && parent.Kind() == wrapperchecker.KindShorthandPropertyAssignment {
		sym = ctx.Checker().ShorthandAssignmentValueSymbol(parent)
	} else {
		sym = ctx.Checker().SymbolOf(n)
	}
	if sym == nil {
		return
	}
	if !symbolIsClass(sym) {
		return
	}
	ctx.Report(n, fmt.Sprintf("'%s' is a class.", n.LiteralText()))
}

func symbolIsClass(sym *wrapperchecker.Symbol) bool {
	for _, decl := range sym.Declarations() {
		switch decl.Kind() {
		case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			return true
		}
	}
	return false
}
