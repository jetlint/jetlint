package format_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tommymorgan/tsgolint/internal/format"
)

func TestGitHubActionsFormatter_OneCommandPerDiagnostic(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("a.ts", 1, 5, "no-floating-promises", "an error"),
		diag("a.ts", 2, 7, "no-base-to-string", "another"),
	}
	if err := (format.GitHubActions{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), buf.String())
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "::error ") {
			t.Errorf("expected ::error prefix, got: %s", line)
		}
	}
}

func TestGitHubActionsFormatter_IncludesAllRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("path/to/file.ts", 7, 13, "no-floating-promises", "promise not awaited"),
	}
	if err := (format.GitHubActions{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"::error ",
		"file=path/to/file.ts",
		"line=7",
		"col=13",
		"endLine=",
		"endColumn=",
		"title=no-floating-promises",
		"::promise not awaited",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}

func TestGitHubActionsFormatter_LevelTracksSeverity(t *testing.T) {
	var buf bytes.Buffer
	warning := diag("a.ts", 1, 1, "rule-x", "msg")
	warning.Severity = "warning"
	if err := (format.GitHubActions{}).Format(&buf, []format.Diagnostic{warning}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "::warning ") {
		t.Errorf("expected ::warning prefix for warning severity, got: %s", buf.String())
	}
}

func TestGitHubActionsFormatter_EscapesNewlinesAndPercentInMessage(t *testing.T) {
	var buf bytes.Buffer
	d := diag("a.ts", 1, 1, "rule-x", "first line\nsecond 100% done")
	if err := (format.GitHubActions{}).Format(&buf, []format.Diagnostic{d}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// A literal newline inside the command body would terminate the
	// command early; it must be encoded as %0A. Likewise, a literal
	// percent must become %25 so the value isn't ambiguous.
	if strings.Contains(out, "first line\nsecond") {
		t.Errorf("expected literal newline to be escaped, got: %q", out)
	}
	if !strings.Contains(out, "%0A") {
		t.Errorf("expected %%0A escape for newline, got: %q", out)
	}
	if !strings.Contains(out, "100%25 done") {
		t.Errorf("expected %% to become %%25, got: %q", out)
	}
	// The output should still be exactly one line (one workflow command).
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected single-line workflow command, got %d newlines: %q", strings.Count(out, "\n"), out)
	}
}

func TestGitHubActionsFormatter_OrdersDiagnosticsByFileThenPosition(t *testing.T) {
	var buf bytes.Buffer
	d := []format.Diagnostic{
		diag("b.ts", 1, 1, "r", "later file"),
		diag("a.ts", 5, 1, "r", "later line"),
		diag("a.ts", 1, 1, "r", "first"),
	}
	if err := (format.GitHubActions{}).Format(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	idx := func(s string) int { return strings.Index(out, s) }
	if !(idx("first") < idx("later line") && idx("later line") < idx("later file")) {
		t.Errorf("expected stable ordering by file then position; got:\n%s", out)
	}
}

func TestGitHubActionsFormatter_ProducesByteIdenticalOutputForIdenticalInput(t *testing.T) {
	d := []format.Diagnostic{
		diag("a.ts", 1, 1, "rule-x", "msg"),
		diag("b.ts", 2, 2, "rule-y", "other"),
	}
	var first, second bytes.Buffer
	if err := (format.GitHubActions{}).Format(&first, d); err != nil {
		t.Fatal(err)
	}
	if err := (format.GitHubActions{}).Format(&second, d); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Errorf("output differs across identical runs:\n%s\nvs\n%s", first.String(), second.String())
	}
}

func TestLookup_ReturnsGitHubActionsFormatter(t *testing.T) {
	f, err := format.Lookup("github")
	if err != nil {
		t.Fatalf("Lookup(github): %v", err)
	}
	if f.Name() != "github" {
		t.Errorf("expected name 'github', got %q", f.Name())
	}
}
