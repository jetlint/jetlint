// Package nounusedfunctionparameters implements no-unused-function-parameters:
// flag parameters that are declared and never read inside their
// function body. Underscore-prefixed parameter names are treated as
// intentional placeholders (matching ESLint's `argsIgnorePattern`
// default). Similarly, a function whose own name starts with `_` is
// treated as a private/unused entry point, and its parameters are
// not checked — biome's fixtures rely on that escape hatch.
//
// The body walk skips into nested function-likes so a parameter
// referenced inside a closure still counts as used by the outer
// function.
package nounusedfunctionparameters

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-function-parameters"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindArrowFunction:       visit,
		wrapperchecker.KindMethodDeclaration:   visit,
		wrapperchecker.KindConstructor:         visit,
	}
}

func visit(ctx *engine.Context, fn *wrapperchecker.Node) {
	if functionNameSkipped(fn) {
		return
	}
	params := functionParameters(fn)
	if len(params) == 0 {
		return
	}
	body := functionBody(fn)
	if body == nil {
		return
	}
	used := collectIdentifierReads(body)
	for _, p := range params {
		var names []*wrapperchecker.Node
		collectBindingNames(p, &names)
		for _, name := range names {
			text := name.LiteralText()
			if strings.HasPrefix(text, "_") {
				continue
			}
			if used[text] {
				continue
			}
			ctx.Report(name, "parameter '"+text+"' is declared but never used")
		}
	}
}

// functionNameSkipped reports whether fn carries a name that biome's
// convention treats as intentionally-unused (leading underscore).
// Anonymous functions (arrow, expression with no name, methods with
// non-identifier keys) are NOT skipped — biome flags unused params
// inside them.
func functionNameSkipped(fn *wrapperchecker.Node) bool {
	switch fn.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindMethodDeclaration:
		var name *wrapperchecker.Node
		fn.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier {
				name = c
				return true
			}
			return false
		})
		if name == nil {
			return false
		}
		return strings.HasPrefix(name.LiteralText(), "_")
	}
	return false
}

// functionParameters returns the Parameter nodes of fn in source
// order. Works across all function-like Kinds because Parameter is
// always a direct child of the function-like node.
func functionParameters(fn *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindParameter {
			out = append(out, c)
		}
		return false
	})
	return out
}

// functionBody returns the executable body of a function-like node:
// a Block for declarations / expressions / methods, or the
// expression body of a concise arrow function. Returns nil for an
// overload signature with no body — those don't have parameter
// usage to inspect.
func functionBody(fn *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindBlock:
			body = c
			return true
		}
		return false
	})
	if body != nil {
		return body
	}
	// Concise arrow body — the last child is the expression.
	var last *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	if last != nil && isExpressionKind(last.Kind()) {
		return last
	}
	return nil
}

func isExpressionKind(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindBlock,
		wrapperchecker.KindParameter,
		wrapperchecker.KindIdentifier:
		// Identifier and parameter shouldn't appear as the body of
		// a concise arrow; Block is handled separately. Treat as
		// "looks like an expression" for everything else.
	}
	return k != wrapperchecker.KindBlock &&
		k != wrapperchecker.KindParameter
}

// collectBindingNames walks a Parameter's binding target and appends
// every identifier introduced as a binding. Skips type annotations
// and default-value expressions — those are not bindings.
func collectBindingNames(param *wrapperchecker.Node, out *[]*wrapperchecker.Node) {
	// A Parameter's first child is the binding (an identifier,
	// ObjectBindingPattern, or ArrayBindingPattern); subsequent
	// children are dotDotDot, question token, type annotation, and
	// initializer. We walk only the binding subtree.
	var first *wrapperchecker.Node
	param.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			first = c
			return true
		}
		return false
	})
	if first == nil {
		return
	}
	walkBindingTarget(first, out)
}

func walkBindingTarget(n *wrapperchecker.Node, out *[]*wrapperchecker.Node) {
	switch n.Kind() {
	case wrapperchecker.KindIdentifier:
		*out = append(*out, n)
		return
	case wrapperchecker.KindObjectBindingPattern:
		// Object patterns with a `...rest` element: skip the named
		// siblings (ignoreRestSiblings=true is the default). The
		// author is using the named props as the "everything except"
		// list, and flagging them defeats the pattern.
		hasRest := false
		var elements []*wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindBindingElement {
				elements = append(elements, c)
				if c.BindingElementIsRest() {
					hasRest = true
				}
			}
			return false
		})
		for _, el := range elements {
			if hasRest && !el.BindingElementIsRest() {
				continue
			}
			walkBindingTarget(el, out)
		}
		return
	case wrapperchecker.KindBindingElement:
		// BindingElement children: optional propertyName, the
		// binding (identifier/pattern), optional initializer. Find
		// the binding and recurse.
		var bind *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			switch c.Kind() {
			case wrapperchecker.KindIdentifier,
				wrapperchecker.KindObjectBindingPattern,
				wrapperchecker.KindArrayBindingPattern:
				bind = c
			}
			return false
		})
		if bind != nil {
			walkBindingTarget(bind, out)
		}
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		walkBindingTarget(c, out)
		return false
	})
}

// collectIdentifierReads walks body and returns the set of bare
// identifier reads. Property access RHS names (`.foo`), type-only
// references, and JSX attribute names are skipped — they don't
// count as parameter usage.
func collectIdentifierReads(body *wrapperchecker.Node) map[string]bool {
	used := make(map[string]bool)
	var walk func(n *wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		switch n.Kind() {
		case wrapperchecker.KindPropertyAccessExpression:
			// Visit the receiver, but skip the property name (which
			// is an identifier but doesn't reference a binding).
			recv := n.PropertyAccessReceiver()
			if recv != nil {
				walk(recv)
			}
			return
		case wrapperchecker.KindIdentifier:
			used[n.LiteralText()] = true
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return used
}
