package format_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jetlint/jetlint/internal/format"
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

func TestHumanFormatter_HeaderHasTrailingSeparatorBar(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	// Biome pads the header line with a heavy horizontal bar to the right
	// edge. Match that: the header line must contain at least one ━.
	if !strings.Contains(buf.String(), "━") {
		t.Errorf("expected ━ separator on header line, got:\n%s", buf.String())
	}
}

func TestHumanFormatter_ErrorBulletIsCrossMark(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	// Biome uses × (multiplication sign) for errors. Anything else
	// drifts from the visual conventions developers expect.
	if !strings.Contains(buf.String(), "×") {
		t.Errorf("expected × bullet for error severity, got:\n%s", buf.String())
	}
}

func TestHumanFormatter_WarningBulletIsExclamation(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	warn := diag(path, 1, 1, "rule-x", "msg")
	warn.Severity = "warning"
	if err := (format.Human{}).Format(&buf, []format.Diagnostic{warn}); err != nil {
		t.Fatal(err)
	}
	// Biome uses ! for warnings. The bullet must be a literal ! and not
	// e.g. ⚠ — ASCII renders cleanly across every terminal.
	out := buf.String()
	// Look for the bullet pattern: two-space indent then "! ".
	if !strings.Contains(out, "  ! ") {
		t.Errorf("expected '  ! ' bullet for warning severity, got:\n%s", out)
	}
}

func TestHumanFormatter_MultiLineRangeGetsMarkerOnEveryAffectedLine(t *testing.T) {
	src := "const x = a +\n  b +\n  c;\n"
	path := writeSource(t, "main.ts", src)
	var buf bytes.Buffer
	d := format.Diagnostic{
		Range:    format.SourceRange{File: path, StartLine: 1, StartColumn: 11, EndLine: 3, EndColumn: 4},
		RuleID:   "rule-x",
		Severity: "error",
		Message:  "spans three lines",
	}
	if err := (format.Human{}).Format(&buf, []format.Diagnostic{d}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Each line within the range carries a > marker, not just the first.
	// Three lines in the range → three > markers minimum.
	if got := strings.Count(out, "> "); got < 3 {
		t.Errorf("expected at least 3 '> ' markers for a 3-line range, got %d:\n%s", got, out)
	}
}

func TestHumanFormatter_SummaryReportsFilesCheckedAndDuration(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	h := format.Human{FilesChecked: 1682, Duration: 8 * time.Second}
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := h.Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Checked 1682 files") {
		t.Errorf("expected 'Checked 1682 files' line, got:\n%s", out)
	}
	if !strings.Contains(out, "8s") {
		t.Errorf("expected duration '8s' in summary, got:\n%s", out)
	}
}

func TestHumanFormatter_SummaryListsErrorAndWarningCountsOnSeparateLines(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\ny;\n")
	var buf bytes.Buffer
	warn := diag(path, 2, 1, "rule-y", "warn-msg")
	warn.Severity = "warning"
	d := []format.Diagnostic{
		diag(path, 1, 1, "rule-x", "err-msg"),
		warn,
	}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Found 1 error.") {
		t.Errorf("expected separate 'Found 1 error.' line, got:\n%s", out)
	}
	if !strings.Contains(out, "Found 1 warning.") {
		t.Errorf("expected separate 'Found 1 warning.' line, got:\n%s", out)
	}
}

func TestHumanFormatter_TruncatesRenderedDiagnosticsAtMaxDiagnostics(t *testing.T) {
	path := writeSource(t, "main.ts", "a;\nb;\nc;\nd;\ne;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag(path, 1, 1, "rule", "first"),
		diag(path, 2, 1, "rule", "second"),
		diag(path, 3, 1, "rule", "third"),
		diag(path, 4, 1, "rule", "fourth"),
		diag(path, 5, 1, "rule", "fifth"),
	}
	if err := (format.Human{MaxDiagnostics: 2}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Only the first two messages should appear in code-frame blocks.
	for _, want := range []string{"first", "second"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in truncated output, got:\n%s", want, out)
		}
	}
	for _, banned := range []string{"third", "fourth", "fifth"} {
		if strings.Contains(out, banned) {
			t.Errorf("expected truncated output to omit %q, got:\n%s", banned, out)
		}
	}
}

func TestHumanFormatter_ReportsCountOfTruncatedDiagnostics(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := make([]format.Diagnostic, 0, 5)
	for i := 0; i < 5; i++ {
		d = append(d, diag(path, 1, 1, "rule", "msg"))
	}
	if err := (format.Human{MaxDiagnostics: 2}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Biome surfaces a "Diagnostics not shown: N." line plus a hint
	// pointing at the override flag. Match the count and the flag name.
	if !strings.Contains(out, "Diagnostics not shown: 3") {
		t.Errorf("expected 'Diagnostics not shown: 3' line, got:\n%s", out)
	}
	if !strings.Contains(out, "--max-diagnostics") {
		t.Errorf("expected hint mentioning --max-diagnostics, got:\n%s", out)
	}
}

func TestHumanFormatter_SummaryReflectsTotalNotTruncatedCount(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := make([]format.Diagnostic, 0, 5)
	for i := 0; i < 5; i++ {
		d = append(d, diag(path, 1, 1, "rule", "msg"))
	}
	if err := (format.Human{MaxDiagnostics: 2}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	// All five errors must show up in the summary even though only two
	// were rendered with code frames.
	if !strings.Contains(buf.String(), "Found 5 errors.") {
		t.Errorf("expected summary to count all 5 errors, got:\n%s", buf.String())
	}
}

func TestHumanFormatter_MaxZeroDisablesTruncation(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag(path, 1, 1, "rule", "first"),
		diag(path, 1, 1, "rule", "second"),
		diag(path, 1, 1, "rule", "third"),
	}
	if err := (format.Human{MaxDiagnostics: 0}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"first", "second", "third"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q present when MaxDiagnostics=0, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not shown") {
		t.Errorf("expected no truncation notice when MaxDiagnostics=0, got:\n%s", out)
	}
}

func TestHumanFormatter_MaxAboveCountIsNotTruncated(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag(path, 1, 1, "rule", "only"),
	}
	if err := (format.Human{MaxDiagnostics: 100}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "not shown") {
		t.Errorf("expected no truncation notice when below max, got:\n%s", buf.String())
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
	// stay silent so a clean 'jetlint' invocation prints nothing.
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

func TestHumanFormatter_DefaultIsPlaintextWhenWriterIsNotATerminal(t *testing.T) {
	// A bytes.Buffer is never a TTY, so auto-detection must yield
	// plaintext. CI logs and tests rely on this.
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := (format.Human{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("expected no ANSI escape in default output to a non-TTY, got: %q", buf.String())
	}
}

func TestHumanFormatter_ColorAlwaysProducesANSIEscapes(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := (format.Human{Color: format.ColorAlways}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("expected ANSI escapes when Color=Always, got: %q", buf.String())
	}
}

func TestHumanFormatter_ColorNeverSuppressesEscapesEvenWhenForced(t *testing.T) {
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := (format.Human{Color: format.ColorNever}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("expected no ANSI when Color=Never, got: %q", buf.String())
	}
}

func TestHumanFormatter_NoColorEnvOverridesColorAlways(t *testing.T) {
	// Per the NO_COLOR convention (https://no-color.org), setting
	// NO_COLOR must suppress color output regardless of any explicit
	// "always" preference.
	t.Setenv("NO_COLOR", "1")
	path := writeSource(t, "main.ts", "x;\n")
	var buf bytes.Buffer
	d := []format.Diagnostic{diag(path, 1, 1, "rule-x", "msg")}
	if err := (format.Human{Color: format.ColorAlways}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("expected NO_COLOR to suppress ANSI, got: %q", buf.String())
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
