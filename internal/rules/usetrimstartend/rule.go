// Package usetrimstartend implements use-trim-start-end:
// `trimLeft`/`trimRight` are non-standard MDN aliases. `trimStart`/
// `trimEnd` match the spec.
package usetrimstartend

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-trim-start-end"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	_, name := propParts(n)
	if name != "trimLeft" && name != "trimRight" {
		return
	}
	// Must be the callee of a no-arg CallExpression. We detect by
	// looking at the parent's source: it must end with `()` (with
	// optional comments/whitespace inside).
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindCallExpression {
		return
	}
	psrc := p.SourceText()
	// Must start with the PropertyAccess's source and end with `()`.
	if len(psrc) < 2 {
		return
	}
	end := strings.TrimSpace(psrc[len(psrc)-2:])
	if end != "()" {
		return
	}
	if name == "trimLeft" {
		ctx.Report(n, "use trimStart instead of trimLeft")
	} else {
		ctx.Report(n, "use trimEnd instead of trimRight")
	}
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
