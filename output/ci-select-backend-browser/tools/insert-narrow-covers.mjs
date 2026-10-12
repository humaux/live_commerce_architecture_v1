#!/usr/bin/env node
// Purpose: round-2 writer of the NARROW lc_covers data into scripts/dev/test-local.sh (CI-SELECT, owner decision
//   2026-10-10 "narrow per domain + nightly"): replaces the lc_covers declaration of every browser-universe mode arm
//   with its derived DOMAIN packages (data: covers-narrow.json, derived by derive-narrow-covers.mjs from the API paths
//   each mode's Playwright specs and Go harness actually exercise; SHARED_BACKEND_PACKAGES entries are already
//   subtracted by the derivation, BACKEND_ONLY entries never appear). Mechanical rewrite only — arm order,
//   lc_build/lc_fixture lines, lc_prepare/lc_run bodies and every non-browser arm must come out byte-identical
//   (asserted before the write). Idempotent: re-running with the same covers-narrow.json rewrites nothing.
// Depends on: covers-narrow.json (same directory), scripts/dev/pr-modes.mjs (modeEntries, browserModes), scripts/dev/test-local.sh.
// Used by: unit ci-select-backend-browser only; re-run after any harness rewiring or route/spec change, then review the git diff.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { browserModes, modeEntries } from "../../../scripts/dev/pr-modes.mjs";

const root = path.resolve(import.meta.dirname, "../../..");
const file = path.join(root, "scripts/dev/test-local.sh");
const source = readFileSync(file, "utf8");
const narrow = JSON.parse(readFileSync(path.join(import.meta.dirname, "covers-narrow.json"), "utf8"));
const universe = new Set(browserModes(source));
const before = modeEntries(source);

// Fail closed before touching anything: every universe arm must already carry an lc_covers line (round-1 insertion;
// this tool only rewrites existing declarations, it never invents placement), and covers-narrow.json must describe
// exactly the universe — node-only modes included (with []).
for (const e of before)
  if (universe.has(e.name) && e.covers === null) throw new Error(`${e.name} has no lc_covers line; run insert-covers.mjs (round 1) or fix the registry first`);
for (const name of universe) if (!(name in narrow)) throw new Error(`covers-narrow.json has no entry for ${name}: re-run derive-narrow-covers.mjs first`);
for (const name of Object.keys(narrow)) if (!universe.has(name)) throw new Error(`covers-narrow.json entry ${name} is not in the browser universe: stale derivation`);
for (const [name, list] of Object.entries(narrow))
  for (const p of list)
    if (!/^[A-Za-z0-9_./-]+$/.test(p)) throw new Error(`${name}: covers entry ${p} is not a plain package path`);

let next = source, rewritten = 0, unchanged = 0;
for (const e of before) {
  if (!universe.has(e.name)) continue;
  const want = narrow[e.name].join(" ");
  if (e.covers.join(" ") === want) { unchanged++; continue; }
  // Lazy match: the arm's own 4-space lc_covers line is the first such line after the arm header (every universe arm
  // declares one — checked above — and function bodies are indented deeper than 4 spaces).
  const line = new RegExp(`(^  ${e.name}\\)\\n[\\s\\S]*?^    lc_covers=")[^"]*(")$`, "m");
  const m = line.exec(next);
  if (!m) throw new Error(`cannot locate the lc_covers line of ${e.name}`);
  next = next.slice(0, m.index) + m[1] + want + m[2] + next.slice(m.index + m[0].length);
  rewritten++;
  console.log(`rewrite ${e.name}: ${e.covers.length} -> ${narrow[e.name].length} packages`);
}

// Post-check with the real parser before writing (modeEntries throws if a rewritten line is not accepted as a
// declaration): only covers may change, only for universe arms, and only to the derived data.
const after = modeEntries(next);
assert.equal(after.length, before.length, "arm count changed");
for (let i = 0; i < after.length; i++) {
  const a = after[i], b = before[i];
  assert.equal(a.name, b.name, `arm order changed at ${i}`);
  assert.equal(a.build, b.build, `${a.name}: lc_build changed`);
  assert.equal(a.fixture, b.fixture, `${a.name}: lc_fixture changed`);
  assert.equal(a.prepare, b.prepare, `${a.name}: lc_prepare changed`);
  assert.equal(a.run, b.run, `${a.name}: lc_run changed`);
  if (universe.has(a.name)) assert.deepEqual(a.covers, narrow[a.name], `${a.name}: lc_covers != covers-narrow.json`);
  else assert.deepEqual(a.covers, b.covers, `${a.name}: non-browser arm covers changed`);
}
// Belt and braces: outside the lc_covers lines the file must be byte-identical.
const stripCovers = (s) => s.split("\n").filter((l) => !/^    lc_covers="/.test(l)).join("\n");
assert.equal(stripCovers(next), stripCovers(source), "a non-lc_covers line changed; refusing to write");

if (rewritten) writeFileSync(file, next);
console.log(`insert-narrow-covers: rewritten=${rewritten} unchanged=${unchanged} (browser universe=${universe.size}); ${rewritten ? "file written" : "nothing to write"}`);
