// Package usemaxparams implements use-max-params: a function with
// more than a handful of positional parameters becomes a memory
// game at call sites. Prefer an options object past the threshold.
package usemaxparams

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-max-params"

// Options controls the maximum allowed parameter count. Defaults to 4
// when omitted, which matches the biome default.
type Options struct {
	Max int
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	max := 4
	if opts, ok := ctx.Options().(*Options); ok && opts != nil && opts.Max > 0 {
		max = opts.Max
	}
	count := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			count++
		}
		return false
	})
	if count > max {
		ctx.Report(n, "too many parameters — collapse extras into an options object")
	}
}
