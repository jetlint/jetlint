Feature: no-floating-promises rule

  The no-floating-promises rule flags expression statements whose value
  is a Promise (or any thenable) that is not awaited, returned, voided,
  or otherwise handled. Cross-file detection is the rule's headline
  capability: a function imported from another module that returns a
  Promise must be handled at the call site, even though the Promise
  shape is only visible through the type checker.

  @user
  Scenario: Should report an unawaited promise returned from an imported function
    Given a project where a function defined in one file returns a promise
    And the function is called in another file without awaiting or otherwise handling its result
    When the user runs the linter
    Then a diagnostic is reported at the unawaited call site
    And the diagnostic is attributed to the floating-promises rule

  @user
  Scenario: Should not report a promise that is awaited or explicitly handled
    Given a call to a promise-returning function that is awaited
    When the user runs the linter
    Then no floating-promises diagnostic is reported

  @user
  Scenario: Should not report a promise that is explicitly discarded with the void operator
    Given a call to a promise-returning function whose result is prefixed with the void operator
    When the user runs the linter
    Then no floating-promises diagnostic is reported
    And this matches the established eslint-plugin convention for explicit fire-and-forget intent
