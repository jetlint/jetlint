// Package nomisusedspread implements the no-misused-spread rule:
// flag spreading a string into an array literal — the per-character
// expansion is rarely what the author intended.
package nomisusedspread

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "no-misused-spread"

// Options is the configurable surface of the rule.
type Options struct {
	// Allow lists symbol names that should not be flagged when
	// spread (e.g. `["Promise", "Map"]`). Matched against the type's
	// SymbolName for a particular spread expression.
	Allow []string
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSpreadElement:    r.visitSpreadElement,
		wrapperchecker.KindSpreadAssignment: r.visitSpreadAssignment,
	}
}

func (r *rule) isAllowed(name string) bool {
	for _, a := range r.opts.Allow {
		if a == name {
			return true
		}
	}
	return false
}

func (r *rule) visitSpreadElement(ctx *engine.Context, n *wrapperchecker.Node) {
	parent := n.Parent()
	if parent == nil {
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
	switch parent.Kind() {
	case wrapperchecker.KindArrayLiteralExpression,
		wrapperchecker.KindCallExpression,
		wrapperchecker.KindNewExpression:
		if !isStringSpread(t) {
			return
		}
		// Allow lists override: `string` for plain strings, the alias
		// name for branded strings (`string & { __brand }`).
		if r.isAllowed("string") || r.isAllowed(t.AliasSymbolName()) {
			return
		}
		ctx.Report(n, "spreading a string iterates per-character; use Array.from or .split() if that's intentional")
	}
}

func (r *rule) visitSpreadAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	expr := n.FirstChild()
	if expr == nil {
		return
	}
	t := ctx.TypeOf(expr)
	if t == nil {
		return
	}
	if isArraySpread(t) {
		ctx.Report(n, "spreading an array into an object — array indices become string keys, which is rarely intentional")
		return
	}
	// `{ ...someSet }` and `{ ...someMap }` — the iteration protocol
	// produces tuples, not key/value pairs the object literal can use.
	// String spreads in object position are allowed: typescript-eslint
	// has a separate rule for those.
	if t.IsStringLike() {
		// Strings are iterable but the per-character spread into an
		// object is intentional — match upstream behavior.
	} else if name := setOrMapSymbol(t); name != "" &&
		!r.isAllowed(name) && !r.isAllowed(t.AliasSymbolName()) && !r.isAllowed(t.SymbolName()) {
		ctx.Report(n, "spreading a "+name+" into an object — Set/Map iteration doesn't produce string-keyed properties")
		return
	}
	// Class instances and class constructors lose methods and
	// prototype info when spread into a plain object.
	if isClassInstanceOrConstructor(t) && !r.isAllowed(t.SymbolName()) {
		ctx.Report(n, "spreading a class instance into an object — methods on the prototype are not copied")
		return
	}
	// A function (callable) spread into an object copies only the
	// function's own properties — the callability is lost. typescript
	// -eslint flags any spread whose every union arm is callable.
	// Functions with expando properties (`f.prop = 1`) and allow-
	// listed names are exempt: the spread there carries useful data.
	if isFunctionLike(t) && !hasExpandoProperty(t) && !r.isAllowed(t.SymbolName()) {
		ctx.Report(n, "spreading a function into an object — the callable signature is not preserved")
	}
}

// hasExpandoProperty reports whether the function-typed value has
// any property beyond the standard `Function` interface — typically
// from JS-expando assignments (`f.prop = 1`). The presence of an
// expando means the spread carries useful data and shouldn't be
// flagged.
func hasExpandoProperty(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	for _, name := range t.PropertyNames() {
		if !isStandardFunctionProp(name) {
			return true
		}
	}
	return false
}

// isStandardFunctionProp reports whether a property name is one of
// the built-in members on the lib `Function` interface — those are
// inherited by every callable and don't make a spread meaningful.
func isStandardFunctionProp(name string) bool {
	switch name {
	case "toString", "prototype", "length", "arguments", "caller",
		"apply", "call", "bind", "name":
		return true
	}
	// Symbol-keyed members the runtime adds (`Symbol.hasInstance`)
	// surface as `@hasInstance@N` — strip the trailing index and
	// match the well-known portion.
	if len(name) > 2 && name[0] == 0xfe {
		// well-known symbol prefix in typescript-go's internal naming.
		return true
	}
	return false
}

// isFunctionLike reports whether t (or every arm of a union) has a
// call signature but no useful object surface beyond it. Specifically,
// every union member must be callable; otherwise the spread might be a
// legitimate object with a function arm.
func isFunctionLike(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsUnion() {
		anyCallable := false
		for _, m := range t.UnionMembers() {
			if m.IsAny() || m.IsUnknown() {
				return false
			}
			if isFunctionLike(m) {
				anyCallable = true
				continue
			}
			// Plain `object` — no call signatures, but typescript-eslint
			// still flags `Function | object` because the call signature
			// path makes the spread misleading. Accept here.
			if m.SymbolName() == "Object" || m.SymbolName() == "" && len(m.PropertyNames()) == 0 {
				continue
			}
			return false
		}
		return anyCallable
	}
	return len(t.CallSignatures()) > 0
}

// isClassInstanceOrConstructor reports whether t (or any union/
// intersection member) is a class instance type or constructor.
// Class declarations on the symbol are the indicator.
func isClassInstanceOrConstructor(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isClassInstanceOrConstructor(m) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isClassInstanceOrConstructor(m) {
				return true
			}
		}
		return false
	}
	sym := t.Symbol()
	if sym == nil {
		return false
	}
	for _, decl := range sym.Declarations() {
		switch decl.Kind() {
		case wrapperchecker.KindClassDeclaration, wrapperchecker.KindClassExpression:
			return true
		}
	}
	return false
}

// setOrMapSymbol returns the named flavor of an iteration- or
// reference-typed value that t (or any union/intersection member)
// carries, or "" when t doesn't reference one. Walks unions and
// intersections so `Set<number> | { a: number }` and
// `Map & Set` are still flagged.
func setOrMapSymbol(t *wrapperchecker.Type) string {
	if t == nil {
		return ""
	}
	switch t.SymbolName() {
	case "Set", "ReadonlySet", "WeakSet",
		"Map", "ReadonlyMap", "WeakMap",
		"Promise", "WeakRef", "Date", "Iterator",
		"AsyncIterator", "Iterable", "AsyncIterable",
		"IterableIterator", "AsyncIterableIterator",
		"Generator", "AsyncGenerator", "RegExp":
		return t.SymbolName()
	}
	// Type-parameter: walk the constraint so `<P extends Promise<void>>`
	// behaves like `Promise<void>` for naming purposes.
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return setOrMapSymbol(c)
		}
	}
	// Custom iterables — anything carrying a Symbol.iterator (or
	// asyncIterator) member but not already classified above. The
	// property name surfaces as `__@iterator@N` or similar in
	// typescript-go's internal naming.
	if hasIteratorMember(t) {
		return "Iterable"
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if name := setOrMapSymbol(m); name != "" {
				return name
			}
		}
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if name := setOrMapSymbol(m); name != "" {
				return name
			}
		}
	}
	return ""
}

// hasIteratorMember reports whether t exposes a [Symbol.iterator] or
// [Symbol.asyncIterator] member directly. typescript-go surfaces those
// as property names containing the substring "@iterator" /
// "@asyncIterator". Skip array-like types — those are handled by the
// arraySpread path.
func hasIteratorMember(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsArrayLikeType() {
		return false
	}
	for _, name := range t.PropertyNames() {
		if containsSubstring(name, "@iterator") || containsSubstring(name, "@asyncIterator") {
			return true
		}
	}
	return false
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func isArraySpread(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsTupleType() || t.IsArrayLikeType() || t.ArrayElementType() != nil {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isArraySpread(m) {
				return true
			}
		}
	}
	return false
}

// isStringSpread reports whether t is a string (or primitive
// intersection / union with string) without also being an array. The
// rule only flags when the value is unambiguously a string — a
// `string | number[]` union still has a non-string array branch
// where spread is legitimate, but upstream still flags it.
func isStringSpread(t *wrapperchecker.Type) bool {
	if t.IsAny() || t.IsUnknown() {
		return false
	}
	if t.IsStringLike() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if isStringSpread(m) {
				return true
			}
		}
		return false
	}
	if t.IsIntersection() {
		for _, m := range t.IntersectionMembers() {
			if isStringSpread(m) {
				return true
			}
		}
	}
	if t.IsTypeParameter() {
		if c := t.BaseConstraint(); c != nil && c != t {
			return isStringSpread(c)
		}
	}
	return false
}
