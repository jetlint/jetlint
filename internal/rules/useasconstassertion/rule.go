// Package useasconstassertion implements use-as-const-assertion:
// `x as Const` reads as a custom type. `as const` (the language
// feature) gives the literal-narrowing behavior most people want.
package useasconstassertion

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-as-const-assertion"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindAsExpression:     visit,
		wrapperchecker.KindTypeAssertionExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Find the type side. For AsExpression the type is the second child.
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return
	}
	if second.Kind() != wrapperchecker.KindLiteralType {
		return
	}
	// Look inside the LiteralType for a literal that's used as a manual
	// narrowing (e.g. `as 0`, `as "foo"`). Whether the parent literal
	// matches the operand is the trigger — we conservatively flag when
	// the source side is a plain literal of the same kind.
	var inner *wrapperchecker.Node
	second.ForEachChild(func(c *wrapperchecker.Node) bool {
		if inner == nil {
			inner = c
		}
		return false
	})
	if inner == nil {
		return
	}
	if first == nil {
		return
	}
	if first.Kind() != inner.Kind() {
		return
	}
	if first.SourceText() != inner.SourceText() {
		return
	}
	ctx.Report(n, "use `as const` for literal narrowing")
}
