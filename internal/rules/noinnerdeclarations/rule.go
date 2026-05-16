// Package noinnerdeclarations implements the no-inner-declarations
// rule: `var` and `function` declarations are only meaningful at the
// top level of a script, module, or function body. Declaring them
// inside a non-function block (an `if`, a loop, etc.) is almost
// always a mistake — the binding hoists out of the block to the
// enclosing function scope, which is rarely what the author wanted.
package noinnerdeclarations

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-inner-declarations"

// Mode selects what declarations to flag.
type Mode int

const (
	// ModeFunctions flags inner function declarations only.
	ModeFunctions Mode = iota
	// ModeBoth flags both inner var and function declarations.
	ModeBoth
)

// BlockScopedFunctions controls whether function declarations are
// allowed in nested blocks under strict mode (ES6+ block-scoped fn
// semantics).
type BlockScopedFunctions int

const (
	// BlockScopedAllow lets function declarations in nested blocks
	// through when the enclosing scope is strict.
	BlockScopedAllow BlockScopedFunctions = iota
	// BlockScopedDisallow always flags function declarations in
	// nested blocks regardless of strict mode.
	BlockScopedDisallow
)

// Options configures the rule.
type Options struct {
	Mode                 Mode
	BlockScopedFunctions BlockScopedFunctions
}

// New constructs a rule with default options.
func New() engine.Rule { return &rule{} }

// NewWithOptions constructs a rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindFunctionDeclaration: r.visitFunction,
		wrapperchecker.KindVariableStatement:   r.visitVariableStatement,
	}
}

func (r *rule) visitFunction(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.Mode == ModeFunctions && r.opts.BlockScopedFunctions == BlockScopedAllow {
		if isStrictMode(n) {
			return
		}
	}
	r.checkParent(ctx, n, "function")
}

func (r *rule) visitVariableStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.Mode != ModeBoth {
		return
	}
	if !isVarStatement(n) {
		return
	}
	r.checkParent(ctx, n, "variable")
}

func (r *rule) checkParent(ctx *engine.Context, n *wrapperchecker.Node, decl string) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	allowBareBlockChain := decl == "function" && r.opts.BlockScopedFunctions == BlockScopedAllow
	if isAllowedContainer(parent, allowBareBlockChain) {
		return
	}
	body := enclosingBodyKind(parent)
	ctx.Report(n, "Move "+decl+" declaration to "+body+" root")
}

// isVarStatement reports whether a VariableStatement node declares
// `var` bindings (as opposed to `let`, `const`, `using`, or
// `await using`). The wrapper exposes `const` / `using` predicates
// but not a positive `let` test, so we rely on the source-text prefix
// to distinguish `var` from `let`.
func isVarStatement(n *wrapperchecker.Node) bool {
	list := n.VariableStatementDeclarationList()
	if list == nil {
		return false
	}
	if list.IsConstVariableDeclaration() {
		return false
	}
	if list.IsUsingDeclaration() || list.IsAwaitUsingDeclaration() {
		return false
	}
	src := strings.TrimLeft(n.SourceText(), " \t")
	return strings.HasPrefix(src, "var")
}

// isAllowedContainer reports whether `parent` is one of the contexts
// where a function/var declaration is allowed: a top-level script,
// a function/method/accessor body block, a class static block, or
// the immediate child of an export declaration.
//
// Nested bare `{ … }` blocks inside a function body are also
// permitted: `function foo() { { function bar() {} } }` lifts to
// `foo`'s scope and is treated as a function-body declaration. This
// only applies when the chain consists entirely of plain blocks —
// blocks attached to a control-flow statement (`if`, `do`, `while`,
// `for`, `try`, `switch`) terminate the chain because they're
// not pure scoping aliases.
func isAllowedContainer(parent *wrapperchecker.Node, walkBareBlocks bool) bool {
	switch parent.Kind() {
	case wrapperchecker.KindSourceFile,
		wrapperchecker.KindModuleBlock,
		wrapperchecker.KindExportDeclaration,
		wrapperchecker.KindExportAssignment:
		return true
	case wrapperchecker.KindBlock:
		cur := parent
		if walkBareBlocks {
			for cur.Parent() != nil && cur.Parent().Kind() == wrapperchecker.KindBlock {
				cur = cur.Parent()
			}
		}
		grand := cur.Parent()
		if grand == nil {
			return false
		}
		if isFunctionLike(grand.Kind()) {
			return true
		}
		return isClassStaticBlock(grand)
	}
	return false
}

// isClassStaticBlock reports whether `n` is a ClassStaticBlock
// declaration. The wrapper does not expose
// `KindClassStaticBlockDeclaration`, so we identify it structurally:
// a node whose own kind is neither a function-like nor a class member
// kind, but whose direct parent is a class.
func isClassStaticBlock(n *wrapperchecker.Node) bool {
	if isFunctionLike(n.Kind()) {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindBlock,
		wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindConstructor:
		return false
	}
	parent := n.Parent()
	if parent == nil {
		return false
	}
	return parent.Kind() == wrapperchecker.KindClassDeclaration ||
		parent.Kind() == wrapperchecker.KindClassExpression
}

func isFunctionLike(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor,
		wrapperchecker.KindConstructor:
		return true
	}
	return false
}

// enclosingBodyKind returns the human-readable name of the enclosing
// container — used to phrase the diagnostic message.
func enclosingBodyKind(parent *wrapperchecker.Node) string {
	for cur := parent; cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindSourceFile:
			return "program"
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor,
			wrapperchecker.KindConstructor:
			return "function body"
		}
	}
	return "program"
}

// isStrictMode reports whether `n`'s enclosing scope is in strict
// mode. A scope is strict when it sits inside a class body, when the
// enclosing function/script body opens with a `"use strict"`
// directive, or when the source file is a module (it has at least
// one top-level import/export).
func isStrictMode(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			return true
		}
		if cur.Kind() == wrapperchecker.KindBlock {
			grand := cur.Parent()
			if grand != nil && isFunctionLike(grand.Kind()) {
				if hasUseStrictPrologue(cur) {
					return true
				}
			}
		}
		if cur.Kind() == wrapperchecker.KindSourceFile {
			return sourceFileIsModule(cur) || hasUseStrictPrologue(cur)
		}
	}
	return false
}

// sourceFileIsModule reports whether a SourceFile has any top-level
// `import`, `export`, or `export default` declaration — the syntactic
// signal that makes a file an ES module.
func sourceFileIsModule(sf *wrapperchecker.Node) bool {
	isModule := false
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindImportDeclaration,
			wrapperchecker.KindExportDeclaration,
			wrapperchecker.KindExportAssignment:
			isModule = true
			return true
		}
		return false
	})
	return isModule
}

// hasUseStrictPrologue checks whether the first statement of `n`
// (a Block or SourceFile) is an ExpressionStatement whose expression
// is the string literal `"use strict"`.
func hasUseStrictPrologue(n *wrapperchecker.Node) bool {
	found := false
	first := true
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !first {
			return true
		}
		first = false
		if c.Kind() != wrapperchecker.KindExpressionStatement {
			return true
		}
		expr := c.ExpressionStatementExpression()
		if expr == nil || expr.Kind() != wrapperchecker.KindStringLiteral {
			return true
		}
		txt := strings.TrimSpace(expr.SourceText())
		if txt == `"use strict"` || txt == `'use strict'` {
			found = true
		}
		return true
	})
	return found
}
