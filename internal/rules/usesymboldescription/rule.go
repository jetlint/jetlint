// Package usesymboldescription implements use-symbol-description: a
// symbol's description shows up in `toString` and stack traces.
// Provide one — debugging is hard enough.
package usesymboldescription

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-symbol-description"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := firstChild(n)
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier || callee.SourceText() != "Symbol" {
		return
	}
	// Count args.
	argCount := 0
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			argCount++
		}
		idx++
		return false
	})
	if isShadowed(n, "Symbol") {
		return
	}
	if argCount == 0 {
		ctx.Report(n, "Symbol() needs a description — anonymous symbols are a debugging hazard")
		return
	}
	// Get first arg; if it's an empty string-like literal, flag.
	var firstArg *wrapperchecker.Node
	idx2 := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx2 > 0 && firstArg == nil && c.Kind() != wrapperchecker.KindTypeReference {
			firstArg = c
		}
		idx2++
		return false
	})
	if firstArg == nil {
		return
	}
	switch firstArg.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		if firstArg.LiteralText() == "" {
			ctx.Report(n, "Symbol(\"\") is no help — give the symbol a meaningful description")
		}
	}
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	prev := start
	for p := start.Parent(); p != nil; p = p.Parent() {
		found := false
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c == prev {
				return false
			}
			if c.Kind() == wrapperchecker.KindVariableStatement {
				c.ForEachChild(func(cc *wrapperchecker.Node) bool {
					if cc.Kind() != wrapperchecker.KindVariableDeclarationList {
						return false
					}
					cc.ForEachChild(func(d *wrapperchecker.Node) bool {
						if d.Kind() == wrapperchecker.KindVariableDeclaration && declIdentName(d) == name {
							found = true
							return true
						}
						return false
					})
					return found
				})
			}
			return found
		})
		if found {
			return true
		}
		prev = p
	}
	return false
}

func declIdentName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
		return first.SourceText()
	}
	return ""
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
