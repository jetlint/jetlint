package engine

import (
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// Engine runs registered rules against a Program. The active set of
// rules and their severities is supplied at construction; the engine
// itself is stateless beyond that.
type Engine struct {
	rules          []Rule
	severities     map[string]wrapperlint.Severity
	options        map[string]any
	maxDiagnostics int // 0 = unlimited
}

// New creates an Engine with the given rules and per-rule severities.
// Rules whose ID is absent from severities are skipped (treated as off).
func New(rules []Rule, severities map[string]wrapperlint.Severity) *Engine {
	return &Engine{rules: rules, severities: severities}
}

// WithOptions returns the same Engine instance with rule-specific
// options attached. The options map is keyed by rule ID; the shape
// of the value is agreed between the rule and the caller. Rules
// without an entry in the map see Context.Options() == nil.
func (e *Engine) WithOptions(options map[string]any) *Engine {
	e.options = options
	return e
}

// WithMaxDiagnostics caps the number of error-severity diagnostics the
// engine will collect before bailing out of the per-file walk. A value
// of 0 (the default) means unlimited. Warning-severity diagnostics
// don't count toward the cap; the goal is to spare large projects from
// running every rule against every file once the caller already knows
// it won't display more than N errors.
func (e *Engine) WithMaxDiagnostics(n int) *Engine {
	e.maxDiagnostics = n
	return e
}

// Lint runs every active rule against every user source file in the
// program and returns the collected diagnostics.
func (e *Engine) Lint(program *wrapperchecker.Program) []wrapperlint.Diagnostic {
	chk := program.Checker()
	var diagnostics []wrapperlint.Diagnostic

	// Pre-compute per-kind handler lists across all active rules so the
	// AST walk is one pass with O(1) dispatch per node.
	type entry struct {
		ruleID   string
		severity wrapperlint.Severity
		handler  Handler
	}
	dispatch := map[wrapperchecker.Kind][]entry{}
	for _, r := range e.rules {
		sev, ok := e.severities[r.Meta().ID]
		if !ok || sev == "" {
			continue
		}
		for kind, h := range r.Handlers() {
			dispatch[kind] = append(dispatch[kind], entry{
				ruleID:   r.Meta().ID,
				severity: sev,
				handler:  h,
			})
		}
	}
	if len(dispatch) == 0 {
		return diagnostics
	}

	errorCount := 0
	stopped := false
	for _, file := range program.SourceFiles() {
		if stopped {
			break
		}
		file.Walk(func(n *wrapperchecker.Node) bool {
			if stopped {
				return false
			}
			entries, ok := dispatch[n.Kind()]
			if !ok {
				return true
			}
			for _, ent := range entries {
				before := len(diagnostics)
				ctx := &Context{
					program:     program,
					checker:     chk,
					ruleID:      ent.ruleID,
					severity:    ent.severity,
					options:     e.options[ent.ruleID],
					diagnostics: &diagnostics,
				}
				dispatchSafely(ctx, ent.handler, n, file.Path(), ent.ruleID, &diagnostics)
				// Only error-severity diagnostics count toward the cap.
				// dispatchSafely may also push a rule-panic warning; those
				// must not consume budget intended for real findings.
				for i := before; i < len(diagnostics); i++ {
					if diagnostics[i].Severity == wrapperlint.SeverityError {
						errorCount++
					}
				}
				if e.maxDiagnostics > 0 && errorCount >= e.maxDiagnostics {
					stopped = true
					return false
				}
			}
			return true
		})
	}
	return diagnostics
}

// dispatchSafely invokes a rule handler with a deferred recover. A
// panic from the underlying type checker (or a buggy rule) is captured
// and reported as a per-file tool-error diagnostic so the rest of the
// lint pass continues. Without this guard, one bad node would abort
// the entire invocation.
func dispatchSafely(ctx *Context, h Handler, n *wrapperchecker.Node, file, ruleID string, diags *[]wrapperlint.Diagnostic) {
	defer func() {
		if r := recover(); r != nil {
			startLine, startCol, endLine, endCol := 1, 1, 1, 1
			if n != nil {
				_, sl, sc, el, ec := n.SourceRange()
				startLine, startCol, endLine, endCol = sl, sc, el, ec
			}
			*diags = append(*diags, wrapperlint.Diagnostic{
				Range: wrapperlint.SourceRange{
					File:        file,
					StartLine:   startLine,
					StartColumn: startCol,
					EndLine:     endLine,
					EndColumn:   endCol,
				},
				RuleID:   "jetlint/rule-panic",
				Severity: wrapperlint.SeverityWarning,
				Message:  "rule '" + ruleID + "' panicked while processing this node; remaining files were linted; please report this as a bug",
			})
		}
	}()
	h(ctx, n)
}
