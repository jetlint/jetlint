// Package useisnan implements the use-isnan rule: flag comparisons
// and lookups against NaN, because such patterns never produce the
// result the author expected. `NaN` is not equal to itself, and
// `<`/`>` against NaN is always false; `indexOf`/`lastIndexOf` use
// strict equality and so always miss NaN. The correct check is
// `Number.isNaN(x)`.
//
// Recognized NaN forms: bare `NaN`, `Number.NaN`, and `Number["NaN"]`.
// Parenthesized operands are unwrapped.
//
// Options:
//   - EnforceForSwitchCase (default true): flag `switch (NaN)` and
//     `case NaN:` labels.
//   - EnforceForIndexOf (default false): flag `arr.indexOf(NaN)` and
//     `arr.lastIndexOf(NaN)`.
//
// Out of scope: scope-aware NaN binding shadows are not detected;
// the rule treats `NaN` as the global value regardless of binding.
package useisnan

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-isnan"

// Options is the configurable surface of use-isnan.
type Options struct {
	EnforceForSwitchCase bool
	EnforceForIndexOf    bool
}

// DefaultOptions matches ESLint's defaults: switch-case enforcement
// on, indexOf enforcement off.
func DefaultOptions() Options {
	return Options{EnforceForSwitchCase: true}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("use-isnan options must be a JSON object: %w", err)
	}
	if v, ok := fields["enforceForSwitchCase"]; ok {
		if err := json.Unmarshal(v, &out.EnforceForSwitchCase); err != nil {
			return Options{}, err
		}
	}
	if v, ok := fields["enforceForIndexOf"]; ok {
		if err := json.Unmarshal(v, &out.EnforceForIndexOf); err != nil {
			return Options{}, err
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	h := map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindBinaryExpression: r.visitBinary,
		wrapperchecker.KindCallExpression:   r.visitCall,
	}
	if r.opts.EnforceForSwitchCase {
		h[wrapperchecker.KindSwitchStatement] = r.visitSwitch
	}
	return h
}

func (r *rule) visitBinary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isComparisonOperator(n.BinaryOperatorKind()) {
		return
	}
	if !isNaN(n.BinaryLeft()) && !isNaN(n.BinaryRight()) {
		return
	}
	ctx.Report(n, "use Number.isNaN() instead of comparing with NaN")
}

func (r *rule) visitSwitch(ctx *engine.Context, n *wrapperchecker.Node) {
	// `switch (NaN) { ... }` — discriminant is NaN.
	if disc := n.FirstChild(); disc != nil && isNaN(disc) {
		ctx.Report(disc, "switching on NaN never matches any case")
	}
	// `case NaN:` labels.
	n.ForEachChild(func(block *wrapperchecker.Node) bool {
		block.ForEachChild(func(clause *wrapperchecker.Node) bool {
			if clause.Kind() != wrapperchecker.KindCaseClause {
				return false
			}
			if expr := clause.CaseExpression(); expr != nil && isNaN(expr) {
				ctx.Report(expr, "case NaN never matches; use Number.isNaN()")
			}
			return false
		})
		return false
	})
}

func (r *rule) visitCall(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.EnforceForIndexOf {
		return
	}
	name, ok := methodName(stripParens(n.CalleeExpression()))
	if !ok {
		return
	}
	if name != "indexOf" && name != "lastIndexOf" {
		return
	}
	args := n.CallArguments()
	// Array.prototype.indexOf/lastIndexOf accept (searchValue) or
	// (searchValue, fromIndex). Calls with more arguments don't
	// match the spec; the rule stays silent on those to mirror
	// ESLint/oxlint.
	if len(args) == 0 || len(args) > 2 {
		return
	}
	if !isNaN(args[0]) {
		return
	}
	ctx.Report(n, "use Number.isNaN() — "+name+"(NaN) always returns -1")
}

// methodName returns the static property name being invoked on a
// receiver, for both `foo.bar()` and `foo['bar']()` shapes.
func methodName(callee *wrapperchecker.Node) (string, bool) {
	if callee == nil {
		return "", false
	}
	switch callee.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return callee.PropertyAccessName(), true
	case wrapperchecker.KindElementAccessExpression:
		idx := stripParens(elementAccessIndex(callee))
		if idx == nil {
			return "", false
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText(), true
		}
	}
	return "", false
}

func isComparisonOperator(k wrapperchecker.Kind) bool {
	switch k {
	case wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindGreaterThanToken,
		wrapperchecker.KindGreaterThanEqualsToken,
		wrapperchecker.KindLessThanToken,
		wrapperchecker.KindLessThanEqualsToken:
		return true
	}
	return false
}

// isNaN reports whether n syntactically refers to the global NaN
// value: bare `NaN`, `Number.NaN`, or `Number["NaN"]`. Parentheses
// are unwrapped; comma expressions (a, b) descend to their final
// operand, the value the expression actually produces.
func isNaN(n *wrapperchecker.Node) bool {
	n = unwrap(n)
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindIdentifier:
		return n.LiteralText() == "NaN"
	case wrapperchecker.KindPropertyAccessExpression:
		recv := stripParens(n.PropertyAccessReceiver())
		if recv == nil || recv.Kind() != wrapperchecker.KindIdentifier {
			return false
		}
		return recv.LiteralText() == "Number" && n.PropertyAccessName() == "NaN"
	case wrapperchecker.KindElementAccessExpression:
		recv := stripParens(elementAccessReceiver(n))
		idx := stripParens(elementAccessIndex(n))
		if recv == nil || idx == nil {
			return false
		}
		if recv.Kind() != wrapperchecker.KindIdentifier || recv.LiteralText() != "Number" {
			return false
		}
		return idx.Kind() == wrapperchecker.KindStringLiteral && idx.LiteralText() == "NaN"
	}
	return false
}

// elementAccessReceiver/elementAccessIndex pull the two operands of
// an ElementAccessExpression. The wrapper API doesn't expose them
// directly, so we read the AST children: receiver first, index
// second.
func elementAccessReceiver(n *wrapperchecker.Node) *wrapperchecker.Node {
	var got *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		got = c
		return true
	})
	return got
}

func elementAccessIndex(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first, second *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
			return false
		}
		second = c
		return true
	})
	return second
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}

// unwrap is stripParens plus comma-expression descent: for `(a, b)`
// the comma expression evaluates to its right operand, so isNaN
// follows the chain to its final value.
func unwrap(n *wrapperchecker.Node) *wrapperchecker.Node {
	for {
		n = stripParens(n)
		if n == nil {
			return nil
		}
		if n.Kind() == wrapperchecker.KindBinaryExpression &&
			n.BinaryOperatorKind() == wrapperchecker.KindCommaToken {
			n = n.BinaryRight()
			continue
		}
		return n
	}
}
