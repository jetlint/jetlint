// Package noexportsintest implements no-exports-in-test: a file that
// looks like a test (a top-level `describe`/`test`/`it`/`suite` call)
// should not also export production code. Such "exports" almost always
// indicate the file was repurposed without removing leftover module
// surface, or that production code is being colocated with tests.
package noexportsintest

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-exports-in-test"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindSourceFile: visit,
	}
}

var testFnNames = map[string]bool{
	"describe": true,
	"suite":    true,
	"context":  true,
	"test":     true,
	"it":       true,
	"bench":    true,
}

func visit(ctx *engine.Context, sf *wrapperchecker.Node) {
	if !hasTopLevelTestCall(sf) {
		return
	}
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		switch c.Kind() {
		case wrapperchecker.KindExportDeclaration,
			wrapperchecker.KindExportAssignment:
			ctx.Report(c, "Do not export from a test file.")
			return false
		}
		if c.HasExportModifier() {
			ctx.Report(c, "Do not export from a test file.")
			return false
		}
		if c.Kind() == wrapperchecker.KindExpressionStatement {
			expr := c.ExpressionStatementExpression()
			if expr != nil && isModuleExportsAssign(expr) {
				ctx.Report(c, "Do not export from a test file.")
			}
		}
		return false
	})
}

func hasTopLevelTestCall(sf *wrapperchecker.Node) bool {
	found := false
	sf.ForEachChild(func(c *wrapperchecker.Node) bool {
		if c.Kind() != wrapperchecker.KindExpressionStatement {
			return false
		}
		expr := c.ExpressionStatementExpression()
		if expr == nil || expr.Kind() != wrapperchecker.KindCallExpression {
			return false
		}
		callee := firstChild(expr)
		if callee == nil {
			return false
		}
		var name string
		switch callee.Kind() {
		case wrapperchecker.KindIdentifier:
			name = callee.SourceText()
		case wrapperchecker.KindPropertyAccessExpression:
			target := firstChild(callee)
			if target != nil && target.Kind() == wrapperchecker.KindIdentifier {
				name = target.SourceText()
			}
		}
		if testFnNames[name] {
			found = true
			return true
		}
		return false
	})
	return found
}

func isModuleExportsAssign(expr *wrapperchecker.Node) bool {
	if expr.Kind() != wrapperchecker.KindBinaryExpression {
		return false
	}
	if operatorToken(expr) != "=" {
		return false
	}
	lhs := firstChild(expr)
	for lhs != nil {
		if lhs.Kind() == wrapperchecker.KindPropertyAccessExpression {
			target := firstChild(lhs)
			if target != nil && target.Kind() == wrapperchecker.KindIdentifier &&
				target.SourceText() == "module" && lhs.PropertyAccessName() == "exports" {
				return true
			}
		}
		if lhs.Kind() != wrapperchecker.KindElementAccessExpression &&
			lhs.Kind() != wrapperchecker.KindPropertyAccessExpression {
			return false
		}
		next := firstChild(lhs)
		if next == lhs {
			return false
		}
		lhs = next
	}
	return false
}

func firstChild(n *wrapperchecker.Node) *wrapperchecker.Node {
	var f *wrapperchecker.Node
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if f == nil {
			f = c
		}
		return false
	})
	return f
}

func operatorToken(n *wrapperchecker.Node) string {
	var second *wrapperchecker.Node
	idx := 0
	n.ForEachChild(func(c *wrapperchecker.Node) bool {
		if idx == 1 {
			second = c
			return true
		}
		idx++
		return false
	})
	if second == nil {
		return ""
	}
	return second.SourceText()
}
