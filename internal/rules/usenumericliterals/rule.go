// Package usenumericliterals implements use-numeric-literals:
// `parseInt('11', 2)`, `parseInt('1F7', 16)`, etc. can be written as
// binary/hex/octal literals (`0b11`, `0x1F7`).
package usenumericliterals

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-numeric-literals"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isParseIntCallee(firstChild(n)) {
		return
	}
	args := callArgs(n)
	if len(args) != 2 {
		return
	}
	if !isStringLiteralLike(args[0]) {
		return
	}
	radix := args[1].SourceText()
	if radix != "2" && radix != "8" && radix != "16" {
		return
	}
	if isShadowed(n, "parseInt") {
		return
	}
	ctx.Report(n, "use a numeric literal instead of `parseInt(string, radix)`")
}

func isParseIntCallee(callee *wrapperchecker.Node) bool {
	if callee == nil {
		return false
	}
	if callee.Kind() == wrapperchecker.KindIdentifier && callee.SourceText() == "parseInt" {
		return true
	}
	if callee.Kind() == wrapperchecker.KindPropertyAccessExpression {
		var obj, prop *wrapperchecker.Node
		callee.ForEachChild(func(c *wrapperchecker.Node) bool {
			if obj == nil {
				obj = c
			} else if prop == nil {
				prop = c
			}
			return false
		})
		if obj == nil || prop == nil {
			return false
		}
		if obj.Kind() == wrapperchecker.KindIdentifier && obj.SourceText() == "Number" && prop.SourceText() == "parseInt" {
			return true
		}
	}
	return false
}

func isStringLiteralLike(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
		// Skip empty.
		txt := strings.Trim(n.SourceText(), `"'`+"`")
		return txt != ""
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

func declaresName(c *wrapperchecker.Node, name string) bool {
	switch c.Kind() {
	case wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration:
		var first *wrapperchecker.Node
		c.ForEachChild(func(d *wrapperchecker.Node) bool {
			if first == nil && d.Kind() == wrapperchecker.KindIdentifier {
				first = d
			}
			return false
		})
		return first != nil && first.SourceText() == name
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
	}
	return false
}
