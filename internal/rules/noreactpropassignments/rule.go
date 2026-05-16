// Package noreactpropassignments implements no-react-prop-assignments:
// React props are immutable from the component's perspective.
// Assigning to a property of a component's `props` parameter
// (`props.bar = ...`) silently mutates the parent's data and breaks
// reconciliation. Flag any such assignment.
//
// Detection: a `props.X = ...` assignment inside a function whose
// first parameter is the bare identifier `props`. Destructured
// patterns (`function Foo({bar}) {}`) intentionally rebind into
// local variables and aren't covered — biome treats those
// reassignments as benign.
package noreactpropassignments

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-react-prop-assignments"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isAssignmentOperator(n.BinaryOperatorKind()) {
		return
	}
	lhs := n.BinaryLeft()
	if lhs == nil || lhs.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	recv := lhs.PropertyAccessReceiver()
	if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier ||
		recv.LiteralText() != "props" {
		return
	}
	fn := enclosingFunctionTakingProps(n)
	if fn == nil {
		return
	}
	if !isReactComponent(fn) {
		return
	}
	if propsReassignedBefore(fn, n) {
		return
	}
	ctx.Report(n, "don't assign to a prop — React props are immutable from inside the component")
}

func isAssignmentOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken:
		return true
	}
	return false
}

// enclosingFunctionTakingProps walks up to the nearest function-like
// ancestor whose first parameter is the bare identifier `props`,
// and returns that function. Returns nil if no enclosing function
// takes `props` directly (destructured patterns, no params, or
// different first-param name).
func enclosingFunctionTakingProps(n *wrapperchecker.Node) *wrapperchecker.Node {
	p := n.Parent()
	for p != nil {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration:
			if firstParamIsBareProps(p) {
				return p
			}
		case wrapperchecker.KindConstructor:
			return nil
		}
		p = p.Parent()
	}
	return nil
}

// isReactComponent reports whether fn looks like a React function
// component. The heuristic mirrors what biome accepts:
//   - the function's own name (if any) starts uppercase;
//   - it's the immediate argument to `memo(...)` / `forwardRef(...)`;
//   - it's the initializer of a `const`/`let` whose name starts
//     uppercase.
//
// Anonymous arrows passed as plain callbacks return false even if
// their first parameter is named `props` — biome doesn't treat
// those as components.
func isReactComponent(fn *wrapperchecker.Node) bool {
	if name := functionOwnName(fn); name != "" && startsUpper(name) {
		return true
	}
	p := fn.Parent()
	for p != nil && p.Kind() == wrapperchecker.KindParenthesizedExpression {
		p = p.Parent()
	}
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindCallExpression:
		callee := p.CalleeExpression()
		if callee == nil {
			return false
		}
		var name string
		switch callee.Kind() {
		case wrapperchecker.KindIdentifier:
			name = callee.LiteralText()
		case wrapperchecker.KindPropertyAccessExpression:
			name = callee.PropertyAccessName()
		}
		return name == "memo" || name == "forwardRef"
	case wrapperchecker.KindVariableDeclaration:
		return startsUpper(bindingName(p))
	}
	return false
}

func functionOwnName(fn *wrapperchecker.Node) string {
	switch fn.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindMethodDeclaration:
		var name string
		fn.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				name = c.LiteralText()
				return true
			}
			return false
		})
		return name
	}
	return ""
}

func bindingName(n *wrapperchecker.Node) string {
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

func startsUpper(s string) bool {
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z'
}

// propsReassignedBefore reports whether the function body has
// `props = <expr>` somewhere before assign's position. After such
// a reassignment biome considers subsequent `props` references to
// refer to whatever was reassigned, not the original prop.
func propsReassignedBefore(fn, assign *wrapperchecker.Node) bool {
	body := functionBody(fn)
	if body == nil {
		return false
	}
	cutoff := assign.Pos()
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if found || c.Pos() >= cutoff {
			return found
		}
		if c.Kind() == wrapperchecker.KindBinaryExpression &&
			c.BinaryOperatorKind() == wrapperchecker.KindEqualsToken {
			if lhs := c.BinaryLeft(); lhs != nil &&
				lhs.Kind() == wrapperchecker.KindIdentifier &&
				lhs.LiteralText() == "props" {
				found = true
				return true
			}
		}
		c.ForEachChild(walk)
		return found
	}
	body.ForEachChild(walk)
	return found
}

func functionBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
			return true
		}
		return false
	})
	return body
}

func firstParamIsBareProps(fn *wrapperchecker.Node) bool {
	var first *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			first = c
			return true
		}
		return false
	})
	if first == nil {
		return false
	}
	// A Parameter's first child is the binding target. We need a
	// bare Identifier with text "props"; destructuring patterns
	// (ObjectBindingPattern, ArrayBindingPattern) don't count.
	var binding *wrapperchecker.Node
	first.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			binding = c
			return true
		}
		return false
	})
	return binding != nil &&
		binding.Kind() == wrapperchecker.KindIdentifier &&
		binding.LiteralText() == "props"
}
