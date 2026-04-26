Feature: Command-line interface

  The tsgolint binary exposes a command-line interface for human and AI
  consumers. Information flags such as --version and --help report
  immediately without contacting any daemon.

  @user
  Scenario: --version prints a stable version string and exits zero
    Given the linter binary
    When the user runs the linter with the --version flag
    Then the binary prints a single-line semantic version string
    And the process exits with code 0
    And no daemon is started or contacted

  @user
  Scenario: --help prints stable usage and exits zero
    Given the linter binary
    When the user runs the linter with the --help flag
    Then the usage text is printed listing supported commands, flags, and exit codes
    And the process exits with code 0
    And no daemon is started or contacted
