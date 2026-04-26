package format

import (
	"fmt"
	"io"
)

// Human is the default formatter. It renders one diagnostic per line in a
// scannable layout: file:line:column severity rule message
type Human struct{}

// Name returns "human".
func (Human) Name() string { return "human" }

// Format writes a human-readable rendering of diagnostics to w.
func (Human) Format(w io.Writer, diagnostics []Diagnostic) error {
	SortDiagnostics(diagnostics)
	for _, d := range diagnostics {
		_, err := fmt.Fprintf(w, "%s:%d:%d %s %s %s\n",
			d.Range.File, d.Range.StartLine, d.Range.StartColumn,
			d.Severity, d.RuleID, d.Message)
		if err != nil {
			return err
		}
	}
	return nil
}
