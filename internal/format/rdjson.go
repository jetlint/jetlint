package format

import (
	"encoding/json"
	"io"
)

// RDJSON emits diagnostics in the Reviewdog Diagnostic Format.
// reviewdog (https://github.com/reviewdog/reviewdog) consumes this
// shape and posts inline PR comments on every major CI provider —
// the lingua franca for "lint findings as PR review comments"
// regardless of platform.
type RDJSON struct{}

// Name returns "rdjson".
func (RDJSON) Name() string { return "rdjson" }

const rdjsonSourceURL = "https://github.com/jetlint/jetlint"

type rdjsonDocument struct {
	Source      rdjsonSource       `json:"source"`
	Diagnostics []rdjsonDiagnostic `json:"diagnostics"`
}

type rdjsonSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type rdjsonDiagnostic struct {
	Message  string         `json:"message"`
	Location rdjsonLocation `json:"location"`
	Severity string         `json:"severity"`
	Code     rdjsonCode     `json:"code"`
}

type rdjsonLocation struct {
	Path  string      `json:"path"`
	Range rdjsonRange `json:"range"`
}

type rdjsonRange struct {
	Start rdjsonPos `json:"start"`
	End   rdjsonPos `json:"end"`
}

type rdjsonPos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type rdjsonCode struct {
	Value string `json:"value"`
}

// Format writes an rdjson document. Severity uses the rdjson
// uppercase vocabulary ("ERROR" / "WARNING") rather than the
// linter-internal lowercase form. Diagnostics are sorted so
// repeated runs over identical input produce byte-identical output.
func (RDJSON) Format(w io.Writer, diagnostics []Diagnostic) error {
	SortDiagnostics(diagnostics)

	out := rdjsonDocument{
		Source: rdjsonSource{
			Name: "jetlint",
			URL:  rdjsonSourceURL,
		},
		Diagnostics: make([]rdjsonDiagnostic, 0, len(diagnostics)),
	}
	for _, d := range diagnostics {
		out.Diagnostics = append(out.Diagnostics, rdjsonDiagnostic{
			Message: d.Message,
			Location: rdjsonLocation{
				Path: d.Range.File,
				Range: rdjsonRange{
					Start: rdjsonPos{Line: d.Range.StartLine, Column: d.Range.StartColumn},
					End:   rdjsonPos{Line: d.Range.EndLine, Column: d.Range.EndColumn},
				},
			},
			Severity: rdjsonSeverity(d.Severity),
			Code:     rdjsonCode{Value: d.RuleID},
		})
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

// rdjsonSeverity maps the linter's severity vocabulary to rdjson's
// fixed uppercase keywords. The format defines INFO / WARNING /
// ERROR; we map our two values straightforwardly and leave INFO for
// future use when notice-level diagnostics land.
func rdjsonSeverity(s Severity) string {
	switch s {
	case "error":
		return "ERROR"
	case "warning":
		return "WARNING"
	default:
		return "INFO"
	}
}
