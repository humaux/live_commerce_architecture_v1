// Purpose: deterministic sharding of the G-UI8 click sweep and the G-UI9 visual lint (LC_SWEEP_SHARD=i/N) so each CI job runs one balanced slice, plus the
//   shared unit universe both the runners and tests/ui/sweep-aggregate.mjs count against (a silent drop of a route/variant/journey must be impossible).
// Depends on: node:crypto, node:fs; tests/ui/click-sweep-weights.json (relative unit cost, only used to balance); the admin route registry is passed in by callers.
// Used by: tests/ui/click-sweep.mjs, tests/ui/visual-audit.mjs, tests/ui/sweep-aggregate.mjs, tests/ui/sweep-shard-lib.test.mjs.
// Invariants: every unit is owned by exactly one shard for any N; the partition depends only on (unit keys, weights, N), never on timing or the machine.
import crypto from "node:crypto";
import { readFile } from "node:fs/promises";

/** The three variants the sweep walks every route at (viewport, locale); the single source for click-sweep.mjs and the aggregate. */
export const CLICK_VARIANTS = [
  { viewport: "desktop", size: { width: 1586, height: 992 }, locale: "zh-TW", mobile: false },
  { viewport: "mobile", size: { width: 390, height: 844 }, locale: "zh-TW", mobile: true },
  { viewport: "desktop", size: { width: 1586, height: 992 }, locale: "en", mobile: false },
];
/** Every buyer route of docs/engineering/ui-architecture.md section 4 that the click sweep must open at every variant. */
export const STOREFRONT_ROUTES = ["/", "/products", "/collections", "/collections/[slug]", "/products/[slug]", "/search", "/cart", "/checkout", "/orders/lookup", "/orders/[orderID]", "/order-link", "/claim", "/legal/anti-fraud", "/legal/[slug]", "/privacy", "/data-deletion", "/pages/[slug]"];
/** The `page` text of every journey ledger row; J2/J3 and J2b run after the storefront desktop/zh-TW session (they need its COD order), so they share its shard. */
export const JOURNEY_PAGES = {
  J1: ["J1 product editor -> storefront", "J1 storefront"],
  J23: ["J2/J3 admin orders", "J2 storefront order lookup"],
  J4: ["J4 custom domain"],
  J5: ["J5 sign out"],
};
/** Unit that owns journeys J2/J3: the storefront session whose COD order they read. */
export const J23_OWNER = "storefront|desktop|zh-TW";
/** G-UI9 visual-lint matrix (shared with sweep-aggregate.mjs): every unit is shot at each locale x viewport. */
export const VISUAL_LOCALES = ["zh-TW", "zh-CN", "en"];
export const VISUAL_VIEWPORTS = [
  { name: "desktop", size: { width: 1586, height: 992 }, mobile: false },
  { name: "mobile", size: { width: 390, height: 844 }, mobile: true },
];
/** Stable id of a route in the visual-lint shot tree and unit keys: "/" -> home, "/products/[product]" -> products-product. */
export const routeId = (p) => (p === "/" ? "home" : p.replace(/^\//, "").replace(/\[([^\]]+)\]/g, "$1").replace(/\//g, "-").toLowerCase());
const DEFAULT_WEIGHT = 50;

/** Parses "i/N" (1-based). "" / undefined = not sharded (null). Anything malformed throws: a typo must not silently run the whole sweep. */
export function parseShard(spec) {
  if (spec === undefined || spec === null || spec === "") return null;
  const m = /^([1-9][0-9]*)\/([1-9][0-9]*)$/.exec(String(spec));
  if (!m || Number(m[1]) > Number(m[2])) throw new Error(`LC_SWEEP_SHARD must be i/N with 1 <= i <= N, got ${JSON.stringify(spec)}`);
  return { index: Number(m[1]), of: Number(m[2]) };
}

/** Longest-processing-time bin packing: heaviest unit first onto the lightest shard (ties: lowest index, then key order). Returns Map(key -> 1-based shard). */
export function assignShards(units, of) {
  const keys = new Set(units.map((u) => u.key));
  if (keys.size !== units.length) throw new Error("shard units must have unique keys");
  const load = Array.from({ length: of }, () => 0), owner = new Map();
  const order = [...units].sort((a, b) => b.weight - a.weight || (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));
  for (const u of order) {
    let best = 0;
    for (let i = 1; i < of; i++) if (load[i] < load[best]) best = i;
    load[best] += u.weight; owner.set(u.key, best + 1);
  }
  return owner;
}

/** Relative cost per click-sweep unit (seconds, measured once; only used to balance shards). Keys starting with "_" are notes. */
export async function loadWeights(file = new URL("./click-sweep-weights.json", import.meta.url)) {
  const raw = JSON.parse(await readFile(file, "utf8"));
  return Object.fromEntries(Object.entries(raw).filter(([k]) => !k.startsWith("_")));
}

export const pageKey = (app, page, viewport, locale) => `${app}|${page}|${viewport}|${locale}`;

/**
 * The full click-sweep universe: balanced units and, for each page-load that must exist, the unit that owns it.
 * Admin: one unit per route x variant. Storefront: one unit per variant (the walk shares one cart-bearing browser context, so it is never split).
 * Journeys J1, J4, J5 are units of their own; J2/J3 ride with J23_OWNER.
 */
export function planClick(adminRoutes, weights, of) {
  const units = [], pages = [];
  for (const r of adminRoutes) for (const v of CLICK_VARIANTS) {
    const key = pageKey("admin", r.path, v.viewport, v.locale);
    units.push({ key, weight: weights[key] ?? DEFAULT_WEIGHT }); pages.push({ page: key, unit: key });
  }
  for (const v of CLICK_VARIANTS) {
    const key = `storefront|${v.viewport}|${v.locale}`;
    units.push({ key, weight: weights[key] ?? 200 });
    for (const route of STOREFRONT_ROUTES) pages.push({ page: pageKey("storefront", route, v.viewport, v.locale), unit: key });
  }
  for (const j of ["J1", "J4", "J5"]) units.push({ key: `journey|${j}`, weight: weights[`journey|${j}`] ?? 20 });
  return { units, pages, owner: assignShards(units, of), journeyOwner: { J1: "journey|J1", J23: J23_OWNER, J4: "journey|J4", J5: "journey|J5" } };
}

/** Stable digest of a key set, order-independent. */
export const digest = (keys) => crypto.createHash("sha256").update([...keys].sort().join("\n")).digest("hex");

/** What a shard records in its own ledger/lint summary so the aggregate can prove the shards together cover the whole universe exactly once. */
export function shardReport(shard, universe, owned, extra = {}) {
  return { index: shard.index, of: shard.of, universe: { count: universe.length, sha256: digest(universe) }, owned: [...owned].sort(), ...extra };
}
