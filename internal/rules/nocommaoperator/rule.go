// Package nocommaoperator implements no-comma-operator: the comma
// operator chains expressions in ways that almost always read
// accidentally. The well-known carve-outs are for/for-in headers.
package nocommaoperator

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-comma-operator"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.BinaryOperatorKind() != wrapperchecker.KindCommaToken {
		return
	}
	// Skip when this is the init or update of a for-statement (but flag the condition).
	p := n.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p != nil && p.Kind() == wrapperchecker.KindForStatement {
		cond := p.ForStatementCondition()
		// If we're inside the condition, fall through and report. Otherwise skip.
		if cond == nil || n.Pos() < cond.Pos() || n.End() > cond.End() {
			return
		}
	}
	// Don't double-report when comma op is nested inside another comma op.
	if p != nil && p.Kind() == wrapperchecker.KindBinaryExpression && p.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
		return
	}
	ctx.Report(n, "comma operator chains expressions implicitly — split into statements")
}
