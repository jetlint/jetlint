// Package useyield implements the use-yield rule (eslint's
// require-yield, biome's use-yield): a generator function whose body
// contains no yield expression is almost certainly a mistake — the
// caller will get back a generator that immediately completes with
// `done: true`, which is rarely the author's intent.
//
// "Inside the body" means yields at any depth that aren't shadowed
// by a nested function. A `function*` declaring a non-generator
// `function` inside doesn't count the inner function's yields,
// because each function has its own yield scope.
package useyield

import (
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-yield"

// New constructs a useyield rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: visit,
		wrapperchecker.KindFunctionExpression:  visit,
		wrapperchecker.KindMethodDeclaration:   visit,
	}
}

// visit fires on every function-like node; gates on IsGeneratorFunction
// to avoid scanning the body of a regular function. ArrowFunctions can't
// be generators in JS so they're not registered above.
func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !n.IsGeneratorFunction() {
		return
	}
	if bodyIsEmpty(n) {
		return
	}
	if bodyContainsYield(n) {
		return
	}
	name := generatorName(n)
	if name == "" {
		ctx.Report(n, "This generator function does not have 'yield'.")
		return
	}
	ctx.Report(n, fmt.Sprintf("This generator function '%s' does not have 'yield'.", name))
}

// bodyIsEmpty reports whether fn's body is a Block with zero
// statements. An empty generator never runs anything, so omitting
// yield can't be a programmer error — match eslint and biome by
// treating those as valid.
func bodyIsEmpty(fn *wrapperchecker.Node) bool {
	var block *wrapperchecker.Node
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			block = c
			return true
		}
		return false
	})
	if block == nil {
		return false
	}
	hasStatement := false
	block.ForEachChild(func(c *wrapperchecker.Node) bool {
		hasStatement = true
		return true
	})
	return !hasStatement
}

// bodyContainsYield walks n's body looking for a YieldExpression
// without crossing into a nested function. yield is lexically scoped
// to the nearest enclosing generator, so a `yield` inside an inner
// (non-generator) function does not satisfy the outer generator.
func bodyContainsYield(fn *wrapperchecker.Node) bool {
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if c == fn {
			c.ForEachChild(walk)
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return false
		case wrapperchecker.KindYieldExpression:
			found = true
			return true
		}
		c.ForEachChild(walk)
		return found
	}
	fn.ForEachChild(walk)
	return found
}

// generatorName returns the printable name of a generator function for
// the diagnostic message, or "" when the function is anonymous (a
// FunctionExpression assigned but unnamed, etc.).
func generatorName(fn *wrapperchecker.Node) string {
	var name string
	fn.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}
