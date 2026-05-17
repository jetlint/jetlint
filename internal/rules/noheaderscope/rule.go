// Package noheaderscope implements no-header-scope: the `scope`
// attribute belongs on <th> (table header cell). Putting it on any
// other DOM element is a typo or a misunderstanding — assistive
// tech ignores it there.
package noheaderscope

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-header-scope"

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
	if !jsxutil.IsHTMLElement(tag) || tag == "th" {
		return
	}
	attr := jsxutil.FindAttribute(jsxutil.AttributesNode(el), "scope")
	if attr == nil {
		return
	}
	ctx.Report(attr, "the `scope` attribute is only meaningful on <th>")
}
