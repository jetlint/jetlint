// Package switchexhaustivenesscheck implements the
// switch-exhaustiveness-check rule: flag switches over a union type
// that don't cover every member.
package switchexhaustivenesscheck

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "switch-exhaustiveness-check"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	disc := n.SwitchExpression()
	if disc == nil {
		return
	}
	dt := ctx.TypeOf(disc)
	if dt == nil || !dt.IsUnion() {
		return
	}
	required := map[string]bool{}
	for _, m := range dt.UnionMembers() {
		if !isCoverable(m) {
			return
		}
		required[m.String()] = true
	}
	if len(required) == 0 {
		return
	}
	hasDefault := false
	covered := map[string]bool{}
	n.ForEachChild(func(block *wrapperchecker.Node) bool {
		block.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindDefaultClause {
				hasDefault = true
				return false
			}
			if c.Kind() != wrapperchecker.KindCaseClause {
				return false
			}
			expr := c.CaseExpression()
			if expr == nil {
				return false
			}
			t := ctx.TypeOf(expr)
			if t == nil {
				return false
			}
			covered[t.String()] = true
			return false
		})
		return false
	})
	if hasDefault {
		return
	}
	for label := range required {
		if !covered[label] {
			ctx.Report(disc, "switch is not exhaustive — missing case for "+label)
			return
		}
	}
}

func isCoverable(m *wrapperchecker.Type) bool {
	if m.IsBooleanLike() || m.IsNullOrUndefined() {
		return true
	}
	s := m.String()
	if s == "" {
		return false
	}
	if m.IsStringLike() && s != "string" {
		return true
	}
	if m.IsNumberLike() && s != "number" {
		return true
	}
	if m.IsBigIntLike() && s != "bigint" {
		return true
	}
	if m.IsEnumLike() {
		return true
	}
	return false
}
