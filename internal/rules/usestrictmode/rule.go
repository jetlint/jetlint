// Package usestrictmode implements use-strict-mode: classic scripts
// should start with `"use strict";` to opt into stricter semantics.
package usestrictmode

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "use-strict-mode"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	var stmts []*wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		stmts = append(stmts, c)
		return false
	})
	// Filter out non-statement tokens that ForEachChild may yield.
	var real []*wrapperchecker.Node
	for _, s := range stmts {
		if isStatement(s) {
			real = append(real, s)
		}
	}
	if len(real) == 0 {
		return
	}
	stmts = real
	// Find directive prologue (leading ExpressionStatements with string literals).
	for _, s := range stmts {
		if s.Kind() != wrapperchecker.KindExpressionStatement {
			break
		}
		var inner *wrapperchecker.Node
		s.ForEachChild(func(c *wrapperchecker.Node) bool {
			if inner == nil {
				inner = c
			}
			return false
		})
		if inner == nil || inner.Kind() != wrapperchecker.KindStringLiteral {
			break
		}
		txt := strings.Trim(inner.SourceText(), `"'`+"`")
		if txt == "use strict" {
			return
		}
	}
	ctx.Report(n, `file is missing "use strict" directive`)
}

func isStatement(s *wrapperchecker.Node) bool {
	switch s.Kind() {
	case wrapperchecker.KindVariableStatement, wrapperchecker.KindExpressionStatement,
		wrapperchecker.KindIfStatement, wrapperchecker.KindReturnStatement,
		wrapperchecker.KindFunctionDeclaration, wrapperchecker.KindClassDeclaration,
		wrapperchecker.KindForStatement, wrapperchecker.KindWhileStatement,
		wrapperchecker.KindDoStatement, wrapperchecker.KindBlock,
		wrapperchecker.KindThrowStatement, wrapperchecker.KindTryStatement,
		wrapperchecker.KindForInStatement, wrapperchecker.KindForOfStatement,
		wrapperchecker.KindSwitchStatement, wrapperchecker.KindBreakStatement,
		wrapperchecker.KindContinueStatement, wrapperchecker.KindLabeledStatement,
		wrapperchecker.KindImportDeclaration, wrapperchecker.KindExportDeclaration,
		wrapperchecker.KindExportAssignment, wrapperchecker.KindTypeAliasDeclaration,
		wrapperchecker.KindInterfaceDeclaration, wrapperchecker.KindEnumDeclaration,
		wrapperchecker.KindModuleDeclaration,
		wrapperchecker.KindDebuggerStatement:
		return true
	}
	return false
}
