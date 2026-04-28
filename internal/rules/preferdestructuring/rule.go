// Package preferdestructuring implements the prefer-destructuring
// rule: flag `var foo = obj.foo` and `var foo = array[0]` patterns
// that could be destructured.
package preferdestructuring

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	"github.com/tommymorgan/tsgolint/internal/engine"
)

const id = "prefer-destructuring"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindVariableDeclaration: visit,
		wrapperchecker.KindBinaryExpression:    visitAssignment,
	}
}

// visitAssignment handles `y = x[0]` (array destructuring opportunity).
func visitAssignment(ctx *engine.Context, n *wrapperchecker.Node) {
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
	if right.Kind() == wrapperchecker.KindElementAccessExpression {
		if !arrayElementZero(ctx, right) {
			return
		}
		ctx.Report(n, "use array destructuring: [ "+name+" ] = arr")
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	init := n.VariableDeclarationInitializer()
	if init == nil {
		return
	}
	// Annotated declarations are usually deliberate; skip without an
	// opt-in option (enforceForDeclarationWithTypeAnnotation:true).
	if n.VariableDeclarationType() != nil {
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
		if init.PropertyAccessName() == name {
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
	return rt.IsTupleType() || rt.IsArrayLikeType() || rt.ArrayElementType() != nil
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
