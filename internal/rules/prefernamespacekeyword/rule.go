// Package prefernamespacekeyword implements prefer-namespace-keyword:
// TypeScript still parses `module Foo {}` as a namespace declaration,
// but it's the legacy spelling. Use `namespace` instead. The rule does
// not touch ambient external modules (`declare module 'foo'`) or
// `declare global {}`.
package prefernamespacekeyword

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "prefer-namespace-keyword"

// New constructs a prefer-namespace-keyword rule.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindModuleDeclaration: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip inner segments of qualified names like `module A.B.C {}`,
	// where each segment is a TSModuleDeclaration whose direct parent
	// is the enclosing TSModuleDeclaration. The outermost segment
	// owns the keyword; reporting inner segments would double-flag.
	if p := n.Parent(); p != nil && p.Kind() == wrapperchecker.KindModuleDeclaration {
		return
	}
	// String-named modules (`declare module 'foo'`) describe ambient
	// external modules — exempt.
	if hasStringName(n) {
		return
	}
	if leadingKeyword(n.SourceText()) != "module" {
		return
	}
	ctx.Report(n, "Use `namespace` keyword instead of `module`.")
}

func hasStringName(n *wrapperchecker.Node) bool {
	out := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStringLiteral {
			out = true
			return true
		}
		return false
	})
	return out
}

// leadingKeyword returns the first identifier-shaped token in src
// after stripping leading whitespace, comments, and modifier
// keywords (`export`, `declare`). Returns "" if no such token is
// found.
func leadingKeyword(src string) string {
	s := src
	for {
		s = strings.TrimLeft(s, " \t\n\r")
		if strings.HasPrefix(s, "/*") {
			i := strings.Index(s, "*/")
			if i < 0 {
				return ""
			}
			s = s[i+2:]
			continue
		}
		if strings.HasPrefix(s, "//") {
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				return ""
			}
			s = s[i+1:]
			continue
		}
		stripped := false
		for _, mod := range [...]string{"export", "declare"} {
			if hasKeywordPrefix(s, mod) {
				s = s[len(mod):]
				stripped = true
				break
			}
		}
		if stripped {
			continue
		}
		break
	}
	end := 0
	for end < len(s) && isIdentPart(s[end]) {
		end++
	}
	return s[:end]
}

func hasKeywordPrefix(s, kw string) bool {
	if !strings.HasPrefix(s, kw) {
		return false
	}
	if len(s) == len(kw) {
		return true
	}
	return !isIdentPart(s[len(kw)])
}

func isIdentPart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$' || (c >= '0' && c <= '9')
}
