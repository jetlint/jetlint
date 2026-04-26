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
  Scenario: A list of files supplied on the command line is linted as a batch
    Given the user runs the linter with one or more file paths as positional arguments
    When the linter resolves each path against its discovered TypeScript program
    Then only the named files are linted
    And diagnostics are emitted for those files in the JSON output's documented order

  @user
  Scenario: A list of files supplied on standard input is linted as a batch
    Given the user invokes the linter with the convention that signals "read file list from stdin" (--files-from -)
    And a newline-separated list of file paths is written to standard input
    When the linter completes
    Then only the files named on standard input are linted
    And the same output contract applies as for positional arguments

  @user
  Scenario: --help prints stable usage and exits zero
    Given the linter binary
    When the user runs the linter with the --help flag
    Then the usage text is printed listing supported commands, flags, and exit codes
    And the process exits with code 0
    And no daemon is started or contacted
