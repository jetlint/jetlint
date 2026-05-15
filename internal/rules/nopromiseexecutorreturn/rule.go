// Package nopromiseexecutorreturn implements the no-promise-executor-return
// rule: the executor passed to `new Promise(...)` is invoked synchronously
// and its return value is discarded. Returning anything from the executor
// is dead code or, more often, indicates the author meant to call
// `resolve`/`reject` and accidentally wrote `return resolve(...)`.
//
// Reports:
//   - `new Promise((res, rej) => res(1))` — arrow with implicit return
//   - `new Promise((res, rej) => { return value; })` — return-with-value
//     anywhere in the executor body that is not inside a nested function
//   - same for `function (res, rej) {…}` executors
package nopromiseexecutorreturn

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-promise-executor-return"

// New constructs a nopromiseexecutorreturn rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNewExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil || callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.LiteralText() != "Promise" {
		return
	}
	// Only fire when `Promise` resolves to the global. The TS-go checker
	// keeps lib symbols separate from user declarations, so we walk the
	// AST ourselves and look for any binding named "Promise" in an
	// enclosing scope.
	if hasEnclosingBinding(callee, "Promise") {
		return
	}
	exec := firstArgument(n)
	if exec == nil {
		return
	}
	for exec != nil && exec.Kind() == wrapperchecker.KindParenthesizedExpression {
		exec = exec.FirstChild()
	}
	switch exec.Kind() {
	case wrapperchecker.KindArrowFunction:
		body := arrowBody(exec)
		if body == nil {
			return
		}
		if body.Kind() == wrapperchecker.KindBlock {
			walkReturns(ctx, body)
		} else {
			// Concise-body arrow: the body expression is an implicit return.
			ctx.Report(body, "return value from a Promise executor is discarded")
		}
	case wrapperchecker.KindFunctionExpression:
		body := functionBody(exec)
		if body != nil {
			walkReturns(ctx, body)
		}
	}
}

// firstArgument returns the first child of a NewExpression that is not
// the callee. NewExpression children are: callee, optional type args,
// then the argument list. We pick the first node whose source position
// follows the callee.
func firstArgument(n *wrapperchecker.Node) *wrapperchecker.Node {
	callee := n.CalleeExpression()
	if callee == nil {
		return nil
	}
	calleeEnd := callee.End()
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Pos() < calleeEnd {
			return false
		}
		if first == nil {
			first = c
		}
		return true
	})
	return first
}

func arrowBody(n *wrapperchecker.Node) *wrapperchecker.Node {
	var body *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		body = c
		return false
	})
	return body
}

func functionBody(n *wrapperchecker.Node) *wrapperchecker.Node {
	var block *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindBlock {
			block = c
			return true
		}
		return false
	})
	return block
}

// walkReturns reports every ReturnStatement-with-argument reachable
// from n without crossing a nested function/class/method boundary.
// The Promise executor's own block body is fine to descend into, but
// inner functions have their own callers and may legitimately return.
func walkReturns(ctx *engine.Context, n *wrapperchecker.Node) {
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
			return false
		case wrapperchecker.KindReturnStatement:
			if hasReturnArgument(c) {
				ctx.Report(c, "return value from a Promise executor is discarded")
			}
			return false
		}
		walkReturns(ctx, c)
		return false
	})
}

// hasEnclosingBinding scans n's containing scopes for a declaration
// that introduces `name`. Walks scope-boundary nodes (functions,
// blocks, classes, modules, the source file) and inspects their
// declarations. If found, the surrounding code shadows the global.
func hasEnclosingBinding(n *wrapperchecker.Node, name string) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if scopeDeclaresName(p, name) {
			return true
		}
	}
	return false
}

// scopeDeclaresName reports whether scope (a function body, class
// body, source file, or block) contains a declaration introducing
// `name`. We look at direct children for declarations whose bound
// identifier matches.
func scopeDeclaresName(scope *wrapperchecker.Node, name string) bool {
	if !isScopeNode(scope) {
		return false
	}
	// A named FunctionExpression / ClassExpression binds its own name
	// inside the function body (`var f = function foo() { foo… }`).
	switch scope.Kind() {
	case wrapperchecker.KindFunctionExpression, wrapperchecker.KindClassExpression:
		if functionExpressionName(scope) == name {
			return true
		}
	}
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if declarationBindsName(c, name) {
			found = true
			return true
		}
		return false
	})
	if found {
		return true
	}
	// Function parameters are also bindings in scope: check the
	// containing function's parameter list.
	switch scope.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		for _, p := range scope.FunctionParameters() {
			if declarationBindsName(p, name) {
				return true
			}
		}
	}
	return false
}

// functionExpressionName returns the literal text of the optional name
// identifier on a FunctionExpression or ClassExpression. Empty string
// if anonymous. The name (when present) is the first Identifier child.
func functionExpressionName(n *wrapperchecker.Node) string {
	name := ""
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		return false
	})
	return name
}

func isScopeNode(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindSourceFile,
		wrapperchecker.KindBlock,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindModuleDeclaration,
		wrapperchecker.KindForStatement,
		wrapperchecker.KindForInStatement,
		wrapperchecker.KindForOfStatement,
		wrapperchecker.KindCatchClause:
		return true
	}
	return false
}

// declarationBindsName reports whether the given node introduces a
// binding for `name`. Covers function/variable/class declarations,
// parameters, and catch clause parameters; descends through
// VariableStatement / VariableDeclarationList / BindingPattern wrappers.
func declarationBindsName(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindFunctionExpression:
		// Direct name child if any.
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableStatement:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declarationBindsName(c, name) {
				hit = true
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableDeclarationList:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if declarationBindsName(c, name) {
				hit = true
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement:
		// First child is the bound name (Identifier or nested pattern).
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			if c.Kind() == wrapperchecker.KindObjectBindingPattern ||
				c.Kind() == wrapperchecker.KindArrayBindingPattern {
				if patternBindsName(c, name) {
					hit = true
					return true
				}
			}
			return false
		})
		return hit
	case wrapperchecker.KindImportDeclaration:
		// `import X from 'mod'` / `import {X} from 'mod'` — descend.
		return patternBindsName(n, name)
	}
	return false
}

func patternBindsName(n *wrapperchecker.Node, name string) bool {
	hit := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
			hit = true
			return true
		}
		if patternBindsName(c, name) {
			hit = true
			return true
		}
		return false
	})
	return hit
}

func hasReturnArgument(n *wrapperchecker.Node) bool {
	found := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		_ = c
		found = true
		return true
	})
	return found
}
