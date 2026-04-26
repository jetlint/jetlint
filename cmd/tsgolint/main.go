package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.0.0-dev"

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

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("tsgolint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stdout, usage) }

	versionFlag := fs.Bool("version", false, "print version and exit")
	helpFlag := fs.Bool("help", false, "print usage and exit")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	switch {
	case *versionFlag:
		fmt.Fprintln(stdout, version)
		return 0
	case *helpFlag:
		fs.Usage()
		return 0
	}

	// No lint pipeline yet; future scenarios wire this up.
	return 0
}
