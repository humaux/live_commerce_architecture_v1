#!/usr/bin/env node
// Purpose: turn the `modes` a gates.yml run was asked for into the job matrix: `--browser-click-sweep` and `--browser-visual-lint` fan out into LC_SWEEP_SHARD
//   slices (`--browser-click-sweep@3/10`), `foundation-shards` into the unit shard plus every group of scripts/dev/shard-plan.json, everything else passes through.
//   Also says whether a whole sweep / visual-lint was requested, so the workflow's aggregate job knows to prove the slices cover the full universe.
// Depends on: scripts/dev/shard-plan.json (group ids); env MODES (JSON array) and GITHUB_OUTPUT (matrix, sweep_n, visual_n); no network.
// Used by: .github/workflows/gates.yml (plan job), tests/ci/ci-plan.test.mjs.
// Invariants: an expanded mode always yields ALL its shards (a partial list is only ever the integrator's explicit `@i/N` re-run, which skips the aggregate);
//   artifact names are unique per job, so the aggregate job can find each shard's evidence by pattern.
import { appendFileSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { canonicalEvidenceDrift } from "./matrix-receipt.mjs";

/** Click-sweep shards per full run. Measured on 2026-10-07: ~7600 s of single-worker route time + ~5 min of per-job setup/builds; 10 shards = ~12-16 min per job. Retune from the shard times in PLAN.md. */
export const SWEEP_SHARDS = 10;
/** Visual-lint shards per full run (audit workers stay at 3: it only reads). 14 min whole -> ~7-9 min per shard. */
export const VISUAL_SHARDS = 4;

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const fanOut = (mode, n) => Array.from({ length: n }, (_, i) => `${mode}@${i + 1}/${n}`);

/** Artifact-safe, unique-per-mode name (also the pattern prefix the aggregate job downloads). */
export const artifactName = (mode) => `gate-${mode.replace(/^-+/, "").replace(/[^A-Za-z0-9_.-]+/g, "_").replace(/_+$/, "")}`;

/** modes (strings) -> { matrix: [{ mode, art }], sweepN, visualN } with sweepN/visualN = N when every shard 1..N of that gate is in the matrix, else 0. */
export function expand(modes, { groupIds, sweep = SWEEP_SHARDS, visual = VISUAL_SHARDS, matrixDrift = false, catchAllId = null }) {
  if (!Array.isArray(modes) || !modes.length || modes.some((m) => typeof m !== "string" || !/^[\x20-\x7e]+$/.test(m))) throw new Error("modes must be a non-empty JSON array of printable strings");
  const out = [];
  for (const m of modes) {
    if (m === "foundation-shards") out.push("shard:unit", ...groupIds.map((id) => `shard:${id}`));
    else if (m === "--browser-click-sweep") out.push(...fanOut(m, sweep));
    else if (m === "--browser-visual-lint") out.push(...fanOut(m, visual));
    else out.push(m);
  }
  const unique = [...new Set(out)];
  if (matrixDrift && catchAllId && unique.includes("shard:unit") && unique.includes(`shard:${catchAllId}`) &&
      !unique.includes("--browser-product-editor"))
    unique.push("--browser-product-editor");
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

export function main(env = process.env) {
  const plan = JSON.parse(readFileSync(path.join(root, "scripts/dev/shard-plan.json"), "utf8"));
  const modes = JSON.parse(env.MODES ?? "");
  const catchAll = plan.groups.filter((group) => group.catch_all === true);
  if (catchAll.length !== 1) throw new Error("shard plan must have exactly one catch-all group");
  const hasMatrixFamily = modes.includes("foundation-shards") || modes.includes(`shard:${catchAll[0].id}`);
  const matrixDrift = hasMatrixFamily && canonicalEvidenceDrift().length > 0;
  const result = expand(modes, { groupIds: plan.groups.map((g) => g.id), catchAllId: catchAll[0].id, matrixDrift });
  console.log(`ci-plan: ${result.matrix.length} job(s)${result.sweepN ? `; click-sweep x${result.sweepN} (aggregate)` : ""}${result.visualN ? `; visual-lint x${result.visualN} (aggregate)` : ""}`);
  for (const e of result.matrix) console.log(`  ${e.mode}  -> artifact ${e.art}`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `matrix=${JSON.stringify(result.matrix)}\nsweep_n=${result.sweepN}\nvisual_n=${result.visualN}\n`);
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`ci-plan: ${e.message}`); process.exit(1); }
}
