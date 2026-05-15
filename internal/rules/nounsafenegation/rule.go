// Package nounsafenegation implements the no-unsafe-negation rule:
// `!a in b` and `!a instanceof b` are parsed as `(!a) in b` and
// `(!a) instanceof b` respectively, which almost always reflects an
// operator-precedence mistake; the author meant `!(a in b)`. With the
// `enforceForOrderingRelations` option enabled, the same check
// extends to `<`, `>`, `<=`, and `>=`.
package nounsafenegation

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unsafe-negation"

// Options configures the rule.
type Options struct {
	// EnforceForOrderingRelations also flags `<`, `>`, `<=`, `>=`.
	EnforceForOrderingRelations bool
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
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	left := n.BinaryLeft()
	if left == nil || left.Kind() != wrapperchecker.KindPrefixUnaryExpression {
		return
	}
	if left.PrefixUnaryOperator() != "!" {
		return
	}
	op := operatorText(n)
	if !r.opShouldFlag(op) {
		return
	}
	ctx.Report(n, "Unexpected negation of the left operand of '"+op+"' operator.")
}

func (r *rule) opShouldFlag(op string) bool {
	switch op {
	case "in", "instanceof":
		return true
	case "<", ">", "<=", ">=":
		return r.opts.EnforceForOrderingRelations
	}
	return false
}

// operatorText extracts the operator text of a BinaryExpression by
// looking at the source between the two operands. The TS-go wrapper
// doesn't expose KindInKeyword / KindInstanceOfKeyword, so we inspect
// the source span instead.
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
