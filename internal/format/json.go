package format

import (
	"encoding/json"
	"io"
)

// JSON renders diagnostics as a single JSON object so consumers can read
// the entire result with one Unmarshal call. The schema is documented
// alongside JSONSchemaVersion; bumping the constant is a breaking change.
type JSON struct{}

// Name returns "json".
func (JSON) Name() string { return "json" }

type jsonDocument struct {
	SchemaVersion int              `json:"schemaVersion"`
	Diagnostics   []jsonDiagnostic `json:"diagnostics"`
}

type jsonDiagnostic struct {
	File        string `json:"file"`
	StartLine   int    `json:"startLine"`
	StartColumn int    `json:"startColumn"`
	EndLine     int    `json:"endLine"`
	EndColumn   int    `json:"endColumn"`
	RuleID      string `json:"ruleId"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
}

// Format writes a deterministic JSON document with the diagnostics sorted
// by file then position. The trailing newline is always present so the
// output is shell-friendly.
func (JSON) Format(w io.Writer, diagnostics []Diagnostic) error {
	SortDiagnostics(diagnostics)
	doc := jsonDocument{
		SchemaVersion: JSONSchemaVersion,
		Diagnostics:   make([]jsonDiagnostic, 0, len(diagnostics)),
	}
	for _, d := range diagnostics {
		doc.Diagnostics = append(doc.Diagnostics, jsonDiagnostic{
			File:        d.Range.File,
			StartLine:   d.Range.StartLine,
			StartColumn: d.Range.StartColumn,
			EndLine:     d.Range.EndLine,
			EndColumn:   d.Range.EndColumn,
			RuleID:      d.RuleID,
			Severity:    string(d.Severity),
			Message:     d.Message,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}
