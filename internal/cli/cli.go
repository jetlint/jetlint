// Package cli implements the command-line interface for the tsgolint binary.
// Keeping it out of package main lets tests exercise Run in-process without a
// build step.
package cli

import (
	"flag"
	"fmt"
	"io"

	// Imported to anchor the wrapper-API dependency: the binary statically
	// links against the fork's exported packages. The reference is here
	// (not in main) so the architecture test sees rule-package-shaped imports.
	_ "github.com/microsoft/typescript-go/pkg/lint"
)

// Version is the linter's reported version. Build pipelines can override the
// constant via -ldflags "-X 'github.com/tommymorgan/tsgolint/internal/cli.Version=...'".
var Version = "0.0.0-dev"

const usage = `tsgolint - fast, type-aware TypeScript linter

Usage:
    tsgolint [flags] [files...]

Flags:
    --version    Print the linter version and exit.
    --help       Print this help text and exit.

Exit codes:
    0    No diagnostics produced.
    1    Lint diagnostics produced.
    2    Tooling failure.
`

// Run parses args and executes the requested action. Returns the process exit
// code. stdout receives successful output (version, usage on --help, diagnostics);
// stderr receives error output (parse failures, tooling errors).
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tsgolint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	versionFlag := fs.Bool("version", false, "print version and exit")
	helpFlag := fs.Bool("help", false, "print usage and exit")

	if err := fs.Parse(args); err != nil {
		// flag has already written the error and usage to stderr.
		return 2
	}

	switch {
	case *versionFlag:
		fmt.Fprintln(stdout, Version)
		return 0
	case *helpFlag:
		// --help is a successful invocation; usage goes to stdout.
		fmt.Fprint(stdout, usage)
		return 0
	}

	// No lint pipeline yet; future scenarios wire this up.
	return 0
}
