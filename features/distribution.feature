Feature: Cross-platform distribution

  The linter ships as a single static binary on every platform the
  release matrix covers, so a developer or AI agent can drop it in,
  point it at a project, and use it without language-runtime fuss.

  @technical
  Scenario: The linter ships as a single static binary across major platforms
    Given a release build
    When artifacts are produced
    Then a static binary is produced for linux amd64, linux arm64, darwin amd64, darwin arm64, and windows amd64
    And each artifact is published with a checksum
