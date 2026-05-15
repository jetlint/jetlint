// Package noimportassign implements the no-import-assign rule: ES module
// bindings introduced by `import` are read-only at the language level —
// assigning to them throws a TypeError at runtime in strict mode. The
// rule covers the assignment-target cases (direct, destructuring,
// for-in/of, increment/decrement) and the namespace-mutation patterns
// (`mod.prop = …`, `Object.assign(mod, …)`).
package noimportassign

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-import-assign"

// objectMutationMethods are Object.* methods that mutate their first
// argument; calling them with an imported namespace is forbidden.
var objectMutationMethods = map[string]bool{
	"assign":         true,
	"defineProperty": true,
	"defineProperties": true,
	"freeze":         true,
	"setPrototypeOf": true,
}

// reflectMutationMethods are Reflect.* methods that mutate their first
// argument.
var reflectMutationMethods = map[string]bool{
	"defineProperty": true,
	"deleteProperty": true,
	"set":            true,
	"setPrototypeOf": true,
}

// New constructs a noimportassign rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		// Shorthand destructuring assigns: the identifier resolves to
		// the property symbol; the outer binding is the actual write.
		parent := n.Parent()
		if parent != nil && parent.Kind() == wrapperchecker.KindShorthandPropertyAssignment {
			sym = ctx.Checker().ShorthandAssignmentValueSymbol(parent)
		}
	}
	if sym == nil {
		return
	}
	kind := importKind(sym)
	if kind == importNone {
		return
	}
	// Case 1: the identifier itself is an assignment target.
	if n.IsAssignmentTarget() {
		ctx.Report(n, "'"+n.LiteralText()+"' is a read-only import.")
		return
	}
	// Case 2: namespace import — `mod.x = …`, `Object.assign(mod, …)`, etc.
	if kind == importNamespace {
		checkNamespaceUse(ctx, n)
	}
}

type importKindClass int

const (
	importNone importKindClass = iota
	importDefaultOrNamed
	importNamespace
)

func importKind(sym *wrapperchecker.Symbol) importKindClass {
	for _, d := range sym.Declarations() {
		switch d.Kind() {
		case wrapperchecker.KindNamespaceImport:
			return importNamespace
		case wrapperchecker.KindImportClause,
			wrapperchecker.KindImportSpecifier:
			return importDefaultOrNamed
		}
	}
	return importNone
}

// checkNamespaceUse handles writes to a namespace import's members and
// well-known mutation-function calls that take the namespace as their
// first argument.
func checkNamespaceUse(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	// `mod.prop = …` / `[mod.prop] = …` / etc.
	if parent.Kind() == wrapperchecker.KindPropertyAccessExpression ||
		parent.Kind() == wrapperchecker.KindElementAccessExpression {
		// Only the receiver of the property access counts; if `n` is
		// itself the property name (RHS of `.`), skip.
		if isReceiverOf(parent, n) && parent.IsAssignmentTarget() {
			ctx.Report(parent, "The members of '"+n.LiteralText()+"' are read-only.")
			return
		}
		// `delete mod.prop` is also a write.
		if isReceiverOf(parent, n) && isDeleteOperandOrChain(parent) {
			ctx.Report(parent, "The members of '"+n.LiteralText()+"' are read-only.")
			return
		}
	}
	// `Object.assign(mod, …)` etc.
	if isMutationCallFirstArg(n) {
		ctx.Report(n, "The members of '"+n.LiteralText()+"' are read-only.")
	}
}

// isReceiverOf reports whether `child` is the object/receiver part of
// the member-access `parent` (rather than the property name). The
// wrapper returns fresh *Node values from ForEachChild even for the
// same underlying ast node, so we compare by source range instead of
// pointer identity.
func isReceiverOf(parent, child *wrapperchecker.Node) bool {
	switch parent.Kind() {
	case wrapperchecker.KindPropertyAccessExpression,
		wrapperchecker.KindElementAccessExpression:
	default:
		return false
	}
	var first *wrapperchecker.Node
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		first = c
		return true
	})
	if first == nil {
		return false
	}
	return first.Pos() == child.Pos() && first.End() == child.End()
}

// isDeleteOperandOrChain reports whether `n` is the operand of `delete`,
// optionally through a ChainExpression / parens.
func isDeleteOperandOrChain(n *wrapperchecker.Node) bool {
	for cur := n; cur != nil; {
		p := cur.Parent()
		if p == nil {
			return false
		}
		switch p.Kind() {
		case wrapperchecker.KindParenthesizedExpression:
			cur = p
			continue
		case wrapperchecker.KindDeleteExpression:
			return true
		}
		return false
	}
	return false
}

// isMutationCallFirstArg reports whether `n` is the first argument to
// `Object.<m>` or `Reflect.<m>` for some mutating member `m`.
func isMutationCallFirstArg(n *wrapperchecker.Node) bool {
	parent := n.Parent()
	if parent == nil {
		return false
	}
	if parent.Kind() != wrapperchecker.KindCallExpression {
		return false
	}
	callee := parent.CalleeExpression()
	if callee == nil {
		return false
	}
	// Strip ChainExpression / parens around the callee.
	for callee.Kind() == wrapperchecker.KindParenthesizedExpression {
		callee = callee.FirstChild()
		if callee == nil {
			return false
		}
	}
	if callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return false
	}
	var calleeChildren []*wrapperchecker.Node
	callee.ForEachChild(func(c *wrapperchecker.Node) bool {
		calleeChildren = append(calleeChildren, c)
		return false
	})
	if len(calleeChildren) < 2 {
		return false
	}
	object := calleeChildren[0]
	property := calleeChildren[len(calleeChildren)-1]
	if object.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	if property.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	prop := property.LiteralText()
	switch object.LiteralText() {
	case "Object":
		if !objectMutationMethods[prop] {
			return false
		}
	case "Reflect":
		if !reflectMutationMethods[prop] {
			return false
		}
	default:
		return false
	}
	// `Object` must resolve to the global — bail out if it's shadowed.
	if hasEnclosingBinding(object, object.LiteralText()) {
		return false
	}
	// `n` must be the first argument to the call.
	calleeEnd := callee.End()
	var firstArg *wrapperchecker.Node
	parent.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Pos() < calleeEnd {
			return false
		}
		if firstArg == nil {
			firstArg = c
		}
		return true
	})
	if firstArg == nil {
		return false
	}
	return firstArg.Pos() == n.Pos() && firstArg.End() == n.End()
}

// hasEnclosingBinding mirrors the scope walker used in the
// no-obj-calls / no-new-native-nonconstructor rules.
func hasEnclosingBinding(n *wrapperchecker.Node, name string) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if !isScopeNode(p) {
			continue
		}
		if scopeBinds(p, name) {
			return true
		}
	}
	return false
}

func scopeBinds(scope *wrapperchecker.Node, name string) bool {
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

func declarationBindsName(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			return false
		})
		return hit
	case wrapperchecker.KindVariableStatement,
		wrapperchecker.KindVariableDeclarationList:
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
		wrapperchecker.KindParameter:
		hit := false
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == name {
				hit = true
				return true
			}
			return false
		})
		return hit
	}
	return false
}
