// Package nostaticelementinteractions implements
// no-static-element-interactions: putting an interactive handler on
// a "static" element (div, span, …) with no role gives screen-reader
// users no signal that it does anything. Add a concrete interactive
// role (button, link, …) and a tabIndex.
package nostaticelementinteractions

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "no-static-element-interactions"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// "Static" HTML tags — no implicit interactive or non-interactive role.
var staticTags = map[string]bool{
	"a": true, "area": true, "b": true, "base": true, "bdi": true, "bdo": true,
	"body": true, "cite": true, "col": true, "colgroup": true, "data": true,
	"div": true, "head": true, "header": true, "hgroup": true,
	"i": true, "kbd": true, "map": true, "meta": true, "noscript": true,
	"object": true, "picture": true, "q": true, "rp": true, "rt": true, "s": true,
	"samp": true, "script": true, "section": true, "small": true, "source": true,
	"span": true, "style": true, "title": true, "track": true, "u": true,
	"var": true, "wbr": true,
}

// Roles that DON'T count as giving an element semantic meaning:
// presentation/none explicitly opt out, and the WAI-ARIA "abstract"
// roles are not allowed in author markup at all.
var rolesThatDontCount = map[string]bool{
	"presentation": true, "none": true,
	"command": true, "composite": true, "input": true, "landmark": true,
	"range": true, "roletype": true, "sectionhead": true, "select": true,
	"structure": true, "widget": true, "window": true,
}

// Handlers the rule flags. Limited to mouse/keyboard click/keypress
// surface — touch/input/media handlers aren't "interactive" for this
// rule (they fire for things you'd legitimately attach to a div).
var interactiveHandlers = []string{
	"onClick", "onMouseDown", "onMouseUp", "onKeyDown", "onKeyUp", "onKeyPress",
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	tag := jsxutil.TagName(el)
	if !staticTags[tag] {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	// `<a href>` is interactive, not static.
	if tag == "a" || tag == "area" {
		if jsxutil.FindAttribute(attrs, "href") != nil {
			return
		}
	}
	// aria-hidden truthy → not in a11y tree.
	if h := jsxutil.FindAttribute(attrs, "aria-hidden"); h != nil {
		if v, ok := jsxutil.AttributeStringValue(h); ok {
			if v != "false" {
				return
			}
		} else {
			// bare attribute or {true} expression treated as truthy.
			return
		}
	}
	// Any concrete role (other than presentation/none/abstract) gives
	// the element semantic meaning, which is what the rule asks for.
	if r := jsxutil.FindAttribute(attrs, "role"); r != nil {
		if v, ok := jsxutil.AttributeStringValue(r); ok && v != "" && !rolesThatDontCount[v] {
			return
		}
	}
	// An accessible name is enough for some elements to escape the rule.
	if jsxutil.FindAttribute(attrs, "aria-label") != nil ||
		jsxutil.FindAttribute(attrs, "aria-labelledby") != nil {
		return
	}
	for _, name := range interactiveHandlers {
		h := jsxutil.FindAttribute(attrs, name)
		if h == nil {
			continue
		}
		// Allow {null} handlers — explicit no-op.
		if attributeIsNull(h) {
			continue
		}
		ctx.Report(h, name+" on a static element — give it an interactive role + tabIndex (or use a real control)")
		return
	}
}

func attributeIsNull(attr *wrapperchecker.Node) bool {
	isNull := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxExpression {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			if e.Kind() == wrapperchecker.KindNullKeyword {
				isNull = true
			}
			return true
		})
		return true
	})
	return isNull
}
