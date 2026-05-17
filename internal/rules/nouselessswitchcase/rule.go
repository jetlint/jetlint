// Package nouselessswitchcase implements no-useless-switch-case: an
// empty `case` adjacent to a `default` is redundant because the
// `default` would catch the same value.
package nouselessswitchcase

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-switch-case"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var clauses []*wrapperchecker.Node
	// SwitchStatement child is the discriminant, then a CaseBlock containing
	// the clauses.
	n.ForEachChild(func(block *wrapperchecker.Node) bool {
		if block.Kind() == wrapperchecker.KindCaseClause || block.Kind() == wrapperchecker.KindDefaultClause {
			clauses = append(clauses, block)
			return false
		}
		block.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindCaseClause || c.Kind() == wrapperchecker.KindDefaultClause {
				clauses = append(clauses, c)
			}
			return false
		})
		return false
	})
	// Walk clauses, grouping each maximal run of fall-through clauses (those
	// with no statements) plus the terminating clause that holds the body.
	i := 0
	for i < len(clauses) {
		j := i
		hasDefault := false
		for j < len(clauses) {
			if clauses[j].Kind() == wrapperchecker.KindDefaultClause {
				hasDefault = true
			}
			if clauseHasStatements(clauses[j]) {
				break
			}
			j++
		}
		// Group: clauses[i..j], all empty except maybe clauses[j].
		if j < len(clauses) && clauses[j].Kind() == wrapperchecker.KindDefaultClause {
			hasDefault = true
		}
		if hasDefault {
			for k := i; k <= j && k < len(clauses); k++ {
				if clauses[k].Kind() == wrapperchecker.KindCaseClause && !clauseHasStatements(clauses[k]) {
					ctx.Report(clauses[k], "empty case adjacent to default is redundant — remove it")
				}
			}
		}
		if j < len(clauses) {
			j++
		}
		i = j
	}
}

func clauseHasStatements(c *wrapperchecker.Node) bool {
	count := 0
	idx := 0
	c.ForEachChild(func(child *wrapperchecker.Node) bool {
		// First child of CaseClause is the test expression; statements follow.
		if c.Kind() == wrapperchecker.KindCaseClause && idx == 0 {
			idx++
			return false
		}
		count++
		idx++
		return false
	})
	return count > 0
}
