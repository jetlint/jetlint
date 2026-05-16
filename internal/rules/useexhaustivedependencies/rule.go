// Package useexhaustivedependencies implements use-exhaustive-dependencies:
// React reactive hooks (useEffect, useCallback, useMemo,
// useImperativeHandle, useLayoutEffect, useInsertionEffect) accept a
// dependency array as their second argument. Any value referenced in
// the callback body that comes from an enclosing scope must appear in
// that array, or React may skip re-running the effect when it should.
// The rule walks each call's callback body collecting referenced free
// identifiers, then verifies each one appears in the dependency
// array. Values declared inside the callback are excluded. The rule
// is intentionally conservative: it does not yet handle property
// access dependencies (`obj.foo`) or recognize stable setters from
// useState/useReducer.
package useexhaustivedependencies

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-exhaustive-dependencies"

// reactiveHooks is the set of React hooks whose second argument is a
// dependency array. Matched by callee identifier or property-access
// name; namespace prefixes (`React.useEffect`) are accepted.
var reactiveHooks = map[string]bool{
	"useEffect":            true,
	"useLayoutEffect":      true,
	"useInsertionEffect":   true,
	"useCallback":          true,
	"useMemo":              true,
	"useImperativeHandle":  true,
}

// globallyAvailable names identifiers that don't need to be declared
// as deps because they are not reactive (built-ins, globals,
// commonly-imported singletons).
var globallyAvailable = map[string]bool{
	"undefined": true, "null": true, "true": true, "false": true,
	"Math": true, "Date": true, "JSON": true, "Object": true, "Array": true,
	"String": true, "Number": true, "Boolean": true, "Symbol": true,
	"Promise": true, "Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"RegExp": true, "Error": true, "TypeError": true, "RangeError": true,
	"console": true, "window": true, "document": true, "globalThis": true,
	"setTimeout": true, "setInterval": true, "clearTimeout": true, "clearInterval": true,
	"requestAnimationFrame": true, "cancelAnimationFrame": true,
	"fetch": true, "URL": true, "URLSearchParams": true,
	"parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true,
	"this": true, "arguments": true, "super": true,
}

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
	}
}

func visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	if !isReactiveHook(call) {
		return
	}
	callback, deps := hookCallbackAndDeps(call)
	if callback == nil || deps == nil {
		return
	}
	declared := depNames(deps)
	for _, ref := range freeIdentifiers(callback) {
		name := ref.LiteralText()
		if name == "" || globallyAvailable[name] {
			continue
		}
		if declared[name] {
			continue
		}
		ctx.Report(ref, "This dependency is not specified in the hook dependency list.")
	}
}

func isReactiveHook(call *wrapperchecker.Node) bool {
	callee := call.CalleeExpression()
	if callee == nil {
		return false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return reactiveHooks[callee.LiteralText()]
	case wrapperchecker.KindPropertyAccessExpression:
		return reactiveHooks[callee.PropertyAccessName()]
	}
	return false
}

// hookCallbackAndDeps returns the (callback, depsArray) pair from a
// reactive hook call. Either may be nil — useEffect with one
// argument is intentionally non-reactive, useCallback with a
// non-array second argument is too unusual to lint.
func hookCallbackAndDeps(call *wrapperchecker.Node) (*wrapperchecker.Node, *wrapperchecker.Node) {
	args := callArguments(call)
	if len(args) < 2 {
		return nil, nil
	}
	cb := unwrap(args[0])
	deps := unwrap(args[1])
	if cb == nil || deps == nil {
		return nil, nil
	}
	switch cb.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		// ok
	default:
		return nil, nil
	}
	if deps.Kind() != wrapperchecker.KindArrayLiteralExpression {
		return nil, nil
	}
	return cb, deps
}

func callArguments(call *wrapperchecker.Node) []*wrapperchecker.Node {
	seenCallee := false
	var out []*wrapperchecker.Node
	call.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !seenCallee {
			seenCallee = true
			return false
		}
		// Skip type arguments — they appear between callee and value args.
		switch c.Kind() {
		case wrapperchecker.KindTypeReference, wrapperchecker.KindTypeLiteral:
			return false
		}
		out = append(out, c)
		return false
	})
	return out
}

// depNames returns the set of names listed in a deps array. Only
// plain identifiers and property-access head identifiers are
// recorded — `[user.id]` puts `user` in the set, matching the rule's
// reference-collection granularity.
func depNames(deps *wrapperchecker.Node) map[string]bool {
	out := map[string]bool{}
	deps.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			out[c.LiteralText()] = true
		case wrapperchecker.KindPropertyAccessExpression:
			if head := propertyAccessHead(c); head != "" {
				out[head] = true
			}
		}
		return false
	})
	return out
}

func propertyAccessHead(n *wrapperchecker.Node) string {
	cur := n
	for cur != nil && cur.Kind() == wrapperchecker.KindPropertyAccessExpression {
		var inner *wrapperchecker.Node
		cur.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
				return true
			}
			return false
		})
		if inner == nil {
			return ""
		}
		cur = inner
	}
	if cur != nil && cur.Kind() == wrapperchecker.KindIdentifier {
		return cur.LiteralText()
	}
	return ""
}

// freeIdentifiers walks the callback body collecting identifier
// references that are NOT declared inside the callback itself. The
// pass is intentionally syntactic: it treats every Identifier in
// expression position as a use and subtracts any name introduced by
// a local declaration (var/let/const, function parameters, function
// declarations) inside the callback's own scope tree.
func freeIdentifiers(callback *wrapperchecker.Node) []*wrapperchecker.Node {
	body := functionBody(callback)
	if body == nil {
		return nil
	}
	declared := map[string]bool{}
	collectLocalDeclarations(callback, declared)
	collectLocalDeclarations(body, declared)
	var refs []*wrapperchecker.Node
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindIdentifier:
			if isReferencingIdentifier(n) && !declared[n.LiteralText()] {
				refs = append(refs, n)
			}
			return
		case wrapperchecker.KindPropertyAccessExpression:
			var inner *wrapperchecker.Node
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				if inner == nil {
					inner = c
					return true
				}
				return false
			})
			walk(inner)
			return
		}
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			walk(c)
			return false
		})
	}
	walk(body)
	return dedupeByText(refs)
}

// dedupeByText keeps only the first occurrence of each identifier
// name. The rule reports at most one diagnostic per missing dep.
func dedupeByText(refs []*wrapperchecker.Node) []*wrapperchecker.Node {
	seen := map[string]bool{}
	var out []*wrapperchecker.Node
	for _, r := range refs {
		name := r.LiteralText()
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, r)
	}
	return out
}

// isReferencingIdentifier filters out identifiers that appear in
// non-reference positions (property names, parameter names, type
// references). A reference is an identifier that resolves to a
// value binding the surrounding expression reads from.
func isReferencingIdentifier(id *wrapperchecker.Node) bool {
	p := id.Parent()
	if p == nil {
		return true
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyAssignment,
		wrapperchecker.KindShorthandPropertyAssignment:
		// Property name slot: the first identifier child.
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
				return true
			}
			return false
		})
		if first != nil && first.Pos() == id.Pos() && first.End() == id.End() {
			// Shorthand `{ name }` IS a reference; PropertyAssignment
			// `{ name: value }` is not.
			return p.Kind() == wrapperchecker.KindShorthandPropertyAssignment
		}
	case wrapperchecker.KindPropertyAccessExpression:
		// Only the head identifier is a reference. The tail (property
		// name) is not.
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
				return true
			}
			return false
		})
		if first != nil && (first.Pos() != id.Pos() || first.End() != id.End()) {
			return false
		}
	case wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindEnumDeclaration:
		return false
	case wrapperchecker.KindTypeReference,
		wrapperchecker.KindQualifiedName:
		return false
	}
	return true
}

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
	// Concise arrow body: take the last child.
	var last *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		last = c
		return false
	})
	return last
}

// collectLocalDeclarations walks n looking for declarations that
// introduce names into the scope. Recursion into nested functions
// would discount their locals, but that's fine: any reference inside
// a nested function still has to come from somewhere, and `declared`
// only over-approximates locals (false-positives = miss a missing
// dep, never invent one).
func collectLocalDeclarations(n *wrapperchecker.Node, declared map[string]bool) {
	if n == nil {
		return
	}
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindParameter:
			for _, name := range bindingIdentifiers(c) {
				declared[name] = true
			}
		case wrapperchecker.KindVariableStatement:
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindVariableDeclarationList {
					g.ForEachChild(func(d *wrapperchecker.Node) bool {
						if d.Kind() == wrapperchecker.KindVariableDeclaration {
							for _, name := range bindingIdentifiers(d) {
								declared[name] = true
							}
						}
						return false
					})
				}
				return false
			})
		case wrapperchecker.KindFunctionDeclaration:
			c.ForEachChild(func(g *wrapperchecker.Node) bool {
				if g.Kind() == wrapperchecker.KindIdentifier {
					declared[g.LiteralText()] = true
					return true
				}
				return false
			})
		}
		collectLocalDeclarations(c, declared)
		return false
	})
}

func bindingIdentifiers(decl *wrapperchecker.Node) []string {
	var out []string
	var walk func(*wrapperchecker.Node)
	walk = func(n *wrapperchecker.Node) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case wrapperchecker.KindIdentifier:
			out = append(out, n.LiteralText())
		case wrapperchecker.KindObjectBindingPattern, wrapperchecker.KindArrayBindingPattern:
			n.ForEachChild(func(c *wrapperchecker.Node) bool {
				walk(c)
				return false
			})
		case wrapperchecker.KindBindingElement:
			walk(n.BindingElementName())
		}
	}
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if len(out) > 0 {
			return true
		}
		switch c.Kind() {
		case wrapperchecker.KindIdentifier,
			wrapperchecker.KindObjectBindingPattern,
			wrapperchecker.KindArrayBindingPattern:
			walk(c)
		}
		return false
	})
	return out
}

func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner == nil {
			break
		}
		n = inner
	}
	return n
}
