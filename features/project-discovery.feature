Feature: TypeScript project discovery

  The linter walks upward from each target file to find the governing
  tsconfig.json. The walk stops at the filesystem root; absent a
  tsconfig the linter refuses to operate rather than guessing.

  @user
  Scenario: The linter discovers the governing TypeScript project automatically
    Given a TypeScript file inside a project tree
    When the user runs the linter against that file
    Then the linter discovers the project's TypeScript configuration without explicit user input
    And the file is linted in the context of that project

  @user
  Scenario: A directory with no TypeScript project produces a clear error
    Given a directory with no discoverable TypeScript project configuration
    When the user runs the linter from that directory
    Then the linter exits with code 2
    And the error message explains that a TypeScript project configuration is required
