// Package usenumbertofixeddigitsargument implements
// use-number-to-fixed-digits-argument: `n.toFixed()` defaults to 0
// digits — usually not what callers meant. Pass the digit count
// explicitly.
package usenumbertofixeddigitsargument

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-number-to-fixed-digits-argument"

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
	_, name := propParts(callee)
	if name != "toFixed" {
		return
	}
	argCount := 0
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && c.Kind() != wrapperchecker.KindTypeReference {
			argCount++
		}
		idx++
		return false
	})
	if argCount != 0 {
		return
	}
	// Skip optional chains.
	if strings.Contains(n.SourceText(), "?.") {
		return
	}
	// Skip when called on a `new X()` expression — usually a custom class.
	obj, _ := propParts(callee)
	if obj != nil && obj.Kind() == wrapperchecker.KindNewExpression {
		return
	}
	ctx.Report(n, ".toFixed() defaults to 0 digits — pass the digit count explicitly")
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
