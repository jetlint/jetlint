// Package nofocusedtests implements no-focused-tests: a focused
// test (`.only`, `fit`, `fdescribe`) silently disables every other
// test in the file when committed. CI passes despite running almost
// nothing.
package nofocusedtests

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-focused-tests"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

var focusedPrefixed = map[string]bool{
	"fit": true, "fdescribe": true, "ftest": true,
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil {
		return
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		if focusedPrefixed[callee.SourceText()] {
			ctx.Report(n, callee.SourceText()+" focuses the suite — drop the `f` prefix before committing")
		}
	case wrapperchecker.KindPropertyAccessExpression:
		obj, name := propParts(callee)
		if name != "only" || obj == nil {
			return
		}
		if obj.Kind() == wrapperchecker.KindIdentifier && isTestRunner(obj.SourceText()) {
			ctx.Report(n, obj.SourceText()+".only focuses the suite — remove before committing")
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
