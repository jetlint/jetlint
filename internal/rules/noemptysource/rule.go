// Package noemptysource implements no-empty-source: a source file that
// contains no executable code (only comments, directives, empty
// statements, an empty block, a hashbang line, or nothing at all) is
// almost certainly a mistake — a deleted module, a stale stub, or
// leftover scaffolding.
package noemptysource

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-empty-source"

// kindEmptyStatement / kindEndOfFile are not re-exported by the
// wrapper. Values come from internal/ast/kind_generated.go and are
// stable across the typescript-go fork's API releases (see the
// precedent in noirregularwhitespace for jsx-text kinds).
const (
	kindEmptyStatement = wrapperchecker.Kind(243)
	kindEndOfFile      = wrapperchecker.Kind(1)
)

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	hasMeaningful := false
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		if isMeaningful(c) {
			hasMeaningful = true
			return true
		}
		return false
	})
	if !hasMeaningful {
		ctx.Report(sf, "Unexpected empty source file")
	}
}

// isMeaningful reports whether a top-level child of a SourceFile
// represents real executable code. Returns false for the end-of-file
// token, empty statements (`;`), bare directives (`'use strict';`),
// and empty blocks (`{}`).
func isMeaningful(n *wrapperchecker.Node) bool {
	switch n.Kind() {
	case kindEndOfFile, kindEmptyStatement:
		return false
	case wrapperchecker.KindBlock:
		return len(n.BlockStatements()) > 0
	case wrapperchecker.KindExpressionStatement:
		expr := n.ExpressionStatementExpression()
		if expr == nil {
			return false
		}
		switch expr.Kind() {
		case wrapperchecker.KindStringLiteral,
			wrapperchecker.KindNoSubstitutionTemplateLiteral:
			return false
		}
		return true
	}
	return true
}
