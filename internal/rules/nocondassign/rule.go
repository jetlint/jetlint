// Package nocondassign implements the no-cond-assign rule: an
// assignment inside an if/while/do/for/conditional test is almost
// always a `=` typo for `==` or `===`. The default ("except-parens")
// mode treats `if ((x = …))` as an intentional opt-in; the "always"
// mode warns even with parens.
package nocondassign

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-cond-assign"

// Mode controls which assignments inside a condition are flagged.
type Mode string

const (
	// ModeExceptParens (the ESLint default) tolerates assignments that
	// are wrapped in extra parentheses, treating that as an explicit
	// opt-in by the author.
	ModeExceptParens Mode = "except-parens"
	// ModeAlways reports every assignment found inside a test, even if
	// the author wrapped it in parens.
	ModeAlways Mode = "always"
)

// Options is the rule's configurable shape.
type Options struct {
	Mode Mode
}

// OptionsFromJSON parses the ESLint-style options array. ESLint encodes
// no-cond-assign options as a single string ("always" or "except-parens"),
// not the object shape most rules use.
func OptionsFromJSON(raw any) Options {
	opts := Options{Mode: ModeExceptParens}
	if raw == nil {
		return opts
	}
	if s, ok := raw.(string); ok {
		switch Mode(s) {
		case ModeAlways:
			opts.Mode = ModeAlways
		case ModeExceptParens:
			opts.Mode = ModeExceptParens
		}
		return opts
	}
	// Tolerate the JSON-marshalled form too so downstream callers can
	// just hand the raw bytes from a config file in.
	if b, err := json.Marshal(raw); err == nil {
		var s string
		if json.Unmarshal(b, &s) == nil {
			return OptionsFromJSON(s)
		}
	}
	return opts
}

func New() engine.Rule { return NewWithOptions(Options{Mode: ModeExceptParens}) }

// NewWithOptions returns a rule instance configured with opts. Empty
// or unrecognised modes fall back to ModeExceptParens.
func NewWithOptions(opts Options) engine.Rule {
	if opts.Mode != ModeAlways {
		opts.Mode = ModeExceptParens
	}
	return &rule{opts: opts}
}

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement:           r.visitTest,
		wrapperchecker.KindWhileStatement:        r.visitTest,
		wrapperchecker.KindDoStatement:           r.visitTest,
		wrapperchecker.KindForStatement:          r.visitTest,
		wrapperchecker.KindConditionalExpression: r.visitTest,
	}
}

// visitTest looks at the condition expression of a control-flow node.
// In except-parens mode it reports only a direct assignment; in always
// mode it also reports assignments hidden behind parens, so long as
// they are not nested inside a function/block boundary.
func (r *rule) visitTest(ctx *engine.Context, n *wrapperchecker.Node) {
	test := conditionOf(n)
	if test == nil {
		return
	}
	if r.opts.Mode == ModeAlways {
		r.reportAssignmentsIn(ctx, test, test)
		return
	}
	// ConditionalExpression is special: ESLint unwraps the test's
	// parentheses even in except-parens mode, so `cond ? a : b` reports
	// for `(x = 0) ? a : b` while `if ((x = 0))` does not. Mirror that.
	check := test
	if n.Kind() == wrapperchecker.KindConditionalExpression {
		check = stripParens(check)
	}
	if isAssignmentExpression(check) {
		ctx.Report(check, "Expected a conditional expression and instead saw an assignment.")
	}
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// reportAssignmentsIn walks expr (not crossing function boundaries) and
// reports each assignment expression found. limit is the original test
// span — once the walk passes outside it we stop, so nested control
// flow with its own test does not get the outer reports.
func (r *rule) reportAssignmentsIn(ctx *engine.Context, expr, limit *wrapperchecker.Node) {
	if expr == nil {
		return
	}
	if isFunctionBoundary(expr) {
		return
	}
	if isAssignmentExpression(expr) {
		ctx.Report(expr, "Expected a conditional expression and instead saw an assignment.")
	}
	expr.ForEachChild(func(child *wrapperchecker.Node) bool {
		r.reportAssignmentsIn(ctx, child, limit)
		return false
	})
}

func conditionOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindIfStatement:
		return n.IfCondition()
	case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
		return n.WhileCondition()
	case wrapperchecker.KindForStatement:
		return n.ForStatementCondition()
	case wrapperchecker.KindConditionalExpression:
		return n.ConditionalCondition()
	}
	return nil
}

func isFunctionBoundary(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindFunctionDeclaration,
		wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindConstructor,
		wrapperchecker.KindGetAccessor,
		wrapperchecker.KindSetAccessor:
		return true
	}
	return false
}

// isAssignmentExpression reports whether n is *directly* an assignment
// expression — extra parentheses count as the author's intentional
// opt-in under the except-parens default and so do not unwrap here.
func isAssignmentExpression(n *wrapperchecker.Node) bool {
	if n == nil || n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	return isAssignmentOperator(n.BinaryOperatorKind())
}

func isAssignmentOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindLessThanLessThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindGreaterThanGreaterThanGreaterThanEqualsToken,
		wrapperchecker.KindAmpersandEqualsToken,
		wrapperchecker.KindBarEqualsToken,
		wrapperchecker.KindCaretEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

// Ensure fmt import is retained; used elsewhere for future error wraps.
var _ = fmt.Sprintf
