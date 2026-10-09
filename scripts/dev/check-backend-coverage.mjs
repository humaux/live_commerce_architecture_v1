#!/usr/bin/env node
// Purpose: CI-SELECT classification gate. Every directory under internal/ must be wired by at least one browser mode's
//   lc_covers registry data or be explicitly listed in BACKEND_ONLY_PACKAGES (scripts/dev/pr-modes.mjs) with a one-line
//   reason, so a NEW backend package fails here until someone classifies it. Every browser-universe mode must carry the
//   lc_covers line; one whose body runs Go must not carry it empty (an empty list would silently deselect the mode for
//   every backend change); covers entries must match a real internal package directory (stale entries select nothing,
//   parent prefixes are legal). lc_covers content is derived data — regenerate with
//   output/ci-select-backend-browser/tools/derive-covers.mjs after rewiring a harness, never hand-guess it.
// Depends on: scripts/dev/pr-modes.mjs (modeEntries, browserModes, BACKEND_ONLY_PACKAGES), scripts/dev/test-local.sh, git ls-files.
// Used by: scripts/dev/check-gates.sh, tests/ci/backend-coverage.test.mjs.
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { BACKEND_ONLY_PACKAGES, browserModes, modeEntries } from "./pr-modes.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

/**
 * Classify backend coverage from registry DATA. pkgDirs = internal package directories (non-test Go files).
 * Returns [] when everything is classified; otherwise one error string per finding. backendOnly is injectable for tests.
 */
export function classifyBackendCoverage(registrySource, pkgDirs, backendOnly = BACKEND_ONLY_PACKAGES) {
  const errors = [];
  const entries = modeEntries(registrySource);
  const universe = new Set(browserModes(registrySource));
  const under = (dir, pkg) => dir === pkg || dir.startsWith(pkg + "/");
  for (const e of entries) {
    if (!universe.has(e.name)) continue;
    if (e.covers === null) {
      errors.push(`browser mode ${e.name} has no lc_covers line: declare the internal packages its harness wires (lc_covers="" only for a node-only harness)`);
      continue;
    }
    // Bash comments are prose, not commands: strip them before looking for Go invocations (the --browser-admin-shell
    // comment "no go test events" must not read as a Go run).
    const body = `${e.prepare}\n${e.run}`.split("\n").map((l) => l.replace(/#.*$/, "")).join("\n");
    if (e.covers.length === 0 && (/\bgo test\b/.test(body) || /\bTest[A-Z]/.test(body)))
      errors.push(`browser mode ${e.name} runs Go but declares lc_covers="": every internal change would skip it (derive its covers, see output/ci-select-backend-browser/tools/)`);
  }
  const declared = entries.flatMap((e) => e.covers ?? []);
  for (const c of declared)
    if (!pkgDirs.some((d) => under(d, c))) errors.push(`lc_covers entry ${c} matches no internal package directory (stale; re-derive the covers)`);
  const listed = Object.keys(backendOnly);
  for (const d of pkgDirs) {
    if (declared.some((c) => under(d, c))) continue;
    if (listed.some((b) => under(d, b))) continue;
    errors.push(`${d} is classified by nobody: add it to the lc_covers of every browser mode whose harness wires it (derive: output/ci-select-backend-browser/tools/derive-covers.mjs), or list it in BACKEND_ONLY_PACKAGES (scripts/dev/pr-modes.mjs) with a one-line reason`);
  }
  return errors;
}

function main() {
  const source = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
  // Package dirs of tracked non-test Go files under internal/ (test files never link into a shipped binary).
  const dirs = [...new Set(execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
    .split("\0").filter(Boolean).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go")).map((f) => path.posix.dirname(f)))].sort();
  const errors = classifyBackendCoverage(source, dirs);
  if (errors.length) {
    console.error(`check-backend-coverage: ${errors.length} finding(s):\n${errors.map((e) => `  ${e}`).join("\n")}`);
    return 1;
  }
  console.log(`check-backend-coverage: ok (${dirs.length} internal package dirs classified; ${browserModes(source).length} browser modes declare lc_covers)`);
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`check-backend-coverage: ${e.message}`); process.exit(1); }
}
