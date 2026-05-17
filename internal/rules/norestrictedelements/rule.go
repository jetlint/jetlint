// Package norestrictedelements implements no-restricted-elements:
// teams ban specific JSX tags (e.g., the bare `<img>` in favor of a
// `<NextImage>` wrapper, or the platform `<a>` in favor of a routed
// `<Link>`). The rule fires when an element's tag matches an entry
// in the configured restrictions map. The configuration is opaque
// to the engine — callers pass a *Options blob via the engine's
// per-rule options map.
package norestrictedelements

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-restricted-elements"

// Options configures the rule. Elements is keyed by the JSX tag
// name to flag; the value is the human-readable message that
// explains the replacement (e.g., "use <NextImage> instead").
// A nil or empty Options blob disables the rule for the file.
type Options struct {
	Elements map[string]string
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxSelfClosingElement: visit,
		wrapperchecker.KindJsxOpeningElement:     visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	opts, ok := ctx.Options().(*Options)
	if !ok || opts == nil || len(opts.Elements) == 0 {
		return
	}
	name := tagName(el)
	if name == "" {
		return
	}
	replacement, restricted := opts.Elements[name]
	if !restricted {
		return
	}
	msg := "<" + name + "> is restricted by project policy"
	if replacement != "" {
		msg = msg + ": " + replacement
	}
	ctx.Report(el, msg)
}

func tagName(el *wrapperchecker.Node) string {
	var name string
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
