# jetlint

> A fast, type-aware TypeScript 7 linter. Drop-in compatible with typescript-eslint.

**Site & docs:** https://jetlint.github.io

jetlint is a Go-based linter built on the TypeScript 7 native compiler
([microsoft/typescript-go][tsgo]). It ports typescript-eslint's
type-aware rules to a long-lived daemon model, paying the cost of loading
TypeScript once and returning subsequent lints in milliseconds.

**Status:** pre-1.0. Diagnostics are real and validated against
typescript-eslint's published test fixtures (currently 6193/6193 cases,
100%). The CLI surface and configuration schema may still change.

## Highlights

- **61 type-aware rules**, every one verified against the upstream
  typescript-eslint test suite.
- **6193/6193 fixtures pass** &mdash; same diagnostics as
  typescript-eslint, byte-for-byte.
- **Native checker:** built on TypeScript 7 native (typescript-go), not
  AST heuristics standing in for type queries.
- **Long-lived daemon:** the program and checker stay warm between
  runs; re-lints return in milliseconds.
- **Predictable config:** unknown option keys exit with code `2` and a
  structured error.

## Install (from source)

Requires **Go 1.26+**.

```bash
go install github.com/jetlint/jetlint/cmd/jetlint@latest
```

A pre-built binary distribution will land closer to the 1.0 release.

### External-build caveat

This pre-1.0 source tree currently uses a local sibling checkout of
[jetlint/typescript-go][tsgo-fork] via a `replace` directive in
`go.mod`. Building from a fresh `go install` outside the development
worktree isn't supported yet &mdash; the wrapper APIs in
`pkg/checker` haven't all landed in [microsoft/typescript-go][tsgo].
Stabilizing the dependency path (either upstreaming the wrapper or
tagging the fork) is a 1.0 prerequisite.

To build locally:

```bash
git clone git@github.com:jetlint/jetlint.git
git clone -b tsgolint-wrapper git@github.com:jetlint/typescript-go.git
cd jetlint
go build ./cmd/jetlint
```

## Run

```bash
jetlint --project ./tsconfig.json
```

The **5 MVP rules** default to `error`. The other **56 rules** ship `off`
&mdash; opt in via [`.jetlintrc.json`](https://jetlint.github.io/config/).

## Configuration

`.jetlintrc.json` at the project root:

```json
{
  "rules": {
    "no-array-delete": "error",
    "only-throw-error": ["error", { "allowThrowingAny": false }]
  }
}
```

See [the config docs](https://jetlint.github.io/config/) for the full
schema.

## Compatibility

Per-rule scores against typescript-eslint's published fixtures live in
[`docs/TSEC-COMPAT-OVERVIEW.md`](docs/TSEC-COMPAT-OVERVIEW.md). All 61
rules currently sit at 100%.

Reproduce a single rule's score:

```bash
go test -count=1 -run TypescriptEslintCompatibility -v \
  ./internal/rules/<package>/
```

## Repository layout

```
cmd/
  jetlint/         CLI entrypoint
  probe/           diagnostic helper
internal/
  engine/          single AST walk, dispatches to rules
  rules/           61 rule implementations (one package each)
  daemon/          long-lived sidecar (JSON-RPC over stdio)
  transport/       JSON-RPC framing
  cli/             argv parsing, exit codes
  config/          .jetlintrc.json parser
  format/          diagnostic output formatters
  project/         tsconfig resolution
  bootstrap/       Program loading
  architecture/    cross-package layering checks
  tselintcompat/   upstream test-fixture loader (the 6193 cases)
  toolerr/         exit-code-bearing error types
testdata/          vendored typescript-eslint fixtures
docs/              per-rule compat notes
plans/             plan files (historical record)
features/          living Gherkin specs
```

## License

MIT &mdash; see [LICENSE](LICENSE).

[tsgo]: https://github.com/microsoft/typescript-go
[tsgo-fork]: https://github.com/jetlint/typescript-go
