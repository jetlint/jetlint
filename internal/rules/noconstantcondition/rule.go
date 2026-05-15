// Package noconstantcondition implements the no-constant-condition
// rule: a literal-only test in an `if`, `while`, `do-while`, `for`,
// or ternary either always passes or always fails, which is almost
// always a typo or unfinished development scaffolding.
//
// This is a deliberately conservative port — it flags the cases ESLint
// flags when the test is built only from values that the parser can
// recognise as constant without doing any reference / scope work:
// literals, function / class / object / array expressions, the
// `undefined` global, and parenthesised wrappers around them.
// Cases that need full constant-folding through logical operators,
// template substitutions, and `Boolean(...)` calls are deferred until
// a shared `is_constant` utility lands.
package noconstantcondition

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-constant-condition"

// CheckLoops controls whether constant conditions inside loops are
// flagged.
type CheckLoops int

const (
	// CheckLoopsAllExceptWhileTrue is the default: flag constant
	// conditions in loops except for the canonical `while(true)`
	// infinite-loop idiom.
	CheckLoopsAllExceptWhileTrue CheckLoops = iota
	// CheckLoopsAll flags every constant condition in a loop.
	CheckLoopsAll
	// CheckLoopsNone never flags constant conditions in loops.
	CheckLoopsNone
)

// Options configures the rule.
type Options struct {
	CheckLoops CheckLoops
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
		wrapperchecker.KindIfStatement:           r.visitIf,
		wrapperchecker.KindConditionalExpression: r.visitConditional,
		wrapperchecker.KindWhileStatement:        r.visitWhile,
		wrapperchecker.KindDoStatement:           r.visitDoWhile,
		wrapperchecker.KindForStatement:          r.visitFor,
	}
}

func (r *rule) visitIf(ctx *engine.Context, n *wrapperchecker.Node) {
	r.report(ctx, n.IfCondition())
}

func (r *rule) visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	r.report(ctx, n.ConditionalCondition())
}

func (r *rule) visitWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.CheckLoops == CheckLoopsNone {
		return
	}
	cond := n.WhileCondition()
	if r.opts.CheckLoops == CheckLoopsAllExceptWhileTrue && isLiteralTrue(cond) {
		return
	}
	r.report(ctx, cond)
}

func (r *rule) visitDoWhile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.CheckLoops == CheckLoopsNone {
		return
	}
	r.report(ctx, n.WhileCondition())
}

func (r *rule) visitFor(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.CheckLoops == CheckLoopsNone {
		return
	}
	r.report(ctx, n.ForStatementCondition())
}

func (r *rule) report(ctx *engine.Context, cond *wrapperchecker.Node) {
	if cond == nil {
		return
	}
	if isObviouslyConstant(cond) {
		ctx.Report(cond, "Unexpected constant condition")
	}
}

// isObviouslyConstant reports whether `n` is a value the parser can
// recognise as constant without scope / type information. The check
// is deliberately tight: any expression whose constancy depends on
// resolving an identifier or evaluating an operator returns false.
func isObviouslyConstant(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword,
		wrapperchecker.KindFalseKeyword,
		wrapperchecker.KindNullKeyword,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindRegularExpressionLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindClassExpression:
		return true
	case wrapperchecker.KindParenthesizedExpression:
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isObviouslyConstant(inner)
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "undefined"
	case wrapperchecker.KindPrefixUnaryExpression:
		op := n.PrefixUnaryOperator()
		if op == "void" || op == "typeof" {
			return true
		}
		if op == "!" || op == "+" || op == "-" || op == "~" {
			return isObviouslyConstant(n.PrefixUnaryOperand())
		}
	}
	return false
}

// isLiteralTrue reports whether the expression is the literal `true`
// (or `(true)` etc.) — the canonical infinite-loop idiom that the
// default `allExceptWhileTrue` config exempts.
func isLiteralTrue(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		n.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		return isLiteralTrue(inner)
	}
	return n.Kind() == wrapperchecker.KindTrueKeyword
}
