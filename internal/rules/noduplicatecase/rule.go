// Package noduplicatecase implements the no-duplicate-case rule: flag
// switch statements that contain two case clauses whose expressions
// are structurally identical. JavaScript executes only the first
// matching branch, so the duplicate is unreachable — almost always a
// copy-paste error.
//
// Each switch is analyzed in isolation: identical case values across
// separate switches (or in nested switches) are not flagged.
// Equality is measured by structural AST comparison (ignoring
// whitespace, parens, and comments), matching oxlint and ESLint.
// Cases that evaluate to the same runtime value via syntactically
// different expressions (e.g. `1` vs `0x1`) are not deduplicated.
package noduplicatecase

import (
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-duplicate-case"

// New constructs a noduplicatecase rule instance ready for registration
// with the engine.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSwitchStatement: visit,
	}
}

// canonicalize returns a string that's identical for two expressions
// iff they have the same AST shape and leaf text, ignoring parens
// and whitespace. Used to detect duplicate case labels.
func canonicalize(n *wrapperchecker.Node) string {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	if n == nil {
		return ""
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

// isTextLeaf reports whether LiteralText() is safe to call on the
// given Kind — TypeScript-Go's Node.Text panics on non-leaf nodes.
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

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	seen := map[string]bool{}
	// SwitchStatement's child is a CaseBlock; the CaseBlock's children
	// are CaseClause and DefaultClause nodes.
	n.ForEachChild(func(block *wrapperchecker.Node) bool {
		block.ForEachChild(func(clause *wrapperchecker.Node) bool {
			if clause.Kind() != wrapperchecker.KindCaseClause {
				return false
			}
			expr := clause.CaseExpression()
			if expr == nil {
				return false
			}
			key := canonicalize(expr)
			if key == "" {
				return false
			}
			if seen[key] {
				ctx.Report(clause, "duplicate case label")
				return false
			}
			seen[key] = true
			return false
		})
		return false
	})
}
