// Package nounsafeoptionalchaining implements the
// no-unsafe-optional-chaining rule: an optional-chain expression
// (`obj?.foo`) can short-circuit to `undefined`, and feeding that
// undefined into a context that demands a value — a member access on
// the chain's result, calling it, destructuring it, an arithmetic
// operation with the `disallowArithmeticOperators` option — almost
// always throws at runtime. The rule walks outward from each chain's
// outermost link to identify those contexts and reports them.
package nounsafeoptionalchaining

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unsafe-optional-chaining"

// Options configures the rule.
type Options struct {
	// DisallowArithmeticOperators, when true, also flags optional
	// chains used as operands of arithmetic operators (their `NaN`
	// result is rarely intended).
	DisallowArithmeticOperators bool
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
		wrapperchecker.KindPropertyAccessExpression: r.visit,
		wrapperchecker.KindElementAccessExpression:  r.visit,
		wrapperchecker.KindCallExpression:           r.visit,
	}
}

type errType int

const (
	errNone errType = iota
	errUsage
	errArith
)

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Only react to the outermost link of an optional chain. The
	// engine visits every member-access node, so we need to filter
	// down to "this is part of a chain AND my parent is not part of
	// the same chain".
	if !n.IsOptionalChain() {
		return
	}
	parent := n.Parent()
	if parent != nil && parent.IsOptionalChain() {
		return
	}
	switch r.classify(n) {
	case errUsage:
		ctx.Report(n, "Unsafe usage of optional chaining")
	case errArith:
		ctx.Report(n, "Unsafe arithmetic operation on optional chaining")
	}
}

func (r *rule) classify(top *wrapperchecker.Node) errType {
	curr := top
	parent := curr.Parent()
	for parent != nil {
		switch parent.Kind() {
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindAsExpression,
			wrapperchecker.KindSatisfiesExpression,
			wrapperchecker.KindNonNullExpression,
			wrapperchecker.KindTypeAssertionExpression:
			curr = parent
			parent = curr.Parent()
			continue
		case wrapperchecker.KindAwaitExpression:
			curr = parent
			parent = curr.Parent()
			continue
		case wrapperchecker.KindBinaryExpression:
			result, propagate := r.classifyBinary(parent, curr)
			if propagate {
				curr = parent
				parent = curr.Parent()
				continue
			}
			return result
		case wrapperchecker.KindConditionalExpression:
			wt := parent.ConditionalWhenTrue()
			wf := parent.ConditionalWhenFalse()
			if isPositionMatch(wt, curr) || isPositionMatch(wf, curr) {
				curr = parent
				parent = curr.Parent()
				continue
			}
			return errNone
		case wrapperchecker.KindCallExpression:
			if !parent.IsOptionalChainRoot() && isPositionMatch(parent.CalleeExpression(), curr) {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindNewExpression:
			if isPositionMatch(parent.CalleeExpression(), curr) {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindPropertyAccessExpression:
			if !parent.IsOptionalChainRoot() && isPositionMatch(parent.PropertyAccessReceiver(), curr) {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindElementAccessExpression:
			if !parent.IsOptionalChainRoot() && isPositionMatch(parent.ElementAccessReceiver(), curr) {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindTaggedTemplateExpression:
			if isPositionMatch(firstChild(parent), curr) {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindForOfStatement:
			if isPositionMatch(parent.ForInOrOfExpression(), curr) {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindPrefixUnaryExpression:
			if r.opts.DisallowArithmeticOperators {
				op := parent.PrefixUnaryOperator()
				if op == "+" || op == "-" {
					return errArith
				}
			}
			return errNone
		case wrapperchecker.KindSpreadElement:
			gp := parent.Parent()
			if gp != nil && gp.Kind() == wrapperchecker.KindArrayLiteralExpression {
				return errUsage
			}
			return errNone
		case wrapperchecker.KindVariableDeclaration:
			name := parent.VariableDeclarationName()
			if isDestructuringPattern(name) {
				init := parent.VariableDeclarationInitializer()
				if isPositionMatch(init, curr) {
					return errUsage
				}
			}
			return errNone
		case wrapperchecker.KindExpressionWithTypeArguments:
			gp := parent.Parent()
			if gp != nil && gp.Kind() == wrapperchecker.KindHeritageClause {
				return errUsage
			}
			return errNone
		}
		return errNone
	}
	return errNone
}

// classifyBinary inspects a BinaryExpression parent of `curr`. It
// returns whether the chain's value escapes the binary (propagate)
// alongside the unsafe-context classification that applies if it
// doesn't.
func (r *rule) classifyBinary(bin, curr *wrapperchecker.Node) (errType, bool) {
	op := bin.BinaryOperatorKind()
	left := bin.BinaryLeft()
	right := bin.BinaryRight()
	if isAssignmentOperator(op) {
		if !isPositionMatch(right, curr) {
			return errNone, false
		}
		if left != nil && isDestructuringPattern(left) {
			return errUsage, false
		}
		if r.opts.DisallowArithmeticOperators && isArithmeticAssignOp(op) {
			return errArith, false
		}
		return errNone, false
	}
	if isLogicalOperator(op) {
		if op == wrapperchecker.KindAmpersandAmpersandToken {
			if isPositionMatch(left, curr) || isPositionMatch(right, curr) {
				return errNone, true
			}
			return errNone, false
		}
		if isPositionMatch(right, curr) {
			return errNone, true
		}
		return errNone, false
	}
	if isRelationalBinary(bin) && isPositionMatch(right, curr) {
		return errUsage, false
	}
	if r.opts.DisallowArithmeticOperators && isArithmeticBinaryOp(op) {
		if isPositionMatch(left, curr) || isPositionMatch(right, curr) {
			return errArith, false
		}
	}
	return errNone, false
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		first = c
		return true
	})
	return first
}

func isDestructuringPattern(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindObjectBindingPattern,
		wrapperchecker.KindArrayBindingPattern,
		wrapperchecker.KindObjectLiteralExpression,
		wrapperchecker.KindArrayLiteralExpression:
		return true
	}
	return false
}

func isPositionMatch(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Pos() == b.Pos() && a.End() == b.End()
}

func isAssignmentOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsToken,
		wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken,
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

func isLogicalOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindBarBarToken,
		wrapperchecker.KindQuestionQuestionToken:
		return true
	}
	return false
}

// isRelationalBinary reports whether `bin` is a BinaryExpression
// whose operator is `in` or `instanceof`. The wrapper doesn't expose
// these keyword Kinds, so we extract the operator text from the
// source between the two operands and compare strings.
func isRelationalBinary(bin *wrapperchecker.Node) bool {
	op := operatorText(bin)
	return op == "in" || op == "instanceof"
}

// operatorText returns the trimmed source text between the two
// operands of a BinaryExpression — the textual representation of
// its operator.
func operatorText(n *wrapperchecker.Node) string {
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return ""
	}
	src := n.SourceText()
	nodeStart := n.End() - len(src)
	startRel := left.End() - nodeStart
	endRel := right.Pos() - nodeStart
	if startRel < 0 || endRel <= startRel || endRel > len(src) {
		return ""
	}
	return strings.TrimSpace(src[startRel:endRel])
}

// isArithmeticBinaryOp matches `+` and `-` only; the wrapper does
// not expose KindAsteriskToken / KindSlashToken / KindPercentToken,
// so we leave `*`, `/`, `%`, and `**` to a future expansion.
func isArithmeticBinaryOp(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindPlusToken,
		wrapperchecker.KindMinusToken:
		return true
	}
	return false
}

func isArithmeticAssignOp(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindPlusEqualsToken,
		wrapperchecker.KindMinusEqualsToken,
		wrapperchecker.KindAsteriskEqualsToken,
		wrapperchecker.KindSlashEqualsToken,
		wrapperchecker.KindPercentEqualsToken,
		wrapperchecker.KindAsteriskAsteriskEqualsToken:
		return true
	}
	return false
}
