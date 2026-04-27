// Package nomeaninglessvoidoperator implements the no-meaningless-void-operator
// rule: flag `void X` where X already has type void (or never, when
// CheckNever is enabled).
package nomeaninglessvoidoperator

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-meaningless-void-operator"

// Options is the configurable surface of the rule.
type Options struct {
	// CheckNever: when true, `void neverValue` is also flagged.
	// Default false.
	CheckNever bool
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options { return Options{} }

// OptionsFromJSON parses raw JSON options.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-meaningless-void-operator options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "checkNever":
			if err := json.Unmarshal(val, &out.CheckNever); err != nil {
				return Options{}, fmt.Errorf("no-meaningless-void-operator option %q: %w", key, err)
			}
		default:
			return Options{}, fmt.Errorf("no-meaningless-void-operator has no option %q", key)
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
		wrapperchecker.KindVoidExpression: r.visit,
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	operand := n.FirstChild()
	if operand == nil {
		return
	}
	t := ctx.TypeOf(operand)
	if t == nil {
		return
	}
	if t.IsVoid() {
		ctx.Report(n, "void of a value already typed void is redundant")
		return
	}
	if t.IsNever() && r.opts.CheckNever {
		ctx.Report(n, "void of a value already typed never is redundant")
	}
}
