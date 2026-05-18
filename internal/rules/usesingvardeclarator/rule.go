// Package usesingvardeclarator implements use-single-var-declarator:
// declaring multiple variables in one `let a = 1, b = 2;` reads as
// one statement but it's really many. Separate statements diff and
// move better.
package usesingvardeclarator

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-single-var-declarator"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclarationList: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip if inside a `for`/`for-in`/`for-of` header — comma-separated
	// declarations there are idiomatic.
	if p := n.Parent(); p != nil {
		switch p.Kind() {
		case wrapperchecker.KindForStatement, wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement:
			return
		}
	}
	count := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindVariableDeclaration {
			count++
		}
		return false
	})
	if count > 1 {
		ctx.Report(n, "split into one declaration per statement — easier diffs")
	}
}
