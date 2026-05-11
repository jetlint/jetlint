// Package preferdestructuring implements the prefer-destructuring
// rule: flag `var foo = obj.foo` and `var foo = array[0]` patterns
// that could be destructured.
//
// The rule mirrors typescript-eslint's two-layer behaviour:
//
//  1. A type-aware layer fires only on `obj[<integer literal>]` element
//     accesses. When `obj` is not array-like (no Symbol.iterator) and
//     `enforceForRenamedProperties` is set, the access is reported as
//     an object destructuring candidate (`{ 0: foo } = obj`). When
//     `obj` is iterable, the layer falls through to the base rule.
//  2. The base ESLint rule fires on `obj.foo` / `obj[0]` patterns when
//     the variable name matches the property (or when
//     `enforceForRenamedProperties` is set).
//
// The two-layer split matches upstream and is necessary because per-
// pattern enabling (`{ VariableDeclarator: { array, object } }`) and
// the renamed-properties opt-in must be honoured independently.
package preferdestructuring

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-destructuring"

// Options is the configurable surface of the rule.
type Options struct {
	// EnforceForDeclarationWithTypeAnnotation: when true, also flag
	// `var foo: T = obj.foo` and similar — the explicit annotation is
	// no longer treated as opting out.
	EnforceForDeclarationWithTypeAnnotation bool
	// EnforceForRenamedProperties: also flag `var bar = obj.foo` (the
	// destructured form requires `{ foo: bar } = obj`). Off by default
	// because the rename adds noise without much benefit.
	EnforceForRenamedProperties bool
	// AssignmentExpression configures whether to flag array and/or
	// object destructuring opportunities in plain `=` assignments.
	AssignmentExpression DestructuringConfig
	// VariableDeclarator configures the same for `var`, `let`, `const`
	// declarations with an initializer.
	VariableDeclarator DestructuringConfig
}

// DestructuringConfig says whether to flag array-destructuring and/or
// object-destructuring opportunities in a given statement context.
type DestructuringConfig struct {
	Array  bool
	Object bool
}

// DefaultOptions matches typescript-eslint's defaults: both contexts
// flag both array and object destructuring opportunities; no renamed-
// property or annotated-declaration enforcement.
func DefaultOptions() Options {
	return Options{
		AssignmentExpression: DestructuringConfig{Array: true, Object: true},
		VariableDeclarator:   DestructuringConfig{Array: true, Object: true},
	}
}

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: r.visitDeclarator,
		wrapperchecker.KindBinaryExpression:    r.visitAssignment,
	}
}

func (r *rule) visitAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
	if n.BinaryOperatorKind() != wrapperchecker.KindEqualsToken {
		return
	}
	left := n.BinaryLeft()
	right := n.BinaryRight()
	if left == nil || right == nil {
		return
	}
	leftName := ""
	if left.Kind() == wrapperchecker.KindIdentifier {
		leftName = left.LiteralText()
	}
	r.performCheck(ctx, n, leftName, false, right, r.opts.AssignmentExpression)
}

func (r *rule) visitDeclarator(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	leftName := variableName(n)
	annotated := n.VariableDeclarationType() != nil
	r.performCheck(ctx, n, leftName, annotated, init, r.opts.VariableDeclarator)
}

// performCheck is the shared check between variable declarations and
// `=` assignments. It mirrors the two-layer behaviour described above.
func (r *rule) performCheck(
	ctx *engine.Context,
	reportNode *wrapperchecker.Node,
	leftName string,
	leftAnnotated bool,
	right *wrapperchecker.Node,
	cfg DestructuringConfig,
) {
	if leftAnnotated && !r.opts.EnforceForDeclarationWithTypeAnnotation {
		return
	}
	if right == nil || right.IsOptionalChain() {
		return
	}
	isElement := right.Kind() == wrapperchecker.KindElementAccessExpression
	isProperty := right.Kind() == wrapperchecker.KindPropertyAccessExpression
	if !isElement && !isProperty {
		return
	}
	recv := receiverOf(right)
	if recv == nil || recv.Kind() == wrapperchecker.KindSuperKeyword {
		return
	}
	// Skip `obj.#priv` — private identifiers can't be destructured at
	// the call site, only at the class body.
	if isProperty {
		if name := right.PropertyAccessNameNode(); name != nil &&
			name.Kind() == wrapperchecker.KindPrivateIdentifier {
			return
		}
	}

	// --- TS-layer check: `obj[<integer literal>]` ---
	if isElement {
		idx := right.ElementAccessIndex()
		if isIntegerLiteralIndex(idx) {
			objType := ctx.TypeOf(recv)
			if !isAnyOrIterable(objType) {
				// Not iterable: TS layer takes over from the base rule.
				if r.opts.EnforceForRenamedProperties && cfg.Object {
					ctx.Report(reportNode, "use object destructuring: `{ 0: "+nonEmptyName(leftName)+" } = obj` instead of `obj[0]`")
				}
				return
			}
			// Iterable receiver: fall through to base rule's array path.
		}
	}

	// --- Base rule layer ---
	if isElement {
		idx := right.ElementAccessIndex()
		if isIntegerLiteralIndex(idx) {
			if cfg.Array {
				ctx.Report(reportNode, "use array destructuring: [ "+nonEmptyName(leftName)+" ] = arr")
			}
			return
		}
	}
	// Property access OR element access with non-integer index.
	if cfg.Object && r.opts.EnforceForRenamedProperties {
		ctx.Report(reportNode, "use object destructuring: `{ <prop>: "+nonEmptyName(leftName)+" } = obj`")
		return
	}
	if cfg.Object {
		propName, computed := propertyName(right)
		if !computed && propName != "" && propName == leftName {
			ctx.Report(reportNode, "use object destructuring: `{ "+leftName+" } = obj`")
		}
	}
}

func receiverOf(n *wrapperchecker.Node) *wrapperchecker.Node {
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return n.PropertyAccessReceiver()
	case wrapperchecker.KindElementAccessExpression:
		return n.ElementAccessReceiver()
	}
	return nil
}

// propertyName extracts the simple-name view of a member access for the
// "var foo = obj.foo" match: returns the property name and whether the
// access was computed/indexed (so a literal string index like
// `obj['foo']` is treated like `obj.foo`).
func propertyName(n *wrapperchecker.Node) (string, bool) {
	switch n.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		return n.PropertyAccessName(), false
	case wrapperchecker.KindElementAccessExpression:
		idx := n.ElementAccessIndex()
		if idx == nil {
			return "", true
		}
		switch idx.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return idx.LiteralText(), false
		}
		return "", true
	}
	return "", true
}

// isIntegerLiteralIndex reports whether an element-access index is a
// numeric literal whose value is a non-negative integer (the only form
// that's array-destructurable: `arr[0]`, `arr[1]`, ...).
func isIntegerLiteralIndex(idx *wrapperchecker.Node) bool {
	if idx == nil || idx.Kind() != wrapperchecker.KindNumericLiteral {
		return false
	}
	t := idx.LiteralText()
	if t == "" {
		return false
	}
	for _, c := range t {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func nonEmptyName(s string) string {
	if s == "" {
		return "<name>"
	}
	return s
}

// isAnyOrIterable reports whether t is `any` or carries a
// `[Symbol.iterator]` member — making `t[0]` rewritable as
// `const [first] = t`. Union types qualify only when every member
// qualifies. Mirrors typescript-eslint's `isTypeAnyOrIterableType`.
func isAnyOrIterable(t *wrapperchecker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsAny() {
		return true
	}
	if t.IsUnion() {
		for _, m := range t.UnionMembers() {
			if !isAnyOrIterable(m) {
				return false
			}
		}
		return true
	}
	return t.HasSymbolIterator()
}

func variableName(n *wrapperchecker.Node) string {
	var name string
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() == wrapperchecker.KindIdentifier {
			name = c.LiteralText()
			return true
		}
		// Bail at the first non-Identifier child — for destructuring
		// patterns the name slot is a BindingPattern, not an Identifier,
		// so we deliberately return empty (no name to compare against).
		return true
	})
	return name
}
