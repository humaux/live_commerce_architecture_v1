// Purpose: compare old/new static PR selection on the last ten merged PR path receipts; no tests or servers.
// Depends on: GitHub files/mergedAt receipts, frozen RED planner, current planner and generated ownership.
// Used by: output/ci-speed-3/impact.md; integrator/reviewer CI-SPEED-3 handoff.
// Invariants: one registry snapshot for both policies; counts are modes, never elapsed-time or CI PASS claims.
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { planPr as oldPlan } from "./pr-modes-before.mjs";
import { planPr as newPlan, browserModes } from "../../scripts/dev/pr-modes.mjs";
const read = (name) => readFileSync(new URL(name, import.meta.url), "utf8");
const registry = read("registry-snapshot.sh"), universe = browserModes(registry);
const prs = JSON.parse(read("merged-prs.json"));
const measured = prs.map((pr) => {
  const paths = read(`pr-${pr.number}-paths.txt`).trim().split("\n").filter(Boolean);
  const before = oldPlan(paths, registry), after = newPlan(paths, registry);
  return { ...pr, paths: paths.length, before: before.modes, after: after.modes,
    oldBrowser: before.modes.filter((m) => universe.includes(m)).length,
    newBrowser: after.modes.filter((m) => universe.includes(m)).length };
});
const own = JSON.parse(read("../ci-select-backend-browser/browser-ownership.json"));
const appCount = (app) => Object.values(own.modes).filter((r) => r.apps.includes(app)).length;
const oldTotal = measured.reduce((n, r) => n + r.oldBrowser, 0), newTotal = measured.reduce((n, r) => n + r.newBrowser, 0);
const content = [
  "# CI-SPEED-3 static selection impact", "",
  `Measured at ${new Date().toISOString()}; E1 static source/diff measurement, not runtime acceptance.`,
  "Source: GitHub merged PR metadata and paginated PR files API receipts in this directory. All 15 PRs merged since 2026-10-27 were retrieved, sorted by mergedAt descending, then the newest ten selected (not PR creation order).",
  "Both policies use the same integration registry snapshot (997480141a7959ad6256a29e43bb1af31b97a404) and current source observations; historical PR CI/registry sizes are not compared. Unknown/deleted paths conservatively use the full current universe. Counts exclude foundation-shards, deploy, shards and job bundling.", "",
  `Registry SHA256: ${createHash("sha256").update(registry).digest("hex")}; runnable browser universe: ${universe.length}. Old planner: RED SHA 04036e51ba4e7ed1236a825e1f96c635f563ec44, frozen in pr-modes-before.mjs.`, "",
  "| PR | Merged UTC | Paths | Old browser modes | New browser modes | Reduction |", "| --- | --- | ---: | ---: | ---: | ---: |",
  ...measured.map((r) => `| [#${r.number}](https://github.com/humaux/live_commerce_architecture_v1/pull/${r.number}) | ${r.mergedAt} | ${r.paths} | ${r.oldBrowser} | ${r.newBrowser} | ${r.oldBrowser - r.newBrowser} |`), "",
  `Total mode selections: ${oldTotal} -> ${newTotal} (${oldTotal ? ((oldTotal - newTotal) / oldTotal * 100).toFixed(1) : 0}% reduction).`, "",
  `App-level ownership remains broad: admin ${appCount("admin")}, storefront ${appCount("storefront")}. The <=15 single-app target is NOT achieved by safe app-level ownership on this registry; meeting it needs a separately authorised component/route selection contract. No ownership was omitted to meet a count.`,
  `The base-present live-settings.spec.ts has ${Object.values(own.modes).filter((r) => r.inputs.includes("tests/admin/live-settings.spec.ts")).length} executing browser owner, versus ${universe.length} previously. native-device.ts has ${Object.values(own.modes).filter((r) => r.inputs.includes("tests/admin/fixtures/native-device.ts")).length} importing owners.`,
  "Shared infrastructure in historical diffs can dominate the ten-PR sample and keep the full universe. This comparison claims neither wall-clock savings nor production acceptance.", "",
  "Reproduce the static measurement: node output/ci-speed-3/measure-impact.mjs. CI acceptance is separate and pending.", "",
];
writeFileSync(new URL("impact.json", import.meta.url), JSON.stringify(measured, null, 2) + "\n");
writeFileSync(new URL("impact.md", import.meta.url), content.join("\n"));
console.log(content.slice(7).join("\n"));
