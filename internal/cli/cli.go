// Package cli implements the command-line interface for the jetlint binary.
// Keeping it out of package main lets tests exercise Run in-process without
// a build step.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Imported to anchor the wrapper-API dependency: the binary statically
	// links against the fork's exported packages. The reference is here
	// (not in main) so the architecture test sees rule-package-shaped imports.
	_ "github.com/microsoft/typescript-go/pkg/lint"

	"github.com/jetlint/jetlint/internal/config"
	"github.com/jetlint/jetlint/internal/daemon"
	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/format"
	"github.com/jetlint/jetlint/internal/project"
	"github.com/jetlint/jetlint/internal/rules"
	"github.com/jetlint/jetlint/internal/rules/arraycallbackreturn"
	"github.com/jetlint/jetlint/internal/rules/awaitthenable"
	"github.com/jetlint/jetlint/internal/rules/consistentreturn"
	"github.com/jetlint/jetlint/internal/rules/consistenttypeexports"
	"github.com/jetlint/jetlint/internal/rules/constructorsuper"
	"github.com/jetlint/jetlint/internal/rules/dotnotation"
	"github.com/jetlint/jetlint/internal/rules/fordirection"
	"github.com/jetlint/jetlint/internal/rules/getterreturn"
	"github.com/jetlint/jetlint/internal/rules/namingconvention"
	"github.com/jetlint/jetlint/internal/rules/noarraydelete"
	"github.com/jetlint/jetlint/internal/rules/noasyncpromiseexecutor"
	"github.com/jetlint/jetlint/internal/rules/noawaitinloop"
	"github.com/jetlint/jetlint/internal/rules/nobasetotostring"
	"github.com/jetlint/jetlint/internal/rules/noclassassign"
	"github.com/jetlint/jetlint/internal/rules/nocomparenegzero"
	"github.com/jetlint/jetlint/internal/rules/nocondassign"
	"github.com/jetlint/jetlint/internal/rules/noconfusingvoidexpression"
	"github.com/jetlint/jetlint/internal/rules/noconstantbinaryexpression"
	"github.com/jetlint/jetlint/internal/rules/noconstantcondition"
	"github.com/jetlint/jetlint/internal/rules/noconstassign"
	"github.com/jetlint/jetlint/internal/rules/noconstructorreturn"
	"github.com/jetlint/jetlint/internal/rules/nodebugger"
	"github.com/jetlint/jetlint/internal/rules/nodeprecated"
	"github.com/jetlint/jetlint/internal/rules/nodupeargs"
	"github.com/jetlint/jetlint/internal/rules/nodupeclassmembers"
	"github.com/jetlint/jetlint/internal/rules/nodupeelseif"
	"github.com/jetlint/jetlint/internal/rules/nodupekeys"
	"github.com/jetlint/jetlint/internal/rules/noduplicatecase"
	"github.com/jetlint/jetlint/internal/rules/noduplicateimports"
	"github.com/jetlint/jetlint/internal/rules/noduplicatetypeconstituents"
	"github.com/jetlint/jetlint/internal/rules/noemptypattern"
	"github.com/jetlint/jetlint/internal/rules/noexassign"
	"github.com/jetlint/jetlint/internal/rules/nofallthrough"
	"github.com/jetlint/jetlint/internal/rules/nofloatingpromises"
	"github.com/jetlint/jetlint/internal/rules/noforinarray"
	"github.com/jetlint/jetlint/internal/rules/nofuncassign"
	"github.com/jetlint/jetlint/internal/rules/noimpliedeval"
	"github.com/jetlint/jetlint/internal/rules/noimportassign"
	"github.com/jetlint/jetlint/internal/rules/noinnerdeclarations"
	"github.com/jetlint/jetlint/internal/rules/noirregularwhitespace"
	"github.com/jetlint/jetlint/internal/rules/nolossofprecision"
	"github.com/jetlint/jetlint/internal/rules/nomeaninglessvoidoperator"
	"github.com/jetlint/jetlint/internal/rules/nomisusedpromises"
	"github.com/jetlint/jetlint/internal/rules/nomisusedspread"
	"github.com/jetlint/jetlint/internal/rules/nomixedenums"
	"github.com/jetlint/jetlint/internal/rules/nonewnativenonconstructor"
	"github.com/jetlint/jetlint/internal/rules/nonnullabletypeassertionstyle"
	"github.com/jetlint/jetlint/internal/rules/noobjcalls"
	"github.com/jetlint/jetlint/internal/rules/nopromiseexecutorreturn"
	"github.com/jetlint/jetlint/internal/rules/noprototypebuiltins"
	"github.com/jetlint/jetlint/internal/rules/noredundanttypeconstituents"
	"github.com/jetlint/jetlint/internal/rules/noselfassign"
	"github.com/jetlint/jetlint/internal/rules/noselfcompare"
	"github.com/jetlint/jetlint/internal/rules/nosetterreturn"
	"github.com/jetlint/jetlint/internal/rules/nosparsearrays"
	"github.com/jetlint/jetlint/internal/rules/notemplatecurlyinstring"
	"github.com/jetlint/jetlint/internal/rules/nothisbeforesuper"
	"github.com/jetlint/jetlint/internal/rules/noundef"
	"github.com/jetlint/jetlint/internal/rules/nounexpectedmultiline"
	"github.com/jetlint/jetlint/internal/rules/nounmodifiedloopcondition"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarybooleanliteralcompare"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarycondition"
	"github.com/jetlint/jetlint/internal/rules/nounnecessaryqualifier"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytemplateexpression"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypearguments"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeassertion"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeconversion"
	"github.com/jetlint/jetlint/internal/rules/nounnecessarytypeparameters"
	"github.com/jetlint/jetlint/internal/rules/nounreachable"
	"github.com/jetlint/jetlint/internal/rules/nounsafeargument"
	"github.com/jetlint/jetlint/internal/rules/nounsafeassignment"
	"github.com/jetlint/jetlint/internal/rules/nounsafecall"
	"github.com/jetlint/jetlint/internal/rules/nounsafeenumcomparison"
	"github.com/jetlint/jetlint/internal/rules/nounsafefinally"
	"github.com/jetlint/jetlint/internal/rules/nounsafememberaccess"
	"github.com/jetlint/jetlint/internal/rules/nounsafenegation"
	"github.com/jetlint/jetlint/internal/rules/nounsafeoptionalchaining"
	"github.com/jetlint/jetlint/internal/rules/nounsafereturn"
	"github.com/jetlint/jetlint/internal/rules/nounsafetypeassertion"
	"github.com/jetlint/jetlint/internal/rules/nounsafeunaryminus"
	"github.com/jetlint/jetlint/internal/rules/nounreachableloop"
	"github.com/jetlint/jetlint/internal/rules/nounusedprivateclassmembers"
	"github.com/jetlint/jetlint/internal/rules/nounusedvars"
	"github.com/jetlint/jetlint/internal/rules/nousebeforedefine"
	"github.com/jetlint/jetlint/internal/rules/nouselessdefaultassignment"
	"github.com/jetlint/jetlint/internal/rules/onlythrowerror"
	"github.com/jetlint/jetlint/internal/rules/preferdestructuring"
	"github.com/jetlint/jetlint/internal/rules/preferfind"
	"github.com/jetlint/jetlint/internal/rules/preferincludes"
	"github.com/jetlint/jetlint/internal/rules/prefernullishcoalescing"
	"github.com/jetlint/jetlint/internal/rules/preferoptionalchain"
	"github.com/jetlint/jetlint/internal/rules/preferpromiserejecterrors"
	"github.com/jetlint/jetlint/internal/rules/preferreadonly"
	"github.com/jetlint/jetlint/internal/rules/preferreadonlyparametertypes"
	"github.com/jetlint/jetlint/internal/rules/preferreducetypeparameter"
	"github.com/jetlint/jetlint/internal/rules/preferregexpexec"
	"github.com/jetlint/jetlint/internal/rules/preferreturnthistype"
	"github.com/jetlint/jetlint/internal/rules/preferstringstartsendswith"
	"github.com/jetlint/jetlint/internal/rules/promisefunctionasync"
	"github.com/jetlint/jetlint/internal/rules/relatedgettersetterpairs"
	"github.com/jetlint/jetlint/internal/rules/requirearraysortcompare"
	"github.com/jetlint/jetlint/internal/rules/requireatomicupdates"
	"github.com/jetlint/jetlint/internal/rules/requireawait"
	"github.com/jetlint/jetlint/internal/rules/restrictplusoperands"
	"github.com/jetlint/jetlint/internal/rules/restricttemplateexpressions"
	"github.com/jetlint/jetlint/internal/rules/returnawait"
	"github.com/jetlint/jetlint/internal/rules/strictbooleanexpressions"
	"github.com/jetlint/jetlint/internal/rules/strictvoidreturn"
	"github.com/jetlint/jetlint/internal/rules/switchexhaustivenesscheck"
	"github.com/jetlint/jetlint/internal/rules/unboundmethod"
	"github.com/jetlint/jetlint/internal/rules/useisnan"
	"github.com/jetlint/jetlint/internal/rules/useunknownincatchcallbackvariable"
	"github.com/jetlint/jetlint/internal/rules/validtypeof"
	"github.com/jetlint/jetlint/internal/toolerr"
	"github.com/jetlint/jetlint/internal/transport"
	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"
	wrapperlint "github.com/microsoft/typescript-go/pkg/lint"
)

// Version is the linter's reported version. Build pipelines can override
// the constant via -ldflags "-X 'github.com/jetlint/jetlint/internal/cli.Version=...'".
var Version = "0.0.0-dev"

const usage = `jetlint - fast, type-aware TypeScript linter

Usage:
    jetlint [flags] [files...]

Flags:
    --version          Print the linter version and exit.
    --help             Print this help text and exit.
    --format <name>    Output format. One of: human (default), json,
                       sarif (GitHub Code Scanning, Azure DevOps),
                       github (GitHub Actions inline PR annotations),
                       junit (CI dashboard XML),
                       rdjson (reviewdog inline PR comments).
    --max-diagnostics <n>
                       Cap on rendered diagnostics for the human format.
                       0 disables truncation. Overrides .jetlintrc.json's
                       maxDiagnostics value. Default: 20 (matches biome).
    --only <rule-id>   Restrict execution to the named rule. Repeatable
                       to allow a small set: --only no-floating-promises
                       --only no-base-to-string. Useful for head-to-head
                       comparisons against other linters.

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
	fs := flag.NewFlagSet("jetlint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	versionFlag := fs.Bool("version", false, "print version and exit")
	helpFlag := fs.Bool("help", false, "print usage and exit")
	formatFlag := fs.String("format", "human", "output format (human, json)")
	filesFromFlag := fs.String("files-from", "", "read newline-separated target paths from this file (use - for stdin)")
	// -1 is the sentinel for "not supplied"; the resolved config's
	// value (or DefaultMaxDiagnostics when no config) wins in that
	// case. 0 means "render every diagnostic".
	maxDiagnosticsFlag := fs.Int("max-diagnostics", -1, "cap on rendered diagnostics for the human format (0 = unlimited; overrides config)")
	var onlyRules stringSliceFlag
	fs.Var(&onlyRules, "only", "restrict execution to the named rule (repeatable: --only no-floating-promises --only no-base-to-string)")
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

	return runLint(targets, stdout, stderr, formatter, *maxDiagnosticsFlag, onlyRules)
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
		fmt.Fprintf(stderr, "jetlint daemon: start failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "jetlint daemon: started on %s, idle timeout %s\n",
		socketPath, daemonIdleDefault)
	if err := srv.Run(context.Background()); err != nil {
		fmt.Fprintf(stderr, "jetlint daemon: run failed: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "jetlint daemon: shut down cleanly\n")
	return 0
}

// runLint is the entry point for a normal CLI invocation. For v0.1 it
// proves the daemon round-trip end-to-end and renders the (currently
// always empty) diagnostic set via the chosen formatter.
// stringSliceFlag is a flag.Value implementation for --only and any
// future repeated string flag. Each occurrence on the command line
// appends to the slice; the value method returns a quoted joined
// string so flag's default printout stays readable.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func runLint(targets []string, stdout, stderr io.Writer, formatter format.Formatter, maxDiagnosticsFlag int, onlyRules []string) int {
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

	// Apply --only filtering: validate every named rule against the
	// registry, then restrict the resolved severities map to just
	// those rules. Unknown rules fail fast with a structured error.
	if len(onlyRules) > 0 {
		for _, ruleID := range onlyRules {
			if !rules.IsKnown(ruleID) {
				emitToolError(stderr, formatter.Name(),
					toolerr.New(toolerr.CodeConfigUnknownRule,
						fmt.Sprintf("unknown rule %q passed via --only (known rules: %v)", ruleID, rules.MVPRuleIDs)))
				return 2
			}
		}
		filtered := make(map[string]wrapperlint.Severity, len(onlyRules))
		for _, ruleID := range onlyRules {
			if sev, ok := resolved.Rules[ruleID]; ok {
				filtered[ruleID] = sev
			} else {
				// Rule was disabled in config but explicitly requested
				// via --only; honor the override at error severity so
				// the user always sees the head-to-head comparison they
				// asked for.
				filtered[ruleID] = wrapperlint.SeverityError
			}
		}
		resolved.Rules = filtered
	}

	rulesList, ruleErr := buildRules(resolved.RuleOptions)
	if ruleErr != nil {
		emitToolError(stderr, formatter.Name(), ruleErr)
		return 2
	}
	eng := engine.New(rulesList, resolved.Rules)
	lintStart := time.Now()
	diagnostics := eng.Lint(prog)
	lintDuration := time.Since(lintStart)
	filesChecked := len(prog.SourceFiles())

	// Degraded-mode signal: when the program itself has type errors,
	// every type-aware diagnostic built on it is suspect. Surface a
	// single program-scope tool warning so AI agents can detect the
	// degraded state and decide how to weight the rest of the output.
	if prog.HasTypeErrors() {
		diagnostics = append([]wrapperlint.Diagnostic{{
			Range:    wrapperlint.SourceRange{File: tsconfig, StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1},
			RuleID:   "jetlint/program-has-type-errors",
			Severity: wrapperlint.SeverityWarning,
			Message:  "the TypeScript program has type errors; lint diagnostics may be unreliable until those are resolved",
		}}, diagnostics...)
	}

	// The human formatter benefits from execution stats (files checked,
	// duration, max-diagnostics cap) in its summary block. Other
	// formatters don't carry that state, so we only enrich when the
	// active formatter is Human.
	maxDiagnostics := resolved.MaxDiagnostics
	if maxDiagnosticsFlag >= 0 {
		maxDiagnostics = maxDiagnosticsFlag
	}
	if _, ok := formatter.(format.Human); ok {
		formatter = format.Human{
			FilesChecked:   filesChecked,
			Duration:       lintDuration,
			MaxDiagnostics: maxDiagnostics,
		}
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

// buildRules constructs every shipped rule, applying any per-rule
// options the resolved config supplied. Options for rules that don't
// accept any are rejected with a structured config error so typos
// surface at startup. The engine filters by severity, so rules
// disabled at runtime have zero overhead per node.
func buildRules(ruleOptions map[string]json.RawMessage) ([]engine.Rule, *toolerr.Error) {
	nfpOpts, err := nofloatingpromises.OptionsFromJSON(ruleOptions["no-floating-promises"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	nbtsOpts, err := nobasetotostring.OptionsFromJSON(ruleOptions["no-base-to-string"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	nmpOpts, err := nomisusedpromises.OptionsFromJSON(ruleOptions["no-misused-promises"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	acrOpts, err := arraycallbackreturn.OptionsFromJSON(ruleOptions["array-callback-return"])
	if err != nil {
		return nil, toolerr.New(toolerr.CodeConfigInvalid, err.Error())
	}
	rejectIfOptionsPresent := func(ruleID string) *toolerr.Error {
		if len(ruleOptions[ruleID]) == 0 {
			return nil
		}
		return toolerr.New(toolerr.CodeConfigInvalid,
			fmt.Sprintf("rule %q does not accept options yet", ruleID))
	}
	// Rules without options support: reject any user-supplied options
	// at config-load time so typos are visible.
	rulesWithOptions := map[string]bool{
		"array-callback-return": true,
	}
	for _, ruleID := range append([]string{
		"strict-boolean-expressions",
		"no-unsafe-assignment",
	}, rules.AdditionalTypeAwareRuleIDs...) {
		if rulesWithOptions[ruleID] {
			continue
		}
		if e := rejectIfOptionsPresent(ruleID); e != nil {
			return nil, e
		}
	}
	return []engine.Rule{
		// MVP rules with full options support.
		nofloatingpromises.NewWithOptions(nfpOpts),
		nomisusedpromises.NewWithOptions(nmpOpts),
		strictbooleanexpressions.New(),
		nounsafeassignment.New(),
		nobasetotostring.NewWithOptions(nbtsOpts),
		// Additional type-aware rules — default-off, opt-in via config.
		arraycallbackreturn.NewWithOptions(acrOpts),
		awaitthenable.New(),
		consistentreturn.New(),
		consistenttypeexports.New(),
		constructorsuper.New(),
		dotnotation.New(),
		fordirection.New(),
		getterreturn.New(),
		namingconvention.New(),
		noarraydelete.New(),
		noasyncpromiseexecutor.New(),
		noawaitinloop.New(),
		noclassassign.New(),
		nocomparenegzero.New(),
		nocondassign.New(),
		noconfusingvoidexpression.New(),
		noconstantbinaryexpression.New(),
		noconstantcondition.New(),
		noconstassign.New(),
		noconstructorreturn.New(),
		nodebugger.New(),
		nodeprecated.New(),
		nodupeargs.New(),
		nodupeclassmembers.New(),
		nodupeelseif.New(),
		noemptypattern.New(),
		noexassign.New(),
		nofallthrough.New(),
		nofuncassign.New(),
		nosparsearrays.New(),
		nopromiseexecutorreturn.New(),
		noprototypebuiltins.New(),
		notemplatecurlyinstring.New(),
		nothisbeforesuper.New(),
		noundef.New(),
		nodupekeys.New(),
		noduplicatecase.New(),
		noduplicateimports.New(),
		noduplicatetypeconstituents.New(),
		noforinarray.New(),
		noimpliedeval.New(),
		noimportassign.New(),
		noinnerdeclarations.New(),
		noirregularwhitespace.New(),
		nolossofprecision.New(),
		nomeaninglessvoidoperator.New(),
		nomisusedspread.New(),
		nomixedenums.New(),
		nonewnativenonconstructor.New(),
		noobjcalls.New(),
		nonnullabletypeassertionstyle.New(),
		noredundanttypeconstituents.New(),
		noselfassign.New(),
		noselfcompare.New(),
		nosetterreturn.New(),
		nounexpectedmultiline.New(),
		nounreachable.New(),
		nounreachableloop.New(),
		nounnecessarybooleanliteralcompare.New(),
		nounnecessarycondition.New(),
		nounnecessaryqualifier.New(),
		nounnecessarytemplateexpression.New(),
		nounnecessarytypearguments.New(),
		nounnecessarytypeassertion.New(),
		nounnecessarytypeconversion.New(),
		nounnecessarytypeparameters.New(),
		nounsafeargument.New(),
		nounsafecall.New(),
		nounsafeenumcomparison.New(),
		nounsafememberaccess.New(),
		nounsafenegation.New(),
		nounsafeoptionalchaining.New(),
		nounsafefinally.New(),
		nounsafereturn.New(),
		nounsafetypeassertion.New(),
		nounsafeunaryminus.New(),
		nounmodifiedloopcondition.New(),
		nounusedprivateclassmembers.New(),
		nounusedvars.New(),
		nousebeforedefine.New(),
		nouselessdefaultassignment.New(),
		onlythrowerror.New(),
		preferdestructuring.New(),
		preferfind.New(),
		preferincludes.New(),
		prefernullishcoalescing.New(),
		preferoptionalchain.New(),
		preferpromiserejecterrors.New(),
		preferreadonly.New(),
		preferreadonlyparametertypes.New(),
		preferreducetypeparameter.New(),
		preferregexpexec.New(),
		preferreturnthistype.New(),
		preferstringstartsendswith.New(),
		promisefunctionasync.New(),
		relatedgettersetterpairs.New(),
		requirearraysortcompare.New(),
		requireatomicupdates.New(),
		requireawait.New(),
		restrictplusoperands.New(),
		restricttemplateexpressions.New(),
		returnawait.New(),
		strictvoidreturn.New(),
		switchexhaustivenesscheck.New(),
		unboundmethod.New(),
		useisnan.New(),
		useunknownincatchcallbackvariable.New(),
		validtypeof.New(),
	}, nil
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
// human mode emits a "jetlint: <message>" line.
func emitToolError(stderr io.Writer, formatName string, e *toolerr.Error) {
	if formatName == "json" {
		_ = e.WriteJSON(stderr)
		return
	}
	if e.Path != "" {
		fmt.Fprintf(stderr, "jetlint: %s: %s\n", e.Path, e.Message)
	} else {
		fmt.Fprintf(stderr, "jetlint: %s\n", e.Message)
	}
}

// Ensure errors package import is reachable from here even if all error
// inspection currently lives in helpers; this keeps a stable surface.
var _ = errors.Is
