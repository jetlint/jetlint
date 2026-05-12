#!/usr/bin/env node
// One-time bootstrap publish to claim the six npm names that `release.yml`
// will later publish via OIDC trusted publishing.
//
// Why this exists: npm's trusted-publisher configuration lives on a
// package's settings page, which doesn't exist until the package has been
// published at least once. So we publish a tiny v0.0.0 stub for each of:
//
//   jetlint
//   @jetlint/linux-x64
//   @jetlint/linux-arm64
//   @jetlint/darwin-x64
//   @jetlint/darwin-arm64
//   @jetlint/win32-x64
//
// then `npm deprecate` each one so nobody installs them by accident, then
// configure trusted publishing on each settings page on npmjs.com.
//
// The stubs contain only a README pointing at the GitHub repo and the real
// release plan. No binary, no shim, no JS. They are intentionally useless.
//
// Usage:
//   node scripts/bootstrap-npm-publish.mjs            # generate stubs into bootstrap-dist/
//   node scripts/bootstrap-npm-publish.mjs --publish  # also publish + deprecate
//
// Authentication: relies on `npm whoami` succeeding. Sign in with
// `npm login` (interactive, 2FA-prompted) before running with --publish.
// Trusted publishing is NOT used here — that's the whole point: this run
// is the bootstrap that makes trusted publishing possible afterward.

import { mkdirSync, writeFileSync, existsSync, rmSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(__dirname, "..");
const BOOTSTRAP_VERSION = "0.0.0";
const DEPRECATION_MESSAGE =
  "bootstrap placeholder, do not install. See https://github.com/jetlint/jetlint for the first real release.";

const PACKAGES = [
  { name: "jetlint",                kind: "wrapper",  os: null,     cpu: null   },
  { name: "@jetlint/linux-x64",     kind: "platform", os: "linux",  cpu: "x64"   },
  { name: "@jetlint/linux-arm64",   kind: "platform", os: "linux",  cpu: "arm64" },
  { name: "@jetlint/darwin-x64",    kind: "platform", os: "darwin", cpu: "x64"   },
  { name: "@jetlint/darwin-arm64",  kind: "platform", os: "darwin", cpu: "arm64" },
  { name: "@jetlint/win32-x64",     kind: "platform", os: "win32",  cpu: "x64"   },
];

function packageJson(pkg) {
  const base = {
    name: pkg.name,
    version: BOOTSTRAP_VERSION,
    description:
      pkg.kind === "wrapper"
        ? "Bootstrap placeholder for jetlint. Do not install. See https://github.com/jetlint/jetlint."
        : `Bootstrap placeholder for ${pkg.name}. Do not install.`,
    homepage: "https://jetlint.github.io",
    repository: {
      type: "git",
      url: "git+https://github.com/jetlint/jetlint.git",
      directory: "npm/jetlint",
    },
    license: "MIT",
    engines: { node: ">=20" },
    publishConfig: { access: "public" },
  };
  if (pkg.kind === "platform") {
    base.os = [pkg.os];
    base.cpu = [pkg.cpu];
  }
  return base;
}

function readme(pkg) {
  return [
    `# ${pkg.name}`,
    "",
    "**Bootstrap placeholder. Do not install.**",
    "",
    "This version exists only to create a settings page on npmjs.com so the",
    "real jetlint release workflow can be bound to it as a trusted publisher.",
    "It will be deprecated immediately after publish.",
    "",
    "Real release: https://github.com/jetlint/jetlint",
    "",
  ].join("\n");
}

function buildStubs(outDir) {
  if (existsSync(outDir)) rmSync(outDir, { recursive: true, force: true });
  mkdirSync(outDir, { recursive: true });
  for (const pkg of PACKAGES) {
    const dirName = pkg.name.replace("@", "").replace("/", "-");
    const pkgDir = join(outDir, dirName);
    mkdirSync(pkgDir, { recursive: true });
    writeFileSync(
      join(pkgDir, "package.json"),
      JSON.stringify(packageJson(pkg), null, 2) + "\n",
    );
    writeFileSync(join(pkgDir, "README.md"), readme(pkg));
    console.log(`prepared ${pkg.name}@${BOOTSTRAP_VERSION} in ${pkgDir}`);
  }
  console.log(`\nstubs written to ${outDir}`);
}

function run(cmd, args, cwd) {
  console.log(`$ ${cmd} ${args.join(" ")}  (cwd=${cwd})`);
  const r = spawnSync(cmd, args, { stdio: "inherit", cwd });
  if (r.status !== 0) {
    throw new Error(`${cmd} ${args.join(" ")} exited ${r.status}`);
  }
}

function ensureLoggedIn() {
  const r = spawnSync("npm", ["whoami"], { encoding: "utf8" });
  if (r.status !== 0) {
    throw new Error(
      "not logged in to npm. Run `npm login` first (2FA prompt expected).",
    );
  }
  console.log(`logged in to npm as ${r.stdout.trim()}`);
}

function publishAndDeprecate(outDir) {
  ensureLoggedIn();
  for (const pkg of PACKAGES) {
    const dirName = pkg.name.replace("@", "").replace("/", "-");
    const pkgDir = join(outDir, dirName);
    run("npm", ["publish", "--access", "public"], pkgDir);
    run(
      "npm",
      ["deprecate", `${pkg.name}@${BOOTSTRAP_VERSION}`, DEPRECATION_MESSAGE],
      pkgDir,
    );
  }
  console.log("\nbootstrap complete. Configure trusted publishing on:");
  for (const pkg of PACKAGES) {
    console.log(`  https://www.npmjs.com/package/${pkg.name}/access`);
  }
}

function main() {
  const outDir = resolve(repoRoot, "bootstrap-dist");
  const args = new Set(process.argv.slice(2));
  buildStubs(outDir);
  if (args.has("--publish")) {
    publishAndDeprecate(outDir);
  } else {
    console.log("\ndry run. Re-run with --publish to actually publish + deprecate.");
  }
}

try {
  main();
} catch (err) {
  process.stderr.write(`bootstrap: ${err.message}\n`);
  process.exit(1);
}
