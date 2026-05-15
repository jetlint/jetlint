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
	sym := ctx.Checker().SymbolOf(n)
	if sym == nil {
		ctx.Report(n, "'"+n.SourceText()+"' is not defined.")
	}
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
		wrapperchecker.KindImportSpecifier,
		wrapperchecker.KindImportClause,
		wrapperchecker.KindNamespaceImport,
		wrapperchecker.KindExportSpecifier,
		wrapperchecker.KindNamespaceExport,
		wrapperchecker.KindTypeParameter,
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
