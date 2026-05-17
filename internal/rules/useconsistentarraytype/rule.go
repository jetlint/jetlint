// Package useconsistentarraytype implements use-consistent-array-type:
// pick `T[]` or `Array<T>` and stick with it. Default leans toward
// `T[]` (shorter; matches DOM type docs).
package useconsistentarraytype

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-consistent-array-type"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindTypeReference: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var first *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if first == nil {
			first = c
		}
		return false
	})
	if first == nil || first.Kind() != wrapperchecker.KindIdentifier {
		return
	}
	if first.SourceText() != "Array" && first.SourceText() != "ReadonlyArray" {
		return
	}
	// Find the type argument; only flag if it's a simple type.
	var typeArg *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx > 0 && typeArg == nil {
			typeArg = c
		}
		idx++
		return false
	})
	if typeArg == nil {
		return
	}
	if !isSimpleType(typeArg) {
		return
	}
	if first.SourceText() == "ReadonlyArray" {
		ctx.Report(n, "use `readonly T[]` instead of `ReadonlyArray<T>`")
	} else {
		ctx.Report(n, "use `T[]` instead of `Array<T>`")
	}
}

func isSimpleType(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case wrapperchecker.KindTypeReference, wrapperchecker.KindArrayType,
		wrapperchecker.KindTupleType, wrapperchecker.KindLiteralType,
		wrapperchecker.KindParenthesizedType:
		return true
	}
	return false
}
