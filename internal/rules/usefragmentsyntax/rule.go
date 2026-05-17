// Package usefragmentsyntax implements use-fragment-syntax: a bare
// `<>...</>` reads at a glance. `<Fragment>`/`<React.Fragment>` are
// the same thing dressed up.
package usefragmentsyntax

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-fragment-syntax"

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
	tag := jsxutil.TagName(el)
	if tag == "Fragment" {
		// Has no attributes? Then it's equivalent to <>.
		attrs := jsxutil.AttributesNode(el)
		if attrs == nil || !hasAnyChild(attrs) {
			ctx.Report(el, "use `<>` instead of `<Fragment>`")
		}
		return
	}
	// `<React.Fragment>` — TagName returns "" for member access; check source.
	src := el.SourceText()
	if hasReactFragmentSource(src) {
		attrs := jsxutil.AttributesNode(el)
		if attrs == nil || !hasAnyChild(attrs) {
			ctx.Report(el, "use `<>` instead of `<React.Fragment>`")
		}
	}
}

func hasReactFragmentSource(src string) bool {
	// Trim leading `<` and any whitespace, then check for "React.Fragment".
	if len(src) == 0 || src[0] != '<' {
		return false
	}
	rest := src[1:]
	if len(rest) >= len("React.Fragment") && rest[:len("React.Fragment")] == "React.Fragment" {
		return true
	}
	return false
}

func hasAnyChild(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(_ *wrapperchecker.Node) bool {
		found = true
		return true
	})
	return found
}
