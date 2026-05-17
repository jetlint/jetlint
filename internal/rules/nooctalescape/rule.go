// Package nooctalescape implements no-octal-escape: octal string
// escapes are deprecated in strict mode and confusing — use `\xNN` or
// `\uNNNN`.
package nooctalescape

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-octal-escape"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindStringLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	if len(src) < 2 {
		return
	}
	// Walk inside the quotes and look for octal escapes.
	for i := 1; i < len(src)-1; i++ {
		if src[i] != '\\' {
			continue
		}
		if i+1 >= len(src)-1 {
			break
		}
		next := src[i+1]
		if next == '\\' {
			// Escaped backslash — skip both characters.
			i++
			continue
		}
		if next >= '1' && next <= '7' {
			ctx.Report(n, "octal escape `\\"+string(next)+"...` is deprecated; use `\\xNN` or `\\uNNNN`")
			return
		}
		if next == '0' && i+2 < len(src)-1 {
			second := src[i+2]
			if second >= '0' && second <= '9' {
				ctx.Report(n, "octal escape `\\0"+string(second)+"...` is deprecated; use `\\xNN` or `\\uNNNN`")
				return
			}
		}
	}
}
