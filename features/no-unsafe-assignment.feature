Feature: no-unsafe-assignment rule

  An any-typed value assigned into a more specific declared type
  laundering the lack of type information into apparently-typed code is
  one of the most common sources of "the type checker said it was OK
  and it crashed at runtime" bug reports. The rule catches the most
  common shape: variable declarations with a more specific declared
  type whose initializer is typed `any`.

  @user
  Scenario: Should report an assignment from a value typed any to a typed target
    Given a source value typed as any
    And the value is assigned to a variable with a more specific declared type
    When the user runs the linter
    Then a diagnostic is reported at the assignment
    And the diagnostic explains that type information was lost or fabricated

  @user
  Scenario: Should not report an assignment between properly typed values
    Given a source and target whose types are both fully known and compatible
    When the user runs the linter
    Then no unsafe-assignment diagnostic is reported

  @user
  Scenario: Should not report an assignment from any to a target also typed any
    Given a source value typed as any
    And a target also typed as any
    When the assignment occurs
    And the user runs the linter
    Then no unsafe-assignment diagnostic is reported
