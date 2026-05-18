// Package nodistractingelements implements no-distracting-elements:
// flag <marquee> and <blink>. Both elements have been deprecated for
// over a decade; they trigger motion sickness and break screen
// readers.
package nodistractingelements

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-distracting-elements"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	switch jsxutil.TagName(el) {
	case "marquee", "blink":
		ctx.Report(el, "<marquee> and <blink> are deprecated and cause motion sickness — remove")
	}
}
