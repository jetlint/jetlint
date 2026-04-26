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
