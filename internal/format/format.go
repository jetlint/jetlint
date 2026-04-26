// Package format renders linter diagnostics into bytes for human or
// machine consumption. The package depends only on the standard library
// and the wrapper diagnostic types, keeping it usable from both the CLI
// and the daemon.
package format

import (
	"errors"
	"fmt"
	"io"
	"sort"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// Diagnostic is the linter-internal view of a finding. It is structurally
// identical to the wrapper's lint.Diagnostic, restated here so the format
// package owns the type rather than re-exporting wrapper symbols.
type Diagnostic = wrapperlint.Diagnostic

// Severity is re-exported so format-package callers can compare
// against severity literals without importing the wrapper directly.
type Severity = wrapperlint.Severity

// JSONSchemaVersion is the version number embedded in JSON output. Bumped
// whenever the contract changes in a way consumers must adapt to.
const JSONSchemaVersion = 1

// Formatter renders a sorted slice of diagnostics to w. Implementations
// must be deterministic: identical input must produce identical output.
type Formatter interface {
	Format(w io.Writer, diagnostics []Diagnostic) error
	Name() string
}

// ErrUnknownFormat is returned by Lookup when the requested format name is
// not registered.
var ErrUnknownFormat = errors.New("unknown output format")

// Lookup returns the formatter registered under name, or ErrUnknownFormat.
// The set of supported names is also returned by SupportedNames so the
// CLI can list them in error messages.
func Lookup(name string) (Formatter, error) {
	switch name {
	case "human":
		return Human{}, nil
	case "json":
		return JSON{}, nil
	case "sarif":
		return SARIF{}, nil
	case "github-actions":
		return GitHubActions{}, nil
	default:
		return nil, fmt.Errorf("%w: %q (supported: %v)", ErrUnknownFormat, name, SupportedNames())
	}
}

// SupportedNames returns the set of formatter names supported by Lookup,
// in stable order, so error messages and help text are deterministic.
func SupportedNames() []string {
	return []string{"human", "json", "sarif", "github-actions"}
}

// SortDiagnostics orders diagnostics by file path, then by start position.
// Formatters call this so the byte-identical-output guarantee holds
// regardless of the order rules emitted findings in.
func SortDiagnostics(d []Diagnostic) {
	sort.SliceStable(d, func(i, j int) bool {
		if d[i].Range.File != d[j].Range.File {
			return d[i].Range.File < d[j].Range.File
		}
		if d[i].Range.StartLine != d[j].Range.StartLine {
			return d[i].Range.StartLine < d[j].Range.StartLine
		}
		if d[i].Range.StartColumn != d[j].Range.StartColumn {
			return d[i].Range.StartColumn < d[j].Range.StartColumn
		}
		return d[i].RuleID < d[j].RuleID
	})
}
