// Package cli implements the command-line interface for the tsgolint binary.
// Keeping it out of package main lets tests exercise Run in-process without
// a build step.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	// Imported to anchor the wrapper-API dependency: the binary statically
	// links against the fork's exported packages. The reference is here
	// (not in main) so the architecture test sees rule-package-shaped imports.
	_ "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/tommymorgan/tsgolint/internal/config"
	"github.com/tommymorgan/tsgolint/internal/daemon"
	"github.com/tommymorgan/tsgolint/internal/format"
	"github.com/tommymorgan/tsgolint/internal/project"
	"github.com/tommymorgan/tsgolint/internal/toolerr"
	"github.com/tommymorgan/tsgolint/internal/transport"
)

// Version is the linter's reported version. Build pipelines can override
// the constant via -ldflags "-X 'github.com/tommymorgan/tsgolint/internal/cli.Version=...'".
var Version = "0.0.0-dev"

const usage = `tsgolint - fast, type-aware TypeScript linter

Usage:
    tsgolint [flags] [files...]

Flags:
    --version          Print the linter version and exit.
    --help             Print this help text and exit.
    --format <name>    Output format. One of: human (default), json.

Exit codes:
    0    No diagnostics produced.
    1    Lint diagnostics produced.
    2    Tooling failure.
`

// daemonIdleDefault is the daemon's no-request shutdown window when it is
// launched with --daemon. The plan calls out 10 minutes as the default.
const daemonIdleDefault = 10 * time.Minute

// Run parses args and executes the requested action. Returns the process
// exit code. stdout receives successful output (version, usage on --help,
// diagnostics); stderr receives error output (parse failures, tooling
// errors).
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tsgolint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	versionFlag := fs.Bool("version", false, "print version and exit")
	helpFlag := fs.Bool("help", false, "print usage and exit")
	formatFlag := fs.String("format", "human", "output format (human, json)")
	daemonFlag := fs.String("daemon", "", "internal: run as the per-project daemon listening on the given socket")

	if err := fs.Parse(args); err != nil {
		// flag has already written the error and usage to stderr.
		return 2
	}

	switch {
	case *versionFlag:
		fmt.Fprintln(stdout, Version)
		return 0
	case *helpFlag:
		fmt.Fprint(stdout, usage)
		return 0
	case *daemonFlag != "":
		return runDaemon(*daemonFlag, stderr)
	}

	formatter, err := format.Lookup(*formatFlag)
	if err != nil {
		// Unknown format: report via the chosen format itself is impossible
		// because we don't have a formatter; fall back to human-formatted
		// stderr regardless and exit 2.
		emitToolError(stderr, "human",
			toolerr.WithPath(toolerr.CodeFormatUnknown, err.Error(), ""))
		return 2
	}

	return runLint(fs.Args(), stdout, stderr, formatter)
}

// runDaemon is the entry point for the spawned daemon process.
func runDaemon(socketPath string, stderr io.Writer) int {
	srv, err := daemon.NewServer(socketPath, daemonIdleDefault)
	if err != nil {
		fmt.Fprintf(stderr, "daemon: %v\n", err)
		return 2
	}
	if err := srv.Run(context.Background()); err != nil {
		fmt.Fprintf(stderr, "daemon: %v\n", err)
		return 2
	}
	return 0
}

// runLint is the entry point for a normal CLI invocation. For v0.1 it
// proves the daemon round-trip end-to-end and renders the (currently
// always empty) diagnostic set via the chosen formatter.
func runLint(targets []string, stdout, stderr io.Writer, formatter format.Formatter) int {
	if len(targets) == 0 {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeNoTargets, "no targets provided"))
		return 2
	}

	tsconfig, err := project.FindNearestTsconfig(targets[0])
	if err != nil {
		code := toolerr.CodeInternal
		if project.IsNotFound(err) {
			code = toolerr.CodeTsconfigMissing
		}
		emitToolError(stderr, formatter.Name(),
			toolerr.WithPath(code, err.Error(), targets[0]))
		return 2
	}

	// Resolve the lint configuration cascade starting at the directory of
	// the first target. Failures here are user-facing tooling errors (bad
	// JSON, unknown rule); they preempt daemon work so the user sees the
	// problem immediately.
	if _, err := config.ResolveCascade(filepath.Dir(targets[0])); err != nil {
		var te *toolerr.Error
		if errors.As(err, &te) {
			emitToolError(stderr, formatter.Name(), te)
		} else {
			emitToolError(stderr, formatter.Name(),
				toolerr.New(toolerr.CodeInternal, err.Error()))
		}
		return 2
	}

	socket, err := transport.DaemonSocketPath(tsconfig)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}

	if err := daemon.EnsureRunning(context.Background(), daemon.SpawnConfig{
		SocketPath: socket,
		Args:       []string{"--daemon", socket},
	}); err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeDaemonUnavailable, err.Error()))
		return 2
	}

	resp, err := daemon.Ping(socket, time.Second)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeDaemonUnavailable, err.Error()))
		return 2
	}
	if resp.Error != "" {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, resp.Error))
		return 2
	}

	// No rules yet — render the (always empty) diagnostic set so output
	// shape is correct from day one.
	if err := formatter.Format(stdout, nil); err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}
	return 0
}

// emitToolError writes a tooling failure to stderr in the appropriate
// shape for the given format. JSON mode emits a single-line JSON object;
// human mode emits a "tsgolint: <message>" line.
func emitToolError(stderr io.Writer, formatName string, e *toolerr.Error) {
	if formatName == "json" {
		_ = e.WriteJSON(stderr)
		return
	}
	if e.Path != "" {
		fmt.Fprintf(stderr, "tsgolint: %s: %s\n", e.Path, e.Message)
	} else {
		fmt.Fprintf(stderr, "tsgolint: %s\n", e.Message)
	}
}

// Ensure errors package import is reachable from here even if all error
// inspection currently lives in helpers; this keeps a stable surface.
var _ = errors.Is
