// Package noimplicitanylet implements no-implicit-any-let: declaring
// `let x;` without a type annotation or initializer makes `x` an
// implicit `any` — give it a type.
package noimplicitanylet

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-implicit-any-let"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	list := n.Parent()
	if list == nil || list.Kind() != wrapperchecker.KindVariableDeclarationList {
		return
	}
	src := strings.TrimSpace(list.SourceText())
	// Only flag `let`/`var` (not `const`/`using`).
	if !strings.HasPrefix(src, "let") && !strings.HasPrefix(src, "var") {
		return
	}
	// Skip if list is in for-in / for-of header.
	if lp := list.Parent(); lp != nil {
		switch lp.Kind() {
		case wrapperchecker.KindForInStatement, wrapperchecker.KindForOfStatement:
			return
		}
	}
	var ident *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if ident == nil {
			ident = c
		}
		return false
	})
	if ident == nil || ident.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	declSrc := n.SourceText()
	if strings.ContainsAny(declSrc, ":=") {
		return
	}
	ctx.Report(n, "`let`/`var` declaration without type annotation or initializer is implicit `any`")
}

