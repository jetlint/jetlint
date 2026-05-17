# AGENT.md — running notes for agents working in this repo

Terse, factual. Update via subagent when a new command or workflow is learned.

## Repo layout (jetlint.correctness-batch/)

- `cmd/jetlint` — CLI entrypoint.
- `cmd/oxlint-fixtures` — extractor for oxc/oxlint Rust test vectors → JSON fixtures.
- `cmd/biome-fixtures` (binary at repo root: `./biome-fixtures`) — extractor for Biome Rust test cases.
- `internal/rules/<pkg>/` — one Go package per rule. Each ships `rule.go` and at least one of `tselintcompat_test.go`, `eslintcompat_test.go`, or `biomecompat_test.go`.
- `internal/engine/` — single AST walk; dispatches to rules.
- `internal/checker/` (in `../typescript-go/pkg/checker`) — wrapper around TS7 native; see `docs/RULE-LOOP.md`.
- `testdata/typescript-eslint/<rule>.test.ts` — vendored upstream test fixtures.
- `testdata/eslint/<rule>.json` — JSON fixtures produced by `cmd/oxlint-fixtures`.
- `biome-fixtures/` — Biome rule fixtures (large dir).
- `docs/RULE-CATEGORIES.md` — group taxonomy.
- `docs/RULE-LOOP.md` — 14-step per-rule loop.
- `docs/{OXLINT,TSEC}-COMPAT-OVERVIEW.md` — per-rule scoring.

## VCS

- jj is the VCS. Bookmark `feat/big-batch` is the batch target.
- `jj st`, `jj log -r 'feat/big-batch..@'`, `jj show -s @`.
- New commit on top: `jj new feat/big-batch -m "..."`.
- Advance bookmark: `jj bookmark move feat/big-batch --to @-`.
- Push: `jj git push --bookmark feat/big-batch`.
- Wrapper repo (`../typescript-go/`) uses `git`.

## Build & test

- `go test ./...` — full suite (takes minutes).
- Single rule typescript-eslint: `go test -count=1 -run TypescriptEslintCompatibility -v ./internal/rules/<pkg>/`.
- Single rule oxlint/eslint-core: `go test -count=1 -run EslintCompatibility -v ./internal/rules/<pkg>/`.
- Single rule biome: `go test -count=1 -run BiomeCompatibility -v ./internal/rules/<pkg>/` (varies — check the rule's `*_test.go` for the actual `-run` regex).
- Go version: 1.26+.

## Fixture regeneration

- oxlint/eslint JSON fixtures: `go run ./cmd/oxlint-fixtures --oxc /tmp/oxc --out testdata/eslint --rule <id>` (clone oxc first: `git clone https://github.com/oxc-project/oxc /tmp/oxc`).
- Biome fixtures: see `./biome-fixtures` binary at repo root; ingestion scaffolding lives in `cmd/biome-fixtures`. Biome is cloned at `/tmp/biome`. Usage: `./biome-fixtures --biome /tmp/biome --out testdata/eslint --rule <kebab-id> --category <suspicious|complexity|style|...>`. Output lands at `testdata/eslint/<rule>.json` with a `biomeSHA` field for reproducibility. The kebab → camelCase conversion is direct (`no-confusing-void-type` → `noConfusingVoidType`), so rules biome names differently (e.g. `no-misused-new` lives as `noMisleadingInstantiator`) won't extract by their kebab id.

## Scripts

- `scripts/bootstrap.sh` — initial dev setup.
- `scripts/release.sh` — release flow.
- `scripts/build-npm.mjs`, `scripts/bootstrap-npm-publish.mjs` — npm packaging.
- `scripts/upstream-check.mjs` — checks upstream pins.

## Release

- npm scope: `@jetlint/*` (six packages: `cli`, `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`, `win32-x64`). See `RELEASING.md`.
- Tagging `v*` triggers `.github/workflows/release.yml`. Trusted publishing via OIDC, no `NPM_TOKEN` secret.
- `main` is PR-only, `enforce_admins`, required check `build-and-test`. No force pushes.

## Site

- Public docs site lives at `../jetlint.github.io/` (Astro). Use `git`, not `jj`. Production branch unconfirmed — record once observed.

## GitHub milestones for this batch

- #2 suspicious, #5 complexity, #6 style, #7 a11y.
- `gh issue list --repo jetlint/jetlint --milestone <n> --state open --json number,title`.
- Close with `gh issue close <n> --comment "..."`.
