// Package toolerr defines the structured-error contract emitted to stderr
// when the linter is invoked in JSON mode and a tooling failure occurs.
// AI consumers can rely on the Code values to branch without parsing
// human-readable prose.
package toolerr

import (
	"encoding/json"
	"fmt"
	"io"
)

// Code identifies the class of tooling failure. The set is enumerated
// rather than open so consumers can rely on a stable vocabulary.
type Code string

const (
	// CodeConfigInvalid means a linter configuration file could not be
	// parsed or validated.
	CodeConfigInvalid Code = "config_invalid"

	// CodeConfigUnknownRule means a configuration referenced a rule that
	// the linter does not ship.
	CodeConfigUnknownRule Code = "config_unknown_rule"

	// CodeTsconfigMissing means no tsconfig.json was discovered for a
	// given target.
	CodeTsconfigMissing Code = "tsconfig_missing"

	// CodeTsconfigInvalid means a tsconfig.json existed but could not be
	// loaded as a TypeScript program.
	CodeTsconfigInvalid Code = "tsconfig_invalid"

	// CodeProgramBuildFailed means the TypeScript program build failed
	// for a reason beyond a simple invalid configuration (for example, a
	// resolver error during program construction).
	CodeProgramBuildFailed Code = "program_build_failed"

	// CodeDaemonUnavailable means the CLI could not make a daemon
	// available within the spawn budget.
	CodeDaemonUnavailable Code = "daemon_unavailable"

	// CodeFormatUnknown means the user requested an unsupported output
	// format.
	CodeFormatUnknown Code = "format_unknown"

	// CodeNoTargets means the user invoked the linter with no targets.
	CodeNoTargets Code = "no_targets"

	// CodeInternal is a fallback for failures that do not fit any of the
	// more specific codes. Treat as "tooling broke; report the message".
	CodeInternal Code = "internal"
)

// Error is the structured tooling-failure type. When the CLI encounters
// one in JSON mode, the Marshal method writes a single-line JSON object
// to stderr and the process exits with code 2.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// New constructs an Error with the given code and message. The Path field
// is left empty.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithPath constructs an Error with the given code, message, and offending
// path so consumers can surface it without parsing the message.
func WithPath(code Code, message, path string) *Error {
	return &Error{Code: code, Message: message, Path: path}
}

// Newf is a convenience constructor that formats the message with fmt.
func Newf(code Code, format string, args ...any) *Error {
	return New(code, fmt.Sprintf(format, args...))
}

// Error implements the error interface, returning Message verbatim.
func (e *Error) Error() string { return e.Message }

// WriteJSON emits the Error to w as a single-line JSON object followed
// by a newline. Used by the CLI when the user requested --format json.
func (e *Error) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(e)
}
