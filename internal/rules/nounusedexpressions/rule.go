// Package nounusedexpressions implements the no-unused-expressions
// rule: ExpressionStatements whose expression has no observable side
// effect (a bare identifier, member access, literal, comparison, etc.)
// are almost always a mistake. Calls, assignments, awaits, yields,
// updates, new, delete, void, dynamic import — all have side effects
// and are silently allowed.
//
// Matches oxlint and typescript-eslint's no-unused-expressions:
//   - Directive prologues (`"use strict";` at the start of a program,
//     function body, module body, or static block) are skipped.
//   - TS-specific wrappers (`expr as T`, `expr satisfies T`,
//     `<T>expr`, `expr!`, `expr<T>`) unwrap to their inner expression
//     before the decision is made.
//   - Optional-chain calls (`a?.()`) are allowed because the call
//     itself is a side effect; optional-chain member access is not.
//   - The four upstream options (allowShortCircuit, allowTernary,
//     allowTaggedTemplates, enforceForJSX) are supported.
package nounusedexpressions

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unused-expressions"

// Options configures the rule.
type Options struct {
	AllowShortCircuit    bool
	AllowTernary         bool
	AllowTaggedTemplates bool
	EnforceForJSX        bool
}

// DefaultOptions returns the upstream default configuration.
func DefaultOptions() Options { return Options{} }

// OptionsFromJSON parses the rule's options object from raw JSON.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-unused-expressions options must be a JSON object: %w", err)
	}
	for k, v := range fields {
		switch k {
		case "allowShortCircuit":
			if err := json.Unmarshal(v, &out.AllowShortCircuit); err != nil {
				return Options{}, fmt.Errorf("no-unused-expressions: allowShortCircuit must be boolean: %w", err)
			}
		case "allowTernary":
			if err := json.Unmarshal(v, &out.AllowTernary); err != nil {
				return Options{}, fmt.Errorf("no-unused-expressions: allowTernary must be boolean: %w", err)
			}
		case "allowTaggedTemplates":
			if err := json.Unmarshal(v, &out.AllowTaggedTemplates); err != nil {
				return Options{}, fmt.Errorf("no-unused-expressions: allowTaggedTemplates must be boolean: %w", err)
			}
		case "enforceForJSX":
			if err := json.Unmarshal(v, &out.EnforceForJSX); err != nil {
				return Options{}, fmt.Errorf("no-unused-expressions: enforceForJSX must be boolean: %w", err)
			}
		}
	}
	return out, nil
}

// New constructs the rule with default options.
func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

// NewWithOptions constructs the rule with the supplied options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindExpressionStatement: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.ExpressionStatementExpression()
	if expr == nil {
		return
	}
	if isDirective(n, expr) {
		return
	}
	if !r.isDisallowed(expr) {
		return
	}
	ctx.Report(n, "Expected an assignment or function call and instead saw an expression")
}

// isDisallowed reports whether the given expression, when used as the
// whole expression of an ExpressionStatement, has no observable side
// effect. Mirrors the structure of oxlint's `is_disallowed`.
func (r *rule) isDisallowed(expr *wrapperchecker.Node) bool {
	if expr == nil {
		return false
	}
	switch expr.Kind() {
	// Side-effecting expressions: allowed.
	case wrapperchecker.KindCallExpression,
		wrapperchecker.KindNewExpression,
		wrapperchecker.KindAwaitExpression,
		wrapperchecker.KindYieldExpression,
		wrapperchecker.KindDeleteExpression,
		wrapperchecker.KindVoidExpression,
		wrapperchecker.KindPostfixUnaryExpression,
		wrapperchecker.KindSuperKeyword:
		return false

	// Literals, identifiers, and other pure expressions: disallowed.
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTemplateExpression,
		wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindThisKeyword,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindPropertyAccessExpression,
		wrapperchecker.KindElementAccessExpression,
		wrapperchecker.KindClassExpression,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMetaProperty,
		wrapperchecker.KindTypeOfExpression:
		return true

	// Tagged template literals: only disallowed when not explicitly
	// permitted by the option.
	case wrapperchecker.KindTaggedTemplateExpression:
		return !r.opts.AllowTaggedTemplates

	// JSX: only flagged when enforceForJSX is set.
	case wrapperchecker.KindJsxSelfClosingElement,
		wrapperchecker.KindJsxElement,
		wrapperchecker.KindJsxFragment:
		return r.opts.EnforceForJSX

	// Unary `!`, `+`, `-`, `~` etc. are disallowed; the prefix update
	// operators `++` and `--` have side effects.
	case wrapperchecker.KindPrefixUnaryExpression:
		op := expr.PrefixUnaryOperator()
		if op == "++" || op == "--" {
			return false
		}
		return true

	// TS expression wrappers: recurse into the inner expression.
	case wrapperchecker.KindParenthesizedExpression,
		wrapperchecker.KindNonNullExpression,
		wrapperchecker.KindExpressionWithTypeArguments:
		return r.isDisallowed(firstChild(expr))
	case wrapperchecker.KindAsExpression:
		return r.isDisallowed(expr.AsExpressionSource())
	case wrapperchecker.KindTypeAssertionExpression:
		return r.isDisallowed(expr.TypeAssertionSource())
	case wrapperchecker.KindSatisfiesExpression:
		// `expr satisfies T` is treated as a pure type-level operation
		// in upstream — but oxlint allows it because the satisfies
		// expression itself counts as a use.
		return false

	// BinaryExpression covers assignment, sequence (comma), logical,
	// arithmetic, comparison. Assignment is the only side-effecting
	// shape; logical short-circuit gets the AllowShortCircuit override.
	case wrapperchecker.KindBinaryExpression:
		op := expr.BinaryOperatorKind()
		if isAssignmentToken(op) {
			return false
		}
		if r.opts.AllowShortCircuit && isShortCircuitToken(op) {
			return r.isDisallowed(expr.BinaryRight())
		}
		return true

	case wrapperchecker.KindConditionalExpression:
		if r.opts.AllowTernary {
			whenTrue, whenFalse := expr.ConditionalBranches()
			return r.isDisallowed(whenTrue) || r.isDisallowed(whenFalse)
		}
		return true
	}
	return false
}

// firstChild returns the first child node of n, or nil. Used to unwrap
// single-child wrapper expressions (Parenthesized, NonNull,
// ExpressionWithTypeArguments) when the wrapper doesn't expose a
// dedicated accessor.
func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var out *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		out = c
		return true
	})
	return out
}

func isAssignmentToken(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

func isShortCircuitToken(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken,
		wrapperchecker.KindQuestionQuestionToken:
		return true
	}
	return false
}

// isDirective reports whether stmt is part of a directive prologue at
// the start of its containing block / source file. A directive is a
// string-literal ExpressionStatement, and a prologue runs until the
// first non-directive statement in the same body. Detection is purely
// positional: walk the parent's statement list from the top; if every
// statement before stmt is itself a string-literal ExpressionStatement,
// stmt is in the prologue.
func isDirective(stmt, expr *wrapperchecker.Node) bool {
	if expr.Kind() != wrapperchecker.KindStringLiteral &&
		expr.Kind() != wrapperchecker.KindNoSubstitutionTemplateLiteral {
		return false
	}
	if expr.Kind() == wrapperchecker.KindNoSubstitutionTemplateLiteral {
		// Backtick directives are not recognised by the spec.
		return false
	}
	parent := stmt.Parent()
	if parent == nil {
		return false
	}
	if !isDirectiveContainer(parent) {
		return false
	}
	stmts := containerStatements(parent)
	for _, s := range stmts {
		if s.Same(stmt) {
			return true
		}
		if !isStringLiteralExpressionStatement(s) {
			return false
		}
	}
	return false
}

func isDirectiveContainer(p *wrapperchecker.Node) bool {
	switch p.Kind() {
	case wrapperchecker.KindSourceFile, wrapperchecker.KindModuleBlock:
		return true
	case wrapperchecker.KindBlock:
		// A Block only opens a directive prologue when it is the body
		// of a function-like — function bodies, method bodies, arrow
		// bodies, constructors, and accessors. Free-standing blocks
		// (if/while/try bodies, class static blocks) do not.
		gp := p.Parent()
		if gp == nil {
			return false
		}
		switch gp.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration,
			wrapperchecker.KindConstructor,
			wrapperchecker.KindGetAccessor,
			wrapperchecker.KindSetAccessor:
			return true
		}
	}
	return false
}

func containerStatements(p *wrapperchecker.Node) []*wrapperchecker.Node {
	if p.Kind() == wrapperchecker.KindBlock {
		return p.BlockStatements()
	}
	// SourceFile and ModuleBlock have no dedicated accessor; iterate
	// children and keep only statement-like nodes (filtering trailing
	// EndOfFileToken on the source file).
	var out []*wrapperchecker.Node
	p.ForEachChild(func(c *wrapperchecker.Node) bool {
		out = append(out, c)
		return false
	})
	return out
}

func isStringLiteralExpressionStatement(n *wrapperchecker.Node) bool {
	if n.Kind() != wrapperchecker.KindExpressionStatement {
		return false
	}
	expr := n.ExpressionStatementExpression()
	return expr != nil && expr.Kind() == wrapperchecker.KindStringLiteral
}
