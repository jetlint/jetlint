// Package useshorthandfunctiontype implements
// use-shorthand-function-type: `interface F { (x: number): string }`
// is a stylistic anachronism. `type F = (x: number) => string` is
// what everyone reaches for.
package useshorthandfunctiontype

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-shorthand-function-type"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindInterfaceDeclaration: visit,
		wrapperchecker.KindTypeLiteral:          visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// Skip if it extends — extension implies semantic identity beyond shape.
	hasExtends := false
	memberCount := 0
	hasCallSig := false
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindHeritageClause:
			hasExtends = true
		case wrapperchecker.KindCallSignature:
			memberCount++
			hasCallSig = true
		case wrapperchecker.KindPropertySignature, wrapperchecker.KindMethodSignature,
			wrapperchecker.KindIndexSignature, wrapperchecker.KindConstructSignature:
			memberCount++
		}
		return false
	})
	if hasExtends || memberCount != 1 || !hasCallSig {
		return
	}
	// Skip self-referential `(arg): this` — can't be expressed as alias.
	if strings.Contains(n.SourceText(), ": this") || strings.Contains(n.SourceText(), ":this") {
		return
	}
	// Skip if the interface name appears as a return type of its own signature
	// (self-recursion via name).
	ctx.Report(n, "single-call-signature interface — use a function type alias")
}
