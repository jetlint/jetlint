// Package defaultcaselast implements the default-case-last rule:
// when a switch statement contains a `default` clause, it must be the
// final clause. A default clause placed in the middle reads as the
// catch-all but actually only runs when none of the trailing case
// labels match — easy to misread and almost always a bug.
//
// Matches oxlint and ESLint's default-case-last: zero configurable
// options, syntactic check only. Reports each non-last default clause
// in the switch.
package defaultcaselast

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "default-case-last"

// New constructs a default-case-last rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: r.visitSwitch,
	}
}

func (r *rule) visitSwitch(ctx *engine.Context, n *wrapperchecker.Node) {
	clauses := switchClauses(n)
	if len(clauses) < 2 {
		return
	}
	for _, c := range clauses[:len(clauses)-1] {
		if c.Kind() == wrapperchecker.KindDefaultClause {
			ctx.Report(c, "Default clause should be the last clause")
		}
	}
}

// switchClauses returns the ordered case/default clauses of a switch.
// The wrapper does not expose CaseBlock directly, so we descend one
// level: SwitchStatement's children are the discriminant expression
// and the CaseBlock; the CaseBlock's children are the clauses.
func switchClauses(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var clauses []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		c.ForEachChild(func(g *wrapperchecker.Node) bool {
			switch g.Kind() {
			case wrapperchecker.KindCaseClause, wrapperchecker.KindDefaultClause:
				clauses = append(clauses, g)
			}
			return false
		})
		return false
	})
	return clauses
}
