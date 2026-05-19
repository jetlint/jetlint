# jetlint

> A fast, type-aware TypeScript 7 linter. Rule-compatible with typescript-eslint.

**Site & docs:** https://jetlint.github.io

jetlint is a Go-based linter built on the TypeScript 7 native compiler
([microsoft/typescript-go][tsgo]). It ports typescript-eslint's
type-aware rules to a long-lived daemon model, paying the cost of loading
TypeScript once and returning subsequent lints in milliseconds.

**Status:** pre-1.0. Diagnostics are real and validated against
typescript-eslint's published test fixtures (currently 6193/6193 cases,
100%). The CLI surface and configuration schema may still change.

## Highlights

- **67 rules**: 61 type-aware ports of typescript-eslint plus 6
  non-type-aware correctness rules ported from ESLint core
  (`no-dupe-keys`, `no-duplicate-case`, `no-self-assign`, `no-self-compare`,
  `use-isnan`, `valid-typeof`).
- **6193/6193 typescript-eslint fixtures pass** &mdash; same
  diagnostics as typescript-eslint, byte-for-byte.
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

`go.mod` pins [jetlint/typescript-go][tsgo-fork] via `replace` because
the wrapper APIs jetlint depends on haven't landed in
[microsoft/typescript-go][tsgo] yet. The pin is by tagged release, not
by local checkout, so `go install` and fresh clones build without extra
setup.

## Run

Point `--project` at a tsconfig (or a directory containing one):

```bash
jetlint --project ./tsconfig.json
```

Or pass files and directories directly &mdash; the nearest enclosing
`tsconfig.json` is discovered by walking upward:

```bash
jetlint src/index.ts
jetlint .
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
  },
  "ignorePatterns": [
    "**/generated/**",
    "packages/*/lib/**/*.gen.ts",
    "!packages/keep/lib/api/keep.gen.ts"
  ]
}
```

`ignorePatterns` is a gitignore-flavored list of doublestar globs.
Matching files stay part of the TypeScript program (so their type
information is still available to importers) but emit no diagnostics. A
leading `!` un-ignores. Patterns resolve relative to the directory of
the `.jetlintrc.json` that contains them.

See [the config docs](https://jetlint.github.io/config/) for the full
schema.

## Compatibility

Per-rule scores against typescript-eslint's published fixtures live in
[`docs/TSEC-COMPAT-OVERVIEW.md`](docs/TSEC-COMPAT-OVERVIEW.md). All 61
typescript-eslint ports currently sit at 100%. The six ESLint-core
ports (`no-dupe-keys`, `no-duplicate-case`, `no-self-assign`,
`no-self-compare`, `use-isnan`, `valid-typeof`) ship with hand-rolled
unit tests since ESLint core does not publish a machine-readable
fixture format.

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
  rules/           67 rule implementations (one package each)
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
