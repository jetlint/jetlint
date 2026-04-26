package format_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/format"
)

func diag(file string, line, col int, ruleID, msg string) format.Diagnostic {
	return wrapperlint.Diagnostic{
		Range:    wrapperlint.SourceRange{File: file, StartLine: line, StartColumn: col, EndLine: line, EndColumn: col + 1},
		RuleID:   ruleID,
		Severity: wrapperlint.SeverityError,
		Message:  msg,
	}
}

func TestLookup_ReturnsHumanFormatter(t *testing.T) {
	f, err := format.Lookup("human")
	if err != nil {
		t.Fatalf("Lookup(human): %v", err)
	}
	if f.Name() != "human" {
		t.Errorf("expected name 'human', got %q", f.Name())
	}
}

func TestLookup_ReturnsJSONFormatter(t *testing.T) {
	f, err := format.Lookup("json")
	if err != nil {
		t.Fatalf("Lookup(json): %v", err)
	}
	if f.Name() != "json" {
		t.Errorf("expected name 'json', got %q", f.Name())
	}
}

func TestLookup_UnknownFormatReturnsErrorListingSupported(t *testing.T) {
	_, err := format.Lookup("yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "human") || !strings.Contains(err.Error(), "json") {
		t.Errorf("expected error to list supported formats, got: %v", err)
	}
}

func TestJSONFormatter_EmitsSchemaVersionAndAllRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 2, 3, "rule-x", "msg"),
	}
	if err := (format.JSON{}).Format(&buf, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
	var doc struct {
		SchemaVersion int `json:"schemaVersion"`
		Diagnostics   []map[string]any
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\noutput:\n%s", err, buf.String())
	}
	if doc.SchemaVersion != format.JSONSchemaVersion {
		t.Errorf("expected schemaVersion %d, got %d", format.JSONSchemaVersion, doc.SchemaVersion)
	}
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(doc.Diagnostics))
	}
	for _, k := range []string{"file", "startLine", "startColumn", "endLine", "endColumn", "ruleId", "severity", "message"} {
		if _, ok := doc.Diagnostics[0][k]; !ok {
			t.Errorf("expected key %q in diagnostic, got: %v", k, doc.Diagnostics[0])
		}
	}
}

func TestJSONFormatter_OrdersDiagnosticsByFileThenPosition(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("b.ts", 1, 1, "rule-y", "later file"),
		diag("a.ts", 5, 1, "rule-x", "later line"),
		diag("a.ts", 1, 1, "rule-w", "first"),
	}
	if err := (format.JSON{}).Format(&buf, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
	got := buf.String()
	idx := func(s string) int { return strings.Index(got, s) }
	if !(idx("first") < idx("later line") && idx("later line") < idx("later file")) {
		t.Errorf("expected ordering first, later line, later file; got:\n%s", got)
	}
}

func TestJSONFormatter_ProducesByteIdenticalOutputForIdenticalInput(t *testing.T) {
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "msg"),
		diag("b.ts", 2, 2, "rule-y", "other"),
	}
	var first, second bytes.Buffer
	if err := (format.JSON{}).Format(&first, d); err != nil {
		t.Fatal(err)
	}
	if err := (format.JSON{}).Format(&second, d); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("output differs across identical runs:\nfirst:  %s\nsecond: %s", first.String(), second.String())
	}
}

func TestJSONFormatter_EmptyDiagnosticsListIsStillValid(t *testing.T) {
	var buf bytes.Buffer
	if err := (format.JSON{}).Format(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"diagnostics":[]`) {
		t.Errorf("expected empty array, got: %s", buf.String())
	}
}
