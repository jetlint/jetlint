// Package usecollapsedelseif implements use-collapsed-else-if:
// `else { if (...) {} }` reads as `else if (...) {}` with one less
// indent level.
package usecollapsedelseif

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-collapsed-else-if"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var els *wrapperchecker.Node
	i := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if i == 2 {
			els = c
		}
		i++
		return false
	})
	if els == nil || els.Kind() != wrapperchecker.KindBlock {
		return
	}
	var only *wrapperchecker.Node
	count := 0
	els.ForEachChild(func(c *wrapperchecker.Node) bool {
		only = c
		count++
		return false
	})
	if count != 1 || only.Kind() != wrapperchecker.KindIfStatement {
		return
	}
	ctx.Report(n, "collapse `else { if(...) ... }` into `else if (...)`")
}
