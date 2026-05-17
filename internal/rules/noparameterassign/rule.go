// Package noparameterassign implements no-parameter-assign:
// reassigning a function parameter makes the parameter's name lie —
// rebind to a local variable instead.
package noparameterassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-parameter-assign"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression:       visitBinary,
		wrapperchecker.KindPostfixUnaryExpression: visitUnary,
		wrapperchecker.KindPrefixUnaryExpression:  visitUnary,
		wrapperchecker.KindForInStatement:         visitForBinding,
		wrapperchecker.KindForOfStatement:         visitForBinding,
	}
}

func visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isAssign(n.BinaryOperatorKind()) {
		return
	}
	check(ctx, n, n.BinaryLeft())
}

func visitUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	hasIncDec := false
	for i := 0; i < len(src)-1; i++ {
		if (src[i] == '+' && src[i+1] == '+') || (src[i] == '-' && src[i+1] == '-') {
			hasIncDec = true
			break
		}
	}
	if !hasIncDec {
		return
	}
	var operand *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if operand == nil {
			operand = c
		}
		return false
	})
	check(ctx, n, operand)
}

func visitForBinding(ctx *engine.Context, n *wrapperchecker.Node) {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	// Only flag when the loop variable is a bare identifier (no `let`).
	if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
		check(ctx, n, first)
	}
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

func check(ctx *engine.Context, n, lhs *wrapperchecker.Node) {
	if lhs == nil {
		return
	}
	switch lhs.Kind() {
	case wrapperchecker.KindIdentifier:
		name := lhs.SourceText()
		if isParamName(lhs, name) {
			ctx.Report(n, "do not reassign function parameter `"+name+"`")
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
			if isParamName(c, name) {
				ctx.Report(n, "do not reassign function parameter `"+name+"`")
			}
		case wrapperchecker.KindShorthandPropertyAssignment, wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindBindingElement, wrapperchecker.KindObjectLiteralExpression,
			wrapperchecker.KindArrayLiteralExpression, wrapperchecker.KindBinaryExpression,
			wrapperchecker.KindSpreadElement, wrapperchecker.KindSpreadAssignment:
			walkDestructure(ctx, n, c)
		}
		return false
	})
}

// isParamName walks up looking for an enclosing function whose
// parameters bind `name` — stopping at the first parameter or local
// var declaration that introduces `name`.
func isParamName(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor:
			if functionHasParam(p, name) {
				return true
			}
			// If declared as a `var/let/const` inside this function before
			// reaching the parameters, it's not a parameter.
			if body := functionBody(p); body != nil && bodyHasLocalDecl(body, name) {
				return false
			}
		}
	}
	return false
}

func functionHasParam(fn *wrapperchecker.Node, name string) bool {
	found := false
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			var pn *wrapperchecker.Node
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if pn == nil {
					pn = d
				}
				return false
			})
			if pn != nil && pn.Kind() == wrapperchecker.KindIdentifier && pn.SourceText() == name {
				found = true
			}
		}
		return false
	})
	return found
}

func functionBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
		}
		return false
	})
	return body
}

func bodyHasLocalDecl(body *wrapperchecker.Node, name string) bool {
	found := false
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindVariableStatement {
			return false
		}
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
						found = true
					}
					return false
				})
			}
			return false
		})
		return false
	})
	return found
}
