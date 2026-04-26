// Package engine drives a single AST walk per file and dispatches to
// every rule handler registered for each visited node kind. Rules
// produce diagnostics through Context.Report; the engine collects them
// and returns the final, formatter-ready slice.
package engine

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// Rule is the contract every linter rule satisfies. Meta names the rule
// (matching its identifier in configuration) and Handlers maps each AST
// kind the rule cares about to a function the engine calls when visiting
// such a node.
type Rule interface {
	Meta() Meta
	Handlers() map[wrapperchecker.Kind]Handler
}

// Meta carries the rule's identifying metadata.
type Meta struct {
	ID string
}

// Handler is invoked once per matching AST node visited by the engine.
type Handler func(ctx *Context, n *wrapperchecker.Node)

// Context is the per-rule, per-file state passed to a Handler. Rules
// query types via Context's wrapped Checker and report findings via
// Context.Report.
type Context struct {
	program     *wrapperchecker.Program
	checker     *wrapperchecker.Checker
	ruleID      string
	severity    wrapperlint.Severity
	diagnostics *[]wrapperlint.Diagnostic
}

// Program returns the wrapper-layer Program for the current file.
func (c *Context) Program() *wrapperchecker.Program { return c.program }

// Checker returns the wrapper-layer Checker bound to the program.
func (c *Context) Checker() *wrapperchecker.Checker { return c.checker }

// TypeOf is shorthand for Checker().TypeOf(n).
func (c *Context) TypeOf(n *wrapperchecker.Node) *wrapperchecker.Type {
	return c.checker.TypeOf(n)
}

// Report emits a diagnostic for node n with the given message under the
// active rule's identifier and severity. Handlers can call Report any
// number of times; each call produces a separate diagnostic.
func (c *Context) Report(n *wrapperchecker.Node, message string) {
	if n == nil {
		return
	}
	file, sl, sc, el, ec := n.SourceRange()
	*c.diagnostics = append(*c.diagnostics, wrapperlint.Diagnostic{
		Range: wrapperlint.SourceRange{
			File:        file,
			StartLine:   sl,
			StartColumn: sc,
			EndLine:     el,
			EndColumn:   ec,
		},
		RuleID:   c.ruleID,
		Severity: c.severity,
		Message:  message,
	})
}
