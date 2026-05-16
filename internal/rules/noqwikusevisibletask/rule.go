// Package noqwikusevisibletask implements no-qwik-use-visible-task:
// Qwik's `useVisibleTask$` runs eagerly on the client, which defeats
// the framework's resumability story. The rule flags any direct
// invocation of `useVisibleTask$` so teams can confirm an eager
// task is really what they want — usually `useTask$` is the right
// alternative. Non-call references (imports, assignments, variable
// shadows) are left alone.
package noqwikusevisibletask

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-qwik-use-visible-task"

const target = "useVisibleTask$"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil {
		return
	}
	if callee.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if callee.LiteralText() != target {
		return
	}
	if fileShadowsTarget(call) {
		return
	}
	ctx.Report(call, "avoid `useVisibleTask$`; prefer `useTask$` so Qwik can stay resumable")
}

// fileShadowsTarget walks up to the source file and scans for a
// local binding (var/let/const/function/import) named useVisibleTask$.
// When one exists, the call almost certainly resolves to that local
// rather than Qwik's import — the symbol-resolution check biome
// performs without us paying for type lookup on every call.
func fileShadowsTarget(call *wrapperchecker.Node) bool {
	root := call
	for root.Parent() != nil {
		root = root.Parent()
	}
	if root.Kind() != wrapperchecker.KindSourceFile {
		return false
	}
	return walkForBinding(root)
}

func walkForBinding(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindParameter:
		if nodeNameIs(n, target) {
			return true
		}
	}
	var found bool
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if walkForBinding(c) {
			found = true
			return true
		}
		return false
	})
	return found
}

func nodeNameIs(n *wrapperchecker.Node, want string) bool {
	var match bool
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier && c.LiteralText() == want {
			match = true
			return true
		}
		return false
	})
	return match
}
