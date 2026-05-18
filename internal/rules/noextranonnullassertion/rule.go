// Package noextranonnullassertion implements
// no-extra-non-null-assertion: `x!!` and `x!?.y` are no-ops on the
// outer `!`. They survive refactors that swap the inner expression
// for something already non-nullable, leaving misleading punctuation.
package noextranonnullassertion

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-extra-non-null-assertion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNonNullExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Inner already-non-null assertion: `x!!`
	inner := nonNullOperand(n)
	if inner != nil && inner.Kind() == wrapperchecker.KindNonNullExpression {
		ctx.Report(n, "redundant `!` — the operand is already a non-null assertion")
		return
	}
	// Followed by optional chain: `x!?.y` or `x!?.()`
	p := n.Parent()
	if p == nil {
		return
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyAccessExpression, wrapperchecker.KindCallExpression, wrapperchecker.KindElementAccessExpression:
		// Optional chain shows up as `?.` in the source between the
		// operand and the access.
		if _, rest, ok := strings.Cut(p.SourceText(), n.SourceText()); ok && strings.HasPrefix(rest, "?.") {
			ctx.Report(n, "`!?.` is redundant — drop the `!` or the `?.`")
		}
	}
}

func nonNullOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first
}
