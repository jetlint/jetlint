// Package usegooglefontdisplay implements use-google-font-display:
// when embedding Google Fonts, set `&display=optional` (or another
// `font-display` value other than `auto`/`block`/`fallback`) so the
// browser doesn't block render on the font fetch.
package usegooglefontdisplay

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-google-font-display"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxAttribute: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var name, value *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if name == nil {
			name = c
		} else if value == nil {
			value = c
		}
		return false
	})
	if name == nil || value == nil || name.SourceText() != "href" {
		return
	}
	url := extractURL(value)
	if !strings.Contains(url, "fonts.googleapis.com") {
		return
	}
	disp := extractDisplay(url)
	switch disp {
	case "", "auto", "block", "fallback":
		ctx.Report(n, "Google Fonts URL should use `&display=optional` (or similar non-blocking value)")
	}
}

func extractURL(value *wrapperchecker.Node) string {
	switch value.Kind() {
	case wrapperchecker.KindStringLiteral:
		return strings.Trim(value.SourceText(), `"'`+"`")
	case wrapperchecker.KindJsxExpression:
		var inner *wrapperchecker.Node
		value.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner == nil {
			return ""
		}
		switch inner.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return strings.Trim(inner.SourceText(), `"'`+"`")
		}
	}
	return ""
}

func extractDisplay(url string) string {
	for part := range strings.SplitSeq(url, "&") {
		if k, v, ok := strings.Cut(part, "="); ok && strings.HasSuffix(k, "display") {
			return v
		}
	}
	return ""
}
