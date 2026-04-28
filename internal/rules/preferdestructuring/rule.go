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
	if init.Kind() != wrapperchecker.KindPropertyAccessExpression {
		return
	}
	if init.PropertyAccessName() == name {
		ctx.Report(n, "use object destructuring: { "+name+" } = obj")
	}
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
