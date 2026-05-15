// Package nosparsearrays implements the no-sparse-arrays rule: array
// literals with elided slots — `[1, , 2]` — produce holes rather than
// `undefined`. Sparse arrays interact surprisingly with iteration
// methods (`forEach` skips holes, `map` preserves them, `for…of` yields
// `undefined`) and are almost always a typo.
//
// Trailing commas don't create holes (`[1, 2,]` is fine).
package nosparsearrays

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-sparse-arrays"

// New constructs a nosparsearrays rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindArrayLiteralExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Each elision in a JS array literal shows up as a KindOmittedExpression
	// child of the ArrayLiteralExpression. TypeScript-Go preserves the
	// trailing comma without producing a final OmittedExpression, so this
	// loop won't false-positive on `[1, 2,]`.
	holes := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindOmittedExpression {
			holes++
		}
		return false
	})
	if holes == 0 {
		return
	}
	if holes == 1 {
		ctx.Report(n, "unexpected comma in middle of array")
	} else {
		ctx.Report(n, "unexpected commas in middle of array")
	}
}
