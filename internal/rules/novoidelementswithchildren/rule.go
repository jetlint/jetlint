// Package novoidelementswithchildren implements
// no-void-elements-with-children: HTML void elements (`br`, `img`,
// `input`, `hr`, …) can't contain children — the runtime ignores
// them or throws — so giving a `<br>foo</br>` or React-prop
// equivalent is a bug.
//
// Both JSX and React.createElement shapes are checked. The set of
// void elements is the HTML living-standard list; SVG and other
// namespaces are out of scope.
package novoidelementswithchildren

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-void-elements-with-children"

// reactInnerHTMLProp names the React-only prop that injects raw
// HTML — listed alongside `children` so void elements never carry
// either route to content.
const reactInnerHTMLProp = "danger" + "ouslySetInnerHTML"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visitJsxOpening,
		wrapperchecker.KindJsxSelfClosingElement: visitJsxSelfClosing,
		wrapperchecker.KindCallExpression:        visitCall,
	}
}

func visitJsxOpening(ctx *engine.Context, n *wrapperchecker.Node) {
	name := jsxElementName(n)
	if !voidElements[name] {
		return
	}
	// An OpeningElement implies the element has a paired closing
	// tag, which is itself wrong for a void element. Any
	// React-only content prop on it is also flagged.
	if hasForbiddenAttribute(n) {
		ctx.Report(n, "void element <"+name+"> can't accept content props")
		return
	}
	ctx.Report(n, "void element <"+name+"> shouldn't have a closing tag")
}

func visitJsxSelfClosing(ctx *engine.Context, n *wrapperchecker.Node) {
	name := jsxElementName(n)
	if !voidElements[name] {
		return
	}
	if hasForbiddenAttribute(n) {
		ctx.Report(n, "void element <"+name+"> can't accept content props")
	}
}

// visitCall handles `React.createElement('img', ...)` and the
// bare/aliased `createElement(...)` forms. A void-element call
// is flagged when its props object includes a content prop, or
// when a third (child) argument is passed at all.
func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if !isCreateElementCallee(callee) {
		return
	}
	args := n.CallArguments()
	if len(args) == 0 {
		return
	}
	tag := args[0]
	if tag.Kind() != wrapperchecker.KindStringLiteral {
		return
	}
	name := tag.LiteralText()
	if !voidElements[name] {
		return
	}
	if len(args) >= 3 {
		ctx.Report(n, "void element '"+name+"' can't have children — drop the third argument")
		return
	}
	if len(args) >= 2 && objectHasForbiddenProp(args[1]) {
		ctx.Report(n, "void element '"+name+"' can't accept content props")
	}
}

// jsxElementName returns the tag name of a JsxOpeningElement /
// JsxSelfClosingElement. Returns "" for namespaced or member-access
// tags like `<foo.Bar />`, where the rule doesn't apply.
func jsxElementName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

// hasForbiddenAttribute reports whether the JSX element carries a
// React-only content prop (`children` or the raw-HTML injector) —
// the two ways to inject content into an element from props.
func hasForbiddenAttribute(elem *wrapperchecker.Node) bool {
	found := false
	elem.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindJsxAttributes {
			return false
		}
		c.ForEachChild(func(attr *wrapperchecker.Node) bool {
			if attr.Kind() != wrapperchecker.KindJsxAttribute {
				return false
			}
			var name string
			attr.ForEachChild(func(n *wrapperchecker.Node) bool {
				if n.Kind() == wrapperchecker.KindIdentifier {
					name = n.LiteralText()
					return true
				}
				return false
			})
			if name == "children" || name == reactInnerHTMLProp {
				found = true
				return true
			}
			return false
		})
		return true
	})
	return found
}

func isCreateElementCallee(callee *wrapperchecker.Node) bool {
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return callee.LiteralText() == "createElement"
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName() == "createElement"
	}
	return false
}

func objectHasForbiddenProp(arg *wrapperchecker.Node) bool {
	if arg == nil || arg.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	found := false
	arg.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindPropertyAssignment {
			return false
		}
		name := c.PropertyName()
		if name == "children" || name == reactInnerHTMLProp {
			found = true
			return true
		}
		return false
	})
	return found
}

var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}
