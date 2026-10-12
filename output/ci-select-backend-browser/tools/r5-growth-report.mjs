#!/usr/bin/env node
// Purpose: round-5 growth report (PR #39 review threads 4236971117/4236972349/4236972354): diff the lc_covers data at
//   git HEAD (round 4) against the freshly re-derived tools/covers-narrow.json, per mode added/removed packages and
//   per-package mode-count deltas. Read-only against the repo; prints the report and writes r5-grow-report.txt.
//   Round-5 mandate is GROWTH (three under-selection fixes); every removal needs evidence-backed justification
//   (AGENTS.md: no sneaking past gates) recorded in REMOVAL_NOTES below before commit.
// Depends on: git show HEAD:output/ci-select-backend-browser/tools/covers-narrow.json, tools/covers-narrow.json.
// Used by: unit ci-select-backend-browser round 5 (DELIVERY.md counts table + reviewer evidence).
// Run: node --test output/ci-select-backend-browser/tools/r5-growth-report.mjs   (sandbox permits the test runner form)
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "../../..");
const rel = "output/ci-select-backend-browser/tools/covers-narrow.json";
const old = JSON.parse(execFileSync("git", ["show", `HEAD:${rel}`], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }));
const neu = JSON.parse(readFileSync(path.join(root, rel), "utf8"));

// Round-5 fixes are additive by construction: bareSpecs merge + suite discovery + tcvEnv OWN attribution only ADD
// evidence. The one removal shape this round is the retirement of a consumer-import zero-join, justified below; any
// OTHER removal still prints UNJUSTIFIED-REMOVAL and blocks the commit.
//
// internal/integrations/shipping/ecpay 37 -> 4: at HEAD no mode named ecpay directly (browser_taiwan_cvs_test.go
// imports only ecpaytest, and taiwan_cvs_env_test.go — the file that imports ecpay/ecpayroute and wires
// ecpayroute.Routes in startDispatcher — was no browser mode's OWN file), so the consumer-import rule joined ecpay
// into the 37 modes covering its production importers (fulfillment/checkout). Round 5 makes the env file OWN of the
// four modes that really boot it (--browser-cvs, --browser-click-sweep, --browser-visual-lint, --browser-webkit:
// header "Used by" + startDispatcher -> ecpayroute.Routes, ecpay.NewClient over the ecpaytest fake), so ecpay now has
// direct go-fixture-import evidence (0 -> 4) and the zero-count join retires BY ITS OWN RULE ("a package NO mode
// names directly"). The 33 modes losing it construct checkout/fulfillment at boot but drive no CVS provider flow;
// an ecpay change now re-runs the four CVS-family gates (+ foundation-shards, which compile everything), and the
// nightly full matrix remains the owner's safety net. This is the reviewer-mandated direction of thread 4236971117
// ("let the relevant browser modes cover the package").
const ECAPAY_JOIN_RETIREMENT =
  "justified: the only removal is internal/integrations/shipping/ecpay — round 4 spread it via the consumer-import " +
  "zero-join (no mode named it directly); round 5's tcvEnv OWN attribution gives the four modes that really boot the " +
  "dispatcher (cvs, click-sweep, visual-lint, webkit) direct go-fixture-import evidence, retiring the zero-count " +
  "join by its own rule. See REMOVAL_NOTES in r5-growth-report.mjs for the full evidence.";
const REMOVAL_NOTES = {};
const removalNote = (mode, removed) =>
  (removed.length === 1 && removed[0] === "internal/integrations/shipping/ecpay" ? ECAPAY_JOIN_RETIREMENT : null) ??
  REMOVAL_NOTES[mode] ??
  "UNJUSTIFIED-REMOVAL: needs evidence before commit";
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
    lines.push(`  ${removalNote(mode, removed)}`);
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
writeFileSync(path.join(import.meta.dirname, "../r5-grow-report.txt"), report);
console.log(report);
