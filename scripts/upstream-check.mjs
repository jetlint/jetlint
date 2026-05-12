#!/usr/bin/env node
// Compare jetlint's vendored typescript-eslint fixtures against upstream.
//
// This script is the body of the daily upstream-check workflow. It has
// three subcommands, each cheap-to-expensive:
//
//   fingerprint         Cheap: fetch upstream main HEAD, sha256 every
//                       fixture file that exists in testdata/typescript-eslint/,
//                       diff against testdata/upstream-baseline.json.
//                       Writes a structured report to FINGERPRINT_OUT (or
//                       stdout) and exits 0 in all cases except hard
//                       errors. The report includes a `changed` flag.
//
//   sync <commit>       Mutating: pull upstream files at <commit> down
//                       into testdata/typescript-eslint/. Used by the
//                       workflow only when fingerprint reports changed.
//
//   parse-harness <log> Parse `go test` output for the typescript-eslint
//                       compatibility harness and emit per-rule pass
//                       counts as JSON. The harness logs lines like
//                         typescript-eslint compatibility: 218/218 passed (100.0%)
//                       from each rule's _test.go file.
//
//   issue-body <fp> <hr> Compose the GitHub Issue body that the workflow
//                       posts. Inputs: fingerprint report json + harness
//                       report json (the latter may be missing if the
//                       harness step was skipped).
//
// All network access goes through the GitHub REST API (no git clone) so
// the script runs in seconds and only needs an unauthenticated rate-limit
// budget (workflow uses GITHUB_TOKEN to lift it).

import { readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync } from "node:fs";
import { resolve, dirname, join, basename } from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, "..");
const VENDORED_DIR = join(REPO_ROOT, "testdata/typescript-eslint");
const BASELINE_PATH = join(REPO_ROOT, "testdata/upstream-baseline.json");

const UPSTREAM_OWNER = "typescript-eslint";
const UPSTREAM_REPO = "typescript-eslint";
const UPSTREAM_RULES_PATH = "packages/eslint-plugin/tests/rules";

function sha256(s) {
  return createHash("sha256").update(s).digest("hex");
}

function readJson(p) {
  return JSON.parse(readFileSync(p, "utf8"));
}

function writeJson(p, data) {
  writeFileSync(p, JSON.stringify(data, null, 2) + "\n");
}

function localFixtureNames() {
  return readdirSync(VENDORED_DIR)
    .filter((f) => f.endsWith(".test.ts"))
    .sort();
}

async function ghApi(path) {
  const url = `https://api.github.com${path}`;
  const headers = {
    "User-Agent": "jetlint-upstream-check",
    Accept: "application/vnd.github+json",
  };
  if (process.env.GITHUB_TOKEN) {
    headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  }
  const res = await fetch(url, { headers });
  if (!res.ok) {
    throw new Error(`GitHub API ${path} → ${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function fetchRaw(commitSha, repoPath) {
  // raw.githubusercontent.com is unauthenticated but rate-limited by IP.
  // For a daily run touching ~60 files, this is fine.
  const url = `https://raw.githubusercontent.com/${UPSTREAM_OWNER}/${UPSTREAM_REPO}/${commitSha}/${repoPath}`;
  const res = await fetch(url, {
    headers: { "User-Agent": "jetlint-upstream-check" },
  });
  if (!res.ok) {
    throw new Error(`raw.githubusercontent.com ${repoPath} → ${res.status}`);
  }
  return res.text();
}

async function cmdFingerprint() {
  const baseline = readJson(BASELINE_PATH);
  const localFiles = localFixtureNames();
  if (localFiles.length === 0) {
    throw new Error(`no .test.ts files found under ${VENDORED_DIR}`);
  }

  // Resolve upstream main HEAD to a commit SHA.
  const ref = await ghApi(
    `/repos/${UPSTREAM_OWNER}/${UPSTREAM_REPO}/git/ref/heads/main`,
  );
  const upstreamSha = ref.object.sha;

  const upstream = {};
  for (const name of localFiles) {
    const content = await fetchRaw(upstreamSha, `${UPSTREAM_RULES_PATH}/${name}`).catch(
      (err) => {
        // A fixture jetlint vendored may have been renamed or removed upstream.
        return { __missing: true, __error: err.message };
      },
    );
    if (typeof content === "object" && content.__missing) {
      upstream[name] = { missing_upstream: true };
    } else {
      upstream[name] = { hash: sha256(content) };
    }
  }

  const baselineHashes = baseline.file_hashes || {};
  const drift = { added: [], removed: [], changed: [], missing_upstream: [] };
  for (const name of localFiles) {
    const u = upstream[name];
    const bh = baselineHashes[name];
    if (u.missing_upstream) {
      drift.missing_upstream.push(name);
      continue;
    }
    if (bh === undefined) {
      drift.added.push(name);
    } else if (bh !== u.hash) {
      drift.changed.push(name);
    }
  }
  for (const name of Object.keys(baselineHashes)) {
    if (!upstream[name]) drift.removed.push(name);
  }

  const baselineEmpty = baseline.pinned_commit === null;
  const changed =
    baselineEmpty ||
    drift.added.length > 0 ||
    drift.changed.length > 0 ||
    drift.removed.length > 0 ||
    drift.missing_upstream.length > 0;

  const report = {
    schema: "jetlint.upstream-check.v1",
    baseline_pinned_commit: baseline.pinned_commit,
    upstream_main_commit: upstreamSha,
    upstream_main_url: `https://github.com/${UPSTREAM_OWNER}/${UPSTREAM_REPO}/commit/${upstreamSha}`,
    baseline_empty: baselineEmpty,
    changed,
    drift,
    upstream_file_hashes: Object.fromEntries(
      Object.entries(upstream).map(([k, v]) => [k, v.hash ?? null]),
    ),
    generated_at: new Date().toISOString(),
  };

  const out = process.env.FINGERPRINT_OUT;
  if (out) {
    mkdirSync(dirname(out), { recursive: true });
    writeJson(out, report);
  }
  process.stdout.write(JSON.stringify(report, null, 2) + "\n");
}

async function cmdSync(commit) {
  if (!commit) throw new Error("sync requires <commit>");
  const localFiles = localFixtureNames();
  for (const name of localFiles) {
    const content = await fetchRaw(commit, `${UPSTREAM_RULES_PATH}/${name}`).catch(
      () => null,
    );
    if (content === null) {
      console.error(`skip ${name}: not present upstream at ${commit}`);
      continue;
    }
    writeFileSync(join(VENDORED_DIR, name), content);
    console.log(`synced ${name}`);
  }
}

function cmdParseHarness(logPath) {
  if (!logPath) throw new Error("parse-harness requires <log>");
  const log = readFileSync(logPath, "utf8");
  // matches: typescript-eslint compatibility: 218/218 passed (100.0%)
  const lineRe = /typescript-eslint compatibility: (\d+)\/(\d+) passed/g;
  // We also need the rule name. The harness logs the rule as part of the
  // test name preceding the result. Easiest robust approach: track the most
  // recent "=== RUN   Test<RuleName>_TypescriptEslintCompatibility" before
  // a result line.
  const runRe = /=== RUN\s+Test([A-Za-z0-9_]+)_TypescriptEslintCompatibility/;
  const results = {};
  let currentRule = null;
  for (const line of log.split("\n")) {
    const runMatch = line.match(runRe);
    if (runMatch) {
      currentRule = runMatch[1];
      continue;
    }
    const resMatch = line.match(/typescript-eslint compatibility: (\d+)\/(\d+) passed/);
    if (resMatch && currentRule) {
      results[currentRule] = {
        passed: Number(resMatch[1]),
        total: Number(resMatch[2]),
      };
    }
  }
  process.stdout.write(JSON.stringify({ per_rule: results }, null, 2) + "\n");
}

function cmdIssueBody(fpPath, hrPath) {
  if (!fpPath) throw new Error("issue-body requires <fingerprint-json> [<harness-json>]");
  const fp = readJson(fpPath);
  const hr = hrPath && existsSync(hrPath) ? readJson(hrPath) : null;
  const baseline = readJson(BASELINE_PATH);

  const lines = [];
  lines.push(`### typescript-eslint upstream drift detected`);
  lines.push("");
  lines.push(`Daily upstream-check workflow ran at ${fp.generated_at}.`);
  lines.push("");
  lines.push(`- **Baseline pinned commit:** \`${fp.baseline_pinned_commit ?? "(none — first run)"}\``);
  lines.push(`- **Upstream \`main\` HEAD:** [\`${fp.upstream_main_commit.slice(0, 12)}\`](${fp.upstream_main_url})`);
  lines.push("");
  const d = fp.drift;
  if (fp.baseline_empty) {
    lines.push(`> No baseline recorded yet. The block at the end of this issue is a starter \`testdata/upstream-baseline.json\` — commit it to pin against current upstream.`);
    lines.push("");
  } else {
    lines.push(`#### Fixture-file diff`);
    lines.push("");
    const sections = [
      ["Changed", d.changed],
      ["Added (vendored locally but not in baseline)", d.added],
      ["Removed (in baseline but no longer vendored)", d.removed],
      ["Missing upstream (renamed or deleted)", d.missing_upstream],
    ];
    for (const [label, items] of sections) {
      if (items.length === 0) continue;
      lines.push(`**${label}** (${items.length})`);
      for (const n of items.slice(0, 50)) lines.push(`- \`${n}\``);
      if (items.length > 50) lines.push(`- … and ${items.length - 50} more`);
      lines.push("");
    }
  }

  if (hr) {
    lines.push(`#### Harness results at upstream \`main\``);
    lines.push("");
    lines.push("| Rule | passed/total | vs baseline |");
    lines.push("|---|---:|---:|");
    const baselineCounts = baseline.per_rule_pass_counts || {};
    const rules = Object.keys(hr.per_rule).sort();
    for (const rule of rules) {
      const cur = hr.per_rule[rule];
      const prev = baselineCounts[rule];
      const delta =
        prev !== undefined
          ? `${cur.passed - prev.passed >= 0 ? "+" : ""}${cur.passed - prev.passed} / ${cur.total - prev.total >= 0 ? "+" : ""}${cur.total - prev.total}`
          : "(new)";
      lines.push(`| \`${rule}\` | ${cur.passed}/${cur.total} | ${delta} |`);
    }
    lines.push("");
  } else {
    lines.push(`_Harness step was skipped (fingerprint alone signalled drift; no new compatibility numbers yet)._`);
    lines.push("");
  }

  lines.push(`#### Suggested next steps`);
  lines.push(`1. Review the changed fixture files upstream: [tree at \`${fp.upstream_main_commit.slice(0, 12)}\`](https://github.com/${UPSTREAM_OWNER}/${UPSTREAM_REPO}/tree/${fp.upstream_main_commit}/${UPSTREAM_RULES_PATH}).`);
  lines.push(`2. If acceptable, run \`node scripts/upstream-check.mjs sync ${fp.upstream_main_commit}\` locally and commit the updated fixtures + an updated \`testdata/upstream-baseline.json\`.`);
  lines.push(`3. If the harness shows a parity regression, that's the real signal — investigate before updating the baseline.`);
  lines.push("");
  lines.push(`<details><summary>Proposed baseline snippet</summary>`);
  lines.push("");
  lines.push("```json");
  lines.push(
    JSON.stringify(
      {
        pinned_at: fp.generated_at.slice(0, 10),
        pinned_commit: fp.upstream_main_commit,
        file_hashes: fp.upstream_file_hashes,
        per_rule_pass_counts: hr?.per_rule ?? baseline.per_rule_pass_counts ?? {},
      },
      null,
      2,
    ),
  );
  lines.push("```");
  lines.push(`</details>`);
  lines.push("");
  process.stdout.write(lines.join("\n"));
}

async function main() {
  const [sub, ...rest] = process.argv.slice(2);
  switch (sub) {
    case "fingerprint":
      return cmdFingerprint();
    case "sync":
      return cmdSync(rest[0]);
    case "parse-harness":
      return cmdParseHarness(rest[0]);
    case "issue-body":
      return cmdIssueBody(rest[0], rest[1]);
    default:
      process.stderr.write(
        "usage: upstream-check.mjs <fingerprint|sync <commit>|parse-harness <log>|issue-body <fp.json> [<hr.json>]>\n",
      );
      process.exit(2);
  }
}

main().catch((err) => {
  process.stderr.write(`upstream-check: ${err.message}\n`);
  process.exit(1);
});
