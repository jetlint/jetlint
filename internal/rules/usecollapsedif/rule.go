// Package usecollapsedif implements use-collapsed-if: nested `if`s
// without intervening statements can be collapsed into one with `&&`.
package usecollapsedif

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-collapsed-if"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var test, then, els *wrapperchecker.Node
	i := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch i {
		case 0:
			test = c
		case 1:
			then = c
		case 2:
			els = c
		}
		i++
		return false
	})
	_ = test
	if els != nil || then == nil {
		return
	}
	inner := singleInnerIf(then)
	if inner == nil {
		return
	}
	// Inner if must not have an else.
	innerHasElse := false
	idx := 0
	inner.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 2 {
			innerHasElse = true
		}
		idx++
		return false
	})
	if innerHasElse {
		return
	}
	ctx.Report(n, "collapse nested `if` into a single `if` with `&&`")
}

func singleInnerIf(then *wrapperchecker.Node) *wrapperchecker.Node {
	if then == nil {
		return nil
	}
	if then.Kind() == wrapperchecker.KindIfStatement {
		return then
	}
	if then.Kind() != wrapperchecker.KindBlock {
		return nil
	}
	var only *wrapperchecker.Node
	count := 0
	then.ForEachChild(func(c *wrapperchecker.Node) bool {
		only = c
		count++
		return false
	})
	if count != 1 {
		return nil
	}
	if only.Kind() != wrapperchecker.KindIfStatement {
		return nil
	}
	return only
}
