package format

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
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
// diagnostic — a header naming the location and rule plus a heavy
// horizontal-bar separator, a code frame showing two lines of context
// on either side of the offending source with > markers on every
// affected line, a caret line marking the offending column range, and
// the message — followed by a summary block reporting the number of
// files checked, the elapsed duration, and per-severity counts.
type Human struct {
	// Color selects whether ANSI escapes are emitted. The zero value
	// (ColorAuto) auto-detects from the writer.
	Color ColorMode

	// FilesChecked is the count of source files the engine processed.
	// When zero (the default), the summary omits the "Checked N files"
	// line so unit tests that don't supply stats stay terse.
	FilesChecked int

	// Duration is the wall-clock time the lint pass took. When zero,
	// it is omitted from the summary.
	Duration time.Duration
}

// Name returns "human".
func (Human) Name() string { return "human" }

const (
	// codeFrameContext is the number of source lines shown above and
	// below each offending line.
	codeFrameContext = 2

	// headerWidth is the target column width for the heavy-bar line
	// that pads the header. We hardcode rather than detect terminal
	// width so output is byte-identical across environments.
	headerWidth = 100
)

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
// diagnostic slice with no stats produces no output so a clean lint
// invocation stays silent. When stats are populated, the summary
// block is rendered even if there are no findings.
func (h Human) Format(w io.Writer, diagnostics []Diagnostic) error {
	if len(diagnostics) == 0 && h.FilesChecked == 0 {
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
	if len(diagnostics) > 0 {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return writeSummary(w, c, h.FilesChecked, h.Duration, errCount, warnCount)
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

// bullet returns the single-character marker for a severity. Biome's
// convention: × for errors, ! for warnings. ASCII-clean so terminals
// without good Unicode support still render correctly.
func (p palette) bullet(s Severity) string {
	switch s {
	case "warning":
		return p.wrap(p.yellow, "!")
	default:
		return p.wrap(p.red, "×")
	}
}

// writeBlock renders the header, code frame, caret, and message for a
// single diagnostic. The order matches biome's: header (with trailing
// bar), blank, × message, blank, code frame.
func writeBlock(w io.Writer, c palette, d Diagnostic, srcCache map[string][]string) error {
	location := fmt.Sprintf("%s:%d:%d", d.Range.File, d.Range.StartLine, d.Range.StartColumn)
	prefixVisible := len(location) + 1 + len(d.RuleID) + 1
	bar := ""
	if prefixVisible < headerWidth {
		bar = strings.Repeat("━", headerWidth-prefixVisible)
	}
	header := fmt.Sprintf("%s %s %s\n",
		c.wrap(c.cyan, location),
		c.wrap(c.dim, d.RuleID),
		c.wrap(c.dim, bar),
	)
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n  %s %s\n\n", c.bullet(d.Severity), d.Message); err != nil {
		return err
	}
	return writeCodeFrame(w, c, d, srcCache)
}

// writeCodeFrame renders the source context around the offending
// range. Multi-line ranges receive a > marker on every line within
// the range; the caret underline is rendered for the first and last
// affected lines (matching biome's behavior). When the source file
// cannot be read, the frame is silently omitted.
func writeCodeFrame(w io.Writer, c palette, d Diagnostic, srcCache map[string][]string) error {
	lines, ok := readSourceLines(d.Range.File, srcCache)
	if !ok {
		return nil
	}
	startLine := d.Range.StartLine - codeFrameContext
	if startLine < 1 {
		startLine = 1
	}
	endLine := d.Range.EndLine + codeFrameContext
	if endLine > len(lines) {
		endLine = len(lines)
	}
	gutterWidth := numberWidth(endLine)
	severityColor := c.severityColor(d.Severity)
	pipe := c.wrap(c.dim, "│")

	for ln := startLine; ln <= endLine; ln++ {
		inRange := ln >= d.Range.StartLine && ln <= d.Range.EndLine
		marker := " "
		if inRange {
			marker = c.wrap(severityColor, ">")
		}
		number := c.wrap(c.dim, fmt.Sprintf("%*d", gutterWidth, ln))
		if _, err := fmt.Fprintf(w, "  %s %s %s %s\n", marker, number, pipe, lines[ln-1]); err != nil {
			return err
		}
		if inRange && (ln == d.Range.StartLine || ln == d.Range.EndLine) {
			caret := caretFor(d.Range, ln, lines[ln-1])
			if _, err := fmt.Fprintf(w, "    %*s %s %s\n",
				gutterWidth, "", pipe, c.wrap(severityColor, caret)); err != nil {
				return err
			}
		}
	}
	return nil
}

// caretFor returns the underline for line ln within the diagnostic's
// range. The first line is underlined from StartColumn to the line's
// end; the last line is underlined from column 1 to EndColumn; a
// single-line range is underlined from StartColumn to EndColumn.
func caretFor(r SourceRange, ln int, source string) string {
	switch {
	case r.StartLine == r.EndLine:
		pad := strings.Repeat(" ", maxInt(r.StartColumn-1, 0))
		width := r.EndColumn - r.StartColumn
		if width < 1 {
			width = 1
		}
		return pad + strings.Repeat("^", width)
	case ln == r.StartLine:
		pad := strings.Repeat(" ", maxInt(r.StartColumn-1, 0))
		width := maxInt(len(source)-(r.StartColumn-1), 1)
		return pad + strings.Repeat("^", width)
	case ln == r.EndLine:
		width := maxInt(r.EndColumn-1, 1)
		return strings.Repeat("^", width)
	}
	return ""
}

// readSourceLines returns the file's lines, caching across calls.
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

// writeSummary renders biome's footer block: a "Checked N files in
// Tms. No fixes applied." line, then "Found X errors." and "Found Y
// warnings." on separate lines (each in the relevant severity color).
func writeSummary(w io.Writer, c palette, files int, dur time.Duration, errors, warnings int) error {
	if files > 0 {
		if _, err := fmt.Fprintf(w, "Checked %d %s in %s. No fixes applied.\n",
			files, plural(files, "file", "files"), formatDuration(dur)); err != nil {
			return err
		}
	}
	if errors > 0 {
		line := fmt.Sprintf("Found %d %s.", errors, plural(errors, "error", "errors"))
		if _, err := fmt.Fprintf(w, "%s\n", c.wrap(c.red, line)); err != nil {
			return err
		}
	}
	if warnings > 0 {
		line := fmt.Sprintf("Found %d %s.", warnings, plural(warnings, "warning", "warnings"))
		if _, err := fmt.Fprintf(w, "%s\n", c.wrap(c.yellow, line)); err != nil {
			return err
		}
	}
	if files == 0 && errors == 0 && warnings == 0 {
		// Tests that pass a single diagnostic without stats still want
		// some indication of completion; mirror biome's default for
		// the all-clear case.
		_, err := fmt.Fprintln(w, "No issues found.")
		return err
	}
	return nil
}

// formatDuration produces a human-friendly elapsed time matching
// biome's compact style ("789ms", "2s", "1m32s"). Sub-second values
// render in milliseconds; second-and-above values use Go's stdlib
// duration format which already yields readable output.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(time.Millisecond).String()
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
