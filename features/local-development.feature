Feature: Local development bootstrap

  The bootstrap command prepares a freshly cloned linter repository for
  development work by verifying prerequisites, building the binary, and
  confirming the toolchain functions end to end.

  @user
  Scenario: Bootstrap verifies toolchain prerequisites
    Given a freshly cloned linter repository
    When the developer runs the bootstrap command
    Then the required Go toolchain version is detected and confirmed acceptable
    And the typescript-go fork checkout is detected at the expected sibling path
    And missing prerequisites cause the bootstrap to halt before any build step

  @user
  Scenario: Bootstrap builds the linter binary
    Given the toolchain prerequisites have been verified
    When the developer runs the bootstrap command
    Then the linter binary is produced at the project's bin directory
    And the binary reports its version via --version

  @user
  Scenario: Bootstrap smoke test confirms end-to-end function
    Given the linter binary has been built
    When the bootstrap script runs the smoke lint against a checked-in fixture project
    Then the expected diagnostic is produced
    And the bootstrap exits successfully

  @user
  Scenario: Bootstrap fails fast when prerequisites are missing
    Given a freshly cloned linter repository
    And the required toolchain version is not present
    When the developer runs the bootstrap command
    Then the bootstrap halts before doing further work
    And the missing prerequisite is named in the failure message
