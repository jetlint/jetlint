// Package nodocumentcookie implements no-document-cookie: the
// `document.cookie` API is awkward (string concatenation, no
// async/await, easy to silently lose) — prefer the modern
// `CookieStore` API.
package nodocumentcookie

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-document-cookie"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	op := operatorToken(n)
	if op != "=" && op != "+=" {
		return
	}
	target := leftOperand(n)
	if !isDocumentCookieAccess(target) {
		return
	}
	if isShadowed(n, "document") {
		return
	}
	ctx.Report(n, "document.cookie is awkward — use the CookieStore API")
}

func leftOperand(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	return first
}

func operatorToken(n *wrapperchecker.Node) string {
	var second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 1 {
			second = c
			return true
		}
		idx++
		return false
	})
	if second == nil {
		return ""
	}
	return second.SourceText()
}

func isDocumentCookieAccess(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		obj, name := propParts(n)
		return name == "cookie" && isDocumentRef(obj)
	case wrapperchecker.KindElementAccessExpression:
		obj, name := elemParts(n)
		return name == "cookie" && isDocumentRef(obj)
	}
	return false
}

func isDocumentRef(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindIdentifier {
		return n.SourceText() == "document"
	}
	if n.Kind() == wrapperchecker.KindPropertyAccessExpression {
		_, name := propParts(n)
		return name == "document"
	}
	return false
}

func propParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		} else if second == nil {
			second = c
		}
		return false
	})
	if second == nil {
		return nil, ""
	}
	return first, second.SourceText()
}

func elemParts(n *wrapperchecker.Node) (*wrapperchecker.Node, string) {
	obj, key := propParts(n)
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'' || key[0] == '`') {
		key = key[1 : len(key)-1]
	}
	return obj, key
}

func isShadowed(start *wrapperchecker.Node, name string) bool {
	for n := start.Parent(); n != nil; n = n.Parent() {
		found := false
		walk(n, func(c *wrapperchecker.Node) bool {
			if found {
				return false
			}
			switch c.Kind() {
			case wrapperchecker.KindVariableDeclaration,
				wrapperchecker.KindFunctionDeclaration,
				wrapperchecker.KindParameter:
				if declName(c) == name {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

func walk(n *wrapperchecker.Node, fn func(*wrapperchecker.Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walk(c, fn)
		return false
	})
}

func declName(n *wrapperchecker.Node) string {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first != nil && first.Kind() == wrapperchecker.KindIdentifier {
		return first.SourceText()
	}
	return ""
}
