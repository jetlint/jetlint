Feature: Linter configuration

  Linter behavior is governed by .tsgolintrc.json files that cascade up
  the directory tree from each linted file. Configuration is decoupled
  from tsconfig.json discovery so a project can override one without
  the other. Lists follow replace semantics: child fully supersedes
  parent for any setting it touches.

  @user
  Scenario: A project with no linter config uses sensible defaults
    Given a TypeScript project with no linter configuration file
    When the user runs the linter
    Then all five MVP rules are active at error severity
    And diagnostics are produced wherever rules find violations

  @user
  Scenario: Cascading configurations override defaults level by level
    Given a project with a root linter configuration
    And a child directory with its own linter configuration
    When the user lints a file inside the child directory
    Then the effective configuration is the merge of root and child
    And settings declared in the child win where they conflict

  @user
  Scenario: Cascade merge replaces lists rather than concatenating them
    Given a parent configuration that declares a list-typed setting
    And a child configuration that declares its own value for the same list-typed setting
    When the effective configuration is resolved for a file in the child's subtree
    Then the child's list value replaces the parent's list value entirely

  @user
  Scenario: A child configuration can disable a rule for its subtree
    Given a parent configuration that enables a rule
    And a child configuration that disables that rule
    When the user lints a file in the child's subtree
    Then no diagnostics from that rule are reported for files in the child's subtree
    And the rule continues to apply elsewhere

  @user
  Scenario: An invalid configuration file fails fast with location
    Given a linter configuration file that cannot be parsed or validated
    When the user runs the linter
    Then the linter exits with code 2 before linting any file
    And the failure message identifies the file and the offending location

  @user
  Scenario: An unknown rule name in configuration fails fast
    Given a linter configuration that references a rule the linter does not ship
    When the user runs the linter
    Then the linter exits with code 2 before linting any file
    And the failure message names the unknown rule
