// Package noexcessivenestedtestsuites implements
// no-excessive-nested-test-suites: more than five layers of `describe`
// nesting makes failure output unreadable.
package noexcessivenestedtestsuites

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-excessive-nested-test-suites"

const maxDepth = 5

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isDescribeCall(n) {
		return
	}
	depth := 1
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == wrapperchecker.KindCallExpression && isDescribeCall(p) {
			depth++
		}
	}
	if depth > maxDepth {
		ctx.Report(n, "nesting `describe` more than 5 levels deep makes failures hard to find")
	}
}

func isDescribeCall(n *wrapperchecker.Node) bool {
	var callee *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if callee == nil {
			callee = c
		}
		return false
	})
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	name := callee.SourceText()
	return name == "describe" || name == "suite" || name == "context"
}
