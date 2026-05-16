// Package nodupeargs implements the no-dupe-args rule: flag function
// declarations / expressions that bind the same parameter name twice.
// Strict-mode parsers reject this at parse time, but sloppy-mode
// CommonJS still allows it and the later parameter silently shadows
// the earlier one — almost always a typo. Arrow functions are
// excluded; ES2015 makes duplicates a syntax error there.
package nodupeargs

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dupe-args"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: r.visit,
		wrapperchecker.KindFunctionExpression:  r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	params := n.FunctionParameters()
	if len(params) < 2 {
		return
	}
	seen := map[string]bool{}
	for _, p := range params {
		name := p.ParameterName()
		if name == nil || name.Kind() != wrapperchecker.KindIdentifier {
			// Skip binding patterns (`{ a }`, `[a]`) and rest
			// elements with a non-identifier target — those forms
			// can't legally introduce duplicates of a top-level
			// identifier in a way the rule cares about.
			continue
		}
		text := name.SourceText()
		if text == "" {
			continue
		}
		if seen[text] {
			ctx.Report(name, "Duplicate param '"+text+"'.")
			continue
		}
		seen[text] = true
	}
}
