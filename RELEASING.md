# Releasing jetlint

This document captures everything that must be true **before pushing the first
`v*` tag**. Tagging a release triggers `.github/workflows/release.yml`, which
builds platform binaries and publishes the npm packages described in
[`npm/`](npm/). Until every prerequisite below is satisfied, do not tag.

## One-time prerequisites (manual, off-repo)

These configurations live on npmjs.com and on a separate monitoring repo. They
do not change with each release.

### 1. Reserve the names on npm

Under a publisher account with TOTP-based 2FA on `auth-and-writes`:

- Reserve the unscoped `jetlint` name (placeholder publish or via npm support).
- Create the `@jetlint` scope.
- Restrict the maintainer list. One or two publishers, no more. Every
  additional maintainer is another credential-theft target with the same blast
  radius.

### 2. Configure trusted publishing on npm — **DONE (2026-05-12)**

The unscoped `jetlint` name was rejected by npm's similarity check (too
close to `eslint`), so all six packages live under the `@jetlint` scope:

- `@jetlint/cli`           — the wrapper users install
- `@jetlint/linux-x64`
- `@jetlint/linux-arm64`
- `@jetlint/darwin-x64`
- `@jetlint/darwin-arm64`
- `@jetlint/win32-x64`

Each one was bootstrap-published at `v0.0.0` (via
`scripts/bootstrap-npm-publish.mjs`) so its settings page would exist, then
deprecated, then bound to:

- Repository: `jetlint/jetlint`
- Workflow filename: `release.yml`
- Environment: _(none)_

Publishing access on each is set to **"Require two-factor authentication
and disallow tokens"**, so the only paths to publish are OIDC trusted
publishing from this workflow or interactive WebAuthn from a maintainer's
shell. `NODE_AUTH_TOKEN`/`NPM_TOKEN` are not used and no `NPM_TOKEN` secret
should exist in the repo.

### 3. Stand up publish-alert monitoring

Set up an out-of-band watcher that pings you whenever a new version appears on
npm under `jetlint` or `@jetlint/*`. Two cheap options:

- A separate repo with a scheduled workflow that polls
  `https://registry.npmjs.org/jetlint` (and each scoped package), diffs the
  version list against a checked-in baseline, and opens an issue / posts a
  webhook on any new version that wasn't tagged in this repo within the last
  hour.
- An RSS reader subscribed to the registry feeds, with notifications.

The TanStack postmortem (2026-05-11) put the detection gap at 20 minutes from
malicious publish to public disclosure by a third party. This monitor closes
that gap from your side.

### 4. Branch protection on `main` — **DONE (2026-05-12)**

Current configuration (`gh api /repos/jetlint/jetlint/branches/main/protection`):

- PR required to merge to `main`. `required_approving_review_count = 0`
  for solo-maintainer flow — direct pushes blocked, but no second human
  needed to merge own PRs. Bump to `1` when a second maintainer joins.
- Required status check: `build-and-test` (the only job in `ci.yml`).
  `strict: true`, so PRs must be up to date with `main` before CI can
  satisfy the check.
- `enforce_admins: true` — even repo admins must go through PRs and
  passing CI. No bypass for the owner.
- `allow_force_pushes: false`, `allow_deletions: false`.
- `dismiss_stale_reviews: true`, `required_conversation_resolution: true`.

A repository ruleset additionally protects `v*` tags from deletion,
non-fast-forward updates, and rewrites — so a malicious or accidental
`git push --force origin v0.1.0` cannot rewrite a release tag after the
fact, which would otherwise let an attacker swap published binaries on a
re-publish without npm noticing.

## Per-release checklist

Once the four prerequisites above are in place, every release follows this
sequence:

1. Verify CI is green on `main`.
2. Update `CHANGELOG.md` (if present) and commit on `main`.
3. Pick the semver tag: `vMAJOR.MINOR.PATCH` (no prerelease suffix until
   the workflow's regex is loosened intentionally). While the project is
   in 0.1.x, releases stay on `0.1.y` — patch-bump regardless of how
   additive or wrapper-bumpy the diff looks. The minor digit will be
   promoted to `0.2` by an explicit, separate decision; default to a
   patch until that decision is recorded here.
4. `git tag -s v0.X.Y && git push origin v0.X.Y`
   (signed tag preferred; the workflow regex-validates the tag name).
5. Watch the publish-alert monitor and the workflow run side by side. If
   anything unexpected appears on npm before the workflow finishes, the
   monitor will fire first and you should assume compromise.
6. After the workflow completes, smoke-test on one platform you didn't build
   on:
   ```bash
   npm init -y && npm install jetlint@0.X.Y && npx jetlint --version
   ```

## Incident response sketch

If a malicious version is detected on npm:

1. `npm deprecate jetlint@<bad-version> "compromised - do not use"`
   (and likewise for each affected `@jetlint/*`).
2. Open a GitHub Security Advisory in this repo.
3. Contact npm support to request server-side tarball removal. npm's
   "no unpublish if dependents exist" policy may make local `npm unpublish`
   unavailable; deprecation is the user-visible signal in the meantime.
4. Rotate every credential reachable from any host that may have installed
   the bad version (AWS, GCP, Kubernetes, Vault, GitHub, npm, SSH).
5. Revoke the trusted-publisher binding until the root cause is understood.
6. Postmortem in `docs/postmortems/` covering timeline, root cause, blast
   radius, and which mitigations would have prevented or detected it sooner.
