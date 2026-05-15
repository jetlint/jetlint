// Package noemptypattern implements the no-empty-pattern rule: flag
// destructuring patterns that bind no variables, e.g. `const {} = x;`
// or `function f([]) {}`. Such a pattern has no effect at runtime —
// almost always a typo where the author meant `const {a = {}} = x;`
// or similar.
//
// Matches oxlint and ESLint's default config: any empty array or
// object binding pattern is reported, including patterns nested inside
// a larger destructuring. The `allowObjectPatternsAsParameters`
// option is not implemented (jetlint follows the default).
package noemptypattern

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-empty-pattern"

// New constructs a noemptypattern rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindObjectBindingPattern: visitObject,
		wrapperchecker.KindArrayBindingPattern:  visitArray,
	}
}

func visitObject(ctx *engine.Context, n *wrapperchecker.Node) {
	if hasBindingElement(n) {
		return
	}
	ctx.Report(n, "empty object binding pattern")
}

func visitArray(ctx *engine.Context, n *wrapperchecker.Node) {
	if hasBindingElement(n) {
		return
	}
	ctx.Report(n, "empty array binding pattern")
}

// hasBindingElement reports whether the pattern has at least one
// element node. Tokens like `{`, `}`, `[`, `]` show up as children
// too; we only count BindingElement (the typical case) and
// OmittedExpression (a hole in `[, , a]` — still counts as
// "non-empty" per ESLint's behavior, though such patterns are rarer).
func hasBindingElement(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindBindingElement, wrapperchecker.KindOmittedExpression:
			found = true
			return true
		}
		return false
	})
	return found
}
