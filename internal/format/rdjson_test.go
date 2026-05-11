package format_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jetlint/jetlint/internal/format"
)

func TestRDJSONFormatter_EmitsSourceAndDiagnosticsArray(t *testing.T) {
	var buf bytes.Buffer
	if err := (format.RDJSON{}).Format(&buf, nil); err != nil {
		t.Fatalf("Format: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}
	src, ok := doc["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object, got: %v", doc["source"])
	}
	if src["name"] != "jetlint" {
		t.Errorf("expected source.name 'jetlint', got %v", src["name"])
	}
	if _, ok := doc["diagnostics"].([]any); !ok {
		t.Errorf("expected diagnostics array, got: %v", doc["diagnostics"])
	}
}

func TestRDJSONFormatter_DiagnosticIncludesAllRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("path/to/file.ts", 7, 13, "no-floating-promises", "promise not awaited"),
	}
	if err := (format.RDJSON{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Diagnostics []map[string]any
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}
	if len(doc.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(doc.Diagnostics))
	}
	got := doc.Diagnostics[0]
	if got["message"] != "promise not awaited" {
		t.Errorf("expected message verbatim, got %v", got["message"])
	}
	if got["severity"] != "ERROR" {
		t.Errorf("expected severity 'ERROR' (rdjson uses uppercase), got %v", got["severity"])
	}
	loc, ok := got["location"].(map[string]any)
	if !ok {
		t.Fatalf("expected location object, got: %v", got["location"])
	}
	if loc["path"] != "path/to/file.ts" {
		t.Errorf("expected location.path verbatim, got %v", loc["path"])
	}
	rng, ok := loc["range"].(map[string]any)
	if !ok {
		t.Fatalf("expected location.range object, got: %v", loc["range"])
	}
	start, _ := rng["start"].(map[string]any)
	if start == nil || start["line"] == nil || start["column"] == nil {
		t.Errorf("expected location.range.start.{line,column}, got: %v", rng)
	}
	code, ok := got["code"].(map[string]any)
	if !ok {
		t.Fatalf("expected code object, got: %v", got["code"])
	}
	if code["value"] != "no-floating-promises" {
		t.Errorf("expected code.value to be the rule id, got %v", code["value"])
	}
}

func TestRDJSONFormatter_OrdersDiagnosticsByFileThenPosition(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("b.ts", 1, 1, "r", "later file"),
		diag("a.ts", 5, 1, "r", "later line"),
		diag("a.ts", 1, 1, "r", "first"),
	}
	if err := (format.RDJSON{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	idx := func(s string) int { return strings.Index(out, s) }
	if !(idx("first") < idx("later line") && idx("later line") < idx("later file")) {
		t.Errorf("expected stable ordering by file then position; got:\n%s", out)
	}
}

func TestRDJSONFormatter_ProducesByteIdenticalOutputForIdenticalInput(t *testing.T) {
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "msg"),
		diag("b.ts", 2, 2, "rule-y", "other"),
	}
	var first, second bytes.Buffer
	if err := (format.RDJSON{}).Format(&first, d); err != nil {
		t.Fatal(err)
	}
	if err := (format.RDJSON{}).Format(&second, d); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("output differs across identical runs")
	}
}

func TestLookup_ReturnsRDJSONFormatter(t *testing.T) {
	f, err := format.Lookup("rdjson")
	if err != nil {
		t.Fatalf("Lookup(rdjson): %v", err)
	}
	if f.Name() != "rdjson" {
		t.Errorf("expected name 'rdjson', got %q", f.Name())
	}
}
