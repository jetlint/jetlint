// Package nouselesslabel implements no-useless-label: `break L` /
// `continue L` is redundant when the label refers to the immediately
// enclosing loop or switch — drop the label.
package nouselesslabel

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-label"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBreakStatement:    visit,
		wrapperchecker.KindContinueStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Find the label identifier child.
	var label *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			label = c
		}
		return false
	})
	if label == nil {
		return
	}
	labelName := label.SourceText()
	isContinue := n.Kind() == wrapperchecker.KindContinueStatement
	// Walk up parents to find the nearest break/continue target.
	for p := n.Parent(); p != nil; p = p.Parent() {
		if isBreakTarget(p, isContinue) {
			// If this target is the body of a LabeledStatement matching our label,
			// the label is useless.
			lp := p.Parent()
			if lp != nil && lp.Kind() == wrapperchecker.KindLabeledStatement {
				name := labeledStatementName(lp)
				if name == labelName {
					if isContinue {
						ctx.Report(n, "useless label on `continue` — it already targets the enclosing loop")
					} else {
						ctx.Report(n, "useless label on `break` — it already targets the enclosing loop or switch")
					}
				}
			}
			return
		}
		// Stop at function boundaries.
		if isFunctionLike(p) {
			return
		}
	}
}

func isBreakTarget(n *wrapperchecker.Node, _ bool) bool {
	// Biome treats both loops and switches as barriers (returning false
	// here just means we keep walking up). A switch between the label and
	// the use is enough to make the label non-redundant in biome's eyes.
	switch n.Kind() {
	case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement,
		wrapperchecker.KindForStatement, wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement, wrapperchecker.KindSwitchStatement:
		return true
	}
	return false
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor, wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		return true
	}
	return false
}

func labeledStatementName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil || first.Kind() != wrapperchecker.KindIdentifier {
		return ""
	}
	return first.SourceText()
}
