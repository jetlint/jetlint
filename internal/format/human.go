package format

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ColorMode controls whether the human formatter emits ANSI color
// escapes. ColorAuto detects the writer's TTY status; ColorAlways and
// ColorNever override that decision. NO_COLOR (per https://no-color.org)
// always wins regardless of mode.
type ColorMode int

const (
	// ColorAuto is the zero value: emit color iff the writer is a TTY.
	ColorAuto ColorMode = iota
	// ColorAlways forces ANSI escapes (ignored when NO_COLOR is set).
	ColorAlways
	// ColorNever suppresses ANSI escapes even on a TTY.
	ColorNever
)

// Human is the default formatter. It renders one biome-style block per
// diagnostic — a header naming the location and rule, a code frame
// showing two lines of context on either side of the offending source,
// a caret line marking the offending column range, and the message —
// followed by a summary line that reports the total count.
type Human struct {
	// Color selects whether ANSI escapes are emitted. The zero value
	// (ColorAuto) auto-detects from the writer.
	Color ColorMode
}

// Name returns "human".
func (Human) Name() string { return "human" }

// codeFrameContext is the number of source lines shown above and below
// each offending line.
const codeFrameContext = 2

// ANSI escape sequences. Held as constants so the renderer can decide
// at the start of Format whether to emit them or substitute empty
// strings.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// Format writes a biome-style human-readable report. An empty
// diagnostic slice produces no output so a clean lint invocation
// stays silent.
func (h Human) Format(w io.Writer, diagnostics []Diagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	SortDiagnostics(diagnostics)

	useColor := shouldUseColor(h.Color, w)
	c := paletteFor(useColor)

	srcCache := map[string][]string{}

	errCount, warnCount := 0, 0
	for i, d := range diagnostics {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if err := writeBlock(w, c, d, srcCache); err != nil {
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
	if _, err := fmt.Fprintf(w, "%s\n", c.summary(errCount, warnCount)); err != nil {
		return err
	}
	return nil
}

// shouldUseColor resolves the formatter's color mode against the
// writer's terminal capabilities and the NO_COLOR convention.
func shouldUseColor(mode ColorMode, w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is an *os.File backed by a terminal.
// We avoid the golang.org/x/term dependency; checking
// os.ModeCharDevice on the file's Stat is portable across Unix and
// Windows for the cases we care about.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// palette holds the ANSI escapes (or empty strings) used to wrap
// output fragments. Picking the active palette once per Format call
// removes branching from the hot path.
type palette struct {
	reset, bold, dim, red, yellow, cyan string
}

func paletteFor(use bool) palette {
	if !use {
		return palette{}
	}
	return palette{
		reset:  ansiReset,
		bold:   ansiBold,
		dim:    ansiDim,
		red:    ansiRed,
		yellow: ansiYellow,
		cyan:   ansiCyan,
	}
}

func (p palette) wrap(prefix, s string) string {
	if prefix == "" {
		return s
	}
	return prefix + s + p.reset
}

func (p palette) severityColor(s Severity) string {
	switch s {
	case "warning":
		return p.yellow
	default:
		return p.red
	}
}

func (p palette) bullet(s Severity) string {
	switch s {
	case "warning":
		return p.wrap(p.yellow, "⚠")
	default:
		return p.wrap(p.red, "✖")
	}
}

func (p palette) summary(errors, warnings int) string {
	switch {
	case errors == 0 && warnings == 0:
		return "No errors found."
	case errors > 0 && warnings == 0:
		return p.wrap(p.red, fmt.Sprintf("Found %d %s.", errors, plural(errors, "error", "errors")))
	case errors == 0 && warnings > 0:
		return p.wrap(p.yellow, fmt.Sprintf("Found %d %s.", warnings, plural(warnings, "warning", "warnings")))
	default:
		return p.wrap(p.red, fmt.Sprintf("Found %d %s and %d %s.",
			errors, plural(errors, "error", "errors"),
			warnings, plural(warnings, "warning", "warnings")))
	}
}

// writeBlock renders the header, code frame, caret, and message for a
// single diagnostic. The order matches biome's: header, blank, ✖ message,
// blank, code frame.
func writeBlock(w io.Writer, c palette, d Diagnostic, srcCache map[string][]string) error {
	location := fmt.Sprintf("%s:%d:%d", d.Range.File, d.Range.StartLine, d.Range.StartColumn)
	header := fmt.Sprintf("%s %s\n",
		c.wrap(c.cyan, location),
		c.wrap(c.dim, d.RuleID),
	)
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n  %s %s\n\n", c.bullet(d.Severity), d.Message); err != nil {
		return err
	}
	if err := writeCodeFrame(w, c, d, srcCache); err != nil {
		return err
	}
	return nil
}

// writeCodeFrame renders the source context around the offending line.
// When the source file cannot be read, the frame is silently omitted —
// the header and message above already convey the diagnostic.
func writeCodeFrame(w io.Writer, c palette, d Diagnostic, srcCache map[string][]string) error {
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
	severityColor := c.severityColor(d.Severity)
	pipe := c.wrap(c.dim, "│")

	for ln := startLine; ln <= endLine; ln++ {
		isOffending := ln == d.Range.StartLine
		marker := " "
		if isOffending {
			marker = c.wrap(severityColor, ">")
		}
		number := c.wrap(c.dim, fmt.Sprintf("%*d", gutterWidth, ln))
		if _, err := fmt.Fprintf(w, "  %s %s %s %s\n", marker, number, pipe, lines[ln-1]); err != nil {
			return err
		}
		if isOffending {
			caret := caretLine(d.Range.StartColumn, d.Range.EndLine == d.Range.StartLine, d.Range.EndColumn)
			if _, err := fmt.Fprintf(w, "    %*s %s %s\n",
				gutterWidth, "", pipe, c.wrap(severityColor, caret)); err != nil {
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
func caretLine(startCol int, sameLine bool, endCol int) string {
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
