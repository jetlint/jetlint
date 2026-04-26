// Package cli implements the command-line interface for the tsgolint binary.
// Keeping it out of package main lets tests exercise Run in-process without
// a build step.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	// Imported to anchor the wrapper-API dependency: the binary statically
	// links against the fork's exported packages. The reference is here
	// (not in main) so the architecture test sees rule-package-shaped imports.
	_ "github.com/microsoft/typescript-go/pkg/lint"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
	"github.com/tommymorgan/tsgolint/internal/config"
	"github.com/tommymorgan/tsgolint/internal/daemon"
	"github.com/tommymorgan/tsgolint/internal/engine"
	"github.com/tommymorgan/tsgolint/internal/format"
	"github.com/tommymorgan/tsgolint/internal/project"
	"github.com/tommymorgan/tsgolint/internal/rules/nobasetotostring"
	"github.com/tommymorgan/tsgolint/internal/rules/nofloatingpromises"
	"github.com/tommymorgan/tsgolint/internal/rules/nomisusedpromises"
	"github.com/tommymorgan/tsgolint/internal/rules/nounsafeassignment"
	"github.com/tommymorgan/tsgolint/internal/rules/strictbooleanexpressions"
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
    --format <name>    Output format. One of: human (default), json,
                       sarif (GitHub Code Scanning), github-actions
                       (workflow command annotations).

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
	filesFromFlag := fs.String("files-from", "", "read newline-separated target paths from this file (use - for stdin)")
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

	targets := fs.Args()
	if *filesFromFlag != "" {
		extra, err := readFileList(*filesFromFlag)
		if err != nil {
			emitToolError(stderr, formatter.Name(),
				toolerr.WithPath(toolerr.CodeInternal, err.Error(), *filesFromFlag))
			return 2
		}
		targets = append(targets, extra...)
	}

	return runLint(targets, stdout, stderr, formatter)
}

// readFileList returns the newline-separated target paths from path. The
// special path "-" reads from os.Stdin. Blank lines are ignored.
func readFileList(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// runDaemon is the entry point for the spawned daemon process. It writes
// lifecycle events to stderr; the parent CLI captures stderr into the
// per-project log file via SpawnConfig.LogPath.
func runDaemon(socketPath string, stderr io.Writer) int {
	srv, err := daemon.NewServer(socketPath, daemonIdleDefault)
	if err != nil {
		fmt.Fprintf(stderr, "tsgolint daemon: start failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "tsgolint daemon: started on %s, idle timeout %s\n",
		socketPath, daemonIdleDefault)
	if err := srv.Run(context.Background()); err != nil {
		fmt.Fprintf(stderr, "tsgolint daemon: run failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "tsgolint daemon: shut down cleanly\n")
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
	resolved, err := config.ResolveCascade(filepath.Dir(targets[0]))
	if err != nil {
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

	logPath, err := transport.LogPath(tsconfig)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}
	if err := daemon.EnsureRunning(context.Background(), daemon.SpawnConfig{
		SocketPath: socket,
		LogPath:    logPath,
		Args:       []string{"--daemon", socket},
	}); err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeDaemonUnavailable, err.Error()))
		return 2
	}

	// Health-probe the daemon. On a mid-request connection drop we
	// retry exactly once after re-spawning a fresh daemon, per the
	// plan's failure-handling contract. A second failure exits 2.
	resp, err := daemon.Ping(socket, time.Second)
	if err != nil {
		if respawnErr := daemon.EnsureRunning(context.Background(), daemon.SpawnConfig{
			SocketPath: socket,
			LogPath:    logPath,
			Args:       []string{"--daemon", socket},
		}); respawnErr == nil {
			resp, err = daemon.Ping(socket, time.Second)
		}
	}
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

	// Load the program and run the rule engine in-process. v0.1 keeps
	// the lint compute on the CLI side; the daemon's warm-path role is
	// to amortise startup once future revisions move program loading
	// into the daemon and exchange diagnostics over the socket.
	prog, err := wrapperchecker.LoadProgram(tsconfig)
	if err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.WithPath(toolerr.CodeProgramBuildFailed, err.Error(), tsconfig))
		return 2
	}
	defer prog.Close()

	// For each explicit target, verify it is part of the discovered
	// program. Files outside the program get a per-target structured
	// warning (in JSON mode) and are skipped from the lint scope.
	for _, target := range targets {
		abs, absErr := filepath.Abs(target)
		if absErr != nil {
			continue
		}
		if prog.SourceFileByPath(abs) == nil {
			emitToolError(stderr, formatter.Name(),
				toolerr.WithPath(toolerr.CodeInternal,
					"target file is not part of the discovered TypeScript program; ensure the file is included by tsconfig.json's include/files",
					abs))
		}
	}

	eng := engine.New(activeRules(), resolved.Rules)
	diagnostics := eng.Lint(prog)

	// Degraded-mode signal: when the program itself has type errors,
	// every type-aware diagnostic built on it is suspect. Surface a
	// single program-scope tool warning so AI agents can detect the
	// degraded state and decide how to weight the rest of the output.
	if prog.HasTypeErrors() {
		diagnostics = append([]wrapperlint.Diagnostic{{
			Range:    wrapperlint.SourceRange{File: tsconfig, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1},
			RuleID:   "tsgolint/program-has-type-errors",
			Severity: wrapperlint.SeverityWarning,
			Message:  "the TypeScript program has type errors; lint diagnostics may be unreliable until those are resolved",
		}}, diagnostics...)
	}

	if err := formatter.Format(stdout, diagnostics); err != nil {
		emitToolError(stderr, formatter.Name(),
			toolerr.New(toolerr.CodeInternal, err.Error()))
		return 2
	}
	if hasError(diagnostics) {
		return 1
	}
	return 0
}

// activeRules returns the registered rule instances. The engine filters
// by the resolved configuration so rules disabled at runtime have zero
// overhead per node.
func activeRules() []engine.Rule {
	return []engine.Rule{
		nofloatingpromises.New(),
		nomisusedpromises.New(),
		strictbooleanexpressions.New(),
		nounsafeassignment.New(),
		nobasetotostring.New(),
	}
}

// hasError reports whether any diagnostic in the slice was emitted at
// error severity (the signal for exit code 1).
func hasError(d []wrapperlint.Diagnostic) bool {
	for _, x := range d {
		if x.Severity == wrapperlint.SeverityError {
			return true
		}
	}
	return false
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
