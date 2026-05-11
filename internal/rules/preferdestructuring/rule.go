// Package preferdestructuring implements the prefer-destructuring
// rule: flag `var foo = obj.foo` and `var foo = array[0]` patterns
// that could be destructured.
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
}

func DefaultOptions() Options { return Options{} }

func New() engine.Rule                        { return NewWithOptions(DefaultOptions()) }
func NewWithOptions(opts Options) engine.Rule { return &rule{opts: opts} }

type rule struct{ opts Options }

func (r *rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (r *rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: r.visit,
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
	if left.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	name := left.LiteralText()
	if name == "" {
		return
	}
	switch right.Kind() {
	case wrapperchecker.KindElementAccessExpression:
		if arrayElementZero(ctx, right) {
			ctx.Report(n, "use array destructuring: [ "+name+" ] = arr")
			return
		}
		// `y = x[i]` where i is a non-zero literal index: only flagged
		// with `enforceForRenamedProperties`, since the destructured
		// rewrite would need to rename or reorder.
		if r.opts.EnforceForRenamedProperties && right.ElementAccessIndex() != nil {
			ctx.Report(n, "use array destructuring: [ "+name+" ] = arr")
		}
	case wrapperchecker.KindPropertyAccessExpression:
		if right.IsOptionalChain() {
			return
		}
		if right.PropertyAccessName() == name || r.opts.EnforceForRenamedProperties {
			ctx.Report(n, "use object destructuring: { "+name+" } = obj")
		}
	}
}

func (r *rule) visit(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	// Annotated declarations: deliberate by default. The
	// `enforceForDeclarationWithTypeAnnotation` option opts in.
	if n.VariableDeclarationType() != nil && !r.opts.EnforceForDeclarationWithTypeAnnotation {
		return
	}
	name := variableName(n)
	if name == "" {
		return
	}
	switch init.Kind() {
	case wrapperchecker.KindPropertyAccessExpression:
		if init.IsOptionalChain() {
			return
		}
		// `super.foo` cannot be destructured — there is no destructuring
		// form for `super`, so don't suggest one.
		if recv := init.PropertyAccessReceiver(); recv != nil && recv.Kind() == wrapperchecker.KindSuperKeyword {
			return
		}
		if init.PropertyAccessName() == name || r.opts.EnforceForRenamedProperties {
			ctx.Report(n, "use object destructuring: { "+name+" } = obj")
		}
	case wrapperchecker.KindElementAccessExpression:
		if !arrayElementZero(ctx, init) {
			return
		}
		ctx.Report(n, "use array destructuring: [ "+name+" ] = arr")
	}
}

// arrayElementZero reports whether expr is `recv[0]` on an array-like
// receiver (and not an optional chain).
func arrayElementZero(ctx *engine.Context, expr *wrapperchecker.Node) bool {
	if expr.IsOptionalChain() {
		return false
	}
	idx := expr.ElementAccessIndex()
	if idx == nil || idx.Kind() != wrapperchecker.KindNumericLiteral || idx.LiteralText() != "0" {
		return false
	}
	recv := expr.ElementAccessReceiver()
	if recv == nil {
		return false
	}
	rt := ctx.TypeOf(recv)
	if rt == nil {
		return false
	}
	return isAnyOrIterable(rt)
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
		return false
	})
	return name
}
