Feature: strict-boolean-expressions rule

  Truthiness checks against types that are not strictly boolean hide
  intent and produce subtle bugs (an empty string is falsy, but is the
  developer treating that case the way they meant to?). The rule fires
  on any boolean-context expression whose type is not exactly boolean
  (or a union of boolean literals).

  @user
  Scenario: Should report a boolean test of a value that may be string-or-undefined
    Given a value typed as a string union with undefined
    And the value is used directly as a boolean test
    When the user runs the linter
    Then a diagnostic is reported at the test location
    And the diagnostic identifies the ambiguous coercion

  @user
  Scenario: Should not report a boolean test of a value narrowed by a prior guard
    Given a value typed as a string union with undefined
    And a prior guard such as `if (x !== undefined)` has narrowed the value to a string in the current control-flow branch
    When the value is used as a boolean test inside that branch
    And the user runs the linter
    Then no strict-boolean-expressions diagnostic is reported
    Because the narrowed type is no longer ambiguous

  @user
  Scenario: Should not report a boolean test of a value typed as boolean
    Given a value typed as boolean
    And the value is used as a boolean test
    When the user runs the linter
    Then no strict-boolean-expressions diagnostic is reported
