// Package nounsafefinally implements the no-unsafe-finally rule: a
// return/throw/break/continue inside a `finally` block overrides any
// control-flow statement that was pending from the try or catch block.
// The behavior is almost never intended and silently swallows
// exceptions and return values.
package nounsafefinally

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unsafe-finally"

// New constructs a nounsafefinally rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindReturnStatement:   visit,
		wrapperchecker.KindThrowStatement:    visit,
		wrapperchecker.KindBreakStatement:    visit,
		wrapperchecker.KindContinueStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	kind := n.Kind()
	label := controlFlowLabel(n)
	// Match upstream's SentinelNodeType: bare `break` is stopped by
	// any loop or switch; bare `continue` by any loop; `return`,
	// `throw`, and *labeled* break/continue are only stopped by a
	// function or class boundary because the labeled jump can escape
	// surrounding loops and switches.
	sentinel := kind
	if (kind == wrapperchecker.KindBreakStatement || kind == wrapperchecker.KindContinueStatement) && label != "" {
		sentinel = wrapperchecker.KindReturnStatement
	}
	labelInside := false
	for cur := n; cur != nil; {
		parent := cur.Parent()
		if parent == nil {
			return
		}
		if isFunctionOrClassBoundary(parent) {
			return
		}
		if isLoopOrSwitchSentinel(parent, sentinel) {
			return
		}
		// `break label` is safe if the matching LabeledStatement sits
		// between the current node and the enclosing finally block —
		// i.e., the labeled target is itself inside the finally, so
		// the jump stays inside.
		if parent.Kind() == wrapperchecker.KindLabeledStatement && label != "" {
			if labeledStatementName(parent) == label {
				labelInside = true
			}
		}
		// If the parent is the finally block of a TryStatement, we're
		// inside a finally and must report (unless label-safe).
		if parent.Kind() == wrapperchecker.KindBlock && parent.Parent() != nil &&
			parent.Parent().Kind() == wrapperchecker.KindTryStatement {
			finally := parent.Parent().TryStatementFinallyBlock()
			if finally != nil && finally.Pos() == parent.Pos() && finally.End() == parent.End() {
				if label != "" && labelInside {
					return
				}
				ctx.Report(n, "Unsafe usage of "+kindLabel(kind)+" inside finally block.")
				return
			}
		}
		cur = parent
	}
}

// controlFlowLabel returns the label identifier text from a
// labeled break/continue, or "" if the statement has no label.
func controlFlowLabel(n *wrapperchecker.Node) string {
	switch n.Kind() {
	case wrapperchecker.KindBreakStatement, wrapperchecker.KindContinueStatement:
	default:
		return ""
	}
	var label string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			label = c.LiteralText()
			return true
		}
		return false
	})
	return label
}

// labeledStatementName returns the label identifier of a
// LabeledStatement node.
func labeledStatementName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// isFunctionOrClassBoundary stops the upward walk: a return inside a
// nested function (or method, or accessor, or class) doesn't escape
// into the surrounding try/finally.
func isFunctionOrClassBoundary(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindSourceFile,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindModuleDeclaration:
		return true
	}
	return false
}

// isLoopOrSwitchSentinel matches oxlint's SentinelNodeType.test logic.
// For break: any loop or switch stops the walk. For continue: only
// loops stop. For return/throw: nothing stops short of a function/class
// boundary.
func isLoopOrSwitchSentinel(n *wrapperchecker.Node, kind wrapperchecker.Kind) bool {
	switch kind {
	case wrapperchecker.KindBreakStatement:
		switch n.Kind() {
		case wrapperchecker.KindForStatement,
			wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement,
			wrapperchecker.KindSwitchStatement:
			return true
		}
	case wrapperchecker.KindContinueStatement:
		switch n.Kind() {
		case wrapperchecker.KindForStatement,
			wrapperchecker.KindForInStatement,
			wrapperchecker.KindForOfStatement,
			wrapperchecker.KindWhileStatement,
			wrapperchecker.KindDoStatement:
			return true
		}
	}
	return false
}

func kindLabel(k wrapperchecker.Kind) string {
	switch k {
	case wrapperchecker.KindReturnStatement:
		return "return"
	case wrapperchecker.KindThrowStatement:
		return "throw"
	case wrapperchecker.KindBreakStatement:
		return "break"
	case wrapperchecker.KindContinueStatement:
		return "continue"
	}
	return "control flow"
}
