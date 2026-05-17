// Package useerrormessage implements use-error-message: throwing or
// constructing a built-in Error without a useful message string makes
// the stack trace useless. Pass a string.
package useerrormessage

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-error-message"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNewExpression:  visit,
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	name := callee.SourceText()
	msgIdx, isError := errorMessageArgIndex(name)
	if !isError {
		return
	}
	if isShadowed(n, name) {
		return
	}
	args := callArgs(n)
	// If there's a spread anywhere in the relevant arg positions, skip.
	for i := 0; i <= msgIdx && i < len(args); i++ {
		if args[i].Kind() == wrapperchecker.KindSpreadElement {
			return
		}
	}
	var msg *wrapperchecker.Node
	if msgIdx < len(args) {
		msg = args[msgIdx]
	}
	if !isBadMessage(msg) {
		return
	}
	ctx.Report(n, "pass a non-empty string as the error message")
}

func errorMessageArgIndex(name string) (int, bool) {
	switch name {
	case "Error", "EvalError", "InternalError", "RangeError", "ReferenceError",
		"SyntaxError", "TypeError", "URIError":
		return 0, true
	case "AggregateError":
		return 1, true
	}
	return 0, false
}

func isBadMessage(n *wrapperchecker.Node) bool {
	if n == nil {
		return true
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral:
		return n.SourceText() == `""` || n.SourceText() == `''`
	case wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return n.SourceText() == "``"
	case wrapperchecker.KindArrayLiteralExpression, wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindNumericLiteral, wrapperchecker.KindTrueKeyword, wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword:
		return true
	case wrapperchecker.KindIdentifier:
		t := n.SourceText()
		return t == "undefined" || t == "NaN"
	}
	return false
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}

func callArgs(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var args []*wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			args = append(args, c)
		}
		idx++
		return false
	})
	return args
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for p := start.Parent(); p != nil; p = p.Parent() {
		shadowed := false
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declaresName(c, name) {
				shadowed = true
				return true
			}
			return false
		})
		if shadowed {
			return true
		}
	}
	return false
}

func declaresName(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindVariableStatement:
		found := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindVariableDeclarationList {
				c.ForEachChild(func(d *wrapperchecker.Node) bool {
					var first *wrapperchecker.Node
					d.ForEachChild(func(x *wrapperchecker.Node) bool {
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
		return found
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration:
		var first *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil && c.Kind() == wrapperchecker.KindIdentifier {
				first = c
			}
			return false
		})
		return first != nil && first.SourceText() == name
	}
	return false
}
