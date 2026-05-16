// Package noinvalidbuiltininstantiation implements
// no-invalid-builtin-instantiation: some globals throw when called
// without `new` (Map, Set, all TypedArrays, ArrayBuffer, Proxy,
// WeakRef, FinalizationRegistry, …) and others throw when called
// with `new` (BigInt, Symbol). Both shapes are flagged so authors
// don't ship code that fails at runtime.
//
// Receiver patterns: bare `Map`, `window.Map`, `globalThis.Map`.
// A file-wide check skips the rule when any local binding shadows
// the receiver name — the reference would resolve to the user's
// declaration, not the global builtin.
package noinvalidbuiltininstantiation

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-invalid-builtin-instantiation"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visitCall,
		wrapperchecker.KindNewExpression:  visitNew,
	}
}

func visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	name, ok := builtinName(n.CalleeExpression())
	if !ok {
		return
	}
	if !mustNew[name] {
		return
	}
	ctx.Report(n, name+" must be called with `new`")
}

func visitNew(ctx *engine.Context, n *wrapperchecker.Node) {
	name, ok := builtinName(n.CalleeExpression())
	if !ok {
		return
	}
	if !mustNotNew[name] {
		return
	}
	ctx.Report(n, name+" must be called without `new`")
}

// builtinName extracts the global builtin name from the callee
// expression. Returns ok=false if the callee isn't a recognized
// global form, or if the receiver name is shadowed somewhere in
// the file.
func builtinName(callee *wrapperchecker.Node) (string, bool) {
	callee = stripParens(callee)
	if callee == nil {
		return "", false
	}
	switch callee.Kind() {
	case wrapperchecker.KindIdentifier:
		name := callee.LiteralText()
		if !mustNew[name] && !mustNotNew[name] {
			return "", false
		}
		if nameIsBoundInFile(callee, name) {
			return "", false
		}
		return name, true
	case wrapperchecker.KindPropertyAccessExpression:
		name := callee.PropertyAccessName()
		if !mustNew[name] && !mustNotNew[name] {
			return "", false
		}
		recv := stripParens(callee.PropertyAccessReceiver())
		if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier {
			return "", false
		}
		recvName := recv.LiteralText()
		if recvName != "window" && recvName != "globalThis" {
			return "", false
		}
		return name, true
	}
	return "", false
}

var mustNew = map[string]bool{
	"ArrayBuffer":          true,
	"SharedArrayBuffer":    true,
	"DataView":             true,
	"Proxy":                true,
	"Int8Array":            true,
	"Uint8Array":           true,
	"Uint8ClampedArray":    true,
	"Int16Array":           true,
	"Uint16Array":          true,
	"Int32Array":           true,
	"Uint32Array":          true,
	"BigInt64Array":        true,
	"BigUint64Array":       true,
	"Float32Array":         true,
	"Float64Array":         true,
	"Map":                  true,
	"Set":                  true,
	"WeakMap":              true,
	"WeakSet":              true,
	"WeakRef":              true,
	"FinalizationRegistry": true,
}

var mustNotNew = map[string]bool{
	"BigInt": true,
	"Symbol": true,
}

func nameIsBoundInFile(ref *wrapperchecker.Node, name string) bool {
	root := ref
	for {
		p := root.Parent()
		if p == nil {
			break
		}
		root = p
	}
	found := false
	var walk func(c *wrapperchecker.Node) bool
	walk = func(c *wrapperchecker.Node) bool {
		if found {
			return true
		}
		if isBindingWithName(c, name) {
			found = true
			return true
		}
		c.ForEachChild(walk)
		return found
	}
	root.ForEachChild(walk)
	return found
}

func isBindingWithName(n *wrapperchecker.Node, name string) bool {
	switch n.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindInterfaceDeclaration,
		wrapperchecker.KindTypeAliasDeclaration:
		return bindingName(n) == name
	}
	return false
}

func bindingName(n *wrapperchecker.Node) string {
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

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
