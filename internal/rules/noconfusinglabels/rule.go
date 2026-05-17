// Package noconfusinglabels implements no-confusing-labels: only
// loops and switches need a label. A label on any other statement
// reads as a syntax error to most readers.
package noconfusinglabels

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-confusing-labels"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindLabeledStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Children: [label Identifier, statement].
	var stmt *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 1 {
			stmt = c
		}
		idx++
		return false
	})
	if stmt == nil {
		return
	}
	switch stmt.Kind() {
	case wrapperchecker.KindForStatement, wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement, wrapperchecker.KindWhileStatement,
		wrapperchecker.KindDoStatement:
		return
	}
	ctx.Report(n, "label on a non-loop/non-switch statement reads as confusing syntax")
}
