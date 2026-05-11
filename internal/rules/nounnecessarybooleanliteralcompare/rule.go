// Package nounnecessarybooleanliteralcompare implements the
// no-unnecessary-boolean-literal-compare rule: flag `x === true`,
// `x !== false`, etc. where x already has type boolean.
package nounnecessarybooleanliteralcompare

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-unnecessary-boolean-literal-compare"

// Options is the configurable surface of the rule.
type Options struct {
	// AllowComparingNullableBooleansToTrue: when true, `nullableBool === true`
	// is allowed (the comparison is meaningful for distinguishing
	// undefined/null from false).
	AllowComparingNullableBooleansToTrue bool
	// AllowComparingNullableBooleansToFalse: same for `=== false`.
	AllowComparingNullableBooleansToFalse bool
	// AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing: when
	// false (the default), emit a per-file advisory diagnostic when
	// strictNullChecks is off — the rule's analysis assumes those
	// checks to distinguish plain from nullable booleans.
	AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing bool
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options {
	return Options{
		AllowComparingNullableBooleansToTrue:  true,
		AllowComparingNullableBooleansToFalse: true,
	}
}

// OptionsFromJSON parses raw JSON options.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-unnecessary-boolean-literal-compare options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowComparingNullableBooleansToTrue":
			if err := json.Unmarshal(val, &out.AllowComparingNullableBooleansToTrue); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowComparingNullableBooleansToFalse":
			if err := json.Unmarshal(val, &out.AllowComparingNullableBooleansToFalse); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		case "allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing":
			if err := json.Unmarshal(val, &out.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing); err != nil {
				return Options{}, fmt.Errorf("option %q: %w", key, err)
			}
		default:
			return Options{}, fmt.Errorf("no-unnecessary-boolean-literal-compare has no option %q", key)
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
		wrapperchecker.KindSourceFile:       r.visitSourceFile,
		wrapperchecker.KindBinaryExpression: r.visit,
	}
}

// visitSourceFile emits the per-file `noStrictNullCheck` advisory when
// the program is compiled without strictNullChecks and the caller has
// not opted in via
// `allowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing`. The
// rule's classification of plain vs nullable booleans relies on
// strictNullChecks; without it, valid programs can be incorrectly
// flagged or skipped.
func (r *rule) visitSourceFile(ctx *engine.Context, n *wrapperchecker.Node) {
	if r.opts.AllowRuleToRunWithoutStrictNullChecksIKnowWhatIAmDoing {
		return
	}
	if ctx.Program().HasStrictNullChecks() {
		return
	}
	ctx.Report(n, "this rule requires `strictNullChecks` to be enabled; otherwise nullable booleans cannot be distinguished")
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	switch n.BinaryOperatorKind() {
	case wrapperchecker.KindEqualsEqualsToken,
		wrapperchecker.KindEqualsEqualsEqualsToken,
		wrapperchecker.KindExclamationEqualsToken,
		wrapperchecker.KindExclamationEqualsEqualsToken:
	default:
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftLit := booleanLiteralKind(left)
	rightLit := booleanLiteralKind(right)
	if leftLit == "" && rightLit == "" {
		return
	}
	// Both sides are boolean literals (`true === true`): always
	// pointless, flag without a type check.
	if leftLit != "" && rightLit != "" {
		ctx.Report(n, "comparing two boolean literals is always constant")
		return
	}
	other := left
	literal := rightLit
	if leftLit != "" {
		other = right
		literal = leftLit
	}
	t := ctx.TypeOf(other)
	if t == nil {
		return
	}
	classification := classifyBooleanType(t)
	if classification == notBoolean {
		return
	}
	// Without strictNullChecks, every value can be null/undefined at
	// runtime — distinguishing plain from nullable boolean is no longer
	// meaningful. Treat a plain boolean as nullable so the
	// allow-comparing-nullable-* options apply.
	if classification == plainBoolean && !ctx.Program().HasStrictNullChecks() {
		classification = nullableBoolean
	}
	if classification == nullableBoolean {
		if literal == "true" && r.opts.AllowComparingNullableBooleansToTrue {
			return
		}
		if literal == "false" && r.opts.AllowComparingNullableBooleansToFalse {
			return
		}
	}
	ctx.Report(n, "comparing a boolean to a boolean literal is redundant; use the value directly")
}

type booleanClass int

const (
	notBoolean booleanClass = iota
	plainBoolean
	nullableBoolean
)

// classifyBooleanType reports whether the type is strictly boolean,
// nullable boolean (boolean union'd with null/undefined), or
// neither. Walks generic type-parameter constraints.
func classifyBooleanType(t *wrapperchecker.Type) booleanClass {
	return classify(t, 0)
}

const recursionLimit = 16

func classify(t *wrapperchecker.Type, depth int) booleanClass {
	if t == nil || depth > recursionLimit {
		return notBoolean
	}
	if t.IsBooleanLike() {
		return plainBoolean
	}
	if t.IsUnion() {
		hasBool := false
		hasNullish := false
		for _, m := range t.UnionMembers() {
			if m.IsBooleanLike() {
				hasBool = true
			} else if m.IsNullOrUndefined() {
				hasNullish = true
			} else {
				return notBoolean
			}
		}
		if !hasBool {
			return notBoolean
		}
		if hasNullish {
			return nullableBoolean
		}
		return plainBoolean
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return classify(c, depth+1)
	}
	return notBoolean
}

// booleanLiteralKind returns "true", "false", or empty string.
func booleanLiteralKind(n *wrapperchecker.Node) string {
	switch n.Kind() {
	case wrapperchecker.KindTrueKeyword:
		return "true"
	case wrapperchecker.KindFalseKeyword:
		return "false"
	}
	return ""
}
