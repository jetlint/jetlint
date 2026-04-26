Feature: no-misused-promises rule

  An async function passed where a void-return callback is expected
  silently drops its returned Promise. The classic shape is
  arr.forEach(async () => ...). The rule examines the contextual
  parameter type for each call argument and flags async callbacks
  whose contextual type expects void.

  @user
  Scenario: Should report an async callback passed where the consumer expects no return
    Given an array iteration that accepts a callback returning nothing
    And the user passes an async callback to that iteration
    When the user runs the linter
    Then a diagnostic is reported at the call site
    And the diagnostic explains that the returned promise will be silently dropped

  @user
  Scenario: Should not report an async callback passed where a promise return is expected
    Given an iteration whose callback type accepts a promise return (such as mapping)
    And the user passes an async callback to that iteration
    When the user runs the linter
    Then no misused-promises diagnostic is reported
