Feature: Daemon logging

  Each daemon writes a per-project log file at a predictable location
  so operators (and AI agents triaging a stuck environment) can find
  lifecycle events without running the daemon under a debugger. No log
  output ever leaves the local machine.

  @technical
  Scenario: The daemon writes per-project logs to a predictable location
    Given a running daemon
    When it produces log output
    Then the log file lives under the platform-appropriate state directory
    And the file is keyed by the project the daemon serves
    And no log output is sent off the local machine
