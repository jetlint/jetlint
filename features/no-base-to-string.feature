Feature: no-base-to-string rule

  Interpolating an object that does not declare its own toString into
  a template literal renders "[object Object]" — the universal "I
  forgot to do something" string. The rule checks each template-span
  expression's type and flags the cases that would produce default
  Object.prototype.toString output.

  @user
  Scenario: Should report a template literal that interpolates an object lacking a custom string conversion
    Given an object whose type does not declare a custom string-conversion method
    And the object is interpolated into a template literal
    When the user runs the linter
    Then a diagnostic is reported at the interpolation
    And the diagnostic explains that the result will be the default object representation

  @user
  Scenario: Should not report interpolation of values with a meaningful string conversion
    Given an object whose type declares its own string-conversion method
    And the object is interpolated into a template literal
    When the user runs the linter
    Then no base-to-string diagnostic is reported

    And given a primitive value interpolated into a template literal
    Then no base-to-string diagnostic is reported for the primitive
