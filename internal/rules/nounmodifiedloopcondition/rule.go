// Package nounmodifiedloopcondition implements the
// no-unmodified-loop-condition rule. A `while`/`do-while`/`for` loop
// whose condition reads only variables that the body never assigns
// to or otherwise mutates either runs zero times or forever — almost
// always a bug. We collect the condition's identifier references,
// then walk the loop body looking for assignments / update
// expressions against the same symbols.
package nounmodifiedloopcondition

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unmodified-loop-condition"

// New constructs the rule.
func New() engine.Rule { return &rule{} }

type rule struct{}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindWhileStatement: r.visit,
		wrapperchecker.KindDoStatement:    r.visit,
		wrapperchecker.KindForStatement:   r.visitFor,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	cond := n.WhileCondition()
	body := n.IterationBody()
	r.check(ctx, cond, body, n)
}

func (r *rule) visitFor(ctx *engine.Context, n *wrapperchecker.Node) {
	cond := n.ForStatementCondition()
	body := n.IterationBody()
	r.checkFor(ctx, cond, body, n.ForStatementIncrementor())
}

func (r *rule) check(ctx *engine.Context, cond, body, loop *wrapperchecker.Node) {
	r.checkFor(ctx, cond, body, nil)
	_ = loop
}

func (r *rule) checkFor(ctx *engine.Context, cond, body, updater *wrapperchecker.Node) {
	if cond == nil || body == nil {
		return
	}
	if conditionHasPropertyAccess(cond) {
		// Property access can hide getter side effects; ESLint
		// excludes such conditions from the rule entirely.
		return
	}
	checker := ctx.Checker()
	// Conservative: if the body contains any call or `new`
	// expression, assume the callee mutates one of the variables.
	if bodyHasFunctionLikeCall(body) {
		return
	}
	// Split the condition into top-level operands by short-circuit
	// operators (`&&`, `||`, `??`). Each operand stands as a
	// "group" — within a group, all referenced variables jointly
	// determine the result. A group is static (and gets flagged)
	// when none of its referenced variables is mutated in the
	// body or the for-loop updater.
	for _, group := range splitShortCircuitOperands(cond) {
		if conditionHasSideEffects(group) {
			continue
		}
		refs := collectConditionRefs(group)
		if len(refs) == 0 {
			continue
		}
		type symRef struct {
			id  uintptr
			ref *wrapperchecker.Node
		}
		var refList []symRef
		seen := map[uintptr]bool{}
		for _, ref := range refs {
			sym := checker.SymbolOf(ref)
			if sym == nil {
				continue
			}
			id := sym.ID()
			if seen[id] {
				continue
			}
			seen[id] = true
			refList = append(refList, symRef{id: id, ref: ref})
		}
		if len(refList) == 0 {
			continue
		}
		anyMutated := false
		for _, sr := range refList {
			if bodyMutatesSymbol(body, sr.id, checker) ||
				(updater != nil && bodyMutatesSymbol(updater, sr.id, checker)) {
				anyMutated = true
				break
			}
		}
		if anyMutated {
			continue
		}
		for _, sr := range refList {
			ctx.Report(sr.ref, "'"+sr.ref.SourceText()+"' is not modified in this loop.")
		}
	}
}

// splitShortCircuitOperands flattens a condition into its top-level
// `&&` / `||` / `??` operands. Non-short-circuit operators (`<`,
// `===`, etc.) keep their operands grouped together.
func splitShortCircuitOperands(cond *wrapperchecker.Node) []*wrapperchecker.Node {
	if cond.Kind() == wrapperchecker.KindBinaryExpression {
		op := cond.BinaryOperatorKind()
		if op == wrapperchecker.KindAmpersandAmpersandToken ||
			op == wrapperchecker.KindBarBarToken ||
			op == wrapperchecker.KindQuestionQuestionToken {
			return append(
				splitShortCircuitOperands(cond.BinaryLeft()),
				splitShortCircuitOperands(cond.BinaryRight())...,
			)
		}
	}
	if cond.Kind() == wrapperchecker.KindParenthesizedExpression {
		var inner *wrapperchecker.Node
		cond.ForEachChild(func(c *wrapperchecker.Node) bool {
			inner = c
			return true
		})
		if inner != nil {
			return splitShortCircuitOperands(inner)
		}
	}
	return []*wrapperchecker.Node{cond}
}

// conditionHasPropertyAccess reports whether the condition contains
// any property or element access — those potentially hide getter
// side effects and ESLint excludes them from the analysis.
func conditionHasPropertyAccess(n *wrapperchecker.Node) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindPropertyAccessExpression,
			wrapperchecker.KindElementAccessExpression:
			found = true
			return
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(n)
	return found
}

// conditionHasSideEffects reports whether the condition itself
// updates state — assignment, increment/decrement, function call,
// yield, await, etc. — which by itself can change the loop's
// behavior across iterations.
func conditionHasSideEffects(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression,
			wrapperchecker.KindTaggedTemplateExpression,
			wrapperchecker.KindYieldExpression,
			wrapperchecker.KindAwaitExpression,
			wrapperchecker.KindPostfixUnaryExpression,
			wrapperchecker.KindPrefixUnaryExpression:
			// Prefix +/- still allowed; only ++/-- are side
			// effects. PostfixUnaryExpression is always ++ or --.
			if c.Kind() == wrapperchecker.KindPrefixUnaryExpression {
				op := c.PrefixUnaryOperator()
				if op != "++" && op != "--" {
					break
				}
			}
			found = true
			return
		case wrapperchecker.KindBinaryExpression:
			if isAssignmentOperator(c.BinaryOperatorKind()) {
				found = true
				return
			}
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(n)
	return found
}

func isAssignmentOperator(op wrapperchecker.Kind) bool {
	switch op {
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
		wrapperchecker.KindBarBarEqualsToken,
		wrapperchecker.KindAmpersandAmpersandEqualsToken,
		wrapperchecker.KindQuestionQuestionEqualsToken:
		return true
	}
	return false
}

// collectConditionRefs returns the identifier nodes in `cond` that
// are read as values (not property names, not in a typeof operand,
// not on the LHS of a property access for the property name).
func collectConditionRefs(cond *wrapperchecker.Node) []*wrapperchecker.Node {
	var out []*wrapperchecker.Node
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if c == nil {
			return
		}
		if c.Kind() == wrapperchecker.KindPropertyAccessExpression {
			// Only the receiver is a value reference; recurse into
			// it and skip the property name.
			walk(c.PropertyAccessReceiver())
			return
		}
		if c.Kind() == wrapperchecker.KindIdentifier {
			out = append(out, c)
			return
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(cond)
	return out
}

// bodyMutatesSymbol reports whether `body` contains an assignment
// or ++/-- targeting an identifier whose resolved symbol matches
// `symID`.
func bodyMutatesSymbol(body *wrapperchecker.Node, symID uintptr, checker *wrapperchecker.Checker) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindBinaryExpression:
			if isAssignmentOperator(c.BinaryOperatorKind()) {
				if id := lhsTargetIdentifier(c.BinaryLeft()); id != nil {
					if sym := checker.SymbolOf(id); sym != nil && sym.ID() == symID {
						found = true
						return
					}
				}
			}
		case wrapperchecker.KindPostfixUnaryExpression:
			if id := lhsTargetIdentifier(c.PostfixUnaryOperand()); id != nil {
				if sym := checker.SymbolOf(id); sym != nil && sym.ID() == symID {
					found = true
					return
				}
			}
		case wrapperchecker.KindPrefixUnaryExpression:
			op := c.PrefixUnaryOperator()
			if op == "++" || op == "--" {
				if id := lhsTargetIdentifier(c.PrefixUnaryOperand()); id != nil {
					if sym := checker.SymbolOf(id); sym != nil && sym.ID() == symID {
						found = true
						return
					}
				}
			}
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(body)
	return found
}

// lhsTargetIdentifier extracts the identifier being assigned to or
// updated. For a property access (`obj.foo = ...`), there is no
// single identifier target; this returns nil so the caller skips
// it.
func lhsTargetIdentifier(n *wrapperchecker.Node) *wrapperchecker.Node {
	if n == nil {
		return nil
	}
	if n.Kind() == wrapperchecker.KindIdentifier {
		return n
	}
	return nil
}

// bodyHasFunctionLikeCall returns true if `body` contains any call
// or `new` expression, which we conservatively treat as potential
// mutation of any free variable.
func bodyHasFunctionLikeCall(body *wrapperchecker.Node) bool {
	found := false
	var walk func(c *wrapperchecker.Node)
	walk = func(c *wrapperchecker.Node) {
		if found || c == nil {
			return
		}
		switch c.Kind() {
		case wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression,
			wrapperchecker.KindTaggedTemplateExpression:
			found = true
			return
		}
		c.ForEachChild(func(child *wrapperchecker.Node) bool {
			walk(child)
			return false
		})
	}
	walk(body)
	return found
}
