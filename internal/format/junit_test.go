package format_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/jetlint/jetlint/internal/format"
)

func TestJUnitFormatter_EmitsValidXMLWithTestsuitesRoot(t *testing.T) {
	var buf bytes.Buffer
	if err := (format.JUnit{}).Format(&buf, nil); err != nil {
		t.Fatalf("Format: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "<?xml") {
		t.Errorf("expected XML declaration, got: %s", out)
	}
	// Must parse as XML and have a testsuites root.
	dec := xml.NewDecoder(bytes.NewReader(buf.Bytes()))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if start, ok := tok.(xml.StartElement); ok {
			if start.Name.Local != "testsuites" {
				t.Errorf("expected root <testsuites>, got <%s>", start.Name.Local)
			}
			return
		}
	}
}

func TestJUnitFormatter_EachDiagnosticBecomesATestcaseWithFailure(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 1, 5, "no-floating-promises", "first"),
		diag("a.ts", 2, 7, "no-base-to-string", "second"),
	}
	if err := (format.JUnit{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Two testcases for two diagnostics.
	if got := strings.Count(out, "<testcase "); got != 2 {
		t.Errorf("expected 2 <testcase> elements, got %d:\n%s", got, out)
	}
	// Each testcase carries a <failure>.
	if got := strings.Count(out, "<failure "); got != 2 {
		t.Errorf("expected 2 <failure> elements, got %d:\n%s", got, out)
	}
	// Rule id surfaces somewhere addressable (classname is the JUnit
	// convention for "which check produced this").
	for _, ruleID := range []string{"no-floating-promises", "no-base-to-string"} {
		if !strings.Contains(out, ruleID) {
			t.Errorf("expected rule id %q in output, got:\n%s", ruleID, out)
		}
	}
}

func TestJUnitFormatter_TestsuiteAttributesReflectCounts(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "msg"),
		diag("b.ts", 1, 1, "rule-y", "msg"),
		diag("c.ts", 1, 1, "rule-z", "msg"),
	}
	if err := (format.JUnit{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`tests="3"`, `failures="3"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %s in output, got: %s", want, buf.String())
		}
	}
}

func TestJUnitFormatter_OrdersByFileThenPosition(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("b.ts", 1, 1, "r", "later file"),
		diag("a.ts", 5, 1, "r", "later line"),
		diag("a.ts", 1, 1, "r", "first"),
	}
	if err := (format.JUnit{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	idx := func(s string) int { return strings.Index(out, s) }
	if !(idx("first") < idx("later line") && idx("later line") < idx("later file")) {
		t.Errorf("expected stable ordering by file then position; got:\n%s", out)
	}
}

func TestJUnitFormatter_ProducesByteIdenticalOutputForIdenticalInput(t *testing.T) {
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "msg"),
		diag("b.ts", 2, 2, "rule-y", "other"),
	}
	var first, second bytes.Buffer
	if err := (format.JUnit{}).Format(&first, d); err != nil {
		t.Fatal(err)
	}
	if err := (format.JUnit{}).Format(&second, d); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("output differs across identical runs")
	}
}

func TestJUnitFormatter_EscapesXMLSpecialCharactersInMessage(t *testing.T) {
	var buf bytes.Buffer
	d := diag("a.ts", 1, 1, "rule-x", `bad chars: < > & "quote" 'apos'`)
	if err := (format.JUnit{}).Format(&buf, []format.Diagnostic{d}); err != nil {
		t.Fatal(err)
	}
	// The literal `<`, `>`, `&` must be encoded; otherwise CI dashboards
	// will reject the document or display garbled content.
	if strings.Contains(buf.String(), "< >") {
		t.Errorf("expected XML special characters to be encoded, got raw: %s", buf.String())
	}
	// Document must still parse.
	if err := xml.Unmarshal(buf.Bytes(), new(any)); err != nil {
		t.Errorf("output does not parse as XML: %v\n%s", err, buf.String())
	}
}

func TestLookup_ReturnsJUnitFormatter(t *testing.T) {
	f, err := format.Lookup("junit")
	if err != nil {
		t.Fatalf("Lookup(junit): %v", err)
	}
	if f.Name() != "junit" {
		t.Errorf("expected name 'junit', got %q", f.Name())
	}
}
