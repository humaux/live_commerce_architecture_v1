#!/usr/bin/env node
// Purpose: one-off writer of the derived lc_covers data into scripts/dev/test-local.sh (CI-SELECT): inserts a
//   `    lc_covers="<packages>"` declaration after the first 4-space `lc_fixture=` line of every browser-universe mode
//   arm (data: covers.json, derived by derive-covers.mjs from what each tests/foundation harness actually wires).
//   Idempotent: an arm that already declares lc_covers is skipped. Placement after lc_fixture keeps the
//   lc_build/lc_fixture adjacency that tests/ci/mode-registry.test.mjs replaces against.
// Depends on: covers.json (same directory), scripts/dev/pr-modes.mjs (modeEntries, browserModes), scripts/dev/test-local.sh.
// Used by: unit ci-select-backend-browser only; re-run after any harness rewiring, then review the git diff.
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { modeEntries, browserModes } from "../../../scripts/dev/pr-modes.mjs";

const root = path.resolve(import.meta.dirname, "../../..");
const file = path.join(root, "scripts/dev/test-local.sh");
const source = readFileSync(file, "utf8");
const covers = JSON.parse(readFileSync(path.join(import.meta.dirname, "covers.json"), "utf8"));
const universe = new Set(browserModes(source));

let next = source, inserted = 0, skipped = 0;
for (const e of modeEntries(source)) {
  if (!universe.has(e.name)) continue;
  // Body-text detection (parser-independent): an arm that already declares lc_covers is skipped untouched.
  if (/^    lc_covers=/m.test(e.body)) { skipped++; console.log(`skip ${e.name} (already declares lc_covers)`); continue; }
  if (!(e.name in covers)) throw new Error(`covers.json has no entry for ${e.name}: re-run derive-covers.mjs first`);
  // Lazy match stops at the arm's own first 4-space lc_fixture line (6-space conditional redefinitions never match).
  const arm = new RegExp(`^  ${e.name}\\)\\n[\\s\\S]*?^    lc_fixture=(?:none|pg)$`, "m");
  const m = arm.exec(next);
  if (!m) throw new Error(`cannot locate the lc_fixture line of ${e.name}`);
  const at = m.index + m[0].length;
  next = next.slice(0, at) + `\n    lc_covers="${covers[e.name].join(" ")}"` + next.slice(at);
  inserted++;
}

// Post-check with the real parser before writing (modeEntries throws if the lc_covers lines are not accepted as
// declarations): every browser-universe mode must now declare lc_covers.
const after = modeEntries(next);
for (const e of after)
  if (universe.has(e.name) && !/^    lc_covers=/m.test(e.body)) throw new Error(`post-check failed: ${e.name} still has no lc_covers`);
writeFileSync(file, next);
console.log(`insert-covers: inserted=${inserted} skipped=${skipped} (browser universe=${universe.size}); file written`);
