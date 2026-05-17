// Package useindexof implements use-index-of: `arr.findIndex(x => x === Y)`
// is just `arr.indexOf(Y)` (likewise for `findLastIndex` /
// `lastIndexOf`).
package useindexof

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-index-of"

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
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	_, prop := propParts(callee)
	switch prop {
	case "findIndex", "findLastIndex":
	default:
		return
	}
	args := callArgs(n)
	if len(args) != 1 {
		return
	}
	fn := args[0]
	if fn.Kind() != wrapperchecker.KindArrowFunction && fn.Kind() != wrapperchecker.KindFunctionExpression {
		return
	}
	// Function must take exactly one parameter.
	paramName := singleParamName(fn)
	if paramName == "" {
		return
	}
	body := arrowBody(fn)
	expr := extractSingleReturn(body)
	if expr == nil {
		return
	}
	if !isStrictEqWithParam(expr, paramName) {
		return
	}
	method := "indexOf"
	if prop == "findLastIndex" {
		method = "lastIndexOf"
	}
	ctx.Report(n, "use `."+method+"(...)` for a simple equality search")
}

func singleParamName(fn *wrapperchecker.Node) string {
	count := 0
	var name string
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			count++
			if count == 1 {
				var first *wrapperchecker.Node
				c.ForEachChild(func(d *wrapperchecker.Node) bool {
					if first == nil {
						first = d
					}
					return false
				})
				if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
					name = first.SourceText()
				}
			}
		}
		return false
	})
	if count != 1 {
		return ""
	}
	return name
}

func arrowBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var last *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

func extractSingleReturn(body *wrapperchecker.Node) *wrapperchecker.Node {
	if body == nil {
		return nil
	}
	if body.Kind() != wrapperchecker.KindBlock {
		// Concise body: the expression IS the body.
		return body
	}
	var stmts []*wrapperchecker.Node
	body.ForEachChild(func(c *wrapperchecker.Node) bool {
		stmts = append(stmts, c)
		return false
	})
	if len(stmts) != 1 {
		return nil
	}
	if stmts[0].Kind() != wrapperchecker.KindReturnStatement {
		return nil
	}
	var arg *wrapperchecker.Node
	stmts[0].ForEachChild(func(c *wrapperchecker.Node) bool {
		if arg == nil {
			arg = c
		}
		return false
	})
	return arg
}

func isStrictEqWithParam(expr *wrapperchecker.Node, param string) bool {
	if expr == nil {
		return false
	}
	if expr.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if expr.BinaryOperatorKind() != wrapperchecker.KindEqualsEqualsEqualsToken {
		return false
	}
	left := expr.BinaryLeft()
	right := expr.BinaryRight()
	if isIdentOf(left, param) && !referencesIdent(right, param) {
		return true
	}
	if isIdentOf(right, param) && !referencesIdent(left, param) {
		return true
	}
	return false
}

func isIdentOf(n *wrapperchecker.Node, name string) bool {
	return n != nil && n.Kind() == wrapperchecker.KindIdentifier && n.SourceText() == name
}

func referencesIdent(n *wrapperchecker.Node, name string) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindIdentifier && n.SourceText() == name {
		return true
	}
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if referencesIdent(c, name) {
			found = true
			return true
		}
		return false
	})
	return found
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

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
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
