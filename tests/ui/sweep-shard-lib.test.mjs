// Unit tests of LC_SWEEP_SHARD partitioning and of the whole-run aggregate (tests/ui/sweep-aggregate.mjs): every unit of the live route registry is owned by exactly one
// shard for any N, and the aggregate turns a missing shard / page / journey / registry route into a failure. Run by scripts/dev/test-node.sh (no browser, no PG).
import assert from "node:assert/strict";
import test from "node:test";
import { routes as adminRoutes } from "../../apps/admin/src/routes.ts";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { discover, verifyClick, verifyVisual } from "./sweep-aggregate.mjs";
import { CLICK_VARIANTS, JOURNEY_PAGES, STOREFRONT_ROUTES, VISUAL_LOCALES, VISUAL_VIEWPORTS, assignShards, loadWeights, parseShard, planClick, routeId, shardReport } from "./sweep-shard-lib.mjs";

const weights = await loadWeights();

// The storefront twin of G-UI1 (tests/admin/shell-registry.test.ts): the click sweep and visual lint open STOREFRONT_ROUTES, a hand list, so a
// new buyer page.tsx would otherwise never be clicked or shot. Route groups like "(list)" do not appear in URLs.
test("G-UI8 storefront pages and the sweep's STOREFRONT_ROUTES are the same set", () => {
  const base = "apps/storefront/app/[locale]";
  const walk = (dir) => readdirSync(dir, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? walk(`${dir}/${e.name}`) : [`${dir}/${e.name}`]));
  const pages = walk(base).filter((p) => p.endsWith("/page.tsx"))
    // strip only COMPLETE route-group segments ("/(list)"), never a literal-parenthesis prefix such as "/(review)-link" (a distinct URL)
    .map((p) => p.slice(base.length, -"/page.tsx".length).replace(/\/\([^/()]+\)(?=\/|$)/g, "") || "/");
  assert.deepEqual([...new Set(pages)].sort(), [...STOREFRONT_ROUTES].sort());
  assert.equal(new Set(pages).size, pages.length, "two page.tsx files map to one URL: a group/literal-parenthesis mix-up would hide a page");
});

test("parseShard: i/N, empty means unsharded, anything malformed throws", () => {
  assert.equal(parseShard(""), null);
  assert.equal(parseShard(undefined), null);
  assert.deepEqual(parseShard("3/12"), { index: 3, of: 12 });
  for (const bad of ["0/4", "5/4", "1", "a/b", "1/0", "-1/4", "1/4/2", " 1/4"]) assert.throws(() => parseShard(bad), /LC_SWEEP_SHARD/, bad);
});

test("assignShards: deterministic, every key owned once, near-balanced (LPT bound)", () => {
  const units = Array.from({ length: 40 }, (_, i) => ({ key: `u${i}`, weight: 1 + ((i * 37) % 23) }));
  for (const of of [1, 2, 3, 7, 12, 40, 55]) {
    const a = assignShards(units, of), b = assignShards([...units].reverse(), of);
    assert.deepEqual([...a].sort(), [...b].sort(), "input order must not change the partition");
    assert.equal(a.size, units.length);
    const load = Array(of).fill(0); for (const u of units) { const i = a.get(u.key); assert.ok(i >= 1 && i <= of); load[i - 1] += u.weight; }
    const total = load.reduce((x, y) => x + y, 0), maxW = Math.max(...units.map((u) => u.weight));
    assert.ok(Math.max(...load) <= total / of + maxW, `N=${of}: max load ${Math.max(...load)} exceeds mean ${total / of} + heaviest unit ${maxW}`);
  }
  assert.throws(() => assignShards([{ key: "a", weight: 1 }, { key: "a", weight: 2 }], 2), /unique/);
});

test("planClick: for every N the shards partition the whole universe (every registry route x variant, storefront walk, journey)", () => {
  assert.ok(adminRoutes.length >= 25, "the registry is read, not stubbed");
  for (const of of [1, 2, 6, 10, 12, 20, 200]) {
    const plan = planClick(adminRoutes, weights, of);
    const seen = new Map();
    for (const p of plan.pages) { const i = plan.owner.get(p.unit); assert.ok(i >= 1 && i <= of, p.page); assert.ok(!seen.has(p.page), `duplicate page ${p.page}`); seen.set(p.page, i); }
    assert.equal(seen.size, (adminRoutes.length + 17) * CLICK_VARIANTS.length, "admin routes + 17 storefront routes, each at all variants");
    for (const r of adminRoutes) for (const v of CLICK_VARIANTS) assert.ok(seen.has(`admin|${r.path}|${v.viewport}|${v.locale}`), `${r.path} ${v.viewport}/${v.locale}`);
    for (const [j, unit] of Object.entries(plan.journeyOwner)) assert.ok(plan.owner.has(unit), `journey ${j} has an owning unit`);
    // J2/J3 read the COD order of the storefront desktop/zh-TW session: same shard, always
    assert.equal(plan.owner.get(plan.journeyOwner.J23), plan.owner.get("storefront|desktop|zh-TW"));
    // a storefront walk is never split: all its pages sit in one shard
    for (const v of CLICK_VARIANTS) assert.equal(new Set(plan.pages.filter((p) => p.unit === `storefront|${v.viewport}|${v.locale}`).map((p) => plan.owner.get(p.unit))).size, 1);
  }
});

test("planClick at N=10 keeps every shard within ~1.3x of the mean planned cost", () => {
  const plan = planClick(adminRoutes, weights, 10), load = Array(10).fill(0);
  for (const u of plan.units) load[plan.owner.get(u.key) - 1] += u.weight;
  const mean = load.reduce((a, b) => a + b, 0) / 10;
  assert.ok(Math.max(...load) <= mean * 1.3, `loads ${load.join(",")} mean ${Math.round(mean)}`);
  assert.ok(Math.min(...load) >= mean * 0.7, `loads ${load.join(",")} mean ${Math.round(mean)}`);
});

// ---- the aggregate: build what N honest runners would have written, then break it one way at a time --------------------------------------------------------------
function clickShards(of, routes = adminRoutes) {
  const plan = planClick(routes, weights, of), universe = plan.pages.map((p) => p.page);
  return Array.from({ length: of }, (_, k) => {
    const index = k + 1, shard = { index, of };
    const owned = plan.pages.filter((p) => plan.owner.get(p.unit) === index).map((p) => p.page);
    const journeys = Object.keys(plan.journeyOwner).filter((j) => plan.owner.get(plan.journeyOwner[j]) === index);
    const rows = owned.map((page) => { const [app, route, viewport, locale] = page.split("|"); return { app, page: route, viewport, locale, control: "(page load)", kind: "page", result: "pass" }; });
    for (const j of journeys) for (const page of JOURNEY_PAGES[j]) rows.push({ app: "journey", page, viewport: "desktop", locale: "zh-TW", control: "step", kind: "journey", result: "pass" });
    return { source: `shard-${index}`, platformPass: index === 1, report: { summary: { shard: shardReport(shard, universe, owned, { complete: true, journeys }), counts: { controls: { total: 5 }, journeys: { total: 1 } } }, rows } };
  });
}
const clickArgs = (shards) => ({ shards, adminRoutes, weights, known: [] });

test("aggregate click: honest shards pass for several N", () => {
  for (const of of [1, 4, 10, 12]) assert.deepEqual(verifyClick(clickArgs(clickShards(of))).problems, [], `N=${of}`);
});

test("aggregate click: a missing shard, a dropped page row, a lost journey, a failing row, a stale known defect and a missing platform marker are each a failure", () => {
  const bad = (mutate, re) => { const s = clickShards(6); mutate(s); const r = verifyClick(clickArgs(s)); assert.ok(r.problems.some((p) => re.test(p)), `${re}: ${r.problems.join(" | ") || "no problem raised"}`); };
  bad((s) => s.splice(2, 1), /shard 3\/6 is missing/);
  bad((s) => s.push({ ...s[0], source: "dup" }), /found twice/);
  bad((s) => { s[1].report.rows = s[1].report.rows.filter((r, i) => i !== 0); }, /no page-load row/);
  bad((s) => { for (const x of s) x.report.rows = x.report.rows.filter((r) => r.app !== "journey"); }, /wrote no ledger row/);
  bad((s) => { s[0].report.rows.push({ app: "admin", page: "/x", viewport: "desktop", locale: "en", control: "Save", result: "fail", failure: "no-effect", actual: "nothing" }); }, /FAIL \(shard 1\)/);
  bad((s) => { s[0].platformPass = false; }, /platform-runner/);
  bad((s) => { s[3].report.summary.shard.complete = false; }, /not a full-coverage run/);
  bad((s) => { s[4].report.summary.shard.of = 7; }, /disagree on N/);
  bad((s) => { s[2].report.summary.shard.owned = s[2].report.summary.shard.owned.slice(1); }, /not the planned slice/);
  const stale = clickShards(3); const r = verifyClick({ ...clickArgs(stale), known: [{ id: "K1", route: "/gone", kind: "no-effect" }] });
  assert.ok(r.problems.some((p) => /STALE known-defect K1/.test(p)));
});

test("aggregate click: a route added to the registry after the shards ran cannot pass (the registry is re-read)", () => {
  const shards = clickShards(5, adminRoutes);
  const grown = [...adminRoutes, { path: "/brand-new-route", public: false }];
  const r = verifyClick({ shards, adminRoutes: grown, weights, known: [] });
  assert.ok(r.problems.some((p) => /universe .* differs from the live registry/.test(p)), r.problems.join(" | "));
});

function visualShards(of) {
  const keys = [];
  const units = [...adminRoutes.map((r) => routeId(r.path)), "products-new", "products-new-variants", "signed-out-home"];
  for (const l of VISUAL_LOCALES) for (const v of VISUAL_VIEWPORTS) {
    const size = `${v.size.width}x${v.size.height}`;
    for (const id of units) keys.push(`admin|${id}|${l}|${size}`);
    for (const id of ["home", "products", "cart"]) keys.push(`storefront|${id}|${l}|${size}`);
    for (const id of ["home", "privacy"]) keys.push(`platform|${id}|${l}|${size}`);
  }
  const owner = assignShards(keys.map((key) => ({ key, weight: 1 })), of);
  return Array.from({ length: of }, (_, k) => {
    const owned = keys.filter((x) => owner.get(x) === k + 1);
    return { source: `lint-${k + 1}`, report: { shard: shardReport({ index: k + 1, of }, keys, owned, { complete: true }), units: owned.map((x) => { const [app, id, locale, size] = x.split("|"); return { app, id, locale, size, shot: "x.png" }; }), blockingInstances: 0, verdict: { exit: 0, reasons: [] } } };
  });
}

test("aggregate visual: honest shards pass; a missing shard, a missing shot, a blocking verdict and a registry route outside the universe each fail", () => {
  for (const of of [1, 3, 4]) assert.deepEqual(verifyVisual({ shards: visualShards(of), adminRoutes }).problems, [], `N=${of}`);
  const bad = (mutate, re, routes = adminRoutes) => { const s = visualShards(4); mutate(s); const r = verifyVisual({ shards: s, adminRoutes: routes }); assert.ok(r.problems.some((p) => re.test(p)), `${re}: ${r.problems.join(" | ") || "no problem raised"}`); };
  bad((s) => s.pop(), /shard 4\/4 is missing/);
  bad((s) => { s[1].report.units.pop(); }, /owned shot\(s\) missing/);
  bad((s) => { s[2].report.verdict = { exit: 1, reasons: ["3 blocking violation instances"] }; }, /verdict exit 1/);
  bad((s) => { s[0].report.shard.owned = s[0].report.shard.owned.slice(1); }, /belongs to no shard/);
  bad(() => {}, /registry route \/brand-new-route/, [...adminRoutes, { path: "/brand-new-route" }]);
  const notRun = visualShards(2); const u = notRun[0].report.units.pop(); notRun[0].report.units.push({ ...u, shot: "", notRun: "NOT_RUN: external host" });
  assert.deepEqual(verifyVisual({ shards: notRun, adminRoutes }).problems, [], "an explicit NOT_RUN is a stated gap, as in the unsharded run");
});

// Artifact uploads now preserve output/playwright/<run>/; directory depth must
// not hide a shard or make historical visual baselines count as current evidence.
test("artifact discovery finds nested run evidence and retains platform marker refusal", (t) => {
  const dir = mkdtempSync(path.join(tmpdir(), "lc-sweep-discovery-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const run = path.join(dir, "gate-browser-click-sweep_1_2", "playwright", "run.abcdefgh");
  const click = path.join(run, "ui-click-sweep");
  const visual = path.join(run, "ui-visual-audit", "20261009T120000Z");
  const baseline = path.join(run, "ui-visual-audit", "baseline-reference");
  for (const folder of [click, visual, baseline]) mkdirSync(folder, { recursive: true });
  writeFileSync(path.join(click, "ledger.json"), JSON.stringify({ summary: { shard: { index: 1, of: 2 } }, rows: [] }));
  writeFileSync(path.join(visual, "lint.json"), JSON.stringify({ marker: "current" }));
  writeFileSync(path.join(baseline, "lint.json"), JSON.stringify({ marker: "historical" }));
  assert.equal(discover("click", dir).length, 1);
  assert.equal(discover("click", dir)[0].platformPass, false);
  writeFileSync(path.join(click, "platform-runner.pass"), "fixture-sha\n");
  assert.equal(discover("click", dir)[0].platformPass, true);
  assert.deepEqual(discover("visual", dir).map((item) => item.report.marker), ["current"]);
  assert.deepEqual(discover("click", path.join(dir, "missing")), []);
});
