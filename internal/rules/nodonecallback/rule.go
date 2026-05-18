// Package nodonecallback implements no-done-callback: Jest-style
// `(done) => done()` callbacks are pre-async-await — write an `async`
// callback or return a promise instead.
package nodonecallback

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-done-callback"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	calleeName, ok := plainHookName(n)
	if !ok {
		return
	}
	_ = calleeName
	args := callArgs(n)
	if len(args) == 0 {
		return
	}
	// The callback is usually the second arg (after the test name),
	// or the first arg for hooks like beforeAll/beforeEach.
	cb := args[len(args)-1]
	if cb.Kind() != wrapperchecker.KindArrowFunction && cb.Kind() != wrapperchecker.KindFunctionExpression {
		return
	}
	// Count parameters of the callback.
	paramCount := 0
	cb.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			paramCount++
		}
		return false
	})
	if paramCount == 0 {
		return
	}
	ctx.Report(cb, "drop the `done` callback — use async/await or return a promise")
}

func plainHookName(n *wrapperchecker.Node) (string, bool) {
	var callee *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if callee == nil {
			callee = c
		}
		return false
	})
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return "", false
	}
	name := callee.SourceText()
	switch name {
	case "test", "it", "beforeAll", "beforeEach", "afterAll", "afterEach":
		return name, true
	}
	return "", false
}

func callArgs(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var args []*wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			args = append(args, c)
		}
		idx++
		return false
	})
	return args
}
