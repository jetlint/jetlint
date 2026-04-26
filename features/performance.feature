Feature: Performance budgets

  The linter's whole thesis is that a warm Go-native type checker
  delivers sub-second feedback on a real-world TypeScript project.
  These budgets are enforced as Go tests so a regression fails the
  build, not someone's debugging session three weeks later.

  @user
  Scenario: First invocation against a project completes within the cold-start budget
    Given a project the linter has not seen since system boot
    And the project is the 500-file reference fixture
    When the user runs the linter for the first time
    Then the invocation completes within 5 seconds on the CI runner of record
    And the linter's startup cost is incurred only on this first call

  @user
  Scenario: Subsequent invocations complete within the warm-path budget
    Given a project the linter has already analyzed in this session
    And the project is the 500-file reference fixture
    When the user runs the linter again
    Then the invocation completes within 200 milliseconds at the 95th percentile across 20 runs on the CI runner of record
    And no startup cost is paid

  @technical
  Scenario: Warm-path benchmark enforces the budget in continuous integration
    Given the 500-file reference fixture and a warm daemon
    When the benchmark runs on the CI runner of record across 20 invocations
    Then the 95th percentile invocation time is at or below 200 milliseconds
    And the 99th percentile invocation time is at or below 400 milliseconds
    And the build fails when either threshold regresses
