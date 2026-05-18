// Package nosubstr implements no-substr: `String.prototype.substr`
// is a legacy MDN-deprecated API that overlaps `substring` and
// `slice`. Use `slice` for predictable behavior and a stable spec.
package nosubstr

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-substr"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindPropertyAccessExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	_, name := parts(n)
	switch name {
	case "substr":
		ctx.Report(n, "String.prototype.substr is legacy — use slice")
	case "substring":
		ctx.Report(n, "String.prototype.substring is confusing — use slice")
	}
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
