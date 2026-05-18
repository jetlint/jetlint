// Package norestrictedelements implements no-restricted-elements: a
// project-configurable list of JSX/HTML tags that shouldn't appear
// (e.g. raw <img> in a Next.js app where <NextImage> is preferred).
package norestrictedelements

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-restricted-elements"

// Options configures the rule. `Elements` maps the disallowed tag
// name to a free-form message shown in the diagnostic.
type Options struct {
	Elements map[string]string
}

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
	opts, ok := ctx.Options().(*Options)
	if !ok || opts == nil || len(opts.Elements) == 0 {
		return
	}
	tag := jsxutil.TagName(el)
	hint, found := opts.Elements[tag]
	if !found {
		return
	}
	msg := "<" + tag + "> is restricted in this codebase"
	if hint != "" {
		msg += " — " + hint
	}
	ctx.Report(el, msg)
}
