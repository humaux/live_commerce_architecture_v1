#!/usr/bin/env node
// Purpose: turn the `modes` a gates.yml run was asked for into the job matrix: `--browser-click-sweep` and `--browser-visual-lint` fan out into LC_SWEEP_SHARD
//   slices (`--browser-click-sweep@3/10`), `foundation-shards` into the unit shard plus every group of scripts/dev/shard-plan.json, everything else passes through.
//   Also says whether a whole sweep / visual-lint was requested, so the workflow's aggregate job knows to prove the slices cover the full universe.
//   CI-SPEED: names the matrix modes that consume the run's one prebuilt artifact (`prebuilt_modes`: test-local.sh registry arms that build a Next app or
//   compile ./tests/foundation), so gates.yml builds once per run instead of once per job.
//   CI-SPEED-2: `jobs` packs the short legs (GATE_SECONDS, measured < BUNDLE_LIMIT_S) into bundle jobs; `matrix` (the planned mode list receipts verify) is unchanged.
// Depends on: scripts/dev/shard-plan.json (group ids), scripts/dev/pr-modes.mjs (modeEntries: read-only test-local.sh registry); env MODES (JSON array)
//   BUNDLE ("true" = pack short legs; gates.yml passes false for an explicit dispatch) and GITHUB_OUTPUT (matrix, jobs, sweep_n, visual_n, prebuilt_modes, prebuild); no network.
// Used by: .github/workflows/gates.yml (plan job; its matrix output is also the planned list scripts/dev/matrix-receipts.mjs verifies), tests/ci/ci-plan.test.mjs.
// Invariants: an expanded mode always yields ALL its shards (a partial list is only ever the integrator's explicit `@i/N` re-run, which skips the aggregate);
//   artifact names are unique per job, so the aggregate job can find each shard's evidence by pattern;
//   every matrix mode is in exactly one job of `jobs`, a bundle has 2..BUNDLE_MAX members, all measured short, and each keeps its own mode/art (own receipt + artifact).
import { appendFileSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { modeEntries } from "./pr-modes.mjs";

/** Click-sweep shards per full run. Measured on 2026-10-07: ~7600 s of single-worker route time + ~5 min of per-job setup/builds; 10 shards = ~12-16 min per job. Retune from the shard times in PLAN.md. */
export const SWEEP_SHARDS = 10;
/** Visual-lint shards per full run (audit workers stay at 3: it only reads). 14 min whole -> ~7-9 min per shard. */
export const VISUAL_SHARDS = 4;

/**
 * Gate-step seconds per mode: the max of the three build-once UI-scale runs 40506418652, 40541796588 and 40575283835 (2026-10-27/28, n=3).
 * Only modes listed here AND under BUNDLE_LIMIT_S are bundled; a new or unmeasured mode always gets its own job. Re-measure when a leg's suite grows.
 * Excluded on purpose (>= limit): --browser-live-console 379, --browser-webkit 408, the click-sweep / visual-lint slices and the foundation shards.
 */
export const GATE_SECONDS = {
  "--browser-studio-bff": 14, "--browser-manual-order": 17, "--browser-merchant-buyer": 17, "--browser-merchant-orders-bff": 21, "--browser-buyer": 22,
  "--browser-catalog-media": 22, "--browser-input-delivery": 23, "--browser-storefront": 26, "--browser-store-domains": 26, "--browser-live-claims": 27,
  "--browser-meta-health-ui": 28, "--browser-buyer-comms": 28, "--browser-promotions": 33, "--browser-returns-ui": 33, "--browser-claim-checkout": 35,
  "--browser-operations-ads": 36, "--browser-catalog-core": 36, "--browser-reports": 37, "--browser-design": 37, "--browser-storefront-publish": 39,
  "--browser-home-cod": 42, "--browser-live-settings": 43, "--browser-picklist": 45, "--browser-migration-import": 48, "--browser-checkout-offline": 50,
  "--browser-meta-ads": 51, "--browser-order": 53, "--browser-ops-polish": 57, "--browser-meta-connect": 59, "--browser-identity": 60,
  "--browser-refund-fulfilment": 72, "--browser-studio-ui": 74, "--browser-card-payments": 82, "--browser-product-media-v2": 83, "--browser-admin-legacy": 84,
  "--browser-e2e": 87, "--browser-inbox": 88, "--browser-customers-billing": 104, "--browser-password-auth": 110, "--browser-admin-shell": 117,
  "--browser-merchant-orders-ui": 137, "--live-browser-input": 139, "--browser-cvs": 145, "--browser-tracking-backfill": 146, "--browser-platform-site": 159,
  "--browser-ads-attribution": 162, "--browser-product-editor": 198, "--browser-payment": 279, "--browser-ui-gallery": 315,
};
/** A leg is "short" below this gate time (owner card CI-SPEED-2: ~6 min); long legs keep their own job. */
export const BUNDLE_LIMIT_S = 360;
/** Target gate seconds per bundle (~10 bundles for a UI PR; a bundle stays well under the ~18 min sweep / shard legs, so wall clock is not lengthened). */
export const BUNDLE_TARGET_S = 420;
/** Most members one bundle job carries: gates.yml unrolls exactly this many per-member upload slots (tests/ci/gate-bundle.test.mjs pins both). */
export const BUNDLE_MAX = 8;

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const fanOut = (mode, n) => Array.from({ length: n }, (_, i) => `${mode}@${i + 1}/${n}`);

/** Artifact-safe, unique-per-mode name (also the pattern prefix the aggregate job downloads). */
export const artifactName = (mode) => `gate-${mode.replace(/^-+/, "").replace(/[^A-Za-z0-9_.-]+/g, "_").replace(/_+$/, "")}`;

/** modes (strings) -> { matrix: [{ mode, art }], sweepN, visualN } with sweepN/visualN = N when every shard 1..N of that gate is in the matrix, else 0. */
export function expand(modes, { groupIds, sweep = SWEEP_SHARDS, visual = VISUAL_SHARDS }) {
  if (!Array.isArray(modes) || !modes.length || modes.some((m) => typeof m !== "string" || !/^[\x20-\x7e]+$/.test(m))) throw new Error("modes must be a non-empty JSON array of printable strings");
  const out = [];
  for (const m of modes) {
    if (m === "foundation-shards") out.push("shard:unit", ...groupIds.map((id) => `shard:${id}`));
    else if (m === "--browser-click-sweep") out.push(...fanOut(m, sweep));
    else if (m === "--browser-visual-lint") out.push(...fanOut(m, visual));
    else out.push(m);
  }
  const unique = [...new Set(out)];
  const matrix = unique.map((mode) => ({ mode, art: artifactName(mode) }));
  const arts = matrix.map((e) => e.art);
  if (new Set(arts).size !== arts.length) throw new Error(`two modes map to the same artifact name: ${arts.filter((a, i) => arts.indexOf(a) !== i).join(", ")}`);
  const whole = (prefix) => {
    const re = new RegExp(`^${prefix}@(\\d+)/(\\d+)$`), got = unique.map((m) => re.exec(m)).filter(Boolean);
    const ns = new Set(got.map((g) => g[2]));
    if (ns.size !== 1) return 0;
    const n = Number([...ns][0]);
    return new Set(got.map((g) => g[1])).size === n ? n : 0;
  };
  return { matrix, sweepN: whole("--browser-click-sweep"), visualN: whole("--browser-visual-lint") };
}

/**
 * matrix -> jobs: [{ mode, art, bundle, timeout, members: [{ mode, art }] }]. Short measured legs are packed longest-first into the least-loaded of
 * ceil(total / BUNDLE_TARGET_S) bundles (ties by name, so the plan is deterministic); every other mode is a solo job exactly as before.
 * A one-member "bundle" is just a solo job. Per-mode timeout stays 150 min (the solo job bound); a bundle job gets the hosted maximum, 360.
 */
export function bundleJobs(matrix, { enabled = true, seconds = GATE_SECONDS, limit = BUNDLE_LIMIT_S, target = BUNDLE_TARGET_S, max = BUNDLE_MAX } = {}) {
  const solo = (e) => ({ mode: e.mode, art: e.art, bundle: false, timeout: 150, members: [{ mode: e.mode, art: e.art }] });
  const short = enabled ? matrix.filter((e) => Number.isFinite(seconds[e.mode]) && seconds[e.mode] < limit) : [];
  if (short.length < 2) return matrix.map(solo);
  const total = short.reduce((s, e) => s + seconds[e.mode], 0);
  const bins = Array.from({ length: Math.max(Math.ceil(total / target), Math.ceil(short.length / max)) }, () => ({ s: 0, members: [] }));
  for (const e of [...short].sort((a, b) => seconds[b.mode] - seconds[a.mode] || a.mode.localeCompare(b.mode))) {
    const open = bins.filter((b) => b.members.length < max);
    const bin = open.reduce((lo, b) => (b.s < lo.s ? b : lo), open[0]);
    bin.s += seconds[e.mode]; bin.members.push({ mode: e.mode, art: e.art });
  }
  const isShort = new Set(short.map((e) => e.mode));
  const jobs = matrix.filter((e) => !isShort.has(e.mode)).map(solo);
  let n = 0;
  for (const b of bins.filter((x) => x.members.length)) {
    if (b.members.length === 1) { jobs.push(solo(b.members[0])); continue; }
    const id = String(++n).padStart(2, "0");
    jobs.push({ mode: `bundle:${id}`, art: `gate-bundle_${id}`, bundle: true, timeout: 360, members: b.members });
  }
  return jobs;
}

/** Matrix modes whose test-local.sh arm builds a Next app or compiles ./tests/foundation: they reuse the run's prebuilt Next output + Go build cache. */
export function prebuiltModes(matrix, registrySource) {
  const byName = new Map(modeEntries(registrySource).map((e) => [e.name, e]));
  return matrix.map((e) => e.mode).filter((mode) => {
    const entry = byName.get(mode.replace(/@\d+\/\d+$/, ""));
    return !!entry && (entry.build !== "none" || /go test[^\n]*\.\/tests\/foundation/.test(entry.run.replace(/\\\r?\n/g, " ")));
  });
}

export function main(env = process.env) {
  const plan = JSON.parse(readFileSync(path.join(root, "scripts/dev/shard-plan.json"), "utf8"));
  const result = expand(JSON.parse(env.MODES ?? ""), { groupIds: plan.groups.map((g) => g.id) });
  const prebuiltMembers = new Set(prebuiltModes(result.matrix, readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8")));
  const jobs = bundleJobs(result.matrix, { enabled: env.BUNDLE === "true" });
  // A job (solo or bundle) restores the prebuilt artifact when any of its members consumes it.
  const prebuilt = jobs.filter((j) => j.members.some((m) => prebuiltMembers.has(m.mode))).map((j) => j.mode);
  console.log(`ci-plan: ${result.matrix.length} mode(s)${result.sweepN ? `; click-sweep x${result.sweepN} (aggregate)` : ""}${result.visualN ? `; visual-lint x${result.visualN} (aggregate)` : ""}`);
  for (const e of result.matrix) console.log(`  ${e.mode}  -> artifact ${e.art}`);
  console.log(`ci-plan: ${jobs.length} job(s), ${jobs.filter((j) => j.bundle).length} bundle(s)`);
  for (const j of jobs.filter((x) => x.bundle)) console.log(`  ${j.mode}: ${j.members.map((m) => m.mode).join(" ")}`);
  console.log(`ci-plan: ${prebuilt.length} job(s) reuse the prebuilt Next + Go build cache`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `matrix=${JSON.stringify(result.matrix)}\njobs=${JSON.stringify(jobs)}\nsweep_n=${result.sweepN}\nvisual_n=${result.visualN}\nprebuilt_modes=${JSON.stringify(prebuilt)}\nprebuild=${prebuilt.length > 0}\n`);
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`ci-plan: ${e.message}`); process.exit(1); }
}
