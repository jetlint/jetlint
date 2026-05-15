// Package nosetterreturn implements the no-setter-return rule: a
// setter accessor's return value is discarded by the language, so
// `return value;` is either dead code or a bug.
//
// The check is local: returning from inside a setter directly is flagged,
// but returning from a nested function/method/arrow inside the setter
// is fine — that function has its own callers that use its return.
package nosetterreturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-setter-return"

// New constructs a nosetterreturn rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !hasReturnArgument(n) {
		return
	}
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindSetAccessor:
			ctx.Report(n, "setter cannot return a value")
			return
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindConstructor:
			return
		}
	}
}

// hasReturnArgument reports whether the ReturnStatement actually
// returns a value. TypeScript-Go represents `return;` and `return val;`
// with the same Kind; the value (if any) is the only child.
func hasReturnArgument(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		_ = c
		found = true
		return true
	})
	return found
}
