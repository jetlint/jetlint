// Package jsxutil bundles small helpers many JSX-focused rules need:
// extracting the tag name of a JSX element, walking its attributes,
// and reading literal string attribute values out of the
// wrapper.Node graph.
package jsxutil

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
)

// TagName returns the tag identifier for an opening or self-closing
// JSX element. For member-access tags (`<RadioGroup.Item>`,
// `<context.Provider>`) it returns "" — those are React components
// regardless of the leading identifier's case, so callers that
// only care about DOM elements can treat them uniformly.
func TagName(el *wrapperchecker.Node) string {
	var name string
	var memberAccess bool
	el.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			name = c.LiteralText()
			return true
		case wrapperchecker.KindPropertyAccessExpression:
			memberAccess = true
			return true
		}
		return false
	})
	if memberAccess {
		return ""
	}
	return name
}

// IsHTMLElement reports whether the tag is a lowercase HTML element
// (i.e. a built-in DOM element rather than a React component).
func IsHTMLElement(name string) bool {
	if name == "" {
		return false
	}
	c := name[0]
	return c >= 'a' && c <= 'z'
}

// AttributesNode returns the JsxAttributes child of an opening /
// self-closing element, or nil.
func AttributesNode(el *wrapperchecker.Node) *wrapperchecker.Node {
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

// FindAttribute returns the first JsxAttribute whose name matches.
// Returns nil if absent.
func FindAttribute(attrs *wrapperchecker.Node, name string) *wrapperchecker.Node {
	if attrs == nil {
		return nil
	}
	var out *wrapperchecker.Node
	attrs.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttribute {
			return false
		}
		if AttributeName(c) == name {
			out = c
			return true
		}
		return false
	})
	return out
}

// AttributeName returns the attribute identifier or "" for the
// shorthand `{...spread}` case.
func AttributeName(attr *wrapperchecker.Node) string {
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

// AttributeIsNullishExpression reports whether the attribute's value
// is the literal `{undefined}` or `{null}` — the idiomatic way to
// signal "no value" while keeping the prop present.
func AttributeIsNullishExpression(attr *wrapperchecker.Node) bool {
	hit := false
	seenName := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName && c.Kind() == wrapperchecker.KindIdentifier {
			seenName = true
			return false
		}
		if c.Kind() != wrapperchecker.KindJsxExpression {
			return false
		}
		c.ForEachChild(func(e *wrapperchecker.Node) bool {
			switch e.Kind() {
			case wrapperchecker.KindNullKeyword:
				hit = true
			case wrapperchecker.KindIdentifier:
				if e.LiteralText() == "undefined" {
					hit = true
				}
			}
			return true
		})
		return true
	})
	return hit
}

// HasSpread reports whether the JsxAttributes node contains a
// `{...x}` spread attribute. The wrapper doesn't expose
// KindJsxSpreadAttribute, so we detect the pattern in source text.
func HasSpread(attrs *wrapperchecker.Node) bool {
	if attrs == nil {
		return false
	}
	t := attrs.SourceText()
	for i := 0; i < len(t)-3; i++ {
		if t[i] == '{' && t[i+1] == '.' && t[i+2] == '.' && t[i+3] == '.' {
			return true
		}
	}
	return false
}

// AttributeStringValue returns (value, literal). literal=true means
// the attribute had a static string (string-literal, template with
// no substitutions, or `{ "..." }` JSX expression around either).
func AttributeStringValue(attr *wrapperchecker.Node) (string, bool) {
	var val string
	var ok bool
	seenName := false
	attr.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenName && c.Kind() == wrapperchecker.KindIdentifier {
			seenName = true
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			val = c.LiteralText()
			ok = true
			return true
		case wrapperchecker.KindJsxExpression:
			c.ForEachChild(func(e *wrapperchecker.Node) bool {
				switch e.Kind() {
				case wrapperchecker.KindStringLiteral,
					wrapperchecker.KindNoSubstitutionTemplateLiteral:
					val = e.LiteralText()
					ok = true
				}
				return true
			})
			return true
		}
		return false
	})
	return val, ok
}
