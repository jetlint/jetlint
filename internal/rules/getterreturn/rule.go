// Package getterreturn implements the getter-return rule: every
// path through a getter body must reach `return <value>`, otherwise
// the getter silently returns `undefined`.
//
// Applies to:
//   - `get foo() {}` in class bodies and object literals
//   - `Object.defineProperty(obj, prop, { get: function() {} })` and
//     equivalent `Reflect.defineProperty`, `Object.defineProperties`,
//     `Object.create` patterns
package getterreturn

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/astflow"
	"github.com/jetlint/jetlint/internal/engine"
)

const id = "getter-return"

// Options is the configurable surface of getter-return.
type Options struct {
	// AllowImplicit permits bare `return;` to satisfy the rule.
	// Default is off (matches ESLint).
	AllowImplicit bool
}

func DefaultOptions() Options { return Options{} }

func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("getter-return options must be a JSON object: %w", err)
	}
	for k, v := range fields {
		if k == "allowImplicit" {
			if err := json.Unmarshal(v, &out.AllowImplicit); err != nil {
				return Options{}, err
			}
		}
	}
	return out, nil
}

func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (*rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindGetAccessor:  r.visitGetAccessor,
		wrapperchecker.KindCallExpression: r.visitCall,
	}
}

// visitGetAccessor checks `get foo() {}` style getters defined inside
// classes or object literals.
func (r *rule) visitGetAccessor(ctx *engine.Context, getter *wrapperchecker.Node) {
	r.checkGetter(ctx, getter)
}

// visitCall checks `Object.defineProperty(obj, name, { get: fn })`,
// `Reflect.defineProperty`, `Object.defineProperties`, and
// `Object.create` — patterns where a function literal is used as a
// property descriptor's `get` handler.
func (r *rule) visitCall(ctx *engine.Context, call *wrapperchecker.Node) {
	method, ok := definePropertyKind(call)
	if !ok {
		return
	}
	args := call.CallArguments()
	switch method {
	case "Object.defineProperty", "Reflect.defineProperty":
		// (target, name, descriptor) — descriptor is args[2].
		if len(args) < 3 {
			return
		}
		r.checkDescriptor(ctx, args[2])
	case "Object.defineProperties", "Object.create":
		// (target, descriptors) — descriptors is args[1] for
		// defineProperties, args[1] for create. Each value in the
		// outer object is itself a descriptor.
		descIdx := 1
		if method == "Object.create" {
			descIdx = 1
		}
		if len(args) <= descIdx {
			return
		}
		descriptors := args[descIdx]
		descriptors = stripParens(descriptors)
		if descriptors == nil || descriptors.Kind() != wrapperchecker.KindObjectLiteralExpression {
			return
		}
		descriptors.ForEachChild(func(prop *wrapperchecker.Node) bool {
			if prop.Kind() != wrapperchecker.KindPropertyAssignment {
				return false
			}
			// PropertyAssignment: name + initializer. The initializer
			// is the descriptor.
			children := propertyAssignmentParts(prop)
			if children == nil {
				return false
			}
			r.checkDescriptor(ctx, children.value)
			return false
		})
	}
}

// checkDescriptor visits a property-descriptor object literal and
// runs the getter check on any `get: <function>` entry.
func (r *rule) checkDescriptor(ctx *engine.Context, desc *wrapperchecker.Node) {
	desc = stripParens(desc)
	if desc == nil || desc.Kind() != wrapperchecker.KindObjectLiteralExpression {
		return
	}
	desc.ForEachChild(func(prop *wrapperchecker.Node) bool {
		switch prop.Kind() {
		case wrapperchecker.KindPropertyAssignment:
			parts := propertyAssignmentParts(prop)
			if parts == nil || parts.name != "get" {
				return false
			}
			fn := stripParens(parts.value)
			if !isFunctionLike(fn) {
				return false
			}
			r.checkGetter(ctx, fn)
		case wrapperchecker.KindGetAccessor:
			r.checkGetter(ctx, prop)
		case wrapperchecker.KindMethodDeclaration:
			// `{ get() { ... } }` — shorthand method named "get".
			name := methodName(prop)
			if name != "get" {
				return false
			}
			r.checkGetter(ctx, prop)
		}
		return false
	})
}

// checkGetter runs the body-flow check on a getter function-like node.
func (r *rule) checkGetter(ctx *engine.Context, fn *wrapperchecker.Node) {
	status := astflow.FunctionBodyReturnStatus(fn)
	if status == astflow.AlwaysExplicit {
		return
	}
	if r.opts.AllowImplicit {
		// Bare `return;` is acceptable; even AlwaysMixed paths
		// satisfy if all paths return something.
		if status == astflow.AlwaysExplicit ||
			status == astflow.AlwaysImplicit ||
			status == astflow.AlwaysMixed {
			return
		}
	}
	ctx.Report(fn, "getter must return a value on every path")
}

// definePropertyKind returns the recognized method name (e.g.
// "Object.defineProperty") if the call expression matches one of the
// patterns this rule checks. Optional chaining (`Object?.defineProperty`)
// counts.
func definePropertyKind(call *wrapperchecker.Node) (string, bool) {
	callee := stripParens(call.CalleeExpression())
	if callee == nil {
		return "", false
	}
	if callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return "", false
	}
	receiver := stripParens(propertyAccessReceiver(callee))
	if receiver == nil || receiver.Kind() != wrapperchecker.KindIdentifier {
		return "", false
	}
	objName := receiver.LiteralText()
	methodName := callee.PropertyAccessName()
	switch objName + "." + methodName {
	case "Object.defineProperty",
		"Reflect.defineProperty",
		"Object.defineProperties",
		"Object.create":
		return objName + "." + methodName, true
	}
	return "", false
}

type propAssignment struct {
	name  string
	value *wrapperchecker.Node
}

// propertyAssignmentParts extracts the (name, value) pair from a
// PropertyAssignment. Computed/string keys return name="".
func propertyAssignmentParts(p *wrapperchecker.Node) *propAssignment {
	var name, value *wrapperchecker.Node
	idx := 0
	p.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch idx {
		case 0:
			name = c
		case 1:
			value = c
		}
		idx++
		return false
	})
	if name == nil || value == nil {
		return nil
	}
	var n string
	switch name.Kind() {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		n = name.LiteralText()
	}
	return &propAssignment{name: n, value: value}
}

// methodName returns the name of a MethodDeclaration when its key is
// a simple identifier or string literal. Empty for computed names.
func methodName(m *wrapperchecker.Node) string {
	var key *wrapperchecker.Node
	m.ForEachChild(func(c *wrapperchecker.Node) bool {
		key = c
		return true
	})
	if key == nil {
		return ""
	}
	switch key.Kind() {
	case wrapperchecker.KindIdentifier,
		wrapperchecker.KindStringLiteral,
		wrapperchecker.KindNoSubstitutionTemplateLiteral:
		return key.LiteralText()
	}
	return ""
}

// propertyAccessReceiver returns the `a` in `a.b`.
func propertyAccessReceiver(n *wrapperchecker.Node) *wrapperchecker.Node {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		first = c
		return true
	})
	return first
}

func isFunctionLike(n *wrapperchecker.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case wrapperchecker.KindFunctionExpression,
		wrapperchecker.KindArrowFunction,
		wrapperchecker.KindFunctionDeclaration:
		return true
	}
	return false
}

func stripParens(n *wrapperchecker.Node) *wrapperchecker.Node {
	for n != nil && n.Kind() == wrapperchecker.KindParenthesizedExpression {
		n = n.FirstChild()
	}
	return n
}
