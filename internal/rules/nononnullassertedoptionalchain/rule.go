// Package nononnullassertedoptionalchain implements
// no-non-null-asserted-optional-chain: the whole point of `?.` is
// to short-circuit on nullish — slapping `!` on the result either
// asserts the short-circuit can't happen (in which case drop `?.`)
// or trusts a value the `?.` is explicitly telling you might be
// missing (in which case the assertion lies).
package nononnullassertedoptionalchain

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-non-null-asserted-optional-chain"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNonNullExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// The operand is the first (and only) child.
	var operand *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if operand == nil {
			operand = c
		}
		return false
	})
	if hasOptionalChain(operand) {
		ctx.Report(n, "`!` on an optional chain contradicts the `?.` — drop one of them")
	}
}

func hasOptionalChain(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression, wrapperchecker.KindCallExpression, wrapperchecker.KindElementAccessExpression:
		// True if the outermost operation here uses `?.`.
		if usesOptionalAtTop(n) {
			return true
		}
		// Recurse only into the chain's head — `(foo?.bar).baz!` shouldn't fire.
		// We do walk past the call/element head though, since `foo?.bar()` etc.
		// are part of the same chain.
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		if first != nil && first.Kind() != wrapperchecker.KindParenthesizedExpression {
			return hasOptionalChain(first)
		}
		return false
	case wrapperchecker.KindParenthesizedExpression:
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		return hasOptionalChain(first)
	}
	return false
}

// usesOptionalAtTop checks whether the outermost operator of a
// property/element/call expression is `?.` (vs `.`).
func usesOptionalAtTop(n *wrapperchecker.Node) bool {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil {
		return false
	}
	src := n.SourceText()
	firstSrc := first.SourceText()
	_, after, ok := strings.Cut(src, firstSrc)
	if !ok {
		return false
	}
	after = strings.TrimLeft(after, " \t\n\r")
	return strings.HasPrefix(after, "?.")
}
