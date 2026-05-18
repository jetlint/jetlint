// Package usesimplenumberkeys implements use-simple-number-keys: an
// object key written as `0x1`, `0o12`, `1n`, or `1_0` is the same key
// as `1` / `10` but harder to read — use the decimal form.
package usesimplenumberkeys

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-simple-number-keys"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNumericLiteral: visit,
		wrapperchecker.KindBigIntLiteral:  visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	if !isPropertyKey(n) {
		return
	}
	t := n.SourceText()
	if isSimpleDecimal(t) {
		return
	}
	ctx.Report(n, "use the simple decimal form for this numeric object key")
}

func isPropertyKey(n *wrapperchecker.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	switch p.Kind() {
	case wrapperchecker.KindPropertyAssignment, wrapperchecker.KindMethodDeclaration,
		wrapperchecker.KindGetAccessor, wrapperchecker.KindSetAccessor,
		wrapperchecker.KindPropertyDeclaration:
		// The literal must be the first child (the name).
		var first *wrapperchecker.Node
		p.ForEachChild(func(c *wrapperchecker.Node) bool {
			if first == nil {
				first = c
			}
			return false
		})
		return first == n
	case wrapperchecker.KindComputedPropertyName:
		return true
	}
	return false
}

func isSimpleDecimal(t string) bool {
	if t == "" {
		return false
	}
	if strings.ContainsAny(t, "_nxXoObB") {
		return false
	}
	return true
}
