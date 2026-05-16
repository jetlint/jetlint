// Package nononoctaldecimalescape implements no-nonoctal-decimal-escape:
// flag `\8` and `\9` inside a string literal. They are not valid
// octal escapes (octal only ranges 0–7) and the runtime falls back
// to producing the literal "8"/"9", which is almost never what the
// author intended.
//
// Detection works on the raw source text of each StringLiteral.
// The backslash before the digit is counted: an odd number of
// consecutive backslashes preceding `8`/`9` means the backslash
// itself is unescaped, so the `\8`/`\9` sequence is actually being
// interpreted as an escape. Even counts (`\\8`, `\\\\8`) are pairs
// of escaped backslashes followed by a literal digit and are fine.
//
// Template literals are intentionally out of scope: they have
// distinct escape semantics and biome's fixture doesn't exercise
// them here.
package nononoctaldecimalescape

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-nonoctal-decimal-escape"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindStringLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	text := n.SourceText()
	if len(text) < 2 {
		return
	}
	// Strip leading and trailing quote characters. Both `"..."` and
	// `'...'` reach here as string literals; either way the first
	// and last characters are the matching quote.
	inner := text[1 : len(text)-1]
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\\' {
			continue
		}
		j := i
		for j < len(inner) && inner[j] == '\\' {
			j++
		}
		count := j - i
		if count%2 == 1 && j < len(inner) && (inner[j] == '8' || inner[j] == '9') {
			ctx.Report(n, "don't use \\8 or \\9 escape sequences; they aren't valid")
			return
		}
		i = j - 1
	}
}
