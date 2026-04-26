# Feature: MVP type-aware TypeScript linter

**Created**: 2026-04-26
**Goal**: Ship a fast, daemon-backed CLI linter for TypeScript that reports type-aware diagnostics biome cannot produce, deterministic enough for AI coding agents to consume.

## User Requirements

### Local development bootstrap

# Living: features/local-development.feature::Bootstrap verifies toolchain prerequisites
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: Bootstrap verifies toolchain prerequisites
  Given a freshly cloned linter repository
  When the developer runs the bootstrap command
  Then the required Go toolchain version is detected and confirmed acceptable
  And the typescript-go fork checkout is detected at the expected sibling path
  And missing prerequisites cause the bootstrap to halt before any build step

# Living: features/local-development.feature::Bootstrap builds the linter binary
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: Bootstrap builds the linter binary
  Given the toolchain prerequisites have been verified
  When the developer runs the bootstrap command
  Then the linter binary is produced at the project's bin directory
  And the binary reports its version via `--version`

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Bootstrap smoke test confirms end-to-end function
  Given the linter binary has been built
  When the bootstrap script runs the smoke lint against a checked-in fixture project
  Then the expected diagnostic is produced
  And the bootstrap exits successfully

# Living: features/local-development.feature::Bootstrap fails fast when prerequisites are missing
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: Bootstrap fails fast when prerequisites are missing
  Given a freshly cloned linter repository
  And the required toolchain version is not present
  When the developer runs the bootstrap command
  Then the bootstrap halts before doing further work
  And the missing prerequisite is named in the failure message

### Running the linter

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A clean project exits zero with no diagnostics
  Given a TypeScript project with no rule violations
  When the user runs the linter against the project
  Then the linter prints no diagnostics
  And the process exits with code 0

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A project with violations exits one with diagnostics
  Given a TypeScript project containing at least one rule violation
  When the user runs the linter against the project
  Then each violation is reported with file, line, column, rule identifier, and message
  And the process exits with code 1

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Tooling failure exits two with the cause on stderr
  Given a condition that prevents the linter from completing (such as a broken project configuration)
  When the user runs the linter
  Then the underlying error is written to stderr with enough context to act on
  And the process exits with code 2
  And no lint diagnostics are emitted from the failed invocation

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Tooling failure in JSON mode emits a structured machine-readable error
  Given the linter is invoked with the JSON output format
  And a tooling failure occurs
  When the linter exits
  Then a single-line JSON object is written to stderr
  And the object includes a `code` field whose value is one of the documented failure codes (such as "config_invalid", "config_unknown_rule", "tsconfig_missing", "tsconfig_invalid", "program_build_failed", "daemon_unavailable", "format_unknown")
  And the object includes a `message` field with a human-readable description
  And the object includes a `path` field naming the offending file when one applies
  And the process exits with code 2

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: First invocation against a project completes within the cold-start budget
  Given a project the linter has not seen since system boot
  And the project is the 500-file reference fixture
  When the user runs the linter for the first time
  Then the invocation completes within 5 seconds on the CI runner of record
  And the linter's startup cost is incurred only on this first call

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Subsequent invocations complete within the warm-path budget
  Given a project the linter has already analyzed in this session
  And the project is the 500-file reference fixture
  When the user runs the linter again
  Then the invocation completes within 200 milliseconds at the 95th percentile across 20 runs on the CI runner of record
  And no startup cost is paid

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Concurrent invocations produce results referentially identical to sequential runs
  Given two simultaneous lint invocations targeting the same project
  When both invocations run to completion
  Then the union of diagnostics returned to the two callers equals the diagnostics that a single sequential invocation over the same files would produce
  And neither invocation's output contains data from the other invocation's request

# Living: features/cli.feature::--version prints a stable version string and exits zero
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: --version prints a stable version string and exits zero
  Given the linter binary
  When the user runs the linter with the `--version` flag
  Then the binary prints a single-line semantic version string
  And the process exits with code 0
  And no daemon is started or contacted

# Living: features/cli.feature::--help prints stable usage and exits zero
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: --help prints stable usage and exits zero
  Given the linter binary
  When the user runs the linter with the `--help` flag
  Then the usage text is printed listing supported commands, flags, and exit codes
  And the process exits with code 0
  And no daemon is started or contacted

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A list of files supplied on the command line is linted as a batch
  Given the user runs the linter with one or more file paths as positional arguments
  When the linter resolves each path against its discovered TypeScript program
  Then only the named files are linted
  And diagnostics are emitted for those files in the JSON output's documented order

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A list of files supplied on standard input is linted as a batch
  Given the user invokes the linter with the convention that signals "read file list from stdin" (such as `--files-from -`)
  And a newline-separated list of file paths is written to standard input
  When the linter completes
  Then only the files named on standard input are linted
  And the same output contract applies as for positional arguments

### Output formats

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Default output format is human-readable
  Given a project with violations
  When the user runs the linter without specifying an output format
  Then diagnostics are formatted for terminal display
  And each diagnostic shows file path, position, rule, and message in a scannable layout

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: JSON format produces a fully specified diagnostic contract
  Given a project with violations
  When the user runs the linter requesting JSON output
  Then the output carries a schema version identifying the contract
  And each diagnostic includes the fields file, startLine, startColumn, endLine, endColumn, ruleId, severity, and message
  And line numbers are one-indexed
  And column numbers are one-indexed UTF-16 code units (matching the Language Server Protocol convention)
  And severity is one of the values "error" or "warning"
  And diagnostics are ordered by file path, then by start position

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Repeated runs over unchanged input produce byte-identical JSON output
  Given a project and configuration that have not changed
  When the linter is invoked twice in JSON output mode
  Then the byte sequences emitted by the two invocations are identical
  And no field in the output depends on wall-clock time, process identifiers, or invocation ordering

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Unknown output format fails with a clear message
  Given the user requests an output format the linter does not support
  When the linter starts
  Then it exits with code 2 before producing any output
  And the error message lists the supported formats

### Configuration

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A project with no linter config uses sensible defaults
  Given a TypeScript project with no linter configuration file
  When the user runs the linter
  Then all five MVP rules are active at error severity
  And diagnostics are produced wherever rules find violations

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Cascading configurations override defaults level by level
  Given a project with a root linter configuration
  And a child directory with its own linter configuration
  When the user lints a file inside the child directory
  Then the effective configuration is the merge of root and child
  And settings declared in the child win where they conflict

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Cascade merge replaces lists rather than concatenating them
  Given a parent configuration that declares a list-typed setting (such as a list of disabled rules)
  And a child configuration that declares its own value for the same list-typed setting
  When the effective configuration is resolved for a file in the child's subtree
  Then the child's list value replaces the parent's list value entirely
  And no element of the parent's list survives in the resolved configuration unless the child's list also includes it

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A child configuration can disable a rule for its subtree
  Given a parent configuration that enables a rule
  And a child configuration that disables that rule
  When the user lints a file in the child's subtree
  Then no diagnostics from that rule are reported for files in the child's subtree
  And the rule continues to apply elsewhere

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: An invalid configuration file fails fast with location
  Given a linter configuration file that cannot be parsed or validated
  When the user runs the linter
  Then the linter exits with code 2 before linting any file
  And the failure message identifies the file and the offending location

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: An unknown rule name in configuration fails fast
  Given a linter configuration that references a rule the linter does not ship
  When the user runs the linter
  Then the linter exits with code 2 before linting any file
  And the failure message names the unknown rule

### Project discovery

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: The linter discovers the governing TypeScript project automatically
  Given a TypeScript file inside a project tree
  When the user runs the linter against that file
  Then the linter discovers the project's TypeScript configuration without explicit user input
  And the file is linted in the context of that project

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A directory with no TypeScript project produces a clear error
  Given a directory with no discoverable TypeScript project configuration
  When the user runs the linter from that directory
  Then the linter exits with code 2
  And the error message explains that a TypeScript project configuration is required

### Behavior when the program contains type errors

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Linting proceeds in degraded mode when the program has type errors, with an explicit signal
  Given a TypeScript project whose program currently fails to type-check
  When the user runs the linter
  Then a single tool-warning diagnostic at program scope is emitted noting that type-check errors are present in the program
  And lint diagnostics are still produced for the files in the request
  And the tool-warning diagnostic appears in both human and JSON output so that an AI agent can detect the degraded mode and decide whether to trust subsequent diagnostics

### Rule: detect floating promises

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should report an unawaited promise returned from an imported function
  Given a project where a function defined in one file returns a promise
  And the function is called in another file without awaiting or otherwise handling its result
  When the user runs the linter
  Then a diagnostic is reported at the unawaited call site
  And the diagnostic is attributed to the floating-promises rule

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report a promise that is awaited or explicitly handled
  Given a call to a promise-returning function that is awaited
  And another call whose returned promise is explicitly chained or assigned
  When the user runs the linter
  Then no floating-promises diagnostic is reported for either call

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report a promise that is explicitly discarded with the void operator
  Given a call to a promise-returning function whose result is prefixed with the `void` operator
  When the user runs the linter
  Then no floating-promises diagnostic is reported
  And this matches the established eslint-plugin convention for explicit fire-and-forget intent

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report a promise returned from an arrow function via implicit return
  Given an arrow function whose concise body returns the result of a promise-returning call
  When the user runs the linter
  Then no floating-promises diagnostic is reported at the implicit return
  Because the promise becomes the function's return value, the caller is responsible for handling it

### Rule: detect misused promises

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should report an async callback passed where the consumer expects no return
  Given an array iteration that accepts a callback returning nothing
  And the user passes an async callback to that iteration
  When the user runs the linter
  Then a diagnostic is reported at the call site
  And the diagnostic explains that the returned promise will be silently dropped

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report an async callback passed where a promise return is expected
  Given an iteration whose callback type accepts a promise return (such as mapping)
  And the user passes an async callback to that iteration
  When the user runs the linter
  Then no misused-promises diagnostic is reported

### Rule: enforce strict boolean expressions

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should report a boolean test of a value that may be string-or-undefined
  Given a value typed as a string union with undefined
  And the value is used directly as a boolean test
  When the user runs the linter
  Then a diagnostic is reported at the test location
  And the diagnostic identifies the ambiguous coercion

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report a boolean test of a value typed as boolean
  Given a value typed as boolean
  And the value is used as a boolean test
  When the user runs the linter
  Then no strict-boolean-expressions diagnostic is reported

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report a boolean test of a value narrowed by a prior guard
  Given a value typed as a string union with undefined
  And a prior guard such as `if (x !== undefined)` has narrowed the value to a string in the current control-flow branch
  When the value is used as a boolean test inside that branch
  And the user runs the linter
  Then no strict-boolean-expressions diagnostic is reported
  Because the narrowed type is no longer ambiguous

### Rule: detect unsafe assignment from any

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should report an assignment from a value typed any to a typed target
  Given a source value typed as any
  And the value is assigned to a variable with a more specific declared type
  When the user runs the linter
  Then a diagnostic is reported at the assignment
  And the diagnostic explains that type information was lost or fabricated

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report an assignment between properly typed values
  Given a source and target whose types are both fully known and compatible
  When the user runs the linter
  Then no unsafe-assignment diagnostic is reported

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report an assignment from any to a target also typed any
  Given a source value typed as any
  And a target also typed as any
  When the assignment occurs
  And the user runs the linter
  Then no unsafe-assignment diagnostic is reported
  Because both endpoints are equally unsound and the assignment introduces no new loss

### Rule: detect base-to-string coercion of objects

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should report a template literal that interpolates an object lacking a custom string conversion
  Given an object whose type does not declare a custom string-conversion method
  And the object is interpolated into a template literal
  When the user runs the linter
  Then a diagnostic is reported at the interpolation
  And the diagnostic explains that the result will be the default object representation

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should not report interpolation of values with a meaningful string conversion
  Given an object whose type declares its own string-conversion method
  And the object is interpolated into a template literal
  When the user runs the linter
  Then no base-to-string diagnostic is reported

  And given a primitive value interpolated into a template literal
  Then no base-to-string diagnostic is reported for the primitive

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Should report an array of objects whose elements lack a meaningful string conversion
  Given an array whose element type does not declare a custom string-conversion method
  And the array is interpolated into a template literal or coerced to string
  When the user runs the linter
  Then a diagnostic is reported at the coercion site
  Because Array.prototype.toString joins element string conversions, propagating the default object representation

## Technical Specifications

### Source layout and dependency

# Living: features/architecture.feature::The linter depends on the typescript-go fork via a Go module
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: The linter depends on the typescript-go fork via a Go module
  Given the linter repository
  When the build is performed
  Then the binary statically links against the fork's exported packages
  And no source file in the linter imports from internal packages of the fork

# Living: features/architecture.feature::An architecture test enforces the wrapper-API boundary
# Action: creates
# Status: DONE
# Living updated: YES
Scenario: An architecture test enforces the wrapper-API boundary
  Given a checked-in allowlist of import paths permitted from rule packages and from non-wrapper code in the fork
  When the architecture test runs in continuous integration
  Then the test scans every Go source file in the linter and the fork
  And the test fails the build if any rule source file imports a package outside the wrapper allowlist
  And the test fails the build if any non-wrapper file in the fork imports the fork's own internal packages

### Daemon lifecycle and transport

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A per-project daemon is started on first request
  Given no daemon process is running for a particular TypeScript project
  When the linter is invoked against a file in that project
  Then a daemon process is started bound to that project
  And subsequent CLI invocations against the same project reuse the daemon

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: The daemon socket path is derived deterministically from the project path
  Given two CLI invocations against the same project from any working directory
  When each computes the daemon socket path
  Then both compute the same socket path
  And the path lives under the platform-appropriate runtime directory

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: An idle daemon shuts itself down after a configurable timeout
  Given a daemon that has received no requests for the configured idle period (default 10 minutes)
  When the idle period elapses
  Then the daemon exits cleanly
  And its socket and PID file are removed

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A request arriving during idle shutdown either completes or is cleanly rejected
  Given a daemon whose idle timer has fired and which has begun its shutdown sequence
  When a CLI request arrives before the daemon has released its socket
  Then either the request is served to completion before the daemon exits
  Or the request is rejected with a structured "daemon_unavailable" error so the CLI's retry path can spawn a fresh daemon

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A stale socket from a crashed daemon is detected and replaced
  Given a socket file exists but no daemon process is responding
  When the CLI attempts to connect and its health probe fails to receive a response within 250 milliseconds
  Then the CLI removes the stale socket
  And spawns a fresh daemon before retrying

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Concurrent spawn attempts elect a single spawner via PID-file locking
  Given two CLI invocations both find no running daemon and attempt to spawn one
  When each spawn process acquires an exclusive flock on the daemon's PID file
  Then exactly one spawn process holds the lock
  And the lock holder publishes the socket and the PID file before releasing the lock
  And the loser waits up to 5 seconds for the socket to appear
  And if the socket does not appear within that bound, the loser exits with code 2 and a "daemon_unavailable" error

### Failure handling

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: The CLI retries once on a mid-request connection drop
  Given an in-flight lint request whose connection to the daemon is interrupted
  When the CLI detects the drop
  Then it spawns a fresh daemon and reissues the request once
  And a second failure produces an exit code 2 with the underlying cause

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A panic from the underlying type checker becomes a per-file recoverable error
  Given a file that triggers an internal panic in the type checker
  When the linter processes the project
  Then the panic is captured at the wrapper boundary
  And the offending file produces a tool-error diagnostic with its source range
  And linting continues for the remaining files in the same request
  And the daemon remains alive and serves the next request without restart

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A program build failure fails the entire invocation
  Given a project whose TypeScript program cannot be built (such as a broken configuration)
  When the daemon attempts to load the program
  Then the daemon returns a structured error to the CLI
  And the CLI exits with code 2 without producing partial diagnostics

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A target file outside the discovered program is reported and skipped
  Given a file that exists but is not part of the discovered TypeScript program
  When the user includes it in the lint request
  Then a structured per-file error is reported for that file
  And the linter continues to process the remaining files

### Engine execution model

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A single AST walk dispatches to all registered rule handlers
  Given multiple rules registered in the engine
  When a file is linted
  Then the file's AST is traversed exactly once
  And each visited node dispatches to every rule handler registered for that node kind

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: A modified file is re-parsed before being linted again
  Given a file that has been modified since its last lint
  When the linter is invoked again against that file
  Then the daemon detects the modification by comparing modification time and file size against its cached values
  And the file is re-parsed and re-checked before diagnostics are produced

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Edits within the same modification-time tick are still detected via secondary signal
  Given a file edited so quickly that its modification time is identical to the cached value
  When the linter is invoked again against that file
  Then the daemon's freshness check compares file size in addition to modification time
  And if both match the cached values, the cached parse may be reused
  And this limitation is documented in user-facing release notes so that AI agents and CI tooling can pass `--no-cache` or restart the daemon when they need a guaranteed fresh check

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Rules access type information only through the wrapper
  Given the rule implementations
  When the source is inspected
  Then no rule imports any package outside the linter's public rule API and the wrapper

### Performance

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: Warm-path benchmark enforces the budget in continuous integration
  Given the 500-file reference fixture and a warm daemon
  When the benchmark runs on the CI runner of record across 20 invocations
  Then the 95th percentile invocation time is at or below 200 milliseconds
  And the 99th percentile invocation time is at or below 400 milliseconds
  And the build fails when either threshold regresses

### Distribution

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: The linter ships as a single static binary across major platforms
  Given a release build
  When artifacts are produced
  Then a static binary is produced for linux amd64, linux arm64, darwin amd64, darwin arm64, and windows amd64
  And each artifact is published with a checksum

### Logging

# Living: none (initial implementation)
# Action: creates
# Status: TODO
# Living updated: NO
Scenario: The daemon writes per-project logs to a predictable location
  Given a running daemon
  When it produces log output
  Then the log file lives under the platform-appropriate state directory
  And the file is keyed by the project the daemon serves
  And no log output is sent off the local machine

## Affected Documentation

No existing documentation is affected by these changes. A README, a CONTRIBUTING document, and a `docs/json-schema.md` describing the diagnostic JSON contract will be created as part of the work but are produced fresh, not modified from prior content.

## Notes

### Architectural decisions captured during brainstorming

- **Two-repo layout (decision B').** The typescript-go fork exposes a narrow public API in `pkg/checker`, `pkg/ast`, `pkg/lint`. The linter at `~/src/lint` depends on the fork as a Go module. Local development uses a `replace` directive; releases pin a fork version. The wrapper is the only place that imports the fork's `internal/` packages.
- **Auto-managed per-project daemon.** Lifecycle is invisible to users and agents. The daemon is the product; cold-start amortization is the perf story.
- **Visitor-based rule API, compiled-in rules for MVP.** Plugin loading is deferred. The `Rule` interface stays stable so plugin loading is a future swap, not a redesign.
- **Decoupled cascades.** TypeScript program discovery (walk to nearest `tsconfig.json`) and lint config resolution (cascade `.tsgolintrc` files up the tree) are independent. A package can have its own tsconfig and inherit lint config, or vice versa.
- **Cascade lists replace, not concatenate.** Predictable child-wins semantics for list-typed settings; users wanting to extend a parent list can express that with explicit syntax in a future version.
- **Native diagnostic schema, formatter pipeline.** Diagnostics are an internal data type; formatters render them. JSON and human formats ship in MVP; SARIF and ESLint-compat are deferred but the interface admits them.
- **Differentiator rule: `no-base-to-string`.** Replaces the originally proposed `no-unsafe-enum-comparison` after weighing strategic concerns about the future of enums in the TypeScript ecosystem.
- **Degraded-mode signal when the program has type errors.** Lint proceeds, but a single tool-warning diagnostic at program scope tells AI agents the type graph is unsound so they can decide how to weight subsequent diagnostics.
- **Architecture test enforces the wrapper boundary.** Rather than an unfalsifiable "exports only what is needed" assertion, an import-path allowlist test fails the CI build when a rule reaches around the wrapper.
- **Tooling-failure error contract.** Exit code 2 covers all tooling failures; in JSON mode, a single-line JSON object on stderr names the failure class so AI agents can branch on cause without parsing prose.

### Out of scope for v0.1, deferred to later versions

- LSP server mode for editors (the daemon already does most of this work; LSP is protocol glue).
- True file watching with `fsnotify`. The MVP uses a modification-time-plus-size check on each request.
- Plugin loading for custom rules.
- Autofix.
- `--format sarif` and `--format eslint-compat`.
- Multi-language support (the long-term vision; the MVP exists to validate the daemon-plus-real-checker thesis on TypeScript first).
- Configurable per-rule options beyond on/off.
- Loose-file mode (linting files outside any TypeScript project).
- Log rotation and structured log shipping. The MVP writes per-project log files at a stable location; rotation, size caps, and JSON log lines are v0.2 scope.
- Wedged-daemon detection beyond a missing socket (e.g., daemon answers health probe but stalls under load). Acknowledged as a known gap; explicit retry-with-fresh-daemon escape hatch is the MVP workaround.
- "Extends" or "merges" semantics for list-typed settings in cascade resolution; v0.1 uses replace-only.

### Performance budget rationale

The warm-path budget is 200ms p95 and 400ms p99 over 20 runs of the 500-file reference fixture on the CI runner of record. The cold-start budget is 5 seconds on the same hardware. These numbers are intentionally tight for an MVP: the project's whole thesis is that a warm checker delivers sub-second feedback, and a budget that doesn't bind doesn't protect anything. They will tighten as optimizations land.

### Open questions for implementation

- Exact configuration file format (JSON, JSONC, TOML) — favor JSONC because it parallels `tsconfig.json`. Decide during implementation.
- Exact JSON-RPC framing — favor LSP-style framing (`Content-Length` header) for familiarity.
- Whether to ship the npm wrapper in v0.1 or defer to v0.2 — defer if it adds more than a day's work.
- The exact convention for "read file list from stdin" — `--files-from -` is a working placeholder; finalize during CLI design.
- The reference hardware for the "CI runner of record" budget — fix on first CI setup so subsequent regressions are comparable.
