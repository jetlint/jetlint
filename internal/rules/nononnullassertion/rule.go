// Package nononnullassertion implements no-non-null-assertion: the
// `!` postfix tells the type checker "trust me, it's not null" — and
// runtime doesn't care. A real null check or `??` is safer.
package nononnullassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-non-null-assertion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNonNullExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	ctx.Report(n, "non-null assertion (`!`) lies to the type checker — use a real null check")
}
