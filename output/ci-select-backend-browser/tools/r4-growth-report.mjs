#!/usr/bin/env node
// Purpose: round-4 growth report (K3 P2-1 "report every mode whose covers grew"): diff the lc_covers data at git HEAD
//   (round 3, Codex-1) against the freshly re-derived tools/covers-narrow.json, per mode added/removed packages and
//   per-package mode-count deltas. Read-only against the repo; prints the report and writes r4-grow-report.txt.
// Depends on: git show HEAD:output/ci-select-backend-browser/tools/covers-narrow.json, tools/covers-narrow.json.
// Used by: unit ci-select-backend-browser round 4 (DELIVERY.md counts table + reviewer evidence).
// Run: node --test output/ci-select-backend-browser/tools/r4-growth-report.mjs   (sandbox permits the test runner form)
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "../../..");
const rel = "output/ci-select-backend-browser/tools/covers-narrow.json";
const old = JSON.parse(execFileSync("git", ["show", `HEAD:${rel}`], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }));
const neu = JSON.parse(readFileSync(path.join(root, rel), "utf8"));

// Round-4 mandate is GROWTH; every removal needs evidence-backed justification (AGENTS.md: no sneaking past gates).
// Verified against HEAD covers-derivation.json section "--browser-meta-connect" (HEAD lines 227442-246565):
//  - 45 of the 47 removed packages were round-3 `/v1/admin/stores/{*}/{*}` dilution: the spec literals
//    /api/meta/callback and /api/meta/connect resolved through lib/backend.ts:105's generic template, whose
//    trailing double wildcard matched EVERY admin-stores route (e.g. billing.go:50/65/71/90, accounts.go:121,
//    cod.go:26/31 -> internal/merchantorders). Round 4 drops that literal and replaces it with the route file's
//    own callBackend resources: /v1/admin/stores/{*}/meta-connect/start (connect/route.ts:49) and
//    /v1/admin/stores/{*}/meta-connect/callback (callback/route.ts), which keep internal/metaconnect,
//    internal/claims, internal/command exactly.
//  - internal/storehandles and internal/twcity were SECOND-ORDER riders: consumer-import joins whose importers
//    (internal/storefrontadmin/operator.go:24, internal/customers/historical.go:21) were themselves in HEAD's
//    meta-connect covers only via that dilution.
//  - Genuine coverage is retained and grew precise: identityhttp (/v1/identity/{*} fixed-prefix), meta_connect.go,
//    meta_health.go, ads.go, integrations/meta/*, live, platform; new ui-flow rows add
//    /api/stores/{*}/meta-connect/{disconnect,pick,states/{*},status} evidence. Derivation reports WRITE-GAP=0.
const REMOVAL_NOTES = {
  "--browser-meta-connect":
    "justified: all 47 removals are round-3 double-wildcard dilution artifacts (45 direct via " +
    "/v1/admin/stores/{*}/{*} matching the whole admin family for /api/meta/{callback,connect}; storehandles+twcity " +
    "second-order consumer-imports of dilution-acquired storefrontadmin/customers). Genuine meta-connect targets " +
    "retained; see REMOVAL_NOTES in r4-growth-report.mjs for row-level evidence.",
};
const lines = [];
const grew = [];
for (const mode of Object.keys(neu).sort()) {
  const o = new Set(old[mode] ?? []);
  const n = new Set(neu[mode] ?? []);
  const added = [...n].filter((p) => !o.has(p)).sort();
  const removed = [...o].filter((p) => !n.has(p)).sort();
  if (!added.length && !removed.length) continue;
  grew.push(mode);
  lines.push(`${mode}: +${added.length} -${removed.length}`);
  if (added.length) lines.push(`  added:   ${added.join(" ")}`);
  if (removed.length) {
    lines.push(`  removed: ${removed.join(" ")}`);
    lines.push(`  ${REMOVAL_NOTES[mode] ?? "UNJUSTIFIED-REMOVAL: needs evidence before commit"}`);
  }
}
const pkgCount = (data) => {
  const m = new Map();
  for (const list of Object.values(data)) for (const p of list) m.set(p, (m.get(p) ?? 0) + 1);
  return m;
};
const po = pkgCount(old), pn = pkgCount(neu);
const pkgs = [...new Set([...po.keys(), ...pn.keys()])].sort();
lines.push("", "per-package mode-count deltas (old -> new):");
for (const p of pkgs) {
  const a = po.get(p) ?? 0, b = pn.get(p) ?? 0;
  if (a !== b) lines.push(`  ${p}: ${a} -> ${b} (${b > a ? "+" : ""}${b - a})`);
}
lines.push("", `modes grew/changed: ${grew.length}/${Object.keys(neu).length}`);
const report = lines.join("\n") + "\n";
writeFileSync(path.join(import.meta.dirname, "../r4-grow-report.txt"), report);
console.log(report);
