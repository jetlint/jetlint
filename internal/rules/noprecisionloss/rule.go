// Package noprecisionloss implements no-precision-loss, biome's
// rename of no-loss-of-precision. The detection logic mirrors
// nolossofprecision but adds biome's "ignore extra zeros" carveout:
// a decimal literal whose mantissa has at most 15 significant digits
// after trailing zeros are stripped is treated as exact, even though
// the underlying f64 may round it. The carveout exists because
// developers commonly pad numbers with zeros for readability (price
// columns, scientific notation, etc.) — biome reads that as intent
// rather than imprecision.
package noprecisionloss

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/nolossofprecision"
)

const id = "no-precision-loss"

const biomeIgnoredSignificantDigits = 15

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	inner := nolossofprecision.New().Handlers()
	out := make(map[wrapperchecker.Kind]engine.Handler, len(inner))
	for k, h := range inner {
		handler := h
		out[k] = func(ctx *engine.Context, n *wrapperchecker.Node) {
			if biomeIgnoresLiteral(n.SourceText()) {
				return
			}
			handler(ctx, n)
		}
	}
	return out
}

// biomeIgnoresLiteral mirrors the "Ignore extra zeros" carveout in
// biome's noPrecisionLoss fixture: a base-10 literal whose mantissa
// has at most 15 significant decimal digits (counting only digits
// from the first to the last non-zero) is treated as exact even if
// the source has many additional trailing zeros that the underlying
// f64 cannot represent precisely.
func biomeIgnoresLiteral(raw string) bool {
	if raw == "" {
		return false
	}
	s := strings.TrimLeft(raw, "+-")
	if len(s) >= 2 && s[0] == '0' {
		switch s[1] {
		case 'b', 'B', 'o', 'O', 'x', 'X':
			return false
		}
	}
	mantissa := s
	for i := 0; i < len(mantissa); i++ {
		if mantissa[i] == 'e' || mantissa[i] == 'E' {
			mantissa = mantissa[:i]
			break
		}
	}
	first, last := -1, -1
	digitIdx := 0
	for i := 0; i < len(mantissa); i++ {
		c := mantissa[i]
		if c == '_' || c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		if c != '0' {
			if first == -1 {
				first = digitIdx
			}
			last = digitIdx
		}
		digitIdx++
	}
	if first == -1 {
		return true
	}
	return last-first+1 <= biomeIgnoredSignificantDigits
}
