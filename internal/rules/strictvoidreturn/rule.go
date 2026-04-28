// Package strictvoidreturn implements the strict-void-return rule:
// flag callbacks passed where `() => void` is expected that
// nonetheless return a value.
package strictvoidreturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "strict-void-return"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
		wrapperchecker.KindNewExpression:  visitCall,
	}
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	args := n.CallArguments()
	for i, arg := range args {
		if arg.Kind() != wrapperchecker.KindArrowFunction &&
			arg.Kind() != wrapperchecker.KindFunctionExpression {
			continue
		}
		expected := ctx.Checker().ContextualTypeForArgument(n, i)
		if expected == nil {
			continue
		}
		if !allCallSignaturesExpectVoid(expected) {
			continue
		}
		body := arg.FunctionBody()
		if body == nil {
			continue
		}
		if body.Kind() != wrapperchecker.KindBlock {
			t := ctx.TypeOf(body)
			if t != nil && !t.IsVoid() {
				ctx.Report(arg, "callback expected to return void but the body produces a value")
			}
			continue
		}
		if blockHasValueReturn(ctx, body, arg) {
			ctx.Report(arg, "callback expected to return void but the body produces a value")
		}
	}
}

func blockHasValueReturn(ctx *engine.Context, body, fn *wrapperchecker.Node) bool {
	found := false
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if found || n == nil {
			return
		}
		if n != fn {
			switch n.Kind() {
			case wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindFunctionExpression,
				wrapperchecker.KindArrowFunction,
				wrapperchecker.KindMethodDeclaration:
				return
			}
		}
		if n.Kind() == wrapperchecker.KindReturnStatement {
			expr := n.FirstChild()
			if expr != nil {
				t := ctx.TypeOf(expr)
				if t != nil && !t.IsVoid() {
					found = true
					return
				}
			}
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return found
		})
	}
	walk(body)
	return found
}

func allCallSignaturesExpectVoid(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		any := false
		for _, m := range t.UnionMembers() {
			if m.IsNullOrUndefined() {
				continue
			}
			if !allCallSignaturesExpectVoid(m) {
				return false
			}
			any = true
		}
		return any
	}
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		ret := s.ReturnType()
		if ret == nil || !ret.IsVoid() {
			return false
		}
	}
	return true
}
