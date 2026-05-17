// Package noforeach implements no-for-each: `for…of` reads more
// naturally, supports `break`/`continue`/`await`, and doesn't trip
// up control-flow linters. Reserve `.forEach` for places where the
// callback is genuinely the right shape.
package noforeach

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-for-each"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := unwrap(firstChild(n))
	name := ""
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		_, name = parts(callee)
	case wrapperchecker.KindElementAccessExpression:
		_, name = elemParts(callee)
	default:
		return
	}
	if name != "forEach" {
		return
	}
	args := callArgs(n)
	if len(args) == 0 {
		return
	}
	// Custom forEach (more than 2 args) — not Array.prototype.forEach.
	if len(args) > 2 {
		return
	}
	arity := callbackArity(args[0])
	// Identifier / unknown callback — can't judge intent.
	if arity < 0 {
		return
	}
	// 2+ arg callback uses index/array form — keep.
	if arity >= 2 {
		return
	}
	ctx.Report(n, ".forEach — prefer `for…of` for control flow and async support")
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first
}

func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = firstChild(n)
	}
	if n == nil {
		return nil
	}
	return n
}

func parts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
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

func elemParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	obj, key := parts(n)
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'' || key[0] == '`') {
		key = key[1 : len(key)-1]
	}
	return obj, key
}

func callArgs(n *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx >= 1 && c.Kind() != wrapperchecker.KindTypeReference {
			out = append(out, c)
		}
		idx++
		return false
	})
	return out
}

func callbackArity(n *wrapperchecker.Node) int {
	if n == nil {
		return 0
	}
	if n.Kind() != wrapperchecker.KindArrowFunction && n.Kind() != wrapperchecker.KindFunctionExpression {
		return -1 // unknown / not a literal callback — assume 0 to flag.
	}
	count := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			count++
		}
		return false
	})
	return count
}
