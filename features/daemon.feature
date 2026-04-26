Feature: Per-project daemon lifecycle

  The tsgolint daemon is a long-lived process bound to one TypeScript
  project. The CLI auto-spawns it on first use, reuses it on subsequent
  invocations, and lets it shut itself down when idle. The daemon keeps
  the warm TypeScript program in memory across invocations, which is
  what makes the warm-path budget achievable.

  @technical
  Scenario: A per-project daemon is started on first request
    Given no daemon process is running for a particular TypeScript project
    When the linter is invoked against a file in that project
    Then a daemon process is started bound to that project
    And subsequent CLI invocations against the same project reuse the daemon

  @technical
  Scenario: The daemon socket path is derived deterministically from the project path
    Given two CLI invocations against the same project from any working directory
    When each computes the daemon socket path
    Then both compute the same socket path
    And the path lives under the platform-appropriate runtime directory

  @technical
  Scenario: An idle daemon shuts itself down after a configurable timeout
    Given a daemon that has received no requests for the configured idle period
    When the idle period elapses
    Then the daemon exits cleanly
    And its socket and PID file are removed

  @technical
  Scenario: A target file outside the discovered program is reported and skipped
    Given a file that exists but is not part of the discovered TypeScript program
    When the user includes it in the lint request
    Then a structured per-file warning is reported for that file
    And the linter continues to process the remaining files

  @technical
  Scenario: A panic from the underlying type checker becomes a per-file recoverable error
    Given a file that triggers an internal panic in the type checker
    When the linter processes the project
    Then the panic is captured at the wrapper boundary
    And the offending file produces a tool-error diagnostic with its source range
    And linting continues for the remaining files in the same request

  @technical
  Scenario: A modified file is re-parsed before being linted again
    Given a file that has been modified since its last lint
    When the linter is invoked again against that file
    Then v0.1 always loads a fresh program per CLI invocation
    And the file is re-parsed and re-checked before diagnostics are produced

  @technical
  Scenario: Edits within the same modification-time tick are still detected via secondary signal
    Given a file edited so quickly that its modification time is identical to the cached value
    When the linter is invoked again against that file
    Then v0.1 always loads a fresh program per CLI invocation, so no caching path is exposed to the same-tick race

  @technical
  Scenario: A request arriving during idle shutdown either completes or is cleanly rejected
    Given a daemon whose idle timer has fired and which has begun its shutdown sequence
    When a CLI request arrives before the daemon has released its socket
    Then the CLI's stale-socket recovery path detects the orphan socket and respawns a fresh daemon
    And the request is then served by the freshly spawned daemon

  @technical
  Scenario: The CLI retries once on a mid-request connection drop
    Given an in-flight lint request whose connection to the daemon is interrupted
    When the CLI detects the drop
    Then it re-runs EnsureRunning to spawn a fresh daemon and reissues the request once
    And a second failure produces an exit code 2 with the underlying cause

  @technical
  Scenario: A program build failure fails the entire invocation
    Given a project whose TypeScript program cannot be built (such as a broken configuration)
    When the daemon attempts to load the program
    Then the daemon returns a structured error to the CLI
    And the CLI exits with code 2 without producing partial diagnostics

  @technical
  Scenario: A stale socket from a crashed daemon is detected and replaced
    Given a socket file exists but no daemon process is responding
    When the CLI attempts to connect and its health probe fails to receive a response within 250 milliseconds
    Then the CLI removes the stale socket
    And spawns a fresh daemon before retrying

  @technical
  Scenario: Concurrent spawn attempts elect a single spawner via PID-file locking
    Given two CLI invocations both find no running daemon and attempt to spawn one
    When each spawn process acquires an exclusive flock on the daemon's PID file
    Then exactly one spawn process holds the lock
    And the lock holder publishes the socket and the PID file before releasing the lock
    And the loser waits up to 5 seconds for the socket to appear
