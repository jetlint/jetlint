// Package nodupeelseif implements the no-dupe-else-if rule: flag an
// if/else-if chain whose later branch's condition is already covered
// by an earlier branch. Such a branch can never execute.
//
// Equality between expressions is measured structurally — parentheses
// at the OR/AND split boundaries are ignored, and `&&`/`||` are
// treated as commutative. A later condition C is reported when, after
// splitting C into its OR-operands (each further split into
// AND-operands), every OR-operand is subsumed by some earlier branch
// in the chain. Matches oxlint and ESLint's behavior across the
// no-dupe-else-if fixture set.
package nodupeelseif

import (
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-dupe-else-if"

// New constructs a nodupeelseif rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindIfStatement: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil || parent.Kind() != wrapperchecker.KindIfStatement {
		return
	}
	if parent.IfElse() == nil || !sameNode(parent.IfElse(), n) {
		return
	}

	test := n.IfCondition()
	if test == nil {
		return
	}

	// When the top-level operator is `&&`, every AND-operand is itself
	// an independent condition: all must hold for the branch to fire,
	// so subsuming any one is enough to dead-code the branch.
	var conditionsToCheck []*wrapperchecker.Node
	conditionsToCheck = append(conditionsToCheck, test)
	if isLogical(test, wrapperchecker.KindAmpersandAmpersandToken) {
		conditionsToCheck = append(conditionsToCheck, splitByAnd(test)...)
	}

	listToCheck := make([][][]string, len(conditionsToCheck))
	for i, c := range conditionsToCheck {
		ors := splitByOr(c)
		row := make([][]string, len(ors))
		for j, or := range ors {
			ands := splitByAnd(or)
			canon := make([]string, len(ands))
			for k, a := range ands {
				canon[k] = canonicalize(a)
			}
			row[j] = canon
		}
		listToCheck[i] = row
	}

	current := n
	for {
		p := current.Parent()
		if p == nil || p.Kind() != wrapperchecker.KindIfStatement {
			break
		}
		if p.IfElse() == nil || !sameNode(p.IfElse(), current) {
			break
		}
		current = p

		parentOrOperands := splitByOr(p.IfCondition())
		parentRow := make([][]string, len(parentOrOperands))
		for j, or := range parentOrOperands {
			ands := splitByAnd(or)
			canon := make([]string, len(ands))
			for k, a := range ands {
				canon[k] = canonicalize(a)
			}
			parentRow[j] = canon
		}

		for i, row := range listToCheck {
			filtered := row[:0]
			for _, orOperand := range row {
				covered := false
				for _, parentOr := range parentRow {
					if isSubset(parentOr, orOperand) {
						covered = true
						break
					}
				}
				if !covered {
					filtered = append(filtered, orOperand)
				}
			}
			listToCheck[i] = filtered
		}

		for _, row := range listToCheck {
			if len(row) == 0 {
				ctx.Report(test, "duplicate condition in if-else-if chain")
				return
			}
		}
	}
}

// sameNode compares AST nodes by source position. The wrapper API
// hands back fresh Node wrappers, so pointer identity is not reliable.
func sameNode(a, b *wrapperchecker.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Pos() == b.Pos() && a.End() == b.End() && a.Kind() == b.Kind()
}

func isLogical(n *wrapperchecker.Node, op wrapperchecker.Kind) bool {
	n = unparen(n)
	if n == nil || n.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	return n.BinaryOperatorKind() == op
}

func unparen(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

func splitByOr(n *wrapperchecker.Node) []*wrapperchecker.Node {
	return splitByLogical(n, wrapperchecker.KindBarBarToken)
}

func splitByAnd(n *wrapperchecker.Node) []*wrapperchecker.Node {
	return splitByLogical(n, wrapperchecker.KindAmpersandAmpersandToken)
}

func splitByLogical(n *wrapperchecker.Node, op wrapperchecker.Kind) []*wrapperchecker.Node {
	n = unparen(n)
	if n == nil {
		return nil
	}
	if n.Kind() == wrapperchecker.KindBinaryExpression && n.BinaryOperatorKind() == op {
		left := splitByLogical(n.BinaryLeft(), op)
		right := splitByLogical(n.BinaryRight(), op)
		return append(left, right...)
	}
	return []*wrapperchecker.Node{n}
}

func isSubset(a, b []string) bool {
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// canonicalize returns a string identical for two expressions iff they
// have the same AST shape and leaf text. Inner ParenthesizedExpression
// nodes are NOT stripped — oxc's content_eq distinguishes `a === 1`
// from `a === (1)`. Top-level paren stripping is the splitter's job.
// `&&` and `||` are normalized commutatively so `a && b` matches
// `b && a` and `a || b` matches `b || a`.
func canonicalize(n *wrapperchecker.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind() == wrapperchecker.KindBinaryExpression {
		op := n.BinaryOperatorKind()
		if op == wrapperchecker.KindAmpersandAmpersandToken || op == wrapperchecker.KindBarBarToken {
			left := canonicalize(n.BinaryLeft())
			right := canonicalize(n.BinaryRight())
			if right < left {
				left, right = right, left
			}
			var sb strings.Builder
			sb.WriteString(strconv.Itoa(int(n.Kind())))
			sb.WriteByte(':')
			sb.WriteString(strconv.Itoa(int(op)))
			sb.WriteByte('(')
			sb.WriteString(left)
			sb.WriteByte(',')
			sb.WriteString(right)
			sb.WriteByte(')')
			return sb.String()
		}
	}
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(int(n.Kind())))
	if isTextLeaf(n.Kind()) {
		sb.WriteByte(':')
		sb.WriteString(strconv.Quote(n.LiteralText()))
	}
	sb.WriteByte('(')
	first := true
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if !first {
			sb.WriteByte(',')
		}
		first = false
		sb.WriteString(canonicalize(c))
		return false
	})
	sb.WriteByte(')')
	return sb.String()
}

func isTextLeaf(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindPrivateIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNumericLiteral,
		wrapperchecker.KindBigIntLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral,
		wrapperchecker.KindTemplateHead,
		wrapperchecker.KindTemplateMiddle,
		wrapperchecker.KindTemplateTail,
		wrapperchecker.KindRegularExpressionLiteral:
		return true
	}
	return false
}
