// Package noglobalassign implements no-global-assign: reassigning
// built-in globals like `window`, `Object`, or `undefined` breaks
// every code path that relies on them.
package noglobalassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-global-assign"

var restricted = map[string]bool{
	"Object": true, "String": true, "Number": true, "Boolean": true, "Symbol": true,
	"BigInt": true, "Array": true, "Date": true, "RegExp": true, "Error": true,
	"Function": true, "Math": true, "JSON": true, "Promise": true,
	"Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"window": true, "globalThis": true, "self": true, "document": true,
	"undefined": true, "NaN": true, "Infinity": true,
	"length": true, "top": true,
	"require": true, "module": true, "exports": true,
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression:        visitBinary,
		wrapperchecker.KindPostfixUnaryExpression:  visitUnary,
		wrapperchecker.KindPrefixUnaryExpression:   visitUnary,
	}
}

func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isAssign(n.BinaryOperatorKind()) {
		return
	}
	flagPattern(ctx, n, n.BinaryLeft())
}

func visitUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.PrefixUnaryOperator()
	if op != "++" && op != "--" {
		return
	}
	var operand *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if operand == nil {
			operand = c
		}
		return false
	})
	flagPattern(ctx, n, operand)
}

func isAssign(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindEqualsToken, wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken, wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken, wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken, wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindBarBarEqualsToken, wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

func flagPattern(ctx *engine.Context, n, lhs *wrapperchecker.Node) {
	if lhs == nil {
		return
	}
	switch lhs.Kind() {
	case wrapperchecker.KindIdentifier:
		name := lhs.SourceText()
		if restricted[name] && !isDeclared(lhs, name) {
			ctx.Report(n, "do not reassign the global `"+name+"`")
		}
	case wrapperchecker.KindObjectLiteralExpression, wrapperchecker.KindArrayLiteralExpression:
		walkDestructure(ctx, n, lhs)
	}
}

func walkDestructure(ctx *engine.Context, n, pat *wrapperchecker.Node) {
	pat.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			name := c.SourceText()
			if restricted[name] && !isDeclared(c, name) {
				ctx.Report(n, "do not reassign the global `"+name+"`")
			}
		case wrapperchecker.KindShorthandPropertyAssignment, wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindBindingElement, wrapperchecker.KindObjectLiteralExpression,
			wrapperchecker.KindArrayLiteralExpression, wrapperchecker.KindBinaryExpression,
			wrapperchecker.KindSpreadElement:
			walkDestructure(ctx, n, c)
		}
		return false
	})
}

func isDeclared(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		declared := false
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declaresName(c, name) {
				declared = true
				return true
			}
			return false
		})
		if declared {
			return true
		}
	}
	return false
}

func declaresName(c *wrapperchecker.Node, name string) bool {
	switch c.Kind() {
	case wrapperchecker.KindVariableStatement:
		hit := false
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if d.Kind() == wrapperchecker.KindVariableDeclarationList {
				d.ForEachChild(func(decl *wrapperchecker.Node) bool {
					var first *wrapperchecker.Node
					decl.ForEachChild(func(x *wrapperchecker.Node) bool {
						if first == nil {
							first = x
						}
						return false
					})
					if first != nil && first.Kind() == wrapperchecker.KindIdentifier && first.SourceText() == name {
						hit = true
					}
					return false
				})
			}
			return false
		})
		return hit
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration:
		var first *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if first == nil && d.Kind() == wrapperchecker.KindIdentifier {
				first = d
			}
			return false
		})
		return first != nil && first.SourceText() == name
	}
	return false
}
