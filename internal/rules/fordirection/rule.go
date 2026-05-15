// Package fordirection implements the for-direction rule: a C-style
// `for` loop whose update clause moves the counter in the wrong
// direction relative to its test (e.g. `for (i = 0; i < 10; i--)`)
// is almost always a bug — the loop runs forever.
package fordirection

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "for-direction"

func New() engine.Rule { return &rule{} }

type rule struct{}

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindForStatement: r.visit,
	}
}

type direction int

const (
	dirNone direction = iota
	dirForward
	dirBackward
)

func (r *rule) visit(ctx *engine.Context, loop *wrapperchecker.Node) {
	cond, update := forParts(loop)
	if cond == nil || update == nil {
		return
	}
	cond = stripParens(cond)
	if cond.Kind() != wrapperchecker.KindBinaryExpression {
		return
	}
	op := cond.BinaryOperatorKind()
	if !isOrderComparison(op) {
		return
	}
	left := stripParens(cond.BinaryLeft())
	right := stripParens(cond.BinaryRight())
	counter, counterLeft := extractCounter(left, right)
	if counter == nil {
		return
	}
	expected := expectedDirection(op, counterLeft)
	if expected == dirNone {
		return
	}
	actual := updateDirection(update, counter)
	if actual == dirNone {
		return
	}
	if actual != expected {
		ctx.Report(loop, "the update clause in this loop moves the counter in the wrong direction")
	}
}

// forParts pulls (condition, incrementor) from a ForStatement's
// children. Children order: initializer, condition, incrementor, body.
// Any of the first three may be absent; we identify by looking at
// what's neither a Block (body) nor a VariableDeclarationList
// (init).
func forParts(loop *wrapperchecker.Node) (cond, update *wrapperchecker.Node) {
	cond = loop.ForStatementCondition()
	// The incrementor is the third "header" expression. Walk
	// children and pick the third statement-position child (skipping
	// init / condition).
	idx := 0
	loop.ForEachChild(func(c *wrapperchecker.Node) bool {
		// child 0: init, 1: condition, 2: incrementor, 3: body
		if idx == 2 {
			update = c
		}
		idx++
		return false
	})
	if update != nil && update.Kind() == wrapperchecker.KindBlock {
		// Loop with no incrementor — child[2] is the body block.
		update = nil
	}
	return cond, update
}

func isOrderComparison(op wrapperchecker.Kind) bool {
	switch op {
	case wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken,
		wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken:
		return true
	}
	return false
}

// extractCounter returns the identifier side of a binary comparison
// and whether it was on the left. Returns nil if neither side is an
// identifier.
func extractCounter(left, right *wrapperchecker.Node) (counter *wrapperchecker.Node, leftSide bool) {
	if left != nil && left.Kind() == wrapperchecker.KindIdentifier {
		return left, true
	}
	if right != nil && right.Kind() == wrapperchecker.KindIdentifier {
		return right, false
	}
	return nil, false
}

// expectedDirection: given the test operator and which side holds
// the counter, returns the direction the counter must move for the
// loop to terminate.
//
//	i < n  →ascending  i must increase (forward)
//	i > n  → counter must decrease (backward)
//	n < i  → counter must decrease (backward)
//	n > i  → counter must increase (forward)
func expectedDirection(op wrapperchecker.Kind, counterLeft bool) direction {
	switch op {
	case wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken:
		if counterLeft {
			return dirForward
		}
		return dirBackward
	case wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken:
		if counterLeft {
			return dirBackward
		}
		return dirForward
	}
	return dirNone
}

// updateDirection inspects the for-loop's incrementor expression and
// returns the direction it moves the counter. Returns dirNone for
// expressions that don't modify the counter (a different variable,
// a function call, etc.) or use an unknown step value.
func updateDirection(update *wrapperchecker.Node, counter *wrapperchecker.Node) direction {
	update = stripParens(update)
	if update == nil {
		return dirNone
	}
	counterName := counter.LiteralText()
	switch update.Kind() {
	case wrapperchecker.KindPostfixUnaryExpression:
		if !identifierMatches(stripParens(update.PostfixUnaryOperand()), counterName) {
			return dirNone
		}
		switch update.PostfixUnaryOperator() {
		case "++":
			return dirForward
		case "--":
			return dirBackward
		}
	case wrapperchecker.KindPrefixUnaryExpression:
		if !identifierMatches(stripParens(update.PrefixUnaryOperand()), counterName) {
			return dirNone
		}
		switch update.PrefixUnaryOperator() {
		case "++":
			return dirForward
		case "--":
			return dirBackward
		}
	case wrapperchecker.KindBinaryExpression:
		if !identifierMatches(stripParens(update.BinaryLeft()), counterName) {
			return dirNone
		}
		op := update.BinaryOperatorKind()
		rhsSign := numericSign(update.BinaryRight())
		if rhsSign == signUnknown {
			return dirNone
		}
		switch op {
		case wrapperchecker.KindPlusEqualsToken:
			if rhsSign == signPositive {
				return dirForward
			}
			return dirBackward
		case wrapperchecker.KindMinusEqualsToken:
			if rhsSign == signPositive {
				return dirBackward
			}
			return dirForward
		}
	}
	return dirNone
}

type sign int

const (
	signUnknown sign = iota
	signPositive
	signNegative
)

// numericSign reports whether the expression evaluates to a known
// non-zero positive or negative numeric literal. Unknown for any
// other shape.
func numericSign(n *wrapperchecker.Node) sign {
	n = stripParens(n)
	if n == nil {
		return signUnknown
	}
	switch n.Kind() {
	case wrapperchecker.KindNumericLiteral:
		txt := n.LiteralText()
		if txt == "0" || txt == "" {
			return signUnknown
		}
		// All numeric literals are non-negative syntactically; the
		// `-` is a separate prefix operator.
		return signPositive
	case wrapperchecker.KindBigIntLiteral:
		txt := n.LiteralText()
		// BigInt literals like "0n", "1n", etc. We can't be sure of
		// zero without parsing — accept anything not starting with "0n".
		if txt == "0n" {
			return signUnknown
		}
		return signPositive
	case wrapperchecker.KindPrefixUnaryExpression:
		switch n.PrefixUnaryOperator() {
		case "-":
			inner := numericSign(n.PrefixUnaryOperand())
			if inner == signPositive {
				return signNegative
			}
			if inner == signNegative {
				return signPositive
			}
			return signUnknown
		case "+":
			return numericSign(n.PrefixUnaryOperand())
		}
	}
	return signUnknown
}

// identifierMatches reports whether n is an Identifier with the given
// name.
func identifierMatches(n *wrapperchecker.Node, name string) bool {
	if n == nil {
		return false
	}
	if n.Kind() != wrapperchecker.KindIdentifier {
		return false
	}
	return n.LiteralText() == name
}

// stripParens unwraps ParenthesizedExpression layers.
func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
