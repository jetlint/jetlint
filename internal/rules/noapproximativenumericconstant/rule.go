// Package noapproximativenumericconstant implements
// no-approximative-numeric-constant: a literal like `3.141` is just
// a rounded `Math.PI` — using the named constant keeps full precision
// and signals intent.
package noapproximativenumericconstant

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-approximative-numeric-constant"

// Each constant has the full digit string (no decimal point, no minus).
var constants = []struct {
	name string
	full string // digits including before-decimal, e.g. "3141592653589793"
	dot  int    // position of the implicit decimal point (digits before it)
}{
	{"Math.E", "2718281828459045", 1},
	{"Math.LN10", "2302585092994046", 1},
	{"Math.LN2", "06931471805599453", 1},
	{"Math.LOG10E", "04342944819032518", 1},
	{"Math.LOG2E", "14426950408889634", 1},
	{"Math.PI", "3141592653589793", 1},
	{"Math.SQRT1_2", "07071067811865476", 1},
	{"Math.SQRT2", "14142135623730951", 1},
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNumericLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	text := strings.ReplaceAll(n.SourceText(), "_", "")
	digits, dot := stripDecimal(text)
	// Must have at least 3 digits after the decimal point.
	if dot < 0 || len(digits)-dot < 3 {
		return
	}
	for _, c := range constants {
		if dot != c.dot {
			continue
		}
		// Compare digit-by-digit up to min length.
		minLen := min(len(digits), len(c.full))
		if minLen < c.dot+3 {
			continue
		}
		if digits[:minLen] != c.full[:minLen] {
			continue
		}
		ctx.Report(n, "use `"+c.name+"` instead of an approximation")
		return
	}
}

func stripDecimal(text string) (digits string, dotPos int) {
	dotPos = -1
	for i := 0; i < len(text); i++ {
		if text[i] == '.' {
			dotPos = len(digits)
			continue
		}
		if text[i] >= '0' && text[i] <= '9' {
			digits += string(text[i])
			continue
		}
		// Stop at non-digit (e.g., `e`, `E`).
		break
	}
	return digits, dotPos
}
