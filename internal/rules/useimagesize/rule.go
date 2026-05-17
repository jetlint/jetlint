// Package useimagesize implements use-image-size: an `<img>` JSX
// element without both `width` and `height` lets the browser
// reflow the page once the image loads, causing layout shift.
// Setting both reserves space up front and keeps Core Web Vitals
// (CLS) clean. The rule flags any `<img>` (self-closing or paired)
// missing either attribute.
package useimagesize

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-image-size"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxSelfClosingElement: visitSelfClosing,
		wrapperchecker.KindJsxOpeningElement:     visitOpening,
	}
}

func visitSelfClosing(ctx *engine.Context, el *wrapperchecker.Node) {
	checkImg(ctx, el)
}

func visitOpening(ctx *engine.Context, el *wrapperchecker.Node) {
	checkImg(ctx, el)
}

func checkImg(ctx *engine.Context, el *wrapperchecker.Node) {
	if tagName(el) != "img" {
		return
	}
	hasWidth, hasHeight := collectImgAttrs(el)
	if hasWidth && hasHeight {
		return
	}
	switch {
	case !hasWidth && !hasHeight:
		ctx.Report(el, "add `width` and `height` to this `<img>` to avoid layout shift on load")
	case !hasWidth:
		ctx.Report(el, "add `width` to this `<img>` to avoid layout shift on load")
	default:
		ctx.Report(el, "add `height` to this `<img>` to avoid layout shift on load")
	}
}

// tagName returns the tag identifier of a JSX opening or
// self-closing element. JSX intrinsic tags like `img` come through
// as KindIdentifier; namespaced tags like `svg:rect` are skipped
// (this rule only cares about plain `img`).
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

func collectImgAttrs(el *wrapperchecker.Node) (hasWidth, hasHeight bool) {
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttributes {
			return false
		}
		c.ForEachChild(func(attr *wrapperchecker.Node) bool {
			if attr.Kind() != wrapperchecker.KindJsxAttribute {
				return false
			}
			switch attributeName(attr) {
			case "width":
				hasWidth = true
			case "height":
				hasHeight = true
			}
			return false
		})
		return true
	})
	return
}

func attributeName(attr *wrapperchecker.Node) string {
	var name string
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
