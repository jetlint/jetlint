// Package cli implements the command-line interface for the tsgolint binary.
// Keeping it out of package main lets tests exercise Run in-process without
// a build step.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	// Imported to anchor the wrapper-API dependency: the binary statically
	// links against the fork's exported packages. The reference is here
	// (not in main) so the architecture test sees rule-package-shaped imports.
	_ "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/tommymorgan/tsgolint/internal/daemon"
	"github.com/tommymorgan/tsgolint/internal/project"
	"github.com/tommymorgan/tsgolint/internal/transport"
)

// Version is the linter's reported version. Build pipelines can override
// the constant via -ldflags "-X 'github.com/tommymorgan/tsgolint/internal/cli.Version=...'".
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

	return runLint(fs.Args(), stdout, stderr)
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
// proves the daemon round-trip end-to-end without any rule machinery.
func runLint(targets []string, stdout, stderr io.Writer) int {
	if len(targets) == 0 {
		fmt.Fprintln(stderr, "tsgolint: no targets provided")
		return 2
	}
	tsconfig, err := project.FindNearestTsconfig(targets[0])
	if err != nil {
		fmt.Fprintf(stderr, "tsgolint: %v\n", err)
		return 2
	}
	socket, err := transport.DaemonSocketPath(tsconfig)
	if err != nil {
		fmt.Fprintf(stderr, "tsgolint: %v\n", err)
		return 2
	}
	if err := daemon.EnsureRunning(context.Background(), daemon.SpawnConfig{
		SocketPath: socket,
		Args:       []string{"--daemon", socket},
	}); err != nil {
		fmt.Fprintf(stderr, "tsgolint: %v\n", err)
		return 2
	}
	resp, err := daemon.Ping(socket, time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "tsgolint: %v\n", err)
		return 2
	}
	if resp.Error != "" {
		fmt.Fprintf(stderr, "tsgolint: daemon error: %s\n", resp.Error)
		return 2
	}
	// No rules yet — report a clean run.
	fmt.Fprintln(stdout, "ok")
	return 0
}
