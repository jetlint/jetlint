// Package nouselesselse implements no-useless-else: an `else` after
// an `if` block that always exits (return/throw/break/continue) adds
// indent for no reason — drop the `else`.
package nouselesselse

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-else"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var then, els *wrapperchecker.Node
	i := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch i {
		case 1:
			then = c
		case 2:
			els = c
		}
		i++
		return false
	})
	if then == nil || els == nil {
		return
	}
	// Skip when this `if` is itself the `else` of an outer `if` (else-if chain).
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindIfStatement {
		var outerElse *wrapperchecker.Node
		k := 0
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if k == 2 {
				outerElse = c
			}
			k++
			return false
		})
		if outerElse != nil && outerElse.Pos() == n.Pos() {
			return
		}
	}
	if alwaysExits(then) {
		ctx.Report(els, "`else` after `if` that always exits is unnecessary — flatten")
	}
}

// alwaysExits returns true if every code path through n ends in a
// return/throw/break/continue.
func alwaysExits(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindReturnStatement, wrapperchecker.KindThrowStatement,
		wrapperchecker.KindBreakStatement, wrapperchecker.KindContinueStatement:
		return true
	case wrapperchecker.KindBlock:
		var last *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			last = c
			return false
		})
		return alwaysExits(last)
	case wrapperchecker.KindIfStatement:
		var then, els *wrapperchecker.Node
		i := 0
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch i {
			case 1:
				then = c
			case 2:
				els = c
			}
			i++
			return false
		})
		return els != nil && alwaysExits(then) && alwaysExits(els)
	}
	return false
}
