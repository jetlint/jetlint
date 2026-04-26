Feature: Wrapper-API architectural boundary

  The linter consumes the typescript-go checker only through a narrow
  public API exposed in the fork's pkg/ tree. Rule packages may not reach
  around the wrapper into the fork's internal/ packages, and the linter
  as a whole may not import internal/ paths from the fork. These
  invariants are enforced by build-time tests so that drift is detected
  immediately rather than at upstream-rebase time.

  @technical
  Scenario: The linter depends on the typescript-go fork via a Go module
    Given the linter repository
    When the build is performed
    Then the binary statically links against the fork's exported packages
    And no source file in the linter imports from internal packages of the fork

  @technical
  Scenario: Rules access type information only through the wrapper
    Given the rule implementations
    When the source is inspected
    Then no rule imports any package outside the linter's public rule API and the wrapper

  @technical
  Scenario: An architecture test enforces the wrapper-API boundary
    Given a checked-in allowlist of import paths permitted from rule packages
    When the architecture test runs in continuous integration
    Then the test scans every Go source file in the linter
    And the test fails the build if any rule source file imports a package outside the wrapper allowlist
    And the test fails the build if any source file in the linter imports the typescript-go fork's internal packages
