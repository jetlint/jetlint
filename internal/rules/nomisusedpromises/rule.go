// Package nomisusedpromises implements the no-misused-promises rule:
// flag any expression where a Promise is used in a position the
// language does not handle (conditionals, void-returning callback
// slots, spreads).
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint. The check shape, option set, and contextual
// type semantics derive from upstream's published test fixtures.
package nomisusedpromises

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-misused-promises"

// Options is the configurable surface of the rule.
type Options struct {
	ChecksConditionals bool
	ChecksVoidReturn   bool
	ChecksSpreads      bool
}

func DefaultOptions() Options {
	return Options{ChecksConditionals: true, ChecksVoidReturn: true, ChecksSpreads: true}
}

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-misused-promises options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "checksConditionals":
			if err := json.Unmarshal(val, &out.ChecksConditionals); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q: %w", key, err)
			}
		case "checksSpreads":
			if err := json.Unmarshal(val, &out.ChecksSpreads); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q: %w", key, err)
			}
		case "checksVoidReturn":
			var b bool
			if err := json.Unmarshal(val, &b); err == nil {
				out.ChecksVoidReturn = b
				continue
			}
			var sub map[string]any
			if err := json.Unmarshal(val, &sub); err != nil {
				return Options{}, fmt.Errorf("no-misused-promises option %q must be boolean or sub-options object: %w", key, err)
			}
			out.ChecksVoidReturn = true
		default:
			return Options{}, fmt.Errorf("no-misused-promises has no option %q (expected checksConditionals, checksVoidReturn, or checksSpreads)", key)
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression:        r.visitCallExpression,
		wrapperchecker.KindNewExpression:         r.visitCallExpression,
		wrapperchecker.KindIfStatement:           r.visitConditional,
		wrapperchecker.KindWhileStatement:        r.visitConditional,
		wrapperchecker.KindDoStatement:           r.visitConditional,
		wrapperchecker.KindForStatement:          r.visitForStatement,
		wrapperchecker.KindConditionalExpression: r.visitConditional,
		wrapperchecker.KindBinaryExpression:      r.visitBinaryExpression,
		wrapperchecker.KindPrefixUnaryExpression: r.visitPrefixUnary,
		wrapperchecker.KindSpreadElement:         r.visitSpreadElement,
		wrapperchecker.KindSpreadAssignment:      r.visitSpreadElement,
	}
}

const (
	msgVoidCallback = "async callback returns a Promise that the parameter's void return type silently drops"
	msgConditional  = "promise used in a conditional position; the language tests truthiness, not promise resolution"
	msgSpread       = "promise spread does not unwrap into iterable elements"
)

func (r *rule) visitCallExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksVoidReturn {
		return
	}
	args := n.CallArguments()
	for i, arg := range args {
		if !returnsPromise(ctx, arg) {
			continue
		}
		expected := ctx.Checker().ContextualTypeForArgument(n, i)
		if expected == nil {
			continue
		}
		if !allCallSignaturesExpectVoid(expected) {
			continue
		}
		ctx.Report(arg, msgVoidCallback)
	}
}

func returnsPromise(ctx *engine.Context, n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindArrowFunction, wrapperchecker.KindFunctionExpression:
		if wrapperchecker.IsAsyncFunction(n) {
			return true
		}
		t := ctx.TypeOf(n)
		if t == nil {
			return false
		}
		for _, sig := range t.CallSignatures() {
			rt := sig.ReturnType()
			if rt == nil { continue }
			if rt.IsPromise() { return true }
			if rt.IsUnion() {
				for _, m := range rt.UnionMembers() {
					if m.IsPromise() { return true }
				}
			}
		}
	}
	return false
}

func allCallSignaturesExpectVoid(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !allCallSignaturesExpectVoid(m) {
				return false
			}
		}
		return true
	}
	sigs := t.CallSignatures()
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		ret := s.ReturnType()
		if ret == nil {
			return false
		}
		if !ret.IsVoid() {
			return false
		}
	}
	return true
}

func (r *rule) visitConditional(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	cond := conditionExpression(n)
	if cond == nil {
		return
	}
	checkPromiseInTest(ctx, cond)
}

func (r *rule) visitForStatement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	cond := n.ForStatementCondition()
	if cond == nil {
		return
	}
	checkPromiseInTest(ctx, cond)
}

func (r *rule) visitBinaryExpression(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindBarBarToken,
		wrapperchecker.KindAmpersandAmpersandToken,
		wrapperchecker.KindQuestionQuestionToken:
		if left := n.BinaryLeft(); left != nil {
			checkPromiseInTest(ctx, left)
		}
	}
}

func (r *rule) visitPrefixUnary(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksConditionals {
		return
	}
	if n.PrefixUnaryOperator() != "!" {
		return
	}
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	checkPromiseInTest(ctx, operand)
}

func (r *rule) visitSpreadElement(ctx *engine.Context, n *wrapperchecker.Node) {
	if !r.opts.ChecksSpreads {
		return
	}
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if t.IsPromise() {
		ctx.Report(n, msgSpread)
		return
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				ctx.Report(n, msgSpread)
				return
			}
		}
	}
}

func checkPromiseInTest(ctx *engine.Context, expr *wrapperchecker.Node) {
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if t.IsPromise() {
		ctx.Report(expr, msgConditional)
		return
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsPromise() {
				ctx.Report(expr, msgConditional)
				return
			}
		}
	}
}

func conditionExpression(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindIfStatement:
		return n.IfCondition()
	case wrapperchecker.KindWhileStatement, wrapperchecker.KindDoStatement:
		return n.WhileCondition()
	case wrapperchecker.KindConditionalExpression:
		return n.ConditionalCondition()
	}
	return nil
}
