// Package noexassign implements the no-ex-assign rule: a catch
// clause's exception binding is the only handle on the caught value;
// reassigning it permanently loses access to the error and is almost
// always a mistake.
package noexassign

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-ex-assign"

// New constructs a noexassign rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.IsAssignmentTarget() {
		return
	}
	var sym *wrapperchecker.Symbol
	if parent := n.Parent(); parent != nil && parent.Kind() == wrapperchecker.KindShorthandPropertyAssignment {
		sym = ctx.Checker().ShorthandAssignmentValueSymbol(parent)
	} else {
		sym = ctx.Checker().SymbolOf(n)
	}
	if sym == nil {
		return
	}
	if !symbolIsCatchBinding(sym) {
		return
	}
	ctx.Report(n, fmt.Sprintf("do not assign to '%s' (catch clause exception binding)", n.LiteralText()))
}

// symbolIsCatchBinding reports whether the symbol was introduced by a
// catch clause. The declaration is a VariableDeclaration whose nearest
// statement-context parent is a CatchClause; for destructured catch
// parameters (`catch ({e}) {…}`) the declaration is the BindingElement
// inside an ObjectBindingPattern/ArrayBindingPattern, so we walk up
// through binding-pattern wrappers until we hit the VariableDeclaration.
func symbolIsCatchBinding(sym *wrapperchecker.Symbol) bool {
	for _, decl := range sym.Declarations() {
		if originatesInCatchClause(decl) {
			return true
		}
	}
	return false
}

func originatesInCatchClause(decl *wrapperchecker.Node) bool {
	for n := decl; n != nil; n = n.Parent() {
		switch n.Kind() {
		case wrapperchecker.KindVariableDeclaration:
			if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindCatchClause {
				return true
			}
			return false
		case wrapperchecker.KindBindingElement,
			wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			continue
		default:
			return false
		}
	}
	return false
}
