#!/usr/bin/env node
// Purpose: CI-SELECT round-2 evidence — how many browser modes the narrowed lc_covers data selects for the REAL file
//   lists of PR #30 (b1bfbeb3) and PR #24 (3034c407), the mandated single-path cases, and three realistic domain
//   examples (orders/payments, catalog, migrations), against what round 1's full-closure covers selected for the same
//   lists (round 1: every internal/ hit selected all 48 Go-seeded modes; cmd//migrations/ selected all 47 PG modes).
//   Records universe sizes and each case's selected browser modes; writes selection-counts-r2.json next to this file;
//   numbers are quoted in DELIVERY.md.
// Depends on: scripts/dev/pr-modes.mjs, scripts/dev/test-local.sh, tests/foundation sources (tag lookup), git show.
// Used by: unit ci-select-backend-browser only.
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { BACKEND_ONLY_PACKAGES, SHARED_BACKEND_PACKAGES, backendBrowserModes, browserModes, modeEntries, planPr } from "../../../scripts/dev/pr-modes.mjs";

const root = path.resolve(import.meta.dirname, "../../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
const universe = browserModes(usage);
const entries = modeEntries(usage);
const inUniverse = new Set(universe);
const pg = entries.filter((e) => inUniverse.has(e.name) && e.fixture === "pg").map((e) => e.name);
const goSeeded = entries.filter((e) => inUniverse.has(e.name) && (e.covers ?? []).length > 0).map((e) => e.name);
const goBoot = entries.filter((e) => inUniverse.has(e.name) && (e.fixture === "pg" || (e.covers ?? []).length > 0)).map((e) => e.name);
const coversSum = entries.filter((e) => inUniverse.has(e.name)).reduce((a, e) => a + (e.covers ?? []).length, 0);

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
  "pr-30 b1bfbeb3": filesOf("b1bfbeb3"),
  "pr-24 3034c407": filesOf("3034c407"),
  "orders/payments example: internal/merchantorders + internal/payments": ["internal/merchantorders/orders.go", "internal/payments/payments.go"],
  "catalog example: internal/catalog": ["internal/catalog/products.go"],
  "migrations example: migrations/0169_x.sql": ["migrations/0169_x.sql"],
  "mandated: internal/live/stream.go": ["internal/live/stream.go"],
  "mandated: internal/integrations/metareply/x.go": ["internal/integrations/metareply/x.go"],
  "mandated: internal/platform/x.go (SHARED)": ["internal/platform/x.go"],
  "mandated: docs+contracts only": ["docs/delivery/GATES.md", "contracts/invariants.json"],
};
const out = {
  universe: {
    browser_modes: universe.length, pg_fixture_browser_modes: pg.length, go_seeded_covers_nonempty: goSeeded.length,
    go_booting_modes: goBoot.length,
    shared_packages: Object.keys(SHARED_BACKEND_PACKAGES).length, backend_only_packages: Object.keys(BACKEND_ONLY_PACKAGES).length,
    lc_covers_entries_sum: coversSum, lc_covers_mean_per_mode: Number((coversSum / universe.length).toFixed(1)),
  },
  cases: {},
};
for (const [name, paths] of Object.entries(cases)) {
  const r = planPr(paths, usage);
  const browsers = r.modes.filter((m) => m !== "foundation-shards");
  const tagged = taggedFoundation(paths);
  const backendDriven = backendBrowserModes(paths, usage);
  // Round-1 planner on the same list: full-closure covers meant every internal/ hit selected all Go-seeded modes and
  // cmd//migrations/ all PG modes; browser-tagged foundation files selected the whole universe in both rounds.
  const hitsInternal = paths.some((p) => p.startsWith("internal/"));
  const hitsCmdMig = paths.some((p) => p.startsWith("cmd/") || p.startsWith("migrations/"));
  const round1 = tagged.length ? universe.length : hitsInternal ? goSeeded.length : hitsCmdMig ? pg.length : 0;
  out.cases[name] = {
    files: paths.length, modes: r.modes.length, browser_modes: browsers.length,
    backend_driven_browser_modes: backendDriven.length,
    round1_browser_modes: round1,
    selects_browser_live_console: browsers.includes("--browser-live-console"),
    selected_browser_modes: browsers,
    browser_tagged_foundation_files: tagged,
  };
  console.log(`${name}: files=${paths.length} -> browser modes=${browsers.length} (round 1: ${round1}); total checks=${r.modes.length}; live-console=${browsers.includes("--browser-live-console")}`);
  console.log(`  selected: ${browsers.join(" ") || "(none)"}`);
}
writeFileSync(path.join(import.meta.dirname, "selection-counts-r2.json"), JSON.stringify(out, null, 1));
console.log(`universe: browser=${universe.length} pg=${pg.length} go-seeded=${goSeeded.length} go-booting=${goBoot.length} shared=${Object.keys(SHARED_BACKEND_PACKAGES).length} backend-only=${Object.keys(BACKEND_ONLY_PACKAGES).length} covers-sum=${coversSum} mean=${(coversSum / universe.length).toFixed(1)}`);
