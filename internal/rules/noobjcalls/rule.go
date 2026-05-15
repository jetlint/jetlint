// Package noobjcalls implements the no-obj-calls rule: certain global
// objects (Math, JSON, Atomics, Intl, Reflect) are namespaces — calling
// them as functions or invoking them with `new` throws a TypeError at
// runtime. Flags both direct calls (`Math()`, `new JSON()`) and
// `globalThis.Math()` member-access variants, plus simple alias chains
// (`let j = JSON; j();`).
package noobjcalls

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-obj-calls"

// nonCallableGlobals lists the global namespace objects that throw
// when called as a function. Matches ESLint/oxlint's set.
var nonCallableGlobals = map[string]bool{
	"Atomics": true,
	"Intl":    true,
	"JSON":    true,
	"Math":    true,
	"Reflect": true,
}

// New constructs a noobjcalls rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
		wrapperchecker.KindNewExpression:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	callee := n.CalleeExpression()
	if callee == nil {
		return
	}
	if name := resolveGlobalName(callee); nonCallableGlobals[name] {
		ctx.Report(n, "'"+name+"' is not a function.")
	}
}

// resolveGlobalName returns the global object name that `callee`
// ultimately refers to, or "" if it doesn't resolve to a known global.
// Handles:
//   - bare identifier `Math` (when not shadowed)
//   - `globalThis.Math` / `globalThis?.Math` (when globalThis not shadowed)
//   - `globalThis["Math"]` element access
//   - alias chains: `let m = JSON; m()` / `let m = globalThis.JSON; m()`
//     including transitive chains.
func resolveGlobalName(callee *wrapperchecker.Node) string {
	for callee != nil && callee.Kind() == wrapperchecker.KindParenthesizedExpression {
		callee = callee.FirstChild()
	}
	if callee == nil {
		return ""
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		return resolveIdentifier(callee, callee.LiteralText(), map[string]bool{})
	case wrapperchecker.KindPropertyAccessExpression:
		return globalThisMember(callee)
	case wrapperchecker.KindElementAccessExpression:
		return globalThisIndex(callee)
	}
	return ""
}

// resolveIdentifier walks scopes outward from `ref` looking for a
// binding for `name`. If a binding is found:
//   - VariableDeclaration with no initializer: not a global alias (the
//     name shadows the global with `undefined` / a runtime-assigned
//     value we can't track).
//   - Initializer is another Identifier: recurse from the declaration's
//     scope with that new name.
//   - Initializer is `globalThis.X` / `globalThis["X"]`: return X if X
//     is a non-callable global.
//
// If no binding is found, `name` refers to the global of that name.
// `seen` short-circuits pathological cycles like `let x = x;`.
func resolveIdentifier(ref *wrapperchecker.Node, name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	decl, _ := findBindingDeclaration(ref, name)
	if decl == nil {
		// Unbound identifier — refers to the global of that name.
		return name
	}
	init := directInitializer(decl, name)
	if init == nil {
		return ""
	}
	for init.Kind() == wrapperchecker.KindParenthesizedExpression {
		init = init.FirstChild()
		if init == nil {
			return ""
		}
	}
	switch init.Kind() {
	case wrapperchecker.KindIdentifier:
		if init.LiteralText() == name {
			return ""
		}
		// Continue resolution from the declaration itself, so the
		// recursive lookup includes the scope that contains it.
		return resolveIdentifier(decl, init.LiteralText(), seen)
	case wrapperchecker.KindPropertyAccessExpression:
		return globalThisMember(init)
	case wrapperchecker.KindElementAccessExpression:
		return globalThisIndex(init)
	}
	return ""
}

// globalThisMember returns X if the property access is `globalThis.X`
// (or `globalThis?.X`) and `globalThis` is not shadowed and X is a
// non-callable global. Returns "" otherwise.
func globalThisMember(n *wrapperchecker.Node) string {
	if n.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return ""
	}
	object, property := propertyAccessParts(n)
	if object == nil || property == nil {
		return ""
	}
	if !isGlobalThis(object) {
		return ""
	}
	if property.Kind() != wrapperchecker.KindIdentifier {
		return ""
	}
	name := property.LiteralText()
	if nonCallableGlobals[name] {
		return name
	}
	return ""
}

// globalThisIndex handles `globalThis["Math"]` element access with a
// string-literal index.
func globalThisIndex(n *wrapperchecker.Node) string {
	if n.Kind() != wrapperchecker.KindElementAccessExpression {
		return ""
	}
	object, index := elementAccessParts(n)
	if object == nil || index == nil {
		return ""
	}
	if !isGlobalThis(object) {
		return ""
	}
	if index.Kind() != wrapperchecker.KindStringLiteral && index.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return ""
	}
	name := index.LiteralText()
	if nonCallableGlobals[name] {
		return name
	}
	return ""
}

func isGlobalThis(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	if n.LiteralText() != "globalThis" {
		return false
	}
	// `globalThis` is unshadowed if no enclosing scope binds it.
	return !hasEnclosingBinding(n, "globalThis")
}

// propertyAccessParts returns the object and property identifier of a
// PropertyAccessExpression (or optional-chain variant). The wrapper
// keeps both as direct children; the object is the first child and the
// property identifier is the last identifier child.
func propertyAccessParts(n *wrapperchecker.Node) (object, property *wrapperchecker.Node) {
	var children []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		children = append(children, c)
		return false
	})
	if len(children) < 2 {
		return nil, nil
	}
	return children[0], children[len(children)-1]
}

func elementAccessParts(n *wrapperchecker.Node) (object, index *wrapperchecker.Node) {
	var children []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		children = append(children, c)
		return false
	})
	if len(children) < 2 {
		return nil, nil
	}
	return children[0], children[len(children)-1]
}

// findBindingDeclaration scans enclosing scopes for a VariableDeclaration,
// FunctionDeclaration, ClassDeclaration, parameter, catch binding, or
// import that binds `name`. Returns the matching declaration node and
// the scope node that contains it (used to continue resolution outward).
// Returns (nil, nil) if no binding exists (i.e. `name` is a global).
func findBindingDeclaration(from *wrapperchecker.Node, name string) (decl, scope *wrapperchecker.Node) {
	for p := from.Parent(); p != nil; p = p.Parent() {
		if !isScopeNode(p) {
			continue
		}
		if d := scopeBindingDeclaration(p, name); d != nil {
			return d, p
		}
	}
	return nil, nil
}

func scopeBindingDeclaration(scope *wrapperchecker.Node, name string) *wrapperchecker.Node {
	// A named FunctionExpression / ClassExpression binds its own name
	// inside the body.
	switch scope.Kind() {
	case wrapperchecker.KindFunctionExpression, wrapperchecker.KindClassExpression:
		if functionExpressionName(scope) == name {
			return scope
		}
	}
	var hit *wrapperchecker.Node
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if d := declarationBindsName(c, name); d != nil {
			hit = d
			return true
		}
		return false
	})
	if hit != nil {
		return hit
	}
	// Function parameters.
	switch scope.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		for _, p := range scope.FunctionParameters() {
			if d := declarationBindsName(p, name); d != nil {
				return d
			}
		}
	}
	return nil
}

// declarationBindsName returns the VariableDeclaration (or analogue)
// node that introduces `name`, or nil. For non-variable declarations
// (function/class/parameter/catch/import) returns the declaration node
// itself so the caller can detect "binding exists but has no initializer
// we can track".
func declarationBindsName(n *wrapperchecker.Node, name string) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindFunctionExpression:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			return false
		})
		if hit {
			return n
		}
	case wrapperchecker.KindVariableStatement,
		wrapperchecker.KindVariableDeclarationList:
		var hit *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if d := declarationBindsName(c, name); d != nil {
				hit = d
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableDeclaration:
		if variableDeclarationBindsName(n, name) {
			return n
		}
	case wrapperchecker.KindParameter:
		if parameterBindsName(n, name) {
			return n
		}
	case wrapperchecker.KindBindingElement:
		if parameterBindsName(n, name) {
			return n
		}
	case wrapperchecker.KindImportDeclaration:
		if patternBindsName(n, name) {
			return n
		}
	case wrapperchecker.KindCatchClause:
		var hit *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindVariableDeclaration && variableDeclarationBindsName(c, name) {
				hit = c
				return true
			}
			return false
		})
		return hit
	}
	return nil
}

// variableDeclarationBindsName checks whether a VariableDeclaration
// introduces `name`. The first child is the bound name (Identifier or
// nested binding pattern).
func variableDeclarationBindsName(n *wrapperchecker.Node, name string) bool {
	hit := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			if c.LiteralText() == name {
				hit = true
				return true
			}
		case wrapperchecker.KindObjectBindingPattern, wrapperchecker.KindArrayBindingPattern:
			if patternBindsName(c, name) {
				hit = true
				return true
			}
		}
		return true // only the name slot matters; stop scanning after first child
	})
	return hit
}

func parameterBindsName(n *wrapperchecker.Node, name string) bool {
	hit := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindIdentifier:
			if c.LiteralText() == name {
				hit = true
				return true
			}
		case wrapperchecker.KindObjectBindingPattern, wrapperchecker.KindArrayBindingPattern:
			if patternBindsName(c, name) {
				hit = true
				return true
			}
		}
		return true
	})
	return hit
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

// directInitializer returns the initializer expression of a
// VariableDeclaration *if* `name` is bound directly (the variable's
// name is `name`, not destructured into it). Returns nil for non-
// VariableDeclaration nodes, declarations without an initializer, or
// destructuring patterns — in those cases the binding cannot be
// reduced to "an alias of the initializer expression."
func directInitializer(n *wrapperchecker.Node, name string) *wrapperchecker.Node {
	if n.Kind() != wrapperchecker.KindVariableDeclaration {
		return nil
	}
	var children []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		children = append(children, c)
		return false
	})
	if len(children) < 2 {
		return nil
	}
	first := children[0]
	// Only a direct Identifier name with text == name qualifies.
	if first.Kind() != wrapperchecker.KindIdentifier || first.LiteralText() != name {
		return nil
	}
	last := children[len(children)-1]
	if last == first {
		return nil
	}
	return last
}

// functionExpressionName returns the literal text of the optional name
// identifier on a FunctionExpression or ClassExpression. Empty string
// if anonymous.
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

// hasEnclosingBinding reports whether any enclosing scope binds `name`.
// Used to decide whether `globalThis` refers to the global.
func hasEnclosingBinding(n *wrapperchecker.Node, name string) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if !isScopeNode(p) {
			continue
		}
		if scopeBindingDeclaration(p, name) != nil {
			return true
		}
	}
	return false
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
