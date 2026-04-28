// Package noconfusingvoidexpression implements the
// no-confusing-void-expression rule: flag void-returning calls used
// where a value is expected (assignments, array elements, object
// property values, ternary branches, etc.).
package noconfusingvoidexpression

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-confusing-void-expression"

// Options is the configurable surface of the rule.
type Options struct {
	IgnoreArrowShorthand        bool
	IgnoreVoidOperator          bool
	IgnoreVoidReturningFunctions bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-confusing-void-expression options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "ignoreArrowShorthand":
			if err := json.Unmarshal(val, &out.IgnoreArrowShorthand); err != nil {
				return Options{}, err
			}
		case "ignoreVoidOperator":
			if err := json.Unmarshal(val, &out.IgnoreVoidOperator); err != nil {
				return Options{}, err
			}
		case "ignoreVoidReturningFunctions":
			if err := json.Unmarshal(val, &out.IgnoreVoidReturningFunctions); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindCallExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	t := ctx.TypeOf(n)
	if t == nil || !t.IsVoid() {
		return
	}
	pos, kind := isInValuePosition(n)
	if !pos {
		return
	}
	if r.opts.IgnoreArrowShorthand && kind == "arrow" {
		return
	}
	if r.opts.IgnoreVoidOperator && kind == "void-operator" {
		return
	}
	if r.opts.IgnoreVoidReturningFunctions && enclosingFunctionReturnsVoid(ctx, n) {
		return
	}
	ctx.Report(n, "void-returning call placed where a value is expected")
}

// containsVoid reports whether t is `void` or a union with a void
// member. Functions that return void-or-something accept void
// callees in any return position.
func containsVoid(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsVoid() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if m.IsVoid() {
				return true
			}
		}
	}
	return false
}

func anyCallSignatureReturnsVoid(t *wrapperchecker.Type) bool {
	for _, sig := range t.CallSignatures() {
		if rt := sig.ReturnType(); rt != nil && containsVoid(rt) {
			return true
		}
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if anyCallSignatureReturnsVoid(m) {
				return true
			}
		}
	}
	return false
}

// enclosingFunctionReturnsVoid reports whether the closest function-
// like ancestor's declared (or contextually-inferred) return type is
// void. The rule's ignoreVoidReturningFunctions option suppresses
// flags inside such functions.
func enclosingFunctionReturnsVoid(ctx *engine.Context, n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindFunctionDeclaration,
			wrapperchecker.KindFunctionExpression,
			wrapperchecker.KindArrowFunction,
			wrapperchecker.KindMethodDeclaration:
			if ann := cur.FunctionReturnTypeAnnotation(); ann != nil {
				rt := ctx.Checker().TypeFromTypeNode(ann)
				if rt != nil && containsVoid(rt) {
					return true
				}
				return false
			}
			// No annotation — check contextual return type from
			// surrounding signature (e.g. the function is passed as a
			// callback expecting () => void or () => void | string).
			if t := ctx.Checker().ContextualTypeOf(cur); t != nil {
				if anyCallSignatureReturnsVoid(t) {
					return true
				}
			}
			return false
		}
	}
	return false
}

// isInValuePosition walks up parents through transparent wrappers and
// short-circuit operators, returning whether the expression's value
// is actually consumed and a tag identifying the consuming context
// when one of the option-controlled forms applies.
func isInValuePosition(n *wrapperchecker.Node) (bool, string) {
	cur := n
	for {
		parent := cur.Parent()
		if parent == nil {
			return false, ""
		}
		switch parent.Kind() {
		case wrapperchecker.KindParenthesizedExpression,
			wrapperchecker.KindNonNullExpression,
			wrapperchecker.KindAsExpression,
			wrapperchecker.KindSatisfiesExpression:
			cur = parent
			continue
		case wrapperchecker.KindConditionalExpression:
			cur = parent
			continue
		case wrapperchecker.KindBinaryExpression:
			op := parent.BinaryOperatorKind()
			switch op {
			case wrapperchecker.KindAmpersandAmpersandToken,
				wrapperchecker.KindBarBarToken,
				wrapperchecker.KindQuestionQuestionToken:
				cur = parent
				continue
			case wrapperchecker.KindCommaToken:
				right := parent.BinaryRight()
				if right != nil && right.Pos() == cur.Pos() {
					cur = parent
					continue
				}
				return false, ""
			}
			return true, ""
		case wrapperchecker.KindExpressionStatement,
			wrapperchecker.KindForStatement:
			return false, ""
		case wrapperchecker.KindVariableDeclaration,
			wrapperchecker.KindArrayLiteralExpression,
			wrapperchecker.KindPropertyAssignment,
			wrapperchecker.KindShorthandPropertyAssignment,
			wrapperchecker.KindReturnStatement,
			wrapperchecker.KindTemplateSpan,
			wrapperchecker.KindSpreadElement:
			return true, ""
		case wrapperchecker.KindArrowFunction:
			return true, "arrow"
		case wrapperchecker.KindCallExpression,
			wrapperchecker.KindNewExpression:
			callee := parent.CalleeExpression()
			if callee != nil && callee.Pos() == cur.Pos() {
				return false, ""
			}
			return true, ""
		case wrapperchecker.KindVoidExpression:
			return true, "void-operator"
		}
		return false, ""
	}
}
