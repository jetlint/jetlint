// Package useawait implements use-await: an `async` function that
// never `await`s anything (and isn't an async generator using `yield*`
// or `for await ... of`) doesn't need to be async.
package useawait

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-await"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !wrapperchecker.HasAsyncModifier(n) {
		return
	}
	body := functionBody(n)
	if body == nil {
		return
	}
	// Empty body is fine.
	hasStmt := false
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		hasStmt = true
		return true
	})
	if !hasStmt {
		return
	}
	if bodyHasAsyncOp(body) {
		return
	}
	ctx.Report(n, "async function never awaits — drop `async` (or add an `await`)")
}

func functionBody(n *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	return body
}

func bodyHasAsyncOp(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindAwaitExpression:
		return true
	case wrapperchecker.KindYieldExpression:
		// `yield*` in async generators is enough.
		return true
	case wrapperchecker.KindForOfStatement:
		// Check if it's `for await`.
		if hasAwaitKeyword(n) {
			return true
		}
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor,
		wrapperchecker.KindConstructor:
		// Don't descend into nested functions.
		return false
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if bodyHasAsyncOp(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

func hasAwaitKeyword(n *wrapperchecker.Node) bool {
	src := n.SourceText()
	// `for await (`
	for i := 0; i+10 < len(src); i++ {
		if src[i] == 'f' && src[i+1] == 'o' && src[i+2] == 'r' {
			j := i + 3
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			if j+5 < len(src) && src[j:j+5] == "await" {
				return true
			}
			return false
		}
	}
	return false
}
