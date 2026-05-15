// Package noselfassign implements the no-self-assign rule: flag
// assignments whose right-hand side is textually identical to the
// left-hand side (`a = a`, `obj.foo = obj.foo`). The assignment is a
// no-op and almost always a refactoring leftover or a typo.
//
// Scope: simple `=` only. Compound assignments (`+=`, `||=`, `??=`,
// etc.) have side effects on the value being assigned and are not
// no-ops even with identical operands, so they are not flagged.
// Destructuring self-assignments are out of scope for v1 (the
// straight-line `a = a` case is by far the common one). Operand
// equality is measured by [Node.SourceText] — same idea as
// no-self-compare.
package noselfassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-self-assign"

// New constructs a noselfassign rule instance ready for registration
// with the engine.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.BinaryOperatorKind() != wrapperchecker.KindEqualsToken {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftText := left.SourceText()
	if leftText == "" || leftText != right.SourceText() {
		return
	}
	ctx.Report(n, "assigning a variable to itself is a no-op")
}
