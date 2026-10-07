// Purpose: the whole-run verdict of a sharded G-UI8 click sweep / G-UI9 visual lint: reads every shard's evidence and FAILS unless the shards together cover the full
//   unit universe exactly once (every admin registry route x variant, every storefront route x variant, every journey, every visual shot), with no failure and no stale known defect.
// Depends on: tests/ui/sweep-shard-lib.mjs (the same plan the runners used), tests/ui/click-sweep-lib.mjs (matchKnown), apps/admin/src/routes.ts (the live route registry),
//   tests/ui/click-sweep-weights.json, tests/ui/click-sweep-known-defects.json; node >= 24 (type stripping for routes.ts). No browser, no network, no pnpm install needed.
// Used by: .github/workflows/gates.yml (aggregate job: `node tests/ui/sweep-aggregate.mjs click|visual <artifact dir>`), tests/ui/sweep-shard-lib.test.mjs.
// Invariants: a shard that is missing, incomplete, disagrees about the universe, or whose slice differs from the plan is a failure, never a skip; the registry is re-read here,
//   so a route added to apps/admin/src/routes.ts that no shard opened cannot pass.
import { existsSync, readdirSync, readFileSync, statSync, mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { matchKnown } from "./click-sweep-lib.mjs";
import { JOURNEY_PAGES, VISUAL_LOCALES, VISUAL_VIEWPORTS, digest, loadWeights, pageKey, planClick, routeId } from "./sweep-shard-lib.mjs";

const sizeOf = (v) => `${v.size.width}x${v.size.height}`;
const sameSet = (a, b) => a.length === b.length && new Set([...a, ...b]).size === a.length;

/** Checks the shard indices: all reports agree on N, every index 1..N appears exactly once. Pushes problems; returns N (or 0). */
function checkIndices(shards, problems, label) {
  if (!shards.length) { problems.push(`no ${label} shard evidence found: every shard job failed before writing it, or the artifacts were not downloaded`); return 0; }
  const bare = shards.filter((s) => !s.report.shard);
  for (const s of bare) problems.push(`${s.source}: not a shard ${label} (no summary.shard): the job ran without LC_SWEEP_SHARD`);
  const sharded = shards.filter((s) => s.report.shard);
  const ofs = new Set(sharded.map((s) => s.report.shard.of));
  if (ofs.size !== 1) { problems.push(`${label} shards disagree on N: ${[...ofs].join(", ") || "none"}`); return 0; }
  const of = [...ofs][0], seen = new Map();
  for (const s of sharded) { const i = s.report.shard.index; if (seen.has(i)) problems.push(`${label} shard ${i}/${of} found twice (${seen.get(i)} and ${s.source})`); seen.set(i, s.source); }
  for (let i = 1; i <= of; i++) if (!seen.has(i)) problems.push(`${label} shard ${i}/${of} is missing: its job failed before writing evidence`);
  for (const s of sharded) if (!s.report.shard.complete) problems.push(`${label} shard ${s.report.shard.index}/${of} ran with a page filter / LC_SWEEP_ONLY: not a full-coverage run`);
  return of;
}

/**
 * Click sweep: shards = [{ source, report: { summary, rows }, platformPass }]. Returns { problems, lines }.
 * Re-derives the plan from the live registry and compares each shard's slice with it, then re-checks the ledger rows themselves.
 */
export function verifyClick({ shards, adminRoutes, weights, known }) {
  const problems = [], lines = [];
  const reports = shards.map((s) => ({ ...s, report: { ...s.report, shard: s.report.summary.shard } }));
  const of = checkIndices(reports, problems, "click-sweep");
  if (!of) return { problems, lines };
  const plan = planClick(adminRoutes, weights, of);
  const universe = plan.pages.map((p) => p.page), want = { count: universe.length, sha256: digest(universe) };
  let controls = 0, steps = 0, loads = 0;
  for (const s of reports) {
    const sh = s.report.shard; if (!sh) continue;
    const tag = `click-sweep shard ${sh.index}/${of}`;
    if (sh.universe.count !== want.count || sh.universe.sha256 !== want.sha256) problems.push(`${tag}: its universe (${sh.universe.count} pages) differs from the live registry's (${want.count} pages): stale checkout or a route added after the shard started`);
    const owned = plan.pages.filter((p) => plan.owner.get(p.unit) === sh.index).map((p) => p.page);
    if (!sameSet(sh.owned, owned)) problems.push(`${tag}: its slice (${sh.owned.length} pages) is not the planned slice (${owned.length} pages)`);
    const rows = s.report.rows, opened = new Set(rows.filter((r) => r.control === "(page load)").map((r) => pageKey(r.app, r.page, r.viewport, r.locale)));
    const missing = owned.filter((k) => !opened.has(k));
    if (missing.length) problems.push(`${tag}: ${missing.length} planned page(s) have no page-load row: ${missing.slice(0, 5).join(" ; ")}${missing.length > 5 ? " ..." : ""}`);
    loads += opened.size; controls += s.report.summary.counts.controls.total; steps += s.report.summary.counts.journeys.total;
    const ownedJourneys = Object.keys(plan.journeyOwner).filter((j) => plan.owner.get(plan.journeyOwner[j]) === sh.index);
    if (!sameSet(sh.journeys ?? [], ownedJourneys)) problems.push(`${tag}: it ran journeys [${(sh.journeys ?? []).join(",")}], the plan gives it [${ownedJourneys.join(",")}]`);
    for (const j of ownedJourneys) {
      const have = new Set(rows.filter((r) => r.app === "journey").map((r) => r.page));
      for (const page of JOURNEY_PAGES[j]) if (!have.has(page)) problems.push(`${tag}: journey ${j} wrote no ledger row for "${page}"`);
    }
  }
  const fails = reports.flatMap((s) => s.report.rows.filter((r) => r.result === "fail").map((r) => ({ ...r, shard: s.report.shard?.index })));
  for (const f of fails.slice(0, 20)) problems.push(`FAIL (shard ${f.shard}) ${f.app} ${f.page} ${f.viewport}/${f.locale} "${f.control}" ${f.failure}: ${String(f.actual).slice(0, 160)}`);
  if (fails.length > 20) problems.push(`... and ${fails.length - 20} more failing rows`);
  for (const k of matchKnown(fails, known).stale) problems.push(`STALE known-defect ${k.id} (${k.route} ${k.kind}): it no longer reproduces in any shard; remove the entry`);
  const platform = reports.find((s) => s.report.shard?.index === 1);
  if (platform && !platform.platformPass) problems.push("click-sweep shard 1/N has no platform-runner.pass marker: the PS5 public-route runner (tests/admin/platform-runner.mjs) did not pass");
  lines.push(`click sweep: ${of} shards, ${loads} page units opened of ${want.count} planned (${adminRoutes.length} admin registry routes x 3 variants + storefront), ${controls} control clicks, ${steps} journey steps, ${fails.length} failing rows`);
  return { problems, lines };
}

/**
 * Visual lint: shards = [{ source, report }] with lint.json contents. Every enumerated shot must exist in exactly one shard (or be an explicit NOT_RUN),
 * the live registry's admin routes must be inside the universe, and no shard may carry a blocking verdict.
 */
export function verifyVisual({ shards, adminRoutes }) {
  const problems = [], lines = [];
  const of = checkIndices(shards, problems, "visual-lint");
  if (!of) return { problems, lines };
  const union = shards.flatMap((s) => s.report.shard?.owned ?? []);
  const first = shards.find((s) => s.report.shard), uni = first.report.shard.universe; // checkIndices returned N > 0, so at least one shard report exists
  for (const s of shards) {
    const sh = s.report.shard; if (!sh) continue;
    if (sh.universe.count !== uni.count || sh.universe.sha256 !== uni.sha256) problems.push(`visual-lint shard ${sh.index}/${of}: it enumerated a different universe (${sh.universe.count} shots) than shard ${first.report.shard.index} (${uni.count})`);
  }
  if (new Set(union).size !== union.length) problems.push(`visual-lint shards overlap: ${union.length - new Set(union).size} shot(s) are owned by more than one shard`);
  if (new Set(union).size !== uni.count || digest(new Set(union)) !== uni.sha256) problems.push(`visual-lint shards together own ${new Set(union).size} shots, the universe has ${uni.count}: a unit belongs to no shard`);
  const have = new Set(union);
  for (const r of adminRoutes) for (const l of VISUAL_LOCALES) for (const v of VISUAL_VIEWPORTS) {
    const k = `admin|${routeId(r.path)}|${l}|${sizeOf(v)}`;
    if (!have.has(k)) problems.push(`registry route ${r.path} at ${l}/${sizeOf(v)} is in no shard's slice (${k})`);
  }
  let captured = 0, blocking = 0, notRun = 0;
  for (const s of shards) {
    const sh = s.report.shard; if (!sh) continue;
    const tag = `visual-lint shard ${sh.index}/${of}`;
    const shot = new Set(s.report.units.filter((u) => u.shot).map((u) => `${u.app}|${u.id}|${u.locale}|${u.size}`));
    const nr = new Set(s.report.units.filter((u) => u.notRun).map((u) => `${u.app}|${u.id}|${u.locale}|${u.size}`));
    const missing = sh.owned.filter((k) => !shot.has(k) && !nr.has(k));
    if (missing.length) problems.push(`${tag}: ${missing.length} owned shot(s) missing: ${missing.slice(0, 5).join(" ; ")}${missing.length > 5 ? " ..." : ""}`);
    captured += sh.owned.filter((k) => shot.has(k)).length; notRun += sh.owned.filter((k) => nr.has(k)).length; blocking += s.report.blockingInstances ?? 0;
    if (s.report.verdict?.exit !== 0) problems.push(`${tag}: verdict exit ${s.report.verdict?.exit}: ${(s.report.verdict?.reasons ?? []).join("; ")}`);
  }
  lines.push(`visual lint: ${of} shards, ${captured} shots captured of ${uni.count} enumerated (NOT_RUN ${notRun}), ${blocking} blocking violation instances`);
  return { problems, lines };
}

// ---- artifact discovery (CLI only) ----------------------------------------------------------------------------------------------------------------------------
const walk = (dir) => readdirSync(dir).flatMap((n) => { const p = path.join(dir, n); return statSync(p).isDirectory() ? walk(p) : [p]; });

/** Finds each shard's evidence under an artifact download directory (one sub-directory per shard job). */
export function discover(kind, dir) {
  const files = existsSync(dir) ? walk(dir) : []; // no artifacts downloaded at all = no shard evidence = a failure reported by the verifier, not a crash here
  if (kind === "click") {
    return files.filter((f) => path.basename(f) === "ledger.json" && f.includes(`${path.sep}ui-click-sweep${path.sep}`)).map((f) => ({
      source: path.relative(dir, f), report: JSON.parse(readFileSync(f, "utf8")), platformPass: existsSync(path.join(path.dirname(f), "platform-runner.pass")),
    }));
  }
  // the timestamped output dirs only: output/ui-visual-audit/baseline-* are tracked reference runs, not evidence of this one
  return files.filter((f) => path.basename(f) === "lint.json" && /[\\/]ui-visual-audit[\\/]\d{8}T\d{6}Z[\\/]lint\.json$/.test(f)).map((f) => ({ source: path.relative(dir, f), report: JSON.parse(readFileSync(f, "utf8")) }));
}

export async function main(argv) {
  const [kind, dir] = argv;
  if (!["click", "visual"].includes(kind) || !dir) { console.error("usage: node tests/ui/sweep-aggregate.mjs click|visual <artifact dir>"); return 2; }
  const { routes: adminRoutes } = await import("../../apps/admin/src/routes.ts");
  const shards = discover(kind, dir);
  const result = kind === "click"
    ? verifyClick({ shards, adminRoutes, weights: await loadWeights(), known: JSON.parse(readFileSync(new URL("./click-sweep-known-defects.json", import.meta.url), "utf8")) })
    : verifyVisual({ shards, adminRoutes });
  const ok = result.problems.length === 0;
  const text = [`# ${kind === "click" ? "G-UI8 click sweep" : "G-UI9 visual lint"} aggregate: ${ok ? "PASS" : "FAIL"}`, "", ...result.lines, "", ...result.problems.map((p) => `- ${p}`), ""].join("\n");
  mkdirSync("output/ci-aggregate", { recursive: true });
  writeFileSync(`output/ci-aggregate/${kind}.md`, text);
  console.log(text);
  return ok ? 0 : 1;
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) process.exit(await main(process.argv.slice(2)));
