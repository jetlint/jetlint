// Package nounusedlabels implements the no-unused-labels rule: a
// labeled statement whose label is never referenced by a `break` or
// `continue` inside the labeled body is dead code. The label is at
// best decoration and at worst a hint that the author intended a
// jump that never materialized.
//
// JavaScript labels live in a separate namespace from variables, so
// the symbol-resolution we use for `no-undef` and `no-unused-vars`
// doesn't help here. The rule walks the labeled body manually and
// looks for any `break X` / `continue X` whose label identifier
// text matches the labeled statement's own.
package nounusedlabels

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-labels"

// New constructs a nounusedlabels rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindLabeledStatement: visit,
	}
}

// visit handles a single LabeledStatement: extract the label name from
// its first identifier child, then scan the remainder of the subtree
// for a break/continue carrying the same identifier text. Inner
// labeled statements are skipped — a re-bound label is a SyntaxError
// in valid source, so the parser would already have rejected it.
func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var labelNode *wrapperchecker.Node
	var labelText string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			labelNode = c
			labelText = c.LiteralText()
			return true
		}
		return false
	})
	if labelNode == nil || labelText == "" {
		return
	}

	if labelReferenced(n, labelNode, labelText) {
		return
	}
	ctx.Report(labelNode, fmt.Sprintf("'%s:' is defined but never used.", labelText))
}

// labelReferenced walks the subtree under labeled and returns true the
// first time it sees a `break <name>` or `continue <name>` whose
// identifier text matches name. The label identifier on the
// LabeledStatement itself is excluded (the rule reports unused
// declarations, not unused self-references) by skipping labelNode.
func labelReferenced(labeled, labelNode *wrapperchecker.Node, name string) bool {
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if c == labelNode {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindBreakStatement, wrapperchecker.KindContinueStatement:
			c.ForEachChild(func(lab *wrapperchecker.Node) bool {
				if lab.Kind() == wrapperchecker.KindIdentifier && lab.LiteralText() == name {
					found = true
					return true
				}
				return false
			})
			if found {
				return true
			}
		}
		c.ForEachChild(walk)
		return found
	}
	labeled.ForEachChild(walk)
	return found
}
