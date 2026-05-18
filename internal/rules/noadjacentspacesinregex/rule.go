// Package noadjacentspacesinregex implements no-adjacent-spaces-in-regex:
// `/foo  bar/` looks like one space, so two-space sequences in regex
// literals are a frequent typo. Write `/foo {2}bar/` or use a single
// space.
package noadjacentspacesinregex

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-adjacent-spaces-in-regex"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindRegularExpressionLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	// Strip leading `/` and trailing `/<flags>`.
	if !strings.HasPrefix(src, "/") {
		return
	}
	closing := strings.LastIndex(src, "/")
	if closing <= 0 {
		return
	}
	pattern := src[1:closing]
	if hasAdjacentSpaces(pattern) {
		ctx.Report(n, "adjacent spaces in regex — use `{N}` or escape explicitly")
	}
}

func hasAdjacentSpaces(pattern string) bool {
	inClass := false
	escaping := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if escaping {
			escaping = false
			continue
		}
		if c == '\\' {
			escaping = true
			continue
		}
		if c == '[' {
			inClass = true
			continue
		}
		if c == ']' {
			inClass = false
			continue
		}
		if !inClass && c == ' ' && i+1 < len(pattern) && pattern[i+1] == ' ' {
			return true
		}
	}
	return false
}
