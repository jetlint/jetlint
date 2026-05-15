// Package noduplicatecase implements the no-duplicate-case rule: flag
// switch statements that contain two case clauses whose expressions
// are textually identical. JavaScript executes only the first matching
// branch, so the duplicate is unreachable — almost always a copy-paste
// error.
//
// Each switch is analyzed in isolation: identical case values across
// separate switches (or in nested switches) are not flagged. Equality
// is measured by [Node.SourceText], same as no-self-compare; this
// matches ESLint's token-based equality for all common cases. Cases
// that differ only in whitespace or evaluate to the same runtime value
// via different expressions (e.g. `1` vs `0x1`) are not deduplicated,
// matching ESLint's behavior.
package noduplicatecase

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-duplicate-case"

// New constructs a noduplicatecase rule instance ready for registration
// with the engine.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	seen := map[string]bool{}
	// SwitchStatement's child is a CaseBlock; the CaseBlock's children
	// are CaseClause and DefaultClause nodes.
	n.ForEachChild(func(block *wrapperchecker.Node) bool {
		block.ForEachChild(func(clause *wrapperchecker.Node) bool {
			if clause.Kind() != wrapperchecker.KindCaseClause {
				return false
			}
			expr := clause.CaseExpression()
			if expr == nil {
				return false
			}
			text := expr.SourceText()
			if text == "" {
				return false
			}
			if seen[text] {
				ctx.Report(clause, "duplicate case label")
				return false
			}
			seen[text] = true
			return false
		})
		return false
	})
}
