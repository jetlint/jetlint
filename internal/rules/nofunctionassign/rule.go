// Package nofunctionassign implements no-function-assign: reassigning a
// `function foo() {}` declaration overwrites the binding everyone else
// imports/calls — almost always a bug.
package nofunctionassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-function-assign"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := n.BinaryOperatorKind()
	if !isAssign(op) {
		return
	}
	checkLHS(ctx, n, n.BinaryLeft())
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

func checkLHS(ctx *engine.Context, n, lhs *wrapperchecker.Node) {
	if lhs == nil {
		return
	}
	switch lhs.Kind() {
	case wrapperchecker.KindIdentifier:
		name := lhs.SourceText()
		if name == "" {
			return
		}
		if fn := findFnDecl(lhs, name); fn != nil {
			ctx.Report(n, "reassigning function declaration `"+name+"`")
		}
	case wrapperchecker.KindArrayLiteralExpression, wrapperchecker.KindObjectLiteralExpression:
		walkBindingPattern(ctx, n, lhs)
	}
}

func walkBindingPattern(ctx *engine.Context, n, pat *wrapperchecker.Node) {
	pat.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			name := c.SourceText()
			if fn := findFnDecl(c, name); fn != nil {
				ctx.Report(n, "reassigning function declaration `"+name+"`")
			}
		case wrapperchecker.KindPropertyAssignment, wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindBindingElement:
			walkBindingPattern(ctx, n, c)
		case wrapperchecker.KindBinaryExpression:
			// Default value: `{ x: foo = 0 }` parses as `x:` (Property) `foo = 0` (BinaryExpr).
			walkBindingPattern(ctx, n, c)
		case wrapperchecker.KindArrayLiteralExpression, wrapperchecker.KindObjectLiteralExpression:
			walkBindingPattern(ctx, n, c)
		case wrapperchecker.KindSpreadElement:
			walkBindingPattern(ctx, n, c)
		}
		return false
	})
}

// findFnDecl walks up scope chain. Returns the function declaration node
// if `name` resolves to a function declaration at some enclosing scope
// AND is not shadowed by a closer var/let/const/parameter declaration.
func findFnDecl(start *wrapperchecker.Node, name string) *wrapperchecker.Node {
	for p := start.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction, wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor:
			// Check params.
			if functionHasParam(p, name) {
				return nil
			}
			// Check body for var/let/const/fnDecl named name.
			if body := functionBody(p); body != nil {
				if hit := bodyShadows(body, name); hit != nil {
					if hit.Kind() == wrapperchecker.KindFunctionDeclaration {
						return hit
					}
					return nil
				}
			}
			// Function expressions can bind their own name.
			if p.Kind() == wrapperchecker.KindFunctionExpression {
				var fname *wrapperchecker.Node
				p.ForEachChild(func(c *wrapperchecker.Node) bool {
					if fname == nil && c.Kind() == wrapperchecker.KindIdentifier {
						fname = c
					}
					return false
				})
				if fname != nil && fname.SourceText() == name {
					return nil // function expression's own name — not assignable per ESLint
				}
			}
		case wrapperchecker.KindSourceFile, wrapperchecker.KindBlock:
			if hit := bodyShadows(p, name); hit != nil {
				if hit.Kind() == wrapperchecker.KindFunctionDeclaration {
					return hit
				}
				return nil
			}
		}
	}
	return nil
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

// bodyShadows returns the first declaration (fn decl or var-like) of
// `name` found at the top level of `body`, or nil.
func bodyShadows(body *wrapperchecker.Node, name string) *wrapperchecker.Node {
	var hit *wrapperchecker.Node
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		if hit != nil {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindFunctionDeclaration:
			var fname *wrapperchecker.Node
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if fname == nil && d.Kind() == wrapperchecker.KindIdentifier {
					fname = d
				}
				return false
			})
			if fname != nil && fname.SourceText() == name {
				hit = c
			}
		case wrapperchecker.KindVariableStatement:
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
							hit = decl
						}
						return false
					})
				}
				return false
			})
		case wrapperchecker.KindClassDeclaration:
			var cname *wrapperchecker.Node
			c.ForEachChild(func(d *wrapperchecker.Node) bool {
				if cname == nil && d.Kind() == wrapperchecker.KindIdentifier {
					cname = d
				}
				return false
			})
			if cname != nil && cname.SourceText() == name {
				hit = c
			}
		}
		return false
	})
	return hit
}

func functionHasParam(fn *wrapperchecker.Node, name string) bool {
	found := false
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			var pn *wrapperchecker.Node
			c.ForEachChild(func(p *wrapperchecker.Node) bool {
				if pn == nil {
					pn = p
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
