package format_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tommymorgan/tsgolint/internal/format"
)

// writeSource writes a small TypeScript source file in t.TempDir and
// returns the absolute path. Used by the human-formatter tests so the
// formatter can actually read the source and render a code frame.
func writeSource(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}

func TestHumanFormatter_HeaderShowsFileLocationAndRuleID(t *testing.T) {
	path := writeSource(t, "main.ts", "const x = 1;\nfoo();\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 2, 1, "no-floating-promises", "promise not handled")}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{path, "2:1", "no-floating-promises"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in header, got:\n%s", want, out)
		}
	}
}

func TestHumanFormatter_RendersCodeFrameAroundOffendingLine(t *testing.T) {
	src := "line one\nline two\nline three\nline four\nline five\n"
	path := writeSource(t, "main.ts", src)
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 3, 1, "rule-x", "msg")}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"line one", "line two", "line three", "line four", "line five"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected source line %q in code frame, got:\n%s", want, out)
		}
	}
	// The offending line is highlighted somehow distinct from context. The
	// concrete marker (>, arrow, etc.) is implementation detail; what matters
	// is that the frame renders the surrounding lines.
}

func TestHumanFormatter_CaretMarksOffendingRange(t *testing.T) {
	path := writeSource(t, "main.ts", "abcdefghij\n")
	var buf bytes.Buffer
	// Range spans columns 4..8 (1-indexed): characters d, e, f, g.
	d := format.Diagnostic{
		Range:    format.SourceRange{File: path, StartLine: 1, StartColumn: 4, EndLine: 1, EndColumn: 8},
		RuleID:   "rule-x",
		Severity: "error",
		Message:  "msg",
	}
	if err := (format.Human{}).Format(&buf, []format.Diagnostic{d}); err != nil {
		t.Fatal(err)
	}
	// The caret line must contain at least one ^ aligned past the leading
	// spaces. We don't check exact column alignment because the gutter
	// width is variable; what matters is that ^ characters appear.
	if !strings.Contains(buf.String(), "^") {
		t.Errorf("expected caret marker in code frame, got:\n%s", buf.String())
	}
}

func TestHumanFormatter_RendersSummaryAtEnd(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\ny;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag(path, 1, 1, "rule-x", "first"),
		diag(path, 2, 1, "rule-x", "second"),
	}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Summary line names the count. "Found N error(s)" matches biome's
	// shape; what matters is that the count is surfaced.
	if !strings.Contains(out, "2") || !strings.Contains(strings.ToLower(out), "error") {
		t.Errorf("expected summary mentioning count and 'error', got tail:\n%s", out)
	}
}

func TestHumanFormatter_EmptyDiagnosticsProducesNoOutput(t *testing.T) {
	var buf bytes.Buffer
	if err := (format.Human{}).Format(&buf, nil); err != nil {
		t.Fatal(err)
	}
	// An empty lint run is the success case; the human formatter should
	// stay silent so a clean 'tsgolint' invocation prints nothing.
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty diagnostics, got: %q", buf.String())
	}
}

func TestHumanFormatter_HandlesUnreadableSourceFileGracefully(t *testing.T) {
	var buf bytes.Buffer
	// Pointing at a path that doesn't exist must not crash; the header
	// still renders, but the code frame is omitted.
	d := []format.Diagnostic{
		diag("/no/such/file.ts", 1, 1, "rule-x", "msg"),
	}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "/no/such/file.ts") {
		t.Errorf("expected file path in header even when source unreadable, got:\n%s", buf.String())
	}
}

func TestHumanFormatter_OrdersDiagnosticsByFileThenPosition(t *testing.T) {
	a := writeSource(t, "a.ts", "alpha\nbeta\n")
	b := writeSource(t, "b.ts", "gamma\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag(b, 1, 1, "r", "later file"),
		diag(a, 2, 1, "r", "later line"),
		diag(a, 1, 1, "r", "first"),
	}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	idx := func(s string) int { return strings.Index(out, s) }
	if !(idx("first") < idx("later line") && idx("later line") < idx("later file")) {
		t.Errorf("expected ordering by file then position; got:\n%s", out)
	}
}

func TestHumanFormatter_ProducesByteIdenticalOutputForIdenticalInput(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\ny;\n")
	d := []format.Diagnostic{
		diag(path, 1, 1, "rule-x", "msg"),
		diag(path, 2, 1, "rule-y", "other"),
	}
	var first, second bytes.Buffer
	if err := (format.Human{}).Format(&first, d); err != nil {
		t.Fatal(err)
	}
	if err := (format.Human{}).Format(&second, d); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("output differs across identical runs:\n%s\nvs\n%s", first.String(), second.String())
	}
}
