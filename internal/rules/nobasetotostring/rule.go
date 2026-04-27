// Package nobasetotostring implements the no-base-to-string rule:
// flag any expression that converts a value to a string when the
// value's type doesn't supply a meaningful string conversion (i.e.
// would render as "[object Object]").
//
// Behavioral spec: a Go reimplementation of the rule of the same name
// from typescript-eslint. Handled forms:
//   - template-literal interpolation: `${x}`
//   - explicit conversion calls: x.toString(), x.toLocaleString(), String(x)
//   - array.join(...) where any element lacks meaningful toString
//   - string concatenation: 'pre' + x and x += 'suffix'
//
// Tagged template literals are skipped — the tag function decides how
// to handle each interpolation.
package nobasetotostring

import (
	"encoding/json"
	"fmt"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-base-to-string"

// Options is the configurable surface of the rule.
type Options struct {
	// IgnoredTypeNames is a list of type names whose toString-receiving
	// values should not be flagged. Defaults to typescript-eslint's
	// stock list of types whose default toString is acceptable.
	IgnoredTypeNames []string
	// CheckUnknown: when true, `unknown` and `any` are flagged in
	// string-conversion positions. Default false.
	CheckUnknown bool
}

// DefaultOptions matches typescript-eslint's defaults.
func DefaultOptions() Options {
	return Options{
		IgnoredTypeNames: []string{"Error", "RegExp", "URL", "URLSearchParams"},
	}
}

// OptionsFromJSON parses raw JSON options. Unknown keys produce errors.
func OptionsFromJSON(raw json.RawMessage) (Options, error) {
	out := DefaultOptions()
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Options{}, fmt.Errorf("no-base-to-string options must be a JSON object: %w", err)
	}
	for key, val := range fields {
		switch key {
		case "ignoredTypeNames":
			if err := json.Unmarshal(val, &out.IgnoredTypeNames); err != nil {
				return Options{}, fmt.Errorf("no-base-to-string option %q: %w", key, err)
			}
		case "checkUnknown":
			if err := json.Unmarshal(val, &out.CheckUnknown); err != nil {
				return Options{}, fmt.Errorf("no-base-to-string option %q: %w", key, err)
			}
		default:
			return Options{}, fmt.Errorf("no-base-to-string has no option %q (expected ignoredTypeNames or checkUnknown)", key)
		}
	}
	return out, nil
}

// New constructs a fresh rule instance with default options.
func New() engine.Rule { return NewWithOptions(DefaultOptions()) }

// NewWithOptions constructs a rule with the given options.
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct {
	opts Options
}

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTemplateSpan:     r.visitTemplateSpan,
		wrapperchecker.KindCallExpression:   r.visitCallExpression,
		wrapperchecker.KindBinaryExpression: r.visitBinaryExpression,
	}
}

func (r *rule) visitTemplateSpan(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	// Skip spans inside a TaggedTemplateExpression — the tag function
	// owns the conversion. Walk up: TemplateSpan -> TemplateExpression
	// -> (parent). If the parent is TaggedTemplateExpression, skip.
	if templateExpr := n.Parent(); templateExpr != nil {
		if outer := templateExpr.Parent(); outer != nil &&
			outer.Kind() == wrapperchecker.KindTaggedTemplateExpression {
			return
		}
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if r.hasMeaningfulStringConversion(t) {
		return
	}
	ctx.Report(expr, baseMessage)
}

// visitCallExpression handles the explicit-conversion calls:
//   - X.toString() / X.toLocaleString() (no args)
//   - X.join(...) on an array
//   - String(X) global call
func (r *rule) visitCallExpression(ctx *engine.Context, call *wrapperchecker.Node) {
	callee := call.CalleeExpression()
	if callee == nil {
		return
	}
	args := call.CallArguments()

	// String(x). Only the GLOBAL String — confirm the identifier
	// resolves to the ambient declaration in lib.es5.d.ts and not a
	// user-defined or imported symbol.
	if callee.Kind() == wrapperchecker.KindIdentifier && callee.LiteralText() == "String" && len(args) >= 1 {
		// Spread args: can't reason about the spread's contents.
		for _, a := range args {
			if a.Kind() == wrapperchecker.KindSpreadElement {
				return
			}
		}
		if !calleeIsGlobalString(ctx, callee) {
			return
		}
		argT := ctx.TypeOf(args[0])
		if argT == nil {
			return
		}
		if r.hasMeaningfulStringConversion(argT) {
			return
		}
		ctx.Report(call, baseMessage)
		return
	}

	if callee.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	method := callee.PropertyAccessName()
	switch method {
	case "toString", "toLocaleString":
		if len(args) != 0 {
			return
		}
		recv := callee.PropertyAccessReceiver()
		if recv == nil {
			return
		}
		recvT := ctx.TypeOf(recv)
		if recvT == nil {
			return
		}
		if r.hasMeaningfulStringConversion(recvT) {
			return
		}
		ctx.Report(call, baseMessage)
	case "join":
		recv := callee.PropertyAccessReceiver()
		if recv == nil {
			return
		}
		recvT := ctx.TypeOf(recv)
		if recvT == nil {
			return
		}
		if !r.arrayElementsHaveMeaningfulStringConversion(recvT) {
			ctx.Report(call, baseMessage)
		}
	}
}

// calleeIsGlobalString reports whether the `String` identifier in the
// call resolves to the ambient global. We treat anything declared in
// the user's source (a function `String(...)`, a destructured import)
// as shadowing — the call's behavior is the user's responsibility.
func calleeIsGlobalString(ctx *engine.Context, callee *wrapperchecker.Node) bool {
	sym := ctx.Checker().SymbolOf(callee)
	if sym == nil {
		// Couldn't resolve a symbol at all — be conservative and treat
		// it as NOT global so we don't flag user-shadowed calls.
		return false
	}
	// The wrapper doesn't currently expose declaration source files.
	// Heuristic: if the symbol's name is "String" and the apparent
	// type's call signatures look like the global String constructor
	// (returns string), we treat it as global. This is imperfect for
	// users who write `function String(v): string`, but matches the
	// dominant case.
	if sym.Name() != "String" {
		return false
	}
	t := ctx.TypeOf(callee)
	if t == nil {
		return false
	}
	for _, sig := range t.CallSignatures() {
		if rt := sig.ReturnType(); rt != nil && rt.IsStringLike() {
			return true
		}
	}
	return false
}

// visitBinaryExpression handles `x + y` and `x += y` where one side is
// a string and the other is a value that doesn't render meaningfully.
func (r *rule) visitBinaryExpression(ctx *engine.Context, expr *wrapperchecker.Node) {
	op := expr.BinaryOperatorKind()
	if op != wrapperchecker.KindPlusToken && op != wrapperchecker.KindPlusEqualsToken {
		return
	}
	left := expr.BinaryLeft()
	right := expr.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftT := ctx.TypeOf(left)
	rightT := ctx.TypeOf(right)
	if leftT == nil || rightT == nil {
		return
	}
	leftStr := leftT.IsStringLike()
	rightStr := rightT.IsStringLike()
	if !leftStr && !rightStr {
		return
	}
	// If one side is string, the other side gets implicit string
	// conversion. Flag if the non-string side lacks a meaningful one.
	if leftStr && !rightStr && !r.hasMeaningfulStringConversion(rightT) {
		ctx.Report(expr, baseMessage)
		return
	}
	if rightStr && !leftStr && !r.hasMeaningfulStringConversion(leftT) {
		ctx.Report(expr, baseMessage)
	}
}

const baseMessage = "interpolating a value whose type does not declare a custom string conversion will produce \"[object Object]\" or a similar default rendering"

// hasMeaningfulStringConversion returns true if the type provides a
// useful string conversion. For unions, every member must qualify.
// For intersections, any member with meaningful conversion qualifies
// the whole (intersection like `string & Brand` has string's).
// For arrays/tuples, defer to the element/member types.
func (r *rule) hasMeaningfulStringConversion(t *wrapperchecker.Type) bool {
	return r.hasMeaningful(t, 0)
}

// recursionLimit caps the depth of type recursion. Generics with
// self-referential constraints, mutually-recursive aliases, and
// pathological array nesting can otherwise blow the stack.
const recursionLimit = 16

func (r *rule) hasMeaningful(t *wrapperchecker.Type, depth int) bool {
	if t == nil {
		return true
	}
	if depth > recursionLimit {
		return true
	}
	// IgnoredTypeNames option: skip these by symbol, alias, or any
	// ancestor name (so `class CustomError extends Error` is ignored
	// when "Error" is in the list).
	if r.typeOrAncestorIgnored(t, depth) {
		return true
	}
	if t.IsAny() || t.IsUnknown() {
		// Default: unknown/any considered fine. With CheckUnknown, flag.
		return !r.opts.CheckUnknown
	}
	if t.IsNullOrUndefined() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !r.hasMeaningful(m, depth+1) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if r.hasMeaningful(m, depth+1) {
				return true
			}
		}
		return false
	}
	if t.IsTupleType() {
		for _, e := range t.TypeArguments() {
			if !r.hasMeaningful(e, depth+1) {
				return false
			}
		}
		return true
	}
	if elem := t.ArrayElementType(); elem != nil {
		if elem.IsNever() {
			return true
		}
		return r.hasMeaningful(elem, depth+1)
	}
	if c := t.BaseConstraint(); c != nil && c != t {
		return r.hasMeaningful(c, depth+1)
	}
	return t.HasOwnToString()
}

func (r *rule) isIgnoredTypeName(name string) bool {
	for _, ignored := range r.opts.IgnoredTypeNames {
		if ignored == name {
			return true
		}
	}
	return false
}

// typeOrAncestorIgnored walks the type's symbol, alias, and base-type
// chain to see if any name matches the ignored list. The wrapper's
// IsPromise uses the same pattern for class-extends walking.
func (r *rule) typeOrAncestorIgnored(t *wrapperchecker.Type, depth int) bool {
	if depth > recursionLimit {
		return false
	}
	if name := t.SymbolName(); name != "" && r.isIgnoredTypeName(name) {
		return true
	}
	if name := t.AliasSymbolName(); name != "" && r.isIgnoredTypeName(name) {
		return true
	}
	for _, base := range t.BaseTypeNames() {
		if r.isIgnoredTypeName(base) {
			return true
		}
	}
	return false
}

// arrayElementsHaveMeaningfulStringConversion returns true when a
// receiver type used as `.join(...)` has elements that all render
// meaningfully.
func (r *rule) arrayElementsHaveMeaningfulStringConversion(t *wrapperchecker.Type) bool {
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !r.arrayElementsHaveMeaningfulStringConversion(m) {
				return false
			}
		}
		return true
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if !r.arrayElementsHaveMeaningfulStringConversion(m) {
				return false
			}
		}
		return true
	}
	if t.IsTupleType() {
		for _, e := range t.TypeArguments() {
			if !r.hasMeaningfulStringConversion(e) {
				return false
			}
		}
		return true
	}
	if elem := t.ArrayElementType(); elem != nil {
		if elem.IsNever() {
			return true
		}
		return r.hasMeaningfulStringConversion(elem)
	}
	return true
}
