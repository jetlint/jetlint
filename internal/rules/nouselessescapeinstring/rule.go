// Package nouselessescapeinstring implements no-useless-escape-in-string:
// `\a` (and friends) in a string literal isn't a JS escape — it's
// just the character `a` with extra noise.
package nouselessescapeinstring

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-useless-escape-in-string"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindStringLiteral:                  visit,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:  visit,
		wrapperchecker.KindTemplateHead:                   visit,
		wrapperchecker.KindTemplateMiddle:                 visit,
		wrapperchecker.KindTemplateTail:                   visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip JSX attribute strings.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindJsxAttribute {
		return
	}
	// Skip anything inside a tagged template — the tag function sees raw text.
	for a := n.Parent(); a != nil; a = a.Parent() {
		if a.Kind() == wrapperchecker.KindTaggedTemplateExpression {
			return
		}
	}
	src := n.SourceText()
	if len(src) < 2 {
		return
	}
	// Walk the contents.
	for i := 0; i < len(src)-1; i++ {
		if src[i] != '\\' {
			continue
		}
		next := src[i+1]
		if !isValidEscape(next) {
			ctx.Report(n, "useless escape sequence — drop the backslash")
			return
		}
		// Skip both characters.
		i++
	}
}

func isValidEscape(c byte) bool {
	switch c {
	case '\'', '"', '`', '\\', '/', 'b', 'f', 'n', 'r', 't', 'v', '0',
		'x', 'u', '\n', '\r', '$', '{', '}':
		return true
	}
	// Digits 1-9: legacy / could be octal.
	if c >= '1' && c <= '9' {
		return true
	}
	return false
}
