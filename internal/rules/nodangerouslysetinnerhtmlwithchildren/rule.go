// Package nodangerouslysetinnerhtmlwithchildren implements the
// React lint rule that flags components combining the escape-hatch
// innerHTML prop with children. React silently drops the children
// when both are set, hiding the developer's intent.
//
// Detected shapes:
//   - JSX `<X PROP_NAME={...}>...</X>` (any children),
//   - JSX `<X PROP_NAME={...} children={...} />`,
//   - `React.createElement(type, { PROP_NAME, children })`,
//   - `React.createElement(type, { PROP_NAME }, ...childArgs)`.
package nodangerouslysetinnerhtmlwithchildren

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dangerously-set-inner-html-with-children"
const propName = "dangerouslySetInnerHTML"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visitSelfClosing,
		wrapperchecker.KindCallExpression:        visitCall,
	}
}

// visit handles `<X PROP>...</X>` — if the opening element carries
// the flagged prop AND the element has children (anything inside
// the open/close tags), report.
func visit(ctx *engine.Context, opener *wrapperchecker.Node) {
	if !hasFlaggedAttribute(opener) {
		return
	}
	parent := opener.Parent()
	if parent == nil {
		return
	}
	// The opener's parent is a JsxElement node (not exposed as a
	// dedicated Kind, but iteration finds non-empty content).
	if parentHasNonEmptyChildren(parent, opener) {
		ctx.Report(opener, "innerHTML prop combined with children — React drops the children silently")
	}
}

// visitSelfClosing handles `<X PROP children={...} />` — children
// must come via an explicit attribute since the element has no
// child nodes.
func visitSelfClosing(ctx *engine.Context, el *wrapperchecker.Node) {
	if hasFlaggedAttribute(el) && hasChildrenAttribute(el) {
		ctx.Report(el, "innerHTML prop combined with children — React drops the children silently")
	}
}

// visitCall handles `React.createElement(type, props, ...children)`.
// Trigger when props contains both PROP_NAME and `children`, or
// when there's a third positional argument beyond props.
func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if callee.PropertyAccessName() != "createElement" {
		return
	}
	recv := callee.PropertyAccessReceiver()
	if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier || recv.LiteralText() != "React" {
		return
	}
	args := call.CallArguments()
	if len(args) < 2 {
		return
	}
	props := args[1]
	if !objectHasProp(props, propName) {
		return
	}
	// children appear either as a `children:` property OR as
	// positional arguments after the props object.
	if objectHasProp(props, "children") || len(args) > 2 {
		ctx.Report(call, "createElement: innerHTML prop combined with children — React drops the children silently")
	}
}

func hasFlaggedAttribute(el *wrapperchecker.Node) bool {
	attrs := jsxAttributesNode(el)
	if attrs == nil {
		return false
	}
	return findAttribute(attrs, propName) != nil
}

func hasChildrenAttribute(el *wrapperchecker.Node) bool {
	attrs := jsxAttributesNode(el)
	if attrs == nil {
		return false
	}
	return findAttribute(attrs, "children") != nil
}

// parentHasNonEmptyChildren reports whether the JsxElement parent
// of `opener` contains anything other than the opener and closer.
// We can't query Kind directly (no KindJsxElement exposed), so we
// inspect the parent's children and look for siblings outside the
// open/close JsxOpeningElement / JsxClosingElement.
func parentHasNonEmptyChildren(parent, opener *wrapperchecker.Node) bool {
	found := false
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindJsxOpeningElement,
			wrapperchecker.KindJsxClosingElement,
			wrapperchecker.KindJsxAttributes,
			wrapperchecker.KindIdentifier:
			return false
		}
		// Anything else under the JsxElement parent counts as a
		// child node (text / expression / nested element).
		if c.Pos() != opener.Pos() {
			found = true
			return true
		}
		return false
	})
	return found
}

func objectHasProp(o *wrapperchecker.Node, name string) bool {
	if o == nil || o.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	o.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindMethodDeclaration:
			if c.PropertyName() == name {
				found = true
				return true
			}
		}
		return false
	})
	return found
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
