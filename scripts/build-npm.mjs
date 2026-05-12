#!/usr/bin/env node
// Assemble npm packages from prebuilt binaries.
//
// Inputs:
//   $1                version (no leading "v", e.g. "0.1.0")
//   --artifacts <dir> directory containing one subdir per platform with the binary
//                     (default: ./artifacts; layout: <artifacts>/binary-<name>/jetlint[.exe])
//   --out <dir>       output directory (default: ./npm-dist)
//
// Outputs (under <out>):
//   jetlint/                         — wrapper, version-bumped
//   @jetlint-<name>/                 — one platform package per entry in npm/platforms.json
//
// The publish workflow then publishes every @jetlint-* directory first, then jetlint/.

import { readFileSync, writeFileSync, mkdirSync, copyFileSync, chmodSync, existsSync, rmSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(__dirname, "..");

function parseArgs(argv) {
  const args = { version: null, artifacts: "artifacts", out: "npm-dist" };
  const positional = [];
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--artifacts") args.artifacts = argv[++i];
    else if (a === "--out") args.out = argv[++i];
    else if (a.startsWith("--")) throw new Error(`unknown flag: ${a}`);
    else positional.push(a);
  }
  if (positional.length !== 1) {
    throw new Error("usage: build-npm.mjs <version> [--artifacts <dir>] [--out <dir>]");
  }
  args.version = positional[0];
  if (!/^\d+\.\d+\.\d+(-[\w.]+)?$/.test(args.version)) {
    throw new Error(`version must be semver without leading "v": got "${args.version}"`);
  }
  return args;
}

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function writeJson(path, data) {
  writeFileSync(path, JSON.stringify(data, null, 2) + "\n");
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const artifactsDir = resolve(repoRoot, args.artifacts);
  const outDir = resolve(repoRoot, args.out);

  if (existsSync(outDir)) rmSync(outDir, { recursive: true, force: true });
  mkdirSync(outDir, { recursive: true });

  const platforms = readJson(join(repoRoot, "npm/platforms.json")).platforms;
  const wrapperSrc = join(repoRoot, "npm/jetlint");
  const licensePath = join(repoRoot, "LICENSE");

  for (const p of platforms) {
    const srcBinary = join(artifactsDir, `binary-${p.name}`, p.binary);
    if (!existsSync(srcBinary)) {
      throw new Error(`missing prebuilt binary: ${srcBinary}`);
    }
    const pkgName = `@jetlint/${p.name}`;
    const pkgDir = join(outDir, `@jetlint-${p.name}`);
    mkdirSync(join(pkgDir, "bin"), { recursive: true });
    copyFileSync(srcBinary, join(pkgDir, "bin", p.binary));
    if (process.platform !== "win32") {
      chmodSync(join(pkgDir, "bin", p.binary), 0o755);
    }
    copyFileSync(licensePath, join(pkgDir, "LICENSE"));
    writeJson(join(pkgDir, "package.json"), {
      name: pkgName,
      version: args.version,
      description: `${p.name} binary for jetlint`,
      homepage: "https://jetlint.github.io",
      repository: {
        type: "git",
        url: "git+https://github.com/jetlint/jetlint.git",
        directory: `npm/jetlint`,
      },
      license: "MIT",
      os: [p.os],
      cpu: [p.cpu],
      engines: { node: ">=20" },
      publishConfig: { access: "public", provenance: true },
    });
    writeFileSync(
      join(pkgDir, "README.md"),
      `# ${pkgName}\n\nPrebuilt jetlint binary for ${p.name}. Installed automatically as an optional dependency of the \`jetlint\` package.\n`,
    );
    console.log(`packaged ${pkgName}@${args.version}`);
  }

  const wrapperDir = join(outDir, "jetlint");
  mkdirSync(join(wrapperDir, "bin"), { recursive: true });
  copyFileSync(join(wrapperSrc, "bin/jetlint.js"), join(wrapperDir, "bin/jetlint.js"));
  copyFileSync(join(wrapperSrc, "README.md"), join(wrapperDir, "README.md"));
  copyFileSync(licensePath, join(wrapperDir, "LICENSE"));

  const wrapperPkg = readJson(join(wrapperSrc, "package.json"));
  wrapperPkg.version = args.version;
  for (const dep of Object.keys(wrapperPkg.optionalDependencies)) {
    wrapperPkg.optionalDependencies[dep] = args.version;
  }
  writeJson(join(wrapperDir, "package.json"), wrapperPkg);
  console.log(`packaged jetlint@${args.version}`);
}

try {
  main();
} catch (err) {
  process.stderr.write(`build-npm: ${err.message}\n`);
  process.exit(1);
}
