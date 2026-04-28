// Package switchexhaustivenesscheck implements the
// switch-exhaustiveness-check rule: flag switches over a union type
// that don't cover every member.
package switchexhaustivenesscheck

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "switch-exhaustiveness-check"

// Options is the configurable surface of the rule.
type Options struct {
	AllowDefaultCaseForExhaustiveSwitch bool
	RequireDefaultForNonUnion           bool
	ConsiderDefaultExhaustiveForUnions  bool
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options {
	return Options{
		AllowDefaultCaseForExhaustiveSwitch: true,
	}
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	disc := n.SwitchExpression()
	if disc == nil {
		return
	}
	dt := ctx.TypeOf(disc)
	if dt == nil {
		return
	}
	hasDefault, caseTypes := collectCases(ctx, n)
	// requireDefaultForNonUnion: a switch over a single type (literal,
	// enum, primitive) needs an explicit `default` clause regardless of
	// case coverage. The exhaustiveness check below only reasons about
	// unions; this option fills in the non-union side.
	if r.opts.RequireDefaultForNonUnion && !dt.IsUnion() && !hasDefault {
		ctx.Report(disc, "switch over a non-union value should have a `default` clause")
		return
	}
	// allowDefaultCaseForExhaustiveSwitch=false: even on an exhaustive
	// switch, a `default` is forbidden (it means the union changed
	// without the case being added).
	if !r.opts.AllowDefaultCaseForExhaustiveSwitch && hasDefault && dt.IsUnion() && exhaustiveOverUnion(dt, caseTypes) {
		ctx.Report(disc, "remove the `default` clause from this exhaustive switch")
		return
	}
	if !dt.IsUnion() {
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
	if hasDefault && r.opts.ConsiderDefaultExhaustiveForUnions {
		return
	}
	if hasDefault {
		// Default branch satisfies coverage by absorbing the missing
		// members.
		return
	}
	for label := range required {
		if !caseTypes[label] {
			ctx.Report(disc, "switch is not exhaustive — missing case for "+label)
			return
		}
	}
}

// collectCases walks the switch body once, returning whether a default
// clause exists and the set of case-expression type strings already
// covered.
func collectCases(ctx *engine.Context, sw *wrapperchecker.Node) (bool, map[string]bool) {
	hasDefault := false
	covered := map[string]bool{}
	sw.ForEachChild(func(block *wrapperchecker.Node) bool {
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
			if t := ctx.TypeOf(expr); t != nil {
				covered[t.String()] = true
			}
			return false
		})
		return false
	})
	return hasDefault, covered
}

// exhaustiveOverUnion reports whether every union member is already
// matched by an explicit case.
func exhaustiveOverUnion(dt *wrapperchecker.Type, covered map[string]bool) bool {
	for _, m := range dt.UnionMembers() {
		if !isCoverable(m) {
			return false
		}
		if !covered[m.String()] {
			return false
		}
	}
	return true
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
