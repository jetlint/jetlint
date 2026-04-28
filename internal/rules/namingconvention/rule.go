// Package namingconvention implements the naming-convention rule.
// Default config: variables, functions, parameters, and class members
// must be camelCase, PascalCase, or UPPER_CASE.
package namingconvention

import (
	"strings"
	"unicode"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "naming-convention"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visitDecl,
		wrapperchecker.KindParameter:           visitDecl,
		wrapperchecker.KindFunctionDeclaration: visitDecl,
	}
}

func visitDecl(ctx *engine.Context, n *wrapperchecker.Node) {
	checkIdent(ctx, identName(n))
}

func identName(n *wrapperchecker.Node) *wrapperchecker.Node {
	var ident *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && ident == nil {
			ident = c
			return true
		}
		return false
	})
	return ident
}

func checkIdent(ctx *engine.Context, ident *wrapperchecker.Node) {
	if ident == nil {
		return
	}
	name := ident.LiteralText()
	if name == "" {
		return
	}
	if isCamelCase(name) || isUpperSnakeCase(name) || isPascalCase(name) {
		return
	}
	ctx.Report(ident, "name `"+name+"` doesn't match the camelCase / UPPER_CASE convention")
}

func isCamelCase(s string) bool {
	if s == "" {
		return false
	}
	s = strings.TrimLeft(s, "_")
	if s == "" {
		return true
	}
	first := firstRune(s)
	if !unicode.IsLower(first) && first != '$' {
		return false
	}
	for _, r := range s {
		if r == '_' {
			return false
		}
	}
	return true
}

func isPascalCase(s string) bool {
	if s == "" {
		return false
	}
	first := firstRune(s)
	if !unicode.IsUpper(first) {
		return false
	}
	for _, r := range s {
		if r == '_' {
			return false
		}
	}
	return true
}

func isUpperSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	hasUpper := false
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r), r == '_', r == '$':
		default:
			return false
		}
	}
	return hasUpper
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}
