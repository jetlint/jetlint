// Package usetoplevelregex implements use-top-level-regex: regex
// literals inside function bodies are re-compiled on every call.
// Hoisting them to module scope (or a `const` outside the hot path)
// compiles the pattern once.
//
// The rule flags `RegExp` constructor calls in addition to literal
// `/.../` syntax. Top-level regexes (module scope or initializing a
// top-level `const`) are unflagged.
package usetoplevelregex

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-top-level-regex"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindRegularExpressionLiteral: visit,
		wrapperchecker.KindNewExpression:            visitNew,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if hasStatefulFlags(n.LiteralText()) {
		return
	}
	if !insideFunction(n) {
		return
	}
	ctx.Report(n, "regex literal inside a function is re-compiled on every call — hoist to module scope")
}

// hasStatefulFlags reports whether the regex text carries `g` or `y`
// (sticky) flags. Stateful regexes legitimately need a fresh
// instance per call so caching them would leak `lastIndex` between
// invocations.
func hasStatefulFlags(text string) bool {
	// text looks like `/pattern/flags`. Find the last `/` to isolate
	// the flag group.
	if i := strings.LastIndex(text, "/"); i >= 0 && i < len(text)-1 {
		flags := text[i+1:]
		return strings.ContainsAny(flags, "gy")
	}
	return false
}

func visitNew(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.LiteralText() != "RegExp" {
		return
	}
	if !insideFunction(n) {
		return
	}
	ctx.Report(n, "RegExp() inside a function recompiles on every call — hoist to module scope")
}

// insideFunction reports whether n appears inside a function body
// whose evaluation runs more than once. Carve-outs:
//   - Object literal property value (`{ regex: /.../ }`) is evaluated
//     when the object is constructed; for top-level object literals
//     that's once.
//   - Static class field initializers run once when the class is
//     defined.
func insideFunction(n *wrapperchecker.Node) bool {
	prev := n
	for p := n.Parent(); p != nil; prev, p = p, p.Parent() {
		switch p.Kind() {
		case wrapperchecker.KindPropertyAssignment:
			// Property value of an object literal — skip when this
			// regex IS the value (not nested deeper inside a
			// function within the value).
			if propertyAssignmentValueIs(p, prev) {
				return false
			}
		case wrapperchecker.KindPropertyDeclaration:
			// Class field. Static fields are initialized once at
			// class definition; instance fields run on every
			// `new`, so still flag those.
			if hasStaticModifier(p) {
				return false
			}
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return true
		}
	}
	return false
}

func propertyAssignmentValueIs(pa, child *wrapperchecker.Node) bool {
	init := pa.PropertyInitializer()
	return init != nil && init.Pos() == child.Pos() && init.End() == child.End()
}

func hasStaticModifier(decl *wrapperchecker.Node) bool {
	found := false
	decl.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindStaticKeyword {
			found = true
			return true
		}
		return false
	})
	return found
}
