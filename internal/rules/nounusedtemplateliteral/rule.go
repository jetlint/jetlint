// Package nounusedtemplateliteral implements no-unused-template-literal:
// a template literal with no `${...}` substitution is just a quoted
// string — use the quoted form.
package nounusedtemplateliteral

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-template-literal"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNoSubstitutionTemplateLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip tagged templates — the tag may rely on raw form.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindTaggedTemplateExpression {
		return
	}
	text := n.LiteralText()
	// Skip if the content contains any quote — would force escaping.
	if strings.ContainsAny(text, `'"`) {
		return
	}
	// Skip if it contains a real newline (multiline literal).
	if strings.ContainsRune(text, '\n') {
		return
	}
	ctx.Report(n, "template literal with no substitution — use a normal string")
}
