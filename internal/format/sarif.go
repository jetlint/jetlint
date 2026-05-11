package format

import (
	"encoding/json"
	"io"
	"sort"
)

// SARIF emits diagnostics as SARIF v2.1.0, the OASIS standard
// (Static Analysis Results Interchange Format) consumed by GitHub
// Code Scanning, Azure DevOps, and the security-tool ecosystem at
// large.
type SARIF struct{}

// Name returns "sarif".
func (SARIF) Name() string { return "sarif" }

const sarifSchemaURL = "https://json.schemastore.org/sarif-2.1.0.json"
const sarifVersion = "2.1.0"
const sarifInformationURI = "https://github.com/jetlint/jetlint"

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool      `json:"tool"`
	Results []sarifResult  `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID string `json:"id"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

// Format writes a SARIF v2.1.0 log document. The driver section
// declares every rule that produced at least one result so
// SARIF-aware consumers can group by rule. Diagnostics are sorted
// by file and position so output is byte-identical for identical
// input.
func (SARIF) Format(w io.Writer, diagnostics []Diagnostic) error {
	SortDiagnostics(diagnostics)

	rules := uniqueRuleIDs(diagnostics)
	results := make([]sarifResult, 0, len(diagnostics))
	for _, d := range diagnostics {
		results = append(results, sarifResult{
			RuleID:  d.RuleID,
			Level:   sarifLevel(d.Severity),
			Message: sarifMessage{Text: d.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: d.Range.File},
					Region: sarifRegion{
						StartLine:   d.Range.StartLine,
						StartColumn: d.Range.StartColumn,
						EndLine:     d.Range.EndLine,
						EndColumn:   d.Range.EndColumn,
					},
				},
			}},
		})
	}

	doc := sarifLog{
		Schema:  sarifSchemaURL,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "jetlint",
					InformationURI: sarifInformationURI,
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}

// uniqueRuleIDs returns the rule IDs referenced by diagnostics in
// stable lexicographic order. SARIF requires the driver to declare
// each rule it produced before referencing it from results.
func uniqueRuleIDs(diagnostics []Diagnostic) []sarifRule {
	seen := make(map[string]struct{}, len(diagnostics))
	for _, d := range diagnostics {
		seen[d.RuleID] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]sarifRule, len(ids))
	for i, id := range ids {
		out[i] = sarifRule{ID: id}
	}
	return out
}

// sarifLevel maps the linter's severity vocabulary to SARIF's
// fixed level set. SARIF distinguishes "error" / "warning" /
// "note" / "none"; we map our two values straightforwardly.
func sarifLevel(s Severity) string {
	switch s {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "note"
	}
}
