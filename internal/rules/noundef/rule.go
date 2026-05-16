// Package noundef implements the no-undef rule: flag references to
// identifiers that are not declared anywhere reachable. We rely on
// TS-go's symbol resolution rather than re-implementing scope
// analysis — when a free reference has no resolved symbol, it is
// undeclared. The rule also honors the `typeof <id>` exception
// (typeof a missing identifier is legal in JavaScript).
package noundef

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-undef"

// Options configures the rule.
type Options struct {
	// TypeofMode, when true, flags `typeof <undeclared>` as well as
	// bare references to undeclared identifiers. Defaults to false
	// to match ESLint's `typeof: false`.
	TypeofMode bool
}

// DefaultOptions returns ESLint's defaults.
func DefaultOptions() Options { return Options{TypeofMode: false} }

// New constructs the rule with default options.
func New() engine.Rule { return &rule{opts: DefaultOptions()} }

// NewWithOptions constructs the rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIdentifier: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isFreeReferenceContext(n) {
		return
	}
	// `typeof <id>` is allowed for undeclared `<id>` unless
	// TypeofMode is on.
	if !r.opts.TypeofMode && isTypeofOperand(n) {
		return
	}
	// References inside a TypeScript type-only construct are erased
	// at runtime, so an "undefined value" reading there is harmless.
	if isInTypePosition(n) {
		return
	}
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		ctx.Report(n, "'"+n.SourceText()+"' is not defined.")
		return
	}
	// Destructuring-assignment shorthand like `({a} = {})` gets
	// TS-resolved to a synthetic symbol attached to the pattern
	// itself, even when there is no real binding. Verify a separate
	// declaration with the same name exists somewhere in scope —
	// otherwise the assignment target is undeclared.
	if isDestructuringAssignmentShorthand(n) && !hasLexicalBinding(n, n.LiteralText()) {
		ctx.Report(n, "'"+n.SourceText()+"' is not defined.")
	}
}

// isDestructuringAssignmentShorthand reports whether `n` is the
// identifier of a `{a}` ShorthandPropertyAssignment whose parent
// ObjectLiteralExpression is the assignment target of `=` (i.e.
// `({a} = ...)`).
func isDestructuringAssignmentShorthand(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil || p.Kind() != wrapperchecker.KindShorthandPropertyAssignment {
		return false
	}
	gp := p.Parent()
	if gp == nil || gp.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return false
	}
	// Walk up past parens to find the parent of the literal — must
	// be a BinaryExpression with `=`.
	cur := gp
	for cur.Parent() != nil && cur.Parent().Kind() == wrapperchecker.KindParenthesizedExpression {
		cur = cur.Parent()
	}
	bx := cur.Parent()
	if bx == nil || bx.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	return bx.BinaryOperatorKind() == wrapperchecker.KindEqualsToken && bx.BinaryLeft().Same(cur)
}

// hasLexicalBinding walks the scope-introducing ancestors of `ref`
// looking for a sibling declaration of `name`. Mirrors the helper
// used by `no-constant-condition` for shadowing detection.
func hasLexicalBinding(ref *wrapperchecker.Node, name string) bool {
	if name == "" {
		return false
	}
	cur := ref
	for cur != nil {
		parent := cur.Parent()
		if parent == nil {
			break
		}
		if isScopeIntroducer(parent) {
			if scopeHasSiblingBinding(parent, cur, name) {
				return true
			}
		}
		cur = parent
	}
	return false
}

func isScopeIntroducer(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindSourceFile,
		wrapperchecker.KindBlock,
		wrapperchecker.KindModuleBlock,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction:
		return true
	}
	return false
}

func scopeHasSiblingBinding(scope, skip *wrapperchecker.Node, name string) bool {
	found := false
	scope.ForEachChild(func(c *wrapperchecker.Node) bool {
		if found || c == nil || c == skip {
			return false
		}
		switch c.Kind() {
		case wrapperchecker.KindVariableStatement:
			if list := c.VariableStatementDeclarationList(); list != nil {
				list.ForEachChild(func(decl *wrapperchecker.Node) bool {
					if decl.Kind() == wrapperchecker.KindVariableDeclaration {
						if nm := decl.DeclarationName(); nm != nil && nm.LiteralText() == name {
							found = true
						}
					}
					return found
				})
			}
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindParameter:
			if nm := c.DeclarationName(); nm != nil && nm.LiteralText() == name {
				found = true
			}
		}
		return found
	})
	return found
}

// isFreeReferenceContext reports whether the identifier appears in a
// position where it represents a value reference — not a declaration
// name, not a property access RHS, not a JSX attribute name, etc.
func isFreeReferenceContext(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindVariableDeclaration,
		wrapperchecker.KindParameter,
		wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindPropertyDeclaration,
		wrapperchecker.KindPropertySignature,
		wrapperchecker.KindEnumMember,
		wrapperchecker.KindBindingElement,
		wrapperchecker.KindTypeParameter:
		// These nodes have an identifier *name* plus other
		// children (initializers, types, bodies). Only the name is
		// a declaration; the rest are value/type references that
		// should still be checked.
		if name := p.DeclarationName(); name != nil && name.Same(n) {
			return false
		}
		return true
	case wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindExportSpecifier,
		wrapperchecker.KindNamespaceExport,
		wrapperchecker.KindLabeledStatement,
		wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement,
		wrapperchecker.KindJsxAttribute:
		// The identifier names the declared / imported / labeled
		// thing; not a free reference.
		return false
	case wrapperchecker.KindPropertyAccessExpression:
		// `a.b` — only `a` is a free reference; `b` is the property
		// name (member of the receiver, no scope lookup).
		return p.PropertyAccessReceiver().Same(n)
	case wrapperchecker.KindPropertyAssignment:
		// `{ a: foo }` — the key is not a reference; only the value
		// is. The key is the first child of PropertyAssignment.
		if n.Pos() == firstPropertyAssignmentKeyPos(p) {
			return false
		}
		return true
	case wrapperchecker.KindShorthandPropertyAssignment:
		// `{a}` is both declaration-or-reference depending on
		// context. In an object literal it reads `a`; in an object
		// pattern (destructuring) it declares `a`. The parent of
		// ShorthandPropertyAssignment is an ObjectLiteral or
		// ObjectBindingPattern.
		gp := p.Parent()
		if gp == nil {
			return true
		}
		if gp.Kind() == wrapperchecker.KindObjectBindingPattern {
			return false
		}
		return true
	case wrapperchecker.KindJsxOpeningElement,
		wrapperchecker.KindJsxSelfClosingElement,
		wrapperchecker.KindJsxClosingElement:
		// JSX tag names are value references when they start with
		// uppercase or contain a dot; otherwise they are intrinsic
		// element names (strings). We treat all as references — the
		// caller resolves to nil iff it isn't declared.
		return true
	}
	return true
}

// isInTypePosition reports whether `n` sits inside a TypeScript
// type-only construct (TypeReference, TypeQuery, TypeLiteral,
// FunctionType, InterfaceDeclaration, TypeAliasDeclaration). Such
// references are erased at runtime.
func isInTypePosition(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindTypeReference,
			wrapperchecker.KindTypeQuery,
			wrapperchecker.KindTypeLiteral,
			wrapperchecker.KindFunctionType,
			wrapperchecker.KindInterfaceDeclaration,
			wrapperchecker.KindTypeAliasDeclaration:
			return true
		case wrapperchecker.KindSourceFile,
			wrapperchecker.KindBlock,
			wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindClassDeclaration,
			wrapperchecker.KindClassExpression:
			return false
		}
	}
	return false
}

// isTypeofOperand reports whether `n` is the direct operand of a
// `typeof` expression. We walk up through any parenthesized wrappers.
func isTypeofOperand(n *wrapperchecker.Node) bool {
	cur := n
	for {
		p := cur.Parent()
		if p == nil {
			return false
		}
		if p.Kind() == wrapperchecker.KindParenthesizedExpression {
			cur = p
			continue
		}
		return p.Kind() == wrapperchecker.KindTypeOfExpression
	}
}

// firstPropertyAssignmentKeyPos returns the position of the first
// child of a PropertyAssignment, which is the key. Used to decide
// whether an identifier child of PropertyAssignment is the key (and
// therefore not a free reference).
func firstPropertyAssignmentKeyPos(p *wrapperchecker.Node) int {
	first := p.FirstChild()
	if first == nil {
		return -1
	}
	return first.Pos()
}
