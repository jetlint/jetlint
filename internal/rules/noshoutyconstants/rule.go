// Package noshoutyconstants implements no-shouty-constants: declaring
// `const FOO = "FOO"` is wasted ceremony — the identifier and the
// string carry the same information. Use the string literal directly.
package noshoutyconstants

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-shouty-constants"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Must be inside a const declaration.
	if p := n.Parent(); p == nil || p.Kind() != wrapperchecker.KindVariableDeclarationList {
		return
	} else if !strings.HasPrefix(strings.TrimSpace(p.SourceText()), "const ") {
		return
	}
	var name, init *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch idx {
		case 0:
			name = c
		default:
			init = c
		}
		idx++
		return false
	})
	if name == nil || init == nil {
		return
	}
	if name.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if init.Kind() != wrapperchecker.KindStringLiteral && init.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return
	}
	idName := name.SourceText()
	if !isAllUpper(idName) {
		return
	}
	if init.LiteralText() == idName {
		ctx.Report(n, "`const "+idName+" = \""+idName+"\"` repeats itself — use the literal directly")
	}
}

func isAllUpper(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
