# Third-party notices

jetlint vendors test fixtures derived from third-party projects. The
fixtures live under `testdata/` and are used only for compatibility
testing — they do not ship in any released binary.

## typescript-eslint

`testdata/typescript-eslint/` contains test files copied verbatim from
[typescript-eslint](https://github.com/typescript-eslint/typescript-eslint),
licensed under the MIT License. See the upstream LICENSE at
<https://github.com/typescript-eslint/typescript-eslint/blob/main/LICENSE>.

## oxc

`testdata/eslint/` contains JSON fixtures extracted by
`cmd/oxlint-fixtures` from the Rust sources at
`crates/oxc_linter/src/rules/eslint/*.rs` in the
[oxc](https://github.com/oxc-project/oxc) project, licensed under the
MIT License. The pass/fail vectors in those Rust files are derived
from ESLint core's own test cases plus oxlint-authored additions.

Pinned upstream revision: `7cb5ac96e1fbb59deb6e57dd60226ac80318dcdf`.

See the upstream LICENSE at
<https://github.com/oxc-project/oxc/blob/main/LICENSE>.

## TypeScript / typescript-go

jetlint depends on a vendored fork of
[microsoft/typescript-go](https://github.com/microsoft/typescript-go),
which is itself derived from
[microsoft/TypeScript](https://github.com/microsoft/TypeScript). Both
are licensed under the Apache License, Version 2.0. See the upstream
LICENSE files in their respective repositories.
