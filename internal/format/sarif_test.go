package format_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/jetlint/jetlint/internal/format"
)

func TestSARIFFormatter_EmitsValidSchemaAndVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := (format.SARIF{}).Format(&buf, nil); err != nil {
		t.Fatalf("Format: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}
	if doc["$schema"] == nil || doc["$schema"] == "" {
		t.Errorf("expected $schema, got: %v", doc["$schema"])
	}
	if doc["version"] != "2.1.0" {
		t.Errorf("expected version 2.1.0, got: %v", doc["version"])
	}
	runs, ok := doc["runs"].([]any)
	if !ok || len(runs) != 1 {
		t.Fatalf("expected exactly one run, got: %v", doc["runs"])
	}
}

func TestSARIFFormatter_DriverDeclaresRulesActuallyReferenced(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "no-floating-promises", "msg-a"),
		diag("b.ts", 1, 1, "no-floating-promises", "msg-b"),
		diag("c.ts", 1, 1, "no-base-to-string", "msg-c"),
	}
	if err := (format.SARIF{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name  string
					Rules []struct{ ID string }
				}
			}
			Results []struct {
				RuleID string `json:"ruleId"`
			}
		}
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}
	driver := doc.Runs[0].Tool.Driver
	if driver.Name == "" {
		t.Errorf("expected non-empty driver name")
	}
	if len(driver.Rules) != 2 {
		t.Errorf("expected exactly 2 unique rules in driver, got %d: %v", len(driver.Rules), driver.Rules)
	}
	// Every result's ruleId must appear in the driver's rules list — that's
	// the SARIF requirement we declared rules for in the first place.
	declared := map[string]bool{}
	for _, r := range driver.Rules {
		declared[r.ID] = true
	}
	for _, r := range doc.Runs[0].Results {
		if !declared[r.RuleID] {
			t.Errorf("result rule %q not declared in driver.rules", r.RuleID)
		}
	}
}

func TestSARIFFormatter_LevelsMapToSarifVocabulary(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "an error"),
	}
	if err := (format.SARIF{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	if !bytesContainsString(buf.Bytes(), `"level":"error"`) {
		t.Errorf("expected level 'error' in output, got: %s", buf.String())
	}
}

func TestSARIFFormatter_RegionUsesOneIndexedLineAndColumn(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 7, 13, "rule-x", "msg"),
	}
	if err := (format.SARIF{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"startLine":7`, `"startColumn":13`} {
		if !bytesContainsString(buf.Bytes(), want) {
			t.Errorf("expected %s in output, got: %s", want, buf.String())
		}
	}
}

func TestSARIFFormatter_ProducesByteIdenticalOutputForIdenticalInput(t *testing.T) {
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "msg"),
		diag("b.ts", 2, 2, "rule-y", "other"),
	}
	var first, second bytes.Buffer
	if err := (format.SARIF{}).Format(&first, d); err != nil {
		t.Fatal(err)
	}
	if err := (format.SARIF{}).Format(&second, d); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("SARIF output differs across identical runs")
	}
}

func TestSARIFFormatter_OrdersResultsByFileThenPosition(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("b.ts", 1, 1, "r", "later file"),
		diag("a.ts", 5, 1, "r", "later line"),
		diag("a.ts", 1, 1, "r", "first"),
	}
	if err := (format.SARIF{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	idx := func(s string) int { return bytesIndex([]byte(out), []byte(s)) }
	if !(idx("first") < idx("later line") && idx("later line") < idx("later file")) {
		t.Errorf("expected stable ordering by file then position; got:\n%s", out)
	}
}

func TestLookup_ReturnsSARIFFormatter(t *testing.T) {
	f, err := format.Lookup("sarif")
	if err != nil {
		t.Fatalf("Lookup(sarif): %v", err)
	}
	if f.Name() != "sarif" {
		t.Errorf("expected name 'sarif', got %q", f.Name())
	}
}

// bytesContainsString and bytesIndex avoid importing "strings" again
// in test files where it isn't otherwise needed.
func bytesContainsString(b []byte, s string) bool { return bytesIndex(b, []byte(s)) >= 0 }

func bytesIndex(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
