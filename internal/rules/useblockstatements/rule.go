// Package useblockstatements implements use-block-statements: braces
// around if/else/while/for bodies prevent the "added a second line
// and forgot the braces" bug. Be consistent — always use them.
package useblockstatements

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-block-statements"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:      visitIf,
		wrapperchecker.KindWhileStatement:   visitLoop,
		wrapperchecker.KindDoStatement:      visitLoop,
		wrapperchecker.KindForStatement:     visitLoop,
		wrapperchecker.KindForInStatement:   visitLoop,
		wrapperchecker.KindForOfStatement:   visitLoop,
	}
}

func visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	// IfStatement children: condition, then-stmt, ?else-stmt.
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch idx {
		case 1, 2:
			if c.Kind() != wrapperchecker.KindBlock && c.Kind() != wrapperchecker.KindIfStatement {
				ctx.Report(c, "wrap branch in `{ }`")
			}
		}
		idx++
		return false
	})
}

func visitLoop(ctx *engine.Context, n *wrapperchecker.Node) {
	// The body is typically the last child.
	var last *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	if last == nil || last.Kind() == wrapperchecker.KindBlock {
		return
	}
	// EmptyStatement (just `;`) is also acceptable; check by source.
	if last.SourceText() == ";" {
		return
	}
	ctx.Report(last, "wrap loop body in `{ }`")
}
