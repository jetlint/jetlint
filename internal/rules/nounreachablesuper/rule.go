// Package nounreachablesuper implements no-unreachable-super: flag
// derived-class constructors that misuse `super()`. Biome's rule
// fires on three related shapes; this port covers each with a
// syntactic heuristic rather than a full CFG:
//
//   - duplicate-super: two or more `super(...)` calls anywhere in
//     the constructor (any pair could re-run).
//   - this-before-super: a `this.X = ...` (or any `this.X`) usage
//     that appears in source order before the first `super()` call.
//   - missing-super: a `super.X` member access in the body without
//     any `super()` call — biome's "super used as a method without
//     calling it" case.
//
// Inner functions aren't traversed: their `super` and `this` bind
// to a different receiver and don't tell us anything about the
// outer constructor.
package nounreachablesuper

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unreachable-super"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindClassDeclaration: visit,
		wrapperchecker.KindClassExpression:  visit,
	}
}

func visit(ctx *engine.Context, cls *wrapperchecker.Node) {
	if !classExtends(cls) {
		return
	}
	ctor := findConstructor(cls)
	if ctor == nil {
		return
	}
	body := constructorBody(ctor)
	if body == nil {
		return
	}
	var superCalls []*wrapperchecker.Node
	var superMembers []*wrapperchecker.Node
	var thisUsesPositions []int
	walk(body, body, func(n *wrapperchecker.Node) {
		switch n.Kind() {
		case wrapperchecker.KindCallExpression:
			callee := n.CalleeExpression()
			if callee != nil && callee.Kind() == wrapperchecker.KindSuperKeyword {
				superCalls = append(superCalls, n)
			}
		case wrapperchecker.KindPropertyAccessExpression:
			recv := n.PropertyAccessReceiver()
			if recv != nil && recv.Kind() == wrapperchecker.KindSuperKeyword {
				superMembers = append(superMembers, n)
			}
			if recv != nil && recv.Kind() == wrapperchecker.KindThisKeyword {
				thisUsesPositions = append(thisUsesPositions, n.Pos())
			}
		case wrapperchecker.KindThisKeyword:
			// Stand-alone `this` references (e.g. `return this`)
			// count too. A property-access parent is handled
			// above; bare-this here is enough on its own.
			thisUsesPositions = append(thisUsesPositions, n.Pos())
		}
	})

	// Duplicate super: more than one `super(...)` call.
	if len(superCalls) >= 2 {
		ctx.Report(superCalls[1], "constructor calls super() more than once — biome flags this as unreachable")
		return
	}

	// Missing super: `super.X` used without ever calling `super()`.
	if len(superCalls) == 0 && len(superMembers) > 0 {
		ctx.Report(superMembers[0], "constructor uses super without ever calling super()")
		return
	}

	// This-before-super: any `this` use that appears in source
	// order before the first super() call.
	if len(superCalls) > 0 {
		first := superCalls[0].Pos()
		for _, p := range thisUsesPositions {
			if p < first {
				ctx.Report(superCalls[0], "this is used before super() — derived constructors must call super() first")
				return
			}
		}
	}
}

// classExtends reports whether a class declaration / expression has
// an `extends` heritage clause. Other heritage clauses
// (`implements`) don't make it a derived class for super-call
// purposes.
func classExtends(cls *wrapperchecker.Node) bool {
	found := false
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindHeritageClause {
			return false
		}
		// HeritageClause source starts with `extends` or
		// `implements`; we only care about the extends form.
		if strings.HasPrefix(strings.TrimSpace(c.SourceText()), "extends") {
			found = true
			return true
		}
		return false
	})
	return found
}

// findConstructor returns the first ConstructorDeclaration child of
// cls. Classes can declare at most one runtime constructor; overload
// signatures are also Constructor-kind nodes but only the one with a
// body is meaningful here.
func findConstructor(cls *wrapperchecker.Node) *wrapperchecker.Node {
	var ctor *wrapperchecker.Node
	cls.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindConstructor {
			body := constructorBody(c)
			if body != nil {
				ctor = c
				return true
			}
		}
		return false
	})
	return ctor
}

func constructorBody(ctor *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	ctor.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			body = c
			return true
		}
		return false
	})
	return body
}

// walk visits every descendant of root, in document order, without
// descending into function-like nodes (which have their own
// super/this binding).
func walk(root, n *wrapperchecker.Node, visit func(*wrapperchecker.Node)) {
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			if c != root {
				return false
			}
		}
		visit(c)
		walk(root, c, visit)
		return false
	})
}
