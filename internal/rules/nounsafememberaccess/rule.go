// Package nounsafememberaccess implements the no-unsafe-member-access rule:
// flag `x.foo` or `x[key]` where x has type any.
package nounsafememberaccess

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-unsafe-member-access"

// Options is the configurable surface of the rule.
type Options struct {
	// AllowOptionalChaining permits `?.` accesses on `any`-typed
	// receivers without reporting. Mirrors typescript-eslint's option
	// of the same name; when true, an optional link breaks the
	// "unsafe" propagation through the chain so a later non-optional
	// access on a still-`any` value can be reported separately.
	AllowOptionalChaining bool
}

func DefaultOptions() Options { return Options{AllowOptionalChaining: false} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-unsafe-member-access options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "allowOptionalChaining":
			if err := json.Unmarshal(val, &out.AllowOptionalChaining); err != nil {
				return Options{}, fmt.Errorf("no-unsafe-member-access option %q: %w", key, err)
			}
		default:
			return Options{}, fmt.Errorf("no-unsafe-member-access has no option %q", key)
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
		wrapperchecker.KindPropertyAccessExpression: r.visit,
		wrapperchecker.KindElementAccessExpression:  r.visit,
	}
}

// chainState classifies a member-access node's contribution to the
// chain. Mirrors upstream's State enum.
type chainState int

const (
	stateSafe chainState = iota
	stateUnsafe
	stateChained
)

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip access expressions inside heritage clauses (`extends X.Y`,
	// `implements X.Y`) — these are type-position references, not
	// runtime member accesses.
	if isInHeritagePosition(n) {
		return
	}
	// Optional accesses break the chain when the option is on; the
	// `?.` consumer has already promised to handle nullishness, so we
	// don't second-guess the access on `any`.
	if r.opts.AllowOptionalChaining && n.IsOptionalChainRoot() {
		return
	}
	recv := accessReceiver(n)
	if recv == nil {
		return
	}
	// If the receiver itself is a member access whose recursive state
	// is already Unsafe, the inner access has already been reported —
	// skip to avoid double-reporting through the chain.
	if isMemberAccess(recv) && r.stateOf(ctx, recv) == stateUnsafe {
		return
	}
	rT := ctx.TypeOf(recv)
	if rT == nil {
		return
	}
	if rT.IsAny() {
		ctx.Report(n, "member access on a value of type `any` defeats the type checker")
		return
	}
	if n.Kind() == wrapperchecker.KindElementAccessExpression {
		idx := n.ElementAccessIndex()
		if idx == nil {
			return
		}
		if idxT := ctx.TypeOf(idx); idxT != nil && idxT.IsAny() {
			ctx.Report(n, "indexing with a value of type `any` defeats the type checker")
		}
	}
}

// stateOf walks a chain of member accesses and returns the
// inner-most state without reporting. Used by the outer visit to
// decide whether the current access has already been reported on
// transitively.
func (r *rule) stateOf(ctx *engine.Context, n *wrapperchecker.Node) chainState {
	if !isMemberAccess(n) {
		return stateSafe
	}
	if r.opts.AllowOptionalChaining && n.IsOptionalChainRoot() {
		return stateChained
	}
	recv := accessReceiver(n)
	if recv == nil {
		return stateSafe
	}
	if isMemberAccess(recv) {
		if s := r.stateOf(ctx, recv); s == stateUnsafe {
			return stateUnsafe
		}
	}
	t := ctx.TypeOf(recv)
	if t != nil && t.IsAny() {
		return stateUnsafe
	}
	return stateSafe
}

func accessReceiver(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return n.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		return n.ElementAccessReceiver()
	}
	return nil
}

func isMemberAccess(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression,
		wrapperchecker.KindElementAccessExpression:
		return true
	}
	return false
}

func isInHeritagePosition(n *wrapperchecker.Node) bool {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case wrapperchecker.KindPropertyAccessExpression,
			wrapperchecker.KindElementAccessExpression:
			continue
		case wrapperchecker.KindExpressionWithTypeArguments,
			wrapperchecker.KindHeritageClause:
			return true
		default:
			return false
		}
	}
	return false
}
