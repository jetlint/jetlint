// Package usedefaultparameterlast implements use-default-parameter-last:
// a default-valued parameter before a required one forces callers to
// pass `undefined` explicitly to get the default. Put defaults last.
package usedefaultparameterlast

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-default-parameter-last"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
		wrapperchecker.KindConstructor:         visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var params []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			params = append(params, c)
		}
		return false
	})
	// Walk back-to-front: once we see a required (no default, not optional)
	// parameter, any earlier parameter with a default is misplaced.
	sawRequiredAfter := false
	for i := len(params) - 1; i >= 0; i-- {
		p := params[i]
		if hasDefault(p) {
			if sawRequiredAfter {
				ctx.Report(p, "default parameter precedes a required one — move it after")
			}
			continue
		}
		if isOptional(p) {
			continue
		}
		sawRequiredAfter = true
	}
}

func hasDefault(p *wrapperchecker.Node) bool {
	// Look for `=` after the parameter name in source.
	return strings.Contains(p.SourceText(), "=")
}

func isOptional(p *wrapperchecker.Node) bool {
	return strings.Contains(p.SourceText(), "?:")
}
