// Package noblanktarget implements no-blank-target: flag JSX
// `<a target="_blank">`, `<area target="_blank">`, and
// `<form target="_blank">` that don't carry `rel="noopener"` or
// `rel="noreferrer"`. Without rel, the new tab can navigate the
// opener via `window.opener` — a known phishing vector.
//
// Carve-outs match biome:
//   - target is a non-literal expression (`target={dynamic}`) — skip,
//     biome treats it as opt-in user intent;
//   - element spreads (`{...props}`) anywhere — skip, props might
//     supply the needed rel;
//   - the tag is not one of the navigable HTML elements (a / area
//     / form lowercase only — `<Link>` is a component).
package noblanktarget

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-blank-target"

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
	tag := jsxTagName(el)
	switch tag {
	case "a", "area", "form":
	default:
		return
	}
	attrs := jsxAttributesNode(el)
	if attrs == nil {
		return
	}
	if hasSpread(attrs) {
		return
	}
	target := findAttribute(attrs, "target")
	if target == nil {
		return
	}
	tv, literal := attrLiteralStringValue(target)
	if !literal || tv != "_blank" {
		return
	}
	rel := findAttribute(attrs, "rel")
	if rel == nil {
		ctx.Report(el, `<a target="_blank"> without rel="noopener" or rel="noreferrer" exposes window.opener`)
		return
	}
	rv, relLiteral := attrLiteralStringValue(rel)
	if !relLiteral {
		return
	}
	if !relSatisfies(rv) {
		ctx.Report(el, `<a target="_blank"> needs rel="noopener" or rel="noreferrer"`)
	}
}

// relSatisfies reports whether the rel value contains noopener or
// noreferrer as a whitespace-separated token.
func relSatisfies(v string) bool {
	for tok := range strings.FieldsSeq(v) {
		if tok == "noopener" || tok == "noreferrer" {
			return true
		}
	}
	return false
}

func jsxTagName(el *wrapperchecker.Node) string {
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

func jsxAttributesNode(el *wrapperchecker.Node) *wrapperchecker.Node {
	var out *wrapperchecker.Node
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindJsxAttributes {
			out = c
			return true
		}
		return false
	})
	return out
}

// hasSpread looks for `{...x}` spread attributes. The wrapper
// doesn't surface KindJsxSpreadAttribute, so we detect the pattern
// in the attributes node's source text.
func hasSpread(attrs *wrapperchecker.Node) bool {
	return strings.Contains(attrs.SourceText(), "{...")
}

func findAttribute(attrs *wrapperchecker.Node, name string) *wrapperchecker.Node {
	var out *wrapperchecker.Node
	attrs.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttribute {
			return false
		}
		if jsxAttributeName(c) == name {
			out = c
			return true
		}
		return false
	})
	return out
}

func jsxAttributeName(attr *wrapperchecker.Node) string {
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

// attrLiteralStringValue returns (value, literal). literal=true
// means the attribute had a static string value (either string
// literal, template with no substitutions, or `{ "..." }` JSX
// expression with a literal inside). literal=false means the
// attribute is missing a value or is a non-literal expression.
func attrLiteralStringValue(attr *wrapperchecker.Node) (string, bool) {
	var val string
	var literal bool
	// Walk children: first identifier is the name; subsequent is value.
	seenName := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName && c.Kind() == wrapperchecker.KindIdentifier {
			seenName = true
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
			val = c.LiteralText()
			literal = true
			return true
		case wrapperchecker.KindJsxExpression:
			c.ForEachChild(func(e *wrapperchecker.Node) bool {
				switch e.Kind() {
				case wrapperchecker.KindStringLiteral, wrapperchecker.KindNoSubstitutionTemplateLiteral:
					val = e.LiteralText()
					literal = true
				}
				return true
			})
			return true
		}
		return false
	})
	return val, literal
}
