// Purpose: report request/DTO/response and localized UI acceptance negatives.
// Depends on: node:test/assert/fs and report-only pure modules.
// Used by: focused Node gate and parent independent verification.
// W6-U1 Node half (contracts/reporting-v2.md): the admin BFF fence and decoders for the eight report resources
// (GET reports/{products|channels|funnel|manual-orders}[.csv]) must admit exactly the Go query grammar (from/to once
// each, 0..91 days, session_id only on the funnel, <=512 raw bytes, no key/body on reads) and decode exactly what
// internal/reporting emits, re-checking its invariants (net = captured - refunded per row, closed channel vocabulary
// order, strictly ascending environments, nested funnel counts, <=1000 product rows sorted by net desc). Only the
// public pure functions are imported; route.ts wiring stays a Playwright BFF concern (reports.spec.ts).
// Run: node --test --experimental-strip-types tests/admin/reports-bff.test.ts
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { reportFromQuery, reportsRoute, validReportsRequest, type ReportsRouteKind } from "../../apps/admin/lib/reports-request.ts";
import {
  parseChannelReport,
  parseFunnelReport,
  parseManualReport,
  parseProductReport,
} from "../../apps/admin/lib/reports-model.ts";
import { reportCSVFilename, reportJSON, privateReportHeaders } from "../../apps/admin/lib/reports-response.ts";
import { reportsCopy } from "../../apps/admin/lib/reports-copy.ts";
import { sortedProducts, conversion } from "../../apps/admin/lib/reports-presentation.ts";

const id = "11111111-1111-4111-8111-111111111111";
const origin = "http://127.0.0.1:3100";
const from = "2026-09-01";
const to = "2026-09-30";

// ---------------------------------------------------------------------------------------------- grammar
const names = ["products", "channels", "funnel", "manual-orders"] as const;

test("exactly the eight report GETs are admitted; every other method or path is refused", () => {
  for (const name of names) {
    assert.equal(reportsRoute("GET", `reports/${name}`), name, name);
    assert.equal(reportsRoute("GET", `reports/${name}.csv`), `${name}-csv`, `${name}.csv`);
    for (const wrong of ["POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"])
      assert.equal(reportsRoute(wrong, `reports/${name}`), null, `${wrong} ${name}`);
  }
  for (const path of ["reports", "reports/", "reports/products/", "reports/product", "reports/manual_orders",
    "reports/products.csv.csv", "reports/products.json", `reports/${id}`, "finance/summary", "reports/products/x"])
    assert.equal(reportsRoute("GET", path), null, path);
});

const req = (path: string, init: RequestInit = {}) => new Request(`${origin}/api/stores/${id}/${path}`, { method: "GET", ...init });

test("report reads: query required, no body, no key, no transfer-encoding", () => {
  assert.equal(validReportsRequest("products", req(`reports/products?from=${from}&to=${to}`)), true);
  assert.equal(validReportsRequest("products", req("reports/products")), false); // no query at all
  assert.equal(validReportsRequest("products", req("reports/products?")), false); // bare '?' (Go ForceQuery)
  assert.equal(validReportsRequest("products", req(`reports/products?from=${from}&to=${to}`, { headers: { "Idempotency-Key": "products-12345678" } })), false);
  assert.equal(validReportsRequest("products", req(`reports/products?from=${from}&to=${to}`, { headers: { "transfer-encoding": "chunked" } })), false);
  assert.equal(validReportsRequest("products-csv", req(`reports/products.csv?from=${from}&to=${to}`)), true);
});

test("from/to: exactly once each, calendar-valid days, 0..91 apart", () => {
  const ok = [
    `?from=${from}&to=${to}`,
    `?to=${to}&from=${from}`, // order does not matter
    "?from=2026-09-01&to=2026-09-01", // 0 days
    "?from=2026-01-01&to=2026-04-02", // exactly 91 days
    "?from=2024-02-28&to=2024-02-29", // leap day
  ];
  for (const search of ok) assert.equal(validReportsRequest("channels", req(`reports/channels${search}`)), true, search);
  const bad = [
    `?from=${from}`, `?to=${to}`, `?from=${from}&to=${to}&to=${to}`, `?from=${from}&from=${from}&to=${to}`,
    "?from=2026-9-01&to=2026-09-30", "?from=2026-09-01&to=2026-9-30", "?from=&to=2026-09-30",
    "?from=2026-02-30&to=2026-03-01", // not a calendar day
    "?from=2026-01-01&to=2026-04-03", // 92 days
    "?from=2026-09-30&to=2026-09-01", // end before start
    `?from=${from}&to=${to}&limit=5`, // unknown key
    `?from=${from}&to=${to}&q=x`,
    `?from=2026-09-01&to=2026-09-30&`, // empty trailing segment
    `?from=${from}&to=${"2".repeat(500)}`, // over the 512-byte raw query bound
  ];
  for (const search of bad) assert.equal(validReportsRequest("channels", req(`reports/channels${search}`)), false, search);
});

test("session_id: only on the funnel (json and csv), a canonical uuid, once", () => {
  assert.equal(validReportsRequest("funnel", req(`reports/funnel?from=${from}&to=${to}&session_id=${id}`)), true);
  assert.equal(validReportsRequest("funnel-csv", req(`reports/funnel.csv?from=${from}&to=${to}&session_id=${id}`)), true);
  assert.equal(validReportsRequest("funnel", req(`reports/funnel?from=${from}&to=${to}`)), true); // whole store
  for (const kind of ["products", "channels", "manual-orders", "products-csv", "channels-csv", "manual-orders-csv"] as ReportsRouteKind[])
    assert.equal(validReportsRequest(kind, req(`reports/${kind.replace(/-csv$/, "")}?from=${from}&to=${to}&session_id=${id}`)), false, kind);
  assert.equal(validReportsRequest("funnel", req(`reports/funnel?from=${from}&to=${to}&session_id=not-a-uuid`)), false);
  assert.equal(validReportsRequest("funnel", req(`reports/funnel?from=${from}&to=${to}&session_id=${"abcdefab-cdef-4abc-8abc-abcdefabcdef".toUpperCase()}`)), false);
  assert.equal(validReportsRequest("funnel", req(`reports/funnel?from=${from}&to=${to}&session_id=${id}&session_id=${id}`)), false);
});

// ---------------------------------------------------------------------------------------------- decoders
const throws = (fn: () => unknown) => assert.throws(fn, /unavailable/);
const head = { from, to, timezone: "Asia/Taipei" };
const money = (over: Record<string, unknown> = {}) => ({
  environment: "LIVE", captured_count: 2, captured_minor: 6000, refunded_minor: 1000, net_minor: 5000,
  offline_count: 1, offline_minor: 900, ...over,
});

test("parseProductReport: echoed range/zone, <=1000 rows sorted by net desc, net = captured - refunded", () => {
  const row = (over: Record<string, unknown> = {}) => ({
    sku_id: id, product_id: id, code: "SKU-1", name: "商品", currency: "TWD", environment: "LIVE",
    units: 3, captured_minor: 6000, refunded_minor: 1000, net_minor: 5000, offline_units: 1, offline_minor: 900, ...over,
  });
  const parsed = parseProductReport({ ...head, truncated: false, rows: [row(), row({ sku_id: id.replace(/^1/, "3"), net_minor: 100, captured_minor: 100, refunded_minor: 0, units: 1 })] }, from, to);
  assert.equal(parsed.rows.length, 2);
  assert.equal(parsed.truncated, false);
  assert.deepEqual(parseProductReport({ ...head, truncated: false, rows: [] }, from, to).rows, []);
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row()] }, "2026-09-02", to)); // echo mismatch
  throws(() => parseProductReport({ from, to, timezone: "UTC", truncated: false, rows: [] }, from, to));
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row()], extra: 1 }, from, to));
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row({ net_minor: 4999 })] }, from, to)); // I05
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row({ environment: "PROD" })] }, from, to));
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row({ currency: "TW" })] }, from, to));
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row({ units: -1 })] }, from, to));
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row({ sku_id: "x" })] }, from, to));
  throws(() => parseProductReport({ ...head, truncated: false, rows: [row({ net_minor: 100, captured_minor: 100, refunded_minor: 0 }), row()] }, from, to)); // unsorted
  throws(() => parseProductReport({ ...head, truncated: true, rows: [row()] }, from, to)); // truncated must carry exactly 1000
  const thousand = Array.from({ length: 1000 }, (_, i) => row({ net_minor: 1000 - i, captured_minor: 1000 - i, refunded_minor: 0, units: 1 }));
  assert.equal(parseProductReport({ ...head, truncated: true, rows: thousand }, from, to).rows.length, 1000);
  throws(() => parseProductReport({ ...head, truncated: false, rows: [...thousand, row({ net_minor: 0, captured_minor: 0, refunded_minor: 0, units: 0 })] }, from, to));
});

test("parseChannelReport: closed vocabulary order, money per environment strictly ascending, I05 inside every bucket", () => {
  const row = (over: Record<string, unknown> = {}) => ({
    channel: "facebook_live", currency: "TWD", orders: 4, cancelled_orders: 1, money: [money()], ...over,
  });
  const parsed = parseChannelReport({ ...head, rows: [row(), row({ channel: "facebook_live", currency: "USD" }), row({ channel: "storefront", money: [] })] }, from, to);
  assert.equal(parsed.rows.length, 3);
  assert.deepEqual(parseChannelReport({ ...head, rows: [] }, from, to).rows, []);
  throws(() => parseChannelReport({ ...head, rows: [row({ channel: "tiktok" })] }, from, to)); // closed vocabulary
  throws(() => parseChannelReport({ ...head, rows: [row({ channel: "storefront" }), row()] }, from, to)); // order
  const full = parseChannelReport({ ...head, rows: [row(), row({ channel: "instagram_live" }), row({ channel: "storefront" }), row({ channel: "manual" })] }, from, to);
  assert.equal(full.rows.length, 4);
  throws(() => parseChannelReport({ ...head, rows: [row({ orders: -1 })] }, from, to));
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money({ net_minor: 4999 })] })] }, from, to)); // I05
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money({ environment: "PROD" })] })] }, from, to));
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money(), money()] })] }, from, to)); // envs must ascend strictly
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money({ environment: "SANDBOX" }), money()] })] }, from, to)); // SANDBOX then LIVE
  assert.equal(parseChannelReport({ ...head, rows: [row({ money: [money(), money({ environment: "SANDBOX" })] })] }, from, to).rows[0].money.length, 2);
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money({ captured_count: 0, captured_minor: 100 })] })] }, from, to));
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money({ offline_count: 0, offline_minor: 100 })] })] }, from, to));
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [{ ...money(), extra: 1 }] })] }, from, to));
  throws(() => parseChannelReport({ ...head, rows: [row({ money: [money({ refunded_minor: -1, net_minor: 6001 })] })] }, from, to));
});

test("parseFunnelReport: nested counts, session echo, ordered_without_link bound", () => {
  const funnel = (over: Record<string, unknown> = {}) => ({ ...head, session_id: null, claimed: 10, link_sent: 8, ordered: 5, paid: 3, ordered_without_link: 1, ...over });
  const parsed = parseFunnelReport(funnel(), from, to, "");
  assert.equal(parsed.paid, 3);
  assert.equal(parseFunnelReport(funnel({ session_id: id }), from, to, id).session_id, id);
  assert.equal(parseFunnelReport(funnel({ claimed: 0, link_sent: 0, ordered: 0, paid: 0, ordered_without_link: 0 }), from, to, "").claimed, 0);
  throws(() => parseFunnelReport(funnel(), "2026-09-02", to, "")); // echo mismatch
  throws(() => parseFunnelReport(funnel({ session_id: id }), from, to, "")); // session not requested
  throws(() => parseFunnelReport(funnel({ session_id: null }), from, to, id)); // session requested but absent
  throws(() => parseFunnelReport(funnel({ session_id: id.replace(/^1/, "2") }), from, to, id));
  throws(() => parseFunnelReport(funnel({ paid: 6 }), from, to, "")); // paid > ordered
  throws(() => parseFunnelReport(funnel({ ordered: 9 }), from, to, "")); // ordered > link_sent
  throws(() => parseFunnelReport(funnel({ link_sent: 11 }), from, to, "")); // link_sent > claimed
  throws(() => parseFunnelReport(funnel({ paid: -1, ordered: -1, link_sent: -1, claimed: -1 }), from, to, ""));
  throws(() => parseFunnelReport(funnel({ ordered_without_link: -1 }), from, to, ""));
  throws(() => parseFunnelReport(funnel({ ordered_without_link: 3 }), from, to, "")); // link_sent + owl > claimed
  throws(() => parseFunnelReport(funnel({ extra: 1 }), from, to, ""));
});

test("parseManualReport: nullable principal/session ids, bucketed money like channels", () => {
  const row = (over: Record<string, unknown> = {}) => ({
    principal_id: id, session_id: null, currency: "TWD", orders: 2, cancelled_orders: 0, money: [money()], ...over,
  });
  const parsed = parseManualReport({ ...head, rows: [row(), row({ principal_id: null, session_id: id, money: [] })] }, from, to);
  assert.equal(parsed.rows.length, 2);
  assert.deepEqual(parseManualReport({ ...head, rows: [] }, from, to).rows, []);
  throws(() => parseManualReport({ ...head, rows: [row({ principal_id: "x" })] }, from, to));
  throws(() => parseManualReport({ ...head, rows: [row({ session_id: "x" })] }, from, to));
  throws(() => parseManualReport({ ...head, rows: [row({ orders: -1 })] }, from, to));
  throws(() => parseManualReport({ ...head, rows: [row({ currency: "TWD1" })] }, from, to));
  throws(() => parseManualReport({ ...head, rows: [row({ money: [money({ net_minor: 0 })] })] }, from, to)); // I05
  throws(() => parseManualReport({ ...head, rows: [row()], extra: 1 }, from, to));
});

// ---------------------------------------------------------------------------------------------- copy
test("reports copy: three locales, identical key sets, the integrator's ruled money annotation verbatim", () => {
  for (const locale of ["zh-TW", "zh-CN"] as const)
    assert.deepEqual(Object.keys(reportsCopy[locale]).sort(), Object.keys(reportsCopy.en).sort(), locale);
  // Integrator ruling (2026-10): every report annotates what 實收 counts, and offline money stays separate. No profit chart.
  assert.equal(reportsCopy["zh-TW"].moneyBasis, "實收 = 線上付款成功；貨到付款/轉帳另列");
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const c = reportsCopy[locale];
    for (const channel of ["facebook_live", "instagram_live", "storefront", "manual"])
      assert.ok(c.channels[channel], `${locale}: channel ${channel}`);
    for (const key of ["products", "channels", "funnel", "manual-orders"] as const) assert.ok(c.tabs[key], `${locale}: tab ${key}`);
    assert.ok(c.rangeInvalid.length > 5 && c.csv.length > 0 && c.moneyBasis.length > 5, locale);
  }
});

// ---------------------------------------------------------------------------------------------- source ratchet
test("Reports.tsx: four tabs, CSV per tab, ruled annotation, CSS/SVG charts only (no new dependencies)", () => {
  const reports = readFileSync("apps/admin/components/Reports.tsx", "utf8");
  assert.match(reports, /data-testid="reports-money-basis"/); // the ruled annotation is rendered, not just copied
  for (const tab of ["products", "channels", "funnel", "manual-orders"]) {
    assert.match(reports, new RegExp(`data-testid="reports-tab-${tab}"`), tab);
    assert.match(reports, new RegExp(`data-testid="reports-csv-${tab}"`), `${tab} csv`);
  }
  const views = readFileSync("apps/admin/components/ReportViews.tsx", "utf8");
  assert.match(views, /<svg/); // actual chart renderer, CSS/SVG only
  assert.doesNotMatch(reports, /recharts|chart\.js|d3|canvas/i);
  assert.match(reports, /TabStrip/);
  const manifest = readFileSync("apps/admin/package.json", "utf8");
  assert.doesNotMatch(manifest, /recharts|chart\.js|d3-/);
});

// Audited CSVs are never relayed as arbitrary upstream files or HTML errors.
test("report CSV closed headers use actual Go slug, echoed dates and private no-store", () => {
  const headers = () => new Headers({ "content-type": "text/csv; charset=utf-8", "cache-control": "no-store, private", "content-disposition": `attachment; filename="report-manual_orders-${from}-${to}.csv"` });
  assert.equal(reportCSVFilename(headers(), "manual-orders", from, to), `report-manual_orders-${from}-${to}.csv`);
  for (const [key, value] of [["content-type", "text/html"], ["cache-control", "public, no-store"], ["content-disposition", `attachment; filename="report-products-${from}-${to}.csv"`]]) {
    const h = headers(); h.set(key, value); assert.throws(() => reportCSVFilename(h, "manual-orders", from, to));
  }
  assert.equal(privateReportHeaders(new Headers({ "cache-control": "private, no-store" })), true);
  assert.equal(privateReportHeaders(new Headers({ "cache-control": "private, no-store, public" })), false);
});
test("report response projection rejects unknown keys, mismatched range and non-private responses", async () => {
  const response = (body: unknown, headers = {}) => Response.json(body, { headers: { "Cache-Control": "private, no-store", ...headers } });
  assert.deepEqual(await reportJSON(response({ ...head, rows: [] }), "channels", from, to), { ...head, rows: [] });
  await assert.rejects(() => reportJSON(response({ ...head, rows: [], tenant_id: id }), "channels", from, to));
  await assert.rejects(() => reportJSON(response({ ...head, rows: [] }), "channels", "2026-09-02", to));
  await assert.rejects(() => reportJSON(response({ ...head, rows: [] }, { "Cache-Control": "public" }), "channels", from, to));
});

test("manual buckets follow Go's uncapped row count rather than a made-up 500-row limit", () => {
 const rows = Array.from({length: 501}, () => ({principal_id: null, session_id: null, currency: "TWD", orders: 0, cancelled_orders: 0, money: []}));
 assert.equal(parseManualReport({...head, rows}, from, to).rows.length, 501);
});

test("display sorts only inside currency/environment groups, never sums money or mutates server rows", () => {
 const base = {sku_id:id,product_id:id,code:"SKU",name:"Cup",currency:"TWD",environment:"LIVE" as const,units:1,captured_minor:100,refunded_minor:0,net_minor:100,offline_units:0,offline_minor:0};
 const rows = [{...base,currency:"USD",net_minor:1}, {...base,environment:"SANDBOX" as const,net_minor:9000}, {...base,net_minor:200}, base];
 const before = JSON.stringify(rows);
 assert.deepEqual(sortedProducts(rows,"net_minor",false,"en").map(r=>[r.currency,r.environment,r.net_minor]),[
  ["TWD","LIVE",200],["TWD","LIVE",100],["TWD","SANDBOX",9000],["USD","LIVE",1]
 ]);
 assert.equal(JSON.stringify(rows),before);
 assert.equal(conversion("en",0,0,"No starting count"),"No starting count");
 assert.equal(conversion("en",1,4,"No starting count"),"25%");
});

// Codex review P2 (PR #3): the selected report tab lives in the page URL (`report`), strictly validated; absent means products.
test("page report query accepts exactly the four report names and defaults to products", () => {
  assert.equal(reportFromQuery(undefined), "products");
  for (const name of ["products", "channels", "funnel", "manual-orders"]) assert.equal(reportFromQuery(name), name);
  for (const bad of ["", "Channels", "channels ", "manual_orders", "products.csv", "__proto__", "constructor", "../x"]) assert.equal(reportFromQuery(bad), null, bad);
});
