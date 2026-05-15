// Package notemplatecurlyinstring implements the no-template-curly-in-string
// rule: a single-quoted or double-quoted string containing `${...}` syntax
// is almost always a mistake — the author meant a template literal
// (backticks), which actually interpolates the expression. With regular
// quotes the placeholder ships as the literal string `${...}`.
package notemplatecurlyinstring

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-template-curly-in-string"

// New constructs a notemplatecurlyinstring rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindStringLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	text := n.LiteralText()
	start := strings.Index(text, "${")
	if start < 0 {
		return
	}
	if !strings.Contains(text[start+2:], "}") {
		return
	}
	ctx.Report(n, "template placeholders will not interpolate in regular strings")
}
