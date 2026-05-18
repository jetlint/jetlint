// Package useconsistentarrowreturn implements use-consistent-arrow-return:
// an arrow whose body is just `{ return expr; }` reads more directly
// as the concise body `() => expr`.
package useconsistentarrowreturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-consistent-arrow-return"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindArrowFunction: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	body := functionBody(n)
	if body == nil {
		return
	}
	// Body must contain exactly one statement: a return with an argument.
	var stmts []*wrapperchecker.Node
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		stmts = append(stmts, c)
		return false
	})
	if len(stmts) != 1 {
		return
	}
	ret := stmts[0]
	if ret.Kind() != wrapperchecker.KindReturnStatement {
		return
	}
	var arg *wrapperchecker.Node
	ret.ForEachChild(func(c *wrapperchecker.Node) bool {
		if arg == nil {
			arg = c
		}
		return false
	})
	if arg == nil {
		return
	}
	// Skip when body has comments — converting would drop them.
	src := body.SourceText()
	for i := 0; i < len(src)-1; i++ {
		if src[i] == '/' && (src[i+1] == '/' || src[i+1] == '*') {
			return
		}
	}
	ctx.Report(n, "arrow body with only `return X` reads more directly as `() => X`")
}

func functionBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	return body
}
