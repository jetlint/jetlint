// Package nomeaninglessvoidoperator implements the no-meaningless-void-operator
// rule: flag `void X` where X already has type void or never.
package nomeaninglessvoidoperator

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-meaningless-void-operator"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVoidExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	t := ctx.TypeOf(operand)
	if t == nil {
		return
	}
	if t.IsVoid() || t.IsNever() {
		ctx.Report(n, "void of a value already typed void/never is redundant")
	}
}
