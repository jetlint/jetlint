package format

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Human is the default formatter. It renders one biome-style block per
// diagnostic — a header naming the location and rule, a code frame
// showing two lines of context on either side of the offending source,
// a caret line marking the offending column range, and the message —
// followed by a summary line that reports the total count.
type Human struct{}

// Name returns "human".
func (Human) Name() string { return "human" }

// codeFrameContext is the number of source lines shown above and below
// each offending line.
const codeFrameContext = 2

// Format writes a biome-style human-readable report. An empty
// diagnostic slice produces no output so a clean lint invocation
// stays silent.
func (Human) Format(w io.Writer, diagnostics []Diagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	SortDiagnostics(diagnostics)

	// Cache file source lines so we read each file at most once even
	// when many diagnostics cluster within the same file.
	srcCache := map[string][]string{}

	errCount, warnCount := 0, 0
	for i, d := range diagnostics {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if err := writeBlock(w, d, srcCache); err != nil {
			return err
		}
		switch d.Severity {
		case "error":
			errCount++
		case "warning":
			warnCount++
		}
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", summary(errCount, warnCount)); err != nil {
		return err
	}
	return nil
}

// writeBlock renders the header, code frame, caret, and message for a
// single diagnostic. The order matches biome's: header, blank, ✖ message,
// blank, code frame.
func writeBlock(w io.Writer, d Diagnostic, srcCache map[string][]string) error {
	header := fmt.Sprintf("%s:%d:%d %s\n", d.Range.File, d.Range.StartLine, d.Range.StartColumn, d.RuleID)
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	bullet := bulletFor(d.Severity)
	if _, err := fmt.Fprintf(w, "\n  %s %s\n\n", bullet, d.Message); err != nil {
		return err
	}
	if err := writeCodeFrame(w, d, srcCache); err != nil {
		return err
	}
	return nil
}

// writeCodeFrame renders the source context around the offending line.
// When the source file cannot be read, the frame is silently omitted —
// the header and message above already convey the diagnostic.
func writeCodeFrame(w io.Writer, d Diagnostic, srcCache map[string][]string) error {
	lines, ok := readSourceLines(d.Range.File, srcCache)
	if !ok {
		return nil
	}
	startLine := d.Range.StartLine - codeFrameContext
	if startLine < 1 {
		startLine = 1
	}
	endLine := d.Range.StartLine + codeFrameContext
	if endLine > len(lines) {
		endLine = len(lines)
	}
	gutterWidth := numberWidth(endLine)

	for ln := startLine; ln <= endLine; ln++ {
		marker := " "
		if ln == d.Range.StartLine {
			marker = ">"
		}
		if _, err := fmt.Fprintf(w, "  %s %*d │ %s\n", marker, gutterWidth, ln, lines[ln-1]); err != nil {
			return err
		}
		if ln == d.Range.StartLine {
			caret := caretLine(d.Range.StartColumn, d.Range.EndLine, d.Range.EndLine == d.Range.StartLine, d.Range.EndColumn)
			if _, err := fmt.Fprintf(w, "  %s %*s │ %s\n", " ", gutterWidth, "", caret); err != nil {
				return err
			}
		}
	}
	return nil
}

// readSourceLines returns the file's lines, caching across calls. The
// boolean reports whether the read succeeded; failures collapse the
// rest of the frame to nothing rather than aborting the report.
func readSourceLines(path string, cache map[string][]string) ([]string, bool) {
	if cached, ok := cache[path]; ok {
		return cached, len(cached) > 0
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		cache[path] = nil
		return nil, false
	}
	lines := strings.Split(strings.TrimRight(string(contents), "\n"), "\n")
	cache[path] = lines
	return lines, true
}

// caretLine returns the underline for the offending range. For a
// single-line range it underlines the columns explicitly; for a
// multi-line range only the start position is marked, since rendering
// the full range across multiple lines would clutter the frame.
func caretLine(startCol int, endLine int, sameLine bool, endCol int) string {
	pad := strings.Repeat(" ", maxInt(startCol-1, 0))
	if !sameLine {
		return pad + "^"
	}
	width := endCol - startCol
	if width < 1 {
		width = 1
	}
	return pad + strings.Repeat("^", width)
}

func bulletFor(s Severity) string {
	switch s {
	case "warning":
		return "⚠"
	default:
		return "✖"
	}
}

func summary(errors, warnings int) string {
	switch {
	case errors == 0 && warnings == 0:
		return "No errors found."
	case errors > 0 && warnings == 0:
		return fmt.Sprintf("Found %d %s.", errors, plural(errors, "error", "errors"))
	case errors == 0 && warnings > 0:
		return fmt.Sprintf("Found %d %s.", warnings, plural(warnings, "warning", "warnings"))
	default:
		return fmt.Sprintf("Found %d %s and %d %s.",
			errors, plural(errors, "error", "errors"),
			warnings, plural(warnings, "warning", "warnings"))
	}
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func numberWidth(n int) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	return w
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
