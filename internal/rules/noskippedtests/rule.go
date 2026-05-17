// Package noskippedtests implements no-skipped-tests: a checked-in
// `.skip`/`xit`/`xdescribe` is usually a forgotten investigation
// that hides regressions. Force a conscious decision: delete or fix.
package noskippedtests

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-skipped-tests"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

var prefixSkipNames = map[string]bool{
	"xit": true, "xdescribe": true, "xtest": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil {
		return
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		if prefixSkipNames[callee.SourceText()] {
			ctx.Report(n, callee.SourceText()+" — delete or fix the test")
		}
	case wrapperchecker.KindPropertyAccessExpression:
		obj, name := propParts(callee)
		if name != "skip" {
			return
		}
		if obj == nil {
			return
		}
		// `describe.skip`, `it.skip`, `test.skip` — flag.
		if obj.Kind() == wrapperchecker.KindIdentifier && isTestRunner(obj.SourceText()) {
			ctx.Report(n, obj.SourceText()+".skip — delete or fix the test")
		}
	}
}

func isTestRunner(s string) bool {
	return s == "describe" || s == "it" || s == "test" || s == "context" || s == "suite"
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
}
