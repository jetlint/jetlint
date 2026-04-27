// Package noconfusingvoidexpression implements the
// no-confusing-void-expression rule: flag void-returning calls used
// where a value is expected (assignments, array elements, object
// property values, ternary branches, etc.).
package noconfusingvoidexpression

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-confusing-void-expression"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	t := ctx.TypeOf(n)
	if t == nil || !t.IsVoid() {
		return
	}
	if isInValuePosition(n) {
		ctx.Report(n, "void-returning call placed where a value is expected")
	}
}

// isInValuePosition walks up parents through transparent wrappers
// (parens, non-null assertions, as-expressions) and returns true when
// the surrounding context evaluates the expression for its value.
// Logical operators (&&, ||, ??) and conditional expressions pass
// through to whatever consumes them — if their parent ultimately
// discards the value (an expression statement), so does this call.
func isInValuePosition(n *wrapperchecker.Node) bool {
	cur := n
	for {
		parent := cur.Parent()
		if parent == nil {
			return false
		}
		switch parent.Kind() {
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindNonNullExpression,
			wrapperchecker.KindAsExpression,
			wrapperchecker.KindSatisfiesExpression:
			cur = parent
			continue
		case wrapperchecker.KindConditionalExpression:
			// Each branch produces the conditional's value — pass
			// through to whatever consumes the conditional.
			cur = parent
			continue
		case wrapperchecker.KindBinaryExpression:
			op := parent.BinaryOperatorKind()
			switch op {
			case wrapperchecker.KindAmpersandAmpersandToken,
				wrapperchecker.KindBarBarToken,
				wrapperchecker.KindQuestionQuestionToken:
				// Short-circuit operators forward the chosen operand's
				// value to whatever consumes them.
				cur = parent
				continue
			case wrapperchecker.KindCommaToken:
				right := parent.BinaryRight()
				if right != nil && right.Pos() == cur.Pos() {
					cur = parent
					continue
				}
				return false
			}
			// Assignment, comparison, arithmetic, etc. all consume the
			// value as data.
			return true
		case wrapperchecker.KindExpressionStatement,
			wrapperchecker.KindForStatement:
			return false
		case wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindArrayLiteralExpression,
			wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindReturnStatement,
			wrapperchecker.KindTemplateSpan,
			wrapperchecker.KindSpreadElement:
			return true
		case wrapperchecker.KindArrowFunction:
			// Arrow concise body — the value becomes the return value,
			// which the call site may or may not consume. Without
			// option-aware contextual analysis we conservatively flag.
			return true
		case wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression:
			callee := parent.CalleeExpression()
			if callee != nil && callee.Pos() == cur.Pos() {
				return false
			}
			return true
		case wrapperchecker.KindVoidExpression:
			return true
		}
		return false
	}
}
