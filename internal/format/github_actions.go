package format

import (
	"fmt"
	"io"
	"strings"
)

// GitHubActions emits diagnostics as GitHub Actions workflow commands
// (the ::error / ::warning protocol). When tsgolint runs inside a
// GitHub Actions workflow, this produces inline annotations on the
// PR/commit at the exact source location of each finding — the
// highest-leverage CI integration with no extra setup.
//
// Format reference:
// https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-error-message
type GitHubActions struct{}

// Name returns "github". The short name matches biome's reporter
// convention so users can pass the same flag value to either tool.
func (GitHubActions) Name() string { return "github" }

// Format writes one workflow command per diagnostic. The command
// shape is:
//
//	::<level> file=<path>,line=<l>,col=<c>,endLine=<l>,endColumn=<c>,title=<ruleId>::<message>
//
// Newlines and percent signs in the message are escaped per the
// workflow-command convention so multi-line messages render
// correctly in the PR annotation panel.
func (GitHubActions) Format(w io.Writer, diagnostics []Diagnostic) error {
	SortDiagnostics(diagnostics)
	for _, d := range diagnostics {
		level := githubActionsLevel(d.Severity)
		_, err := fmt.Fprintf(w,
			"::%s file=%s,line=%d,col=%d,endLine=%d,endColumn=%d,title=%s::%s\n",
			level,
			d.Range.File,
			d.Range.StartLine,
			d.Range.StartColumn,
			d.Range.EndLine,
			d.Range.EndColumn,
			d.RuleID,
			escapeWorkflowMessage(d.Message),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// githubActionsLevel maps severity to the workflow-command level
// keyword. GitHub recognises "error", "warning", and "notice".
func githubActionsLevel(s Severity) string {
	switch s {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "notice"
	}
}

// escapeWorkflowMessage applies the escape sequences required for
// values inside workflow commands. GitHub interprets %0A as newline
// and %25 as a literal percent; emitting these unescaped breaks the
// command parser.
func escapeWorkflowMessage(msg string) string {
	r := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	)
	return r.Replace(msg)
}
