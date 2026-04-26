Feature: Degraded mode signal for unsound programs

  When the TypeScript program contains type errors, the lint
  diagnostics built on top of it are unreliable: the type checker is
  effectively reasoning over a graph that has nothing to ground it.
  The linter still produces diagnostics so refactoring users get
  feedback, but it emits a single program-scope tool-warning so AI
  agents and CI gates can detect the degraded state and decide how
  to weight the rest of the output.

  @user
  Scenario: Linting proceeds in degraded mode when the program has type errors, with an explicit signal
    Given a TypeScript project whose program currently fails to type-check
    When the user runs the linter
    Then a single tool-warning diagnostic at program scope is emitted noting that type-check errors are present in the program
    And lint diagnostics are still produced for the files in the request
    And the tool-warning diagnostic appears in both human and JSON output so that an AI agent can detect the degraded mode and decide whether to trust subsequent diagnostics
