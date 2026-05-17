// Package usenumericseparators implements use-numeric-separators:
// `1000000` is a wall of zeros. `1_000_000` reads as a million.
package usenumericseparators

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-numeric-separators"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNumericLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	src := n.SourceText()
	if strings.Contains(src, "_") {
		return
	}
	// Strip prefixes for hex/oct/bin and compare digit count.
	digits := src
	switch {
	case strings.HasPrefix(digits, "0x") || strings.HasPrefix(digits, "0X"):
		digits = digits[2:]
		if len(digits) > 4 {
			ctx.Report(n, "long hex literal — add `_` separators every 4 digits")
		}
		return
	case strings.HasPrefix(digits, "0b") || strings.HasPrefix(digits, "0B"):
		digits = digits[2:]
		if len(digits) > 4 {
			ctx.Report(n, "long binary literal — add `_` separators every 4 bits")
		}
		return
	case strings.HasPrefix(digits, "0o") || strings.HasPrefix(digits, "0O"):
		return
	}
	// Decimal: split on `.` and `e`/`E`. Flag if the integer part has > 4 digits.
	intPart := digits
	if i := strings.IndexAny(intPart, ".eE"); i >= 0 {
		intPart = intPart[:i]
	}
	if len(intPart) > 4 {
		ctx.Report(n, "long numeric literal — add `_` separators every 3 digits")
	}
}
