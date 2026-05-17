// Package noautofocus implements no-autofocus: flag the `autoFocus`
// attribute on built-in HTML elements. Forcing focus on page load
// disorients screen reader and keyboard users, and the browser's
// scroll-to-focus behavior wipes out users' scroll position.
//
// Carve-outs match biome:
//   - <dialog> ancestors expect a focused element on open;
//   - elements carrying `popover` similarly run a focus dance the
//     developer presumably wants.
package noautofocus

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-autofocus"

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
	if !jsxutil.IsHTMLElement(jsxutil.TagName(el)) {
		return
	}
	attr := jsxutil.FindAttribute(jsxutil.AttributesNode(el), "autoFocus")
	if attr == nil {
		return
	}
	if jsxutil.AttributeIsNullishExpression(attr) {
		return
	}
	if insideDialogOrPopover(el) {
		return
	}
	ctx.Report(attr, "autoFocus on a DOM element disrupts keyboard and screen-reader users — remove it")
}

func insideDialogOrPopover(n *wrapperchecker.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		t := strings.TrimSpace(p.SourceText())
		if !strings.HasPrefix(t, "<") {
			continue
		}
		// Limit checks to the opening tag — children can contain
		// the same keywords without changing what the wrapper is.
		open := t
		if i := strings.Index(t, ">"); i >= 0 {
			open = t[:i]
		}
		if strings.HasPrefix(open, "<dialog>") || strings.HasPrefix(open, "<dialog ") || open == "<dialog" {
			return true
		}
		if strings.Contains(open, " popover") {
			return true
		}
	}
	return false
}
