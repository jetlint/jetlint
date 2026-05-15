// Package nodebugger implements the no-debugger rule: `debugger;` is
// a development-time hook for pausing in an attached debugger and
// should not be shipped to production. The rule simply reports every
// DebuggerStatement.
package nodebugger

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-debugger"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindDebuggerStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	ctx.Report(n, "Unexpected `debugger` statement.")
}
