// Package usemediacaption implements use-media-caption: <audio> and
// <video> need a <track kind="captions"> child so deaf/HoH users get
// the audio content. Other track kinds (subtitles, descriptions,
// chapters, metadata) don't count.
package usemediacaption

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-media-caption"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement: visit,
	}
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if tag != "audio" && tag != "video" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	if jsxutil.HasSpread(attrs) {
		return
	}
	if jsxutil.FindAttribute(attrs, "muted") != nil {
		return
	}
	parent := el.Parent()
	if parent == nil {
		return
	}
	// Iterate siblings of the opening tag within the element.
	hasCaptionsTrack := false
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		t := strings.TrimSpace(c.SourceText())
		if !strings.HasPrefix(t, "<track") {
			return false
		}
		// Look for kind="captions" (case-insensitive).
		lower := strings.ToLower(t)
		if strings.Contains(lower, `kind="captions"`) || strings.Contains(lower, `kind='captions'`) {
			hasCaptionsTrack = true
			return true
		}
		return false
	})
	if !hasCaptionsTrack {
		ctx.Report(el, "<audio>/<video> needs a <track kind=\"captions\"> child for deaf and HoH users")
	}
}
