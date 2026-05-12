#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");

const SUPPORTED = new Set([
  "linux-x64",
  "linux-arm64",
  "darwin-x64",
  "darwin-arm64",
  "win32-x64",
]);

function platformKey() {
  return `${process.platform}-${process.arch}`;
}

function resolveBinary() {
  const key = platformKey();
  if (!SUPPORTED.has(key)) {
    throw new Error(
      `jetlint: no prebuilt binary for ${key}. Supported: ${[...SUPPORTED].join(", ")}. ` +
        `Build from source (Go 1.26+): go install github.com/jetlint/jetlint/cmd/jetlint@latest`,
    );
  }
  const pkg = `@jetlint/${key}`;
  const exe = process.platform === "win32" ? "jetlint.exe" : "jetlint";
  try {
    return require.resolve(`${pkg}/bin/${exe}`);
  } catch (err) {
    throw new Error(
      `jetlint: cannot locate ${pkg}/bin/${exe}. The optional platform package ` +
        `was not installed. Reinstall with: npm install --force jetlint (root cause: ${err.message})`,
    );
  }
}

function main() {
  let binary;
  try {
    binary = resolveBinary();
  } catch (err) {
    process.stderr.write(`${err.message}\n`);
    process.exit(1);
  }

  const result = spawnSync(binary, process.argv.slice(2), {
    stdio: "inherit",
    windowsHide: true,
  });

  if (result.error) {
    process.stderr.write(`jetlint: failed to execute ${binary}: ${result.error.message}\n`);
    process.exit(1);
  }
  if (typeof result.status === "number") {
    process.exit(result.status);
  }
  if (result.signal) {
    process.kill(process.pid, result.signal);
  }
  process.exit(1);
}

main();
