Feature: Output formatting

  The linter renders its diagnostics through a Formatter pipeline so the
  same finding can be expressed for humans (terminals, code review) or
  for machines (AI agents, CI gates). The JSON formatter is the
  machine-stable contract the project's AI-determinism thesis depends on.

  @user
  Scenario: Default output format is human-readable
    Given a project with violations
    When the user runs the linter without specifying an output format
    Then diagnostics are formatted for terminal display
    And each diagnostic shows file path, position, rule, and message in a scannable layout

  @user
  Scenario: JSON format produces a fully specified diagnostic contract
    Given a project with violations
    When the user runs the linter requesting JSON output
    Then the output carries a schema version identifying the contract
    And each diagnostic includes the fields file, startLine, startColumn, endLine, endColumn, ruleId, severity, and message
    And line numbers are one-indexed
    And column numbers are one-indexed UTF-16 code units (matching the Language Server Protocol convention)
    And severity is one of the values "error" or "warning"
    And diagnostics are ordered by file path, then by start position

  @user
  Scenario: Repeated runs over unchanged input produce byte-identical JSON output
    Given a project and configuration that have not changed
    When the linter is invoked twice in JSON output mode
    Then the byte sequences emitted by the two invocations are identical
    And no field in the output depends on wall-clock time, process identifiers, or invocation ordering

  @user
  Scenario: Unknown output format fails with a clear message
    Given the user requests an output format the linter does not support
    When the linter starts
    Then it exits with code 2 before producing any output
    And the error message lists the supported formats

  @user
  Scenario: Tooling failure in JSON mode emits a structured machine-readable error
    Given the linter is invoked with the JSON output format
    And a tooling failure occurs
    When the linter exits
    Then a single-line JSON object is written to stderr
    And the object includes a `code` field whose value is one of the documented failure codes
    And the object includes a `message` field with a human-readable description
    And the object includes a `path` field naming the offending file when one applies
    And the process exits with code 2

  @user
  Scenario: Tooling failure exits two with the cause on stderr
    Given a condition that prevents the linter from completing (such as a broken project configuration)
    When the user runs the linter
    Then the underlying error is written to stderr with enough context to act on
    And the process exits with code 2
    And no lint diagnostics are emitted from the failed invocation

  @user
  Scenario: Concurrent invocations produce results referentially identical to sequential runs
    Given two simultaneous lint invocations targeting the same project
    When both invocations run to completion
    Then the union of diagnostics returned to the two callers equals the diagnostics that a single sequential invocation over the same files would produce
    And neither invocation's output contains data from the other invocation's request

  @user
  Scenario: A clean project exits zero with no diagnostics
    Given a TypeScript project with no rule violations
    When the user runs the linter against the project
    Then the linter prints no diagnostics
    And the process exits with code 0
