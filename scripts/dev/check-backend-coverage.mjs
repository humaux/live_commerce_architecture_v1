#!/usr/bin/env node
// Purpose: CI-SELECT classification gate. Every directory under internal/, plus every Go file in a file-split package,
//   must be classified by exactly one of three
//   routes: a browser mode's lc_covers registry data (the DOMAIN packages that mode's harness actually exercises,
//   round-2 narrow derivation), SHARED_BACKEND_PACKAGES (cross-cutting boot/auth packages — selecting all PG browser
//   modes, so they need no per-mode covers entry), or BACKEND_ONLY_PACKAGES (cmd/worker-only packages with a one-line
//   reason — no browser mode runs them), so a NEW backend package fails here until someone classifies it. Every
//   browser-universe mode must carry the lc_covers line; one whose body runs Go must not carry it empty (an empty list
//   would silently deselect the mode for every backend change); covers entries must match a real internal package
//   directory or exact Go file (parent prefixes are forbidden for split packages). lc_covers content is derived data —
//   regenerate with output/ci-select-backend-browser/tools/derive-narrow-covers.mjs after rewiring a harness or adding
//   a route/spec, never hand-guess it.
// Depends on: scripts/dev/pr-modes.mjs (modeEntries, browserModes, SHARED_BACKEND_PACKAGES, BACKEND_ONLY_PACKAGES), scripts/dev/test-local.sh, git ls-files.
// Used by: scripts/dev/check-gates.sh, tests/ci/backend-coverage.test.mjs.
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { BACKEND_ONLY_PACKAGES, SHARED_BACKEND_PACKAGES, FILE_CLASSIFIED_PACKAGES, backendPathMatches, browserModes, modeEntries } from "./pr-modes.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

/**
 * Classify backend coverage from registry DATA. pkgDirs = internal package directories (non-test Go files).
 * Returns [] when everything is classified; otherwise one error string per finding. backendOnly and shared are
 * injectable for tests. Classification mirrors the planner (scripts/dev/pr-modes.mjs backendBrowserModes): a package
 * is classified when an lc_covers entry is an ancestor-or-self of it (under()), a SHARED entry is, or a BACKEND_ONLY
 * entry is — matching the planner's own prefix semantics so the gate and the selection can never drift apart.
 */
export function classifyBackendCoverage(registrySource, pkgDirs, backendOnly = BACKEND_ONLY_PACKAGES, shared = SHARED_BACKEND_PACKAGES) {
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
    if (!pkgDirs.some((d) => c.endsWith(".go") ? d === path.posix.dirname(c) : under(d, c))) errors.push(`lc_covers entry ${c} matches no internal package directory (stale; re-derive the covers)`);
  const listed = Object.keys(backendOnly);
  const sharedList = Object.keys(shared);
  // The planner resolves BACKEND_ONLY before lc_covers (most-specific classification wins), so a covers entry inside a
  // BACKEND_ONLY package would be dead data selecting nothing, and a package both SHARED and BACKEND_ONLY is a
  // contradiction — fail on both here so the planner precedence can never silently hide a classification.
  for (const c of declared)
    if (listed.some((b) => under(c, b))) errors.push(`lc_covers entry ${c} is inside BACKEND_ONLY_PACKAGES: contradictory classification (the planner resolves BACKEND_ONLY first, so this entry would select nothing); resolve one way or the other`);
  for (const b of listed)
    if (sharedList.some((s) => under(b, s) || under(s, b))) errors.push(`${b} is listed in both BACKEND_ONLY_PACKAGES and SHARED_BACKEND_PACKAGES: contradictory classification; resolve one way or the other`);
  for (const d of pkgDirs) {
    // Split packages are checked file-by-file below, never covered just by the existence of their directory.
    if (FILE_CLASSIFIED_PACKAGES.some((p) => under(d, p))) continue;
    if (declared.some((c) => under(d, c))) continue;
    if (sharedList.some((s) => under(d, s))) continue;
    if (listed.some((b) => under(d, b))) continue;
    errors.push(`${d} is classified by nobody: add it to the lc_covers of every browser mode whose harness exercises it (derive: output/ci-select-backend-browser/tools/derive-narrow-covers.mjs), or list it in SHARED_BACKEND_PACKAGES / BACKEND_ONLY_PACKAGES (scripts/dev/pr-modes.mjs) with a one-line reason`);
  }
  return errors;
}

/** Validate exact file coverage, including unit-test files; a new file fails closed until deliberately classified. */
export function classifyBackendFiles(registrySource, files, backendOnly = BACKEND_ONLY_PACKAGES, shared = SHARED_BACKEND_PACKAGES) {
  const errors = [];
  const universe = new Set(browserModes(registrySource));
  const declarations = [...new Set(modeEntries(registrySource).filter((e) => universe.has(e.name)).flatMap((e) => e.covers ?? []))];
  const explicit = [...Object.keys(shared), ...Object.keys(backendOnly)];
  for (const entry of [...declarations, ...explicit]) {
    if (entry.endsWith(".go") && !files.includes(entry)) errors.push(`${entry} is a stale file classification`);
    if (!entry.endsWith(".go") && FILE_CLASSIFIED_PACKAGES.some((p) => backendPathMatches(p, entry) || backendPathMatches(entry, p)))
      errors.push(`${entry} must use file-level classification (split package)`);
  }
  for (const file of files.filter((f) => FILE_CLASSIFIED_PACKAGES.some((p) => backendPathMatches(f, p)))) {
    if ([...declarations, ...explicit].some((entry) => entry.endsWith(".go") && backendPathMatches(file, entry))) continue;
    errors.push(`${file} is classified by nobody: declare its exact lc_covers, SHARED or BACKEND_ONLY file entry`);
  }
  return errors;
}

function main() {
  const source = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
  // Package dirs of tracked non-test Go files under internal/ (test files never link into a shipped binary).
  const files = execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
    .split("\0").filter((f) => f.endsWith(".go"));
  const dirs = [...new Set(files.filter((f) => !f.endsWith("_test.go")).map((f) => path.posix.dirname(f)))].sort();
  const errors = [...classifyBackendCoverage(source, dirs), ...classifyBackendFiles(source, files)];
  if (errors.length) {
    console.error(`check-backend-coverage: ${errors.length} finding(s):\n${errors.map((e) => `  ${e}`).join("\n")}`);
    return 1;
  }
  console.log(`check-backend-coverage: ok (${dirs.length} internal package dirs and ${files.filter((f) => FILE_CLASSIFIED_PACKAGES.some((p) => backendPathMatches(f, p))).length} split-package files classified; ${browserModes(source).length} browser modes declare lc_covers)`);
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`check-backend-coverage: ${e.message}`); process.exit(1); }
}
