#!/usr/bin/env node
// Purpose: CI-SELECT evidence — how many browser modes the repaired planner selects for the REAL file lists of PR #30
//   (b1bfbeb3) and PR #24 (3034c407) plus the mandated single-path cases, against what the old backend-only premise
//   selected. Also records universe sizes (full browser set, lc_fixture=pg set, Go-seeded set with non-empty lc_covers)
//   and each case's browser-tagged foundation files (the only way the OLD planner could reach a browser mode for these
//   lists). Writes selection-counts.json next to this file; numbers are quoted in DELIVERY.md.
// Depends on: scripts/dev/pr-modes.mjs, scripts/dev/test-local.sh, tests/foundation sources (tag lookup), git show.
// Used by: unit ci-select-backend-browser only.
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { BACKEND_ONLY_PACKAGES, backendBrowserModes, browserModes, modeEntries, planPr } from "../../../scripts/dev/pr-modes.mjs";

const root = path.resolve(import.meta.dirname, "../../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
const universe = browserModes(usage);
const entries = modeEntries(usage);
const inUniverse = new Set(universe);
const pg = entries.filter((e) => inUniverse.has(e.name) && e.fixture === "pg").map((e) => e.name);
const goSeeded = entries.filter((e) => inUniverse.has(e.name) && (e.covers ?? []).length > 0).map((e) => e.name);

const filesOf = (sha) => execFileSync("git", ["show", "--name-only", "--format=", sha], { cwd: root, encoding: "utf8" }).split("\n").filter(Boolean);
// The tag rule is unchanged by CI-SELECT; replicate it to state the OLD selection honestly. Missing file = conservative tagged.
function taggedFoundation(paths) {
  return paths.filter((p) => {
    if (!p.startsWith("tests/foundation/") || !p.endsWith(".go") || p.startsWith("tests/foundation/browser_")) return p.startsWith("tests/foundation/browser_");
    try { return /^\s*\/\/(?:go:build|\s*\+build)\b[^\n]*\bbrowser\b/m.test(readFileSync(path.join(root, p), "utf8")); }
    catch { return true; }
  });
}

const cases = {
  "pr-30 b1bfbeb3 (internal/live/stream.go + internal/httpapi/live_stream.go)": filesOf("b1bfbeb3"),
  "pr-24 3034c407 (internal/integrations/metareply + meta/oauth)": filesOf("3034c407"),
  "internal/live/stream.go (mandated case 1)": ["internal/live/stream.go"],
  "internal/integrations/metareply/x.go (mandated case 2)": ["internal/integrations/metareply/x.go"],
  "migrations/0169_x.sql (mandated case 3)": ["migrations/0169_x.sql"],
  "docs+contracts only (mandated case 4)": ["docs/delivery/GATES.md", "contracts/invariants.json"],
};
const out = {
  universe: { browser_modes: universe.length, pg_fixture_browser_modes: pg.length, go_seeded_covers_nonempty: goSeeded.length, backend_only_packages: Object.keys(BACKEND_ONLY_PACKAGES).length },
  cases: {},
};
for (const [name, paths] of Object.entries(cases)) {
  const r = planPr(paths, usage);
  const browsers = r.modes.filter((m) => m !== "foundation-shards");
  const tagged = taggedFoundation(paths);
  const backendDriven = backendBrowserModes(paths, usage);
  // OLD planner: browser modes only when a browser_* path or a browser-tagged foundation file was in the diff.
  const old = tagged.length ? universe.length : 0;
  out.cases[name] = {
    files: paths.length, modes: r.modes.length, browser_modes: browsers.length,
    backend_driven_browser_modes: backendDriven.length,
    selects_browser_live_console: browsers.includes("--browser-live-console"),
    browser_tagged_foundation_files: tagged,
    old_planner_browser_modes: old,
  };
  console.log(`${name}: files=${paths.length} -> modes=${r.modes.length} (browser=${browsers.length}, backend-driven=${backendDriven.length}, --browser-live-console=${browsers.includes("--browser-live-console")}); OLD planner browser modes=${old}${tagged.length ? ` (tagged: ${tagged.join(", ")})` : ""}`);
}
writeFileSync(path.join(import.meta.dirname, "selection-counts.json"), JSON.stringify(out, null, 1));
console.log(`universe: browser=${universe.length} pg=${pg.length} go-seeded=${goSeeded.length} backend-only-listed=${Object.keys(BACKEND_ONLY_PACKAGES).length}`);
