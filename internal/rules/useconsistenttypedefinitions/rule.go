// Package useconsistenttypedefinitions implements
// use-consistent-type-definitions: pick interface or type alias and
// stick with it across the codebase. This rule defaults to favoring
// `interface` for object shapes (better extendability).
package useconsistenttypedefinitions

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-consistent-type-definitions"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTypeAliasDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Find the aliased type (last child).
	var t *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		t = c
		return false
	})
	if t == nil || t.Kind() != wrapperchecker.KindTypeLiteral {
		return
	}
	// An empty type literal `{}` carries different meaning (any non-null
	// value) and isn't worth nagging about.
	hasMembers := false
	t.ForEachChild(func(_ *wrapperchecker.Node) bool {
		hasMembers = true
		return true
	})
	if !hasMembers {
		return
	}
	ctx.Report(n, "use an interface for object shapes — more amenable to declaration merging")
}
