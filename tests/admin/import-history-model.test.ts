// Purpose: independent contract examples for W5-03B archive DTO and read-only query grammar.
// Depends on: actual import-history-model, frozen Go twcity allowlist, node assert/test/fs.
// Used by: W5-U1 local red/green evidence; never substitutes for browser or PostgreSQL gates.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { historicalOrdersQuery, historicalOrdersURL, parseHistoricalOrdersPage, validHistoricalOrdersRequest } from "../../apps/admin/lib/import-history-model.ts";
import { importHistoryCopy } from "../../apps/admin/lib/import-history-copy.ts";
const store = "abcdef11-1111-4111-8111-111111111111";
const row = { order_id: "SL-2026-SYNTHETIC", ordered_at: "2026-10-07T01:30:00.123456789Z", status: "已完成", total_minor: 128000, currency: "TWD", items_summary: "合成商品×2", city: "臺北市" };
const page = { items: [row], next_cursor: "Abc-_1", total: 51 };
test("strict archive rows decode external order text, safe minor units and public keys", () => {
  assert.deepEqual(parseHistoricalOrdersPage(page, 50), page);
  assert.deepEqual(parseHistoricalOrdersPage({ items: [], next_cursor: "", total: 0 }), { items: [], next_cursor: "", total: 0 });
  assert.equal(parseHistoricalOrdersPage({ ...page, items: [{ ...row, city: null, items_summary: "", total_minor: 0 }] }).items[0].city, null);
  assert.equal(parseHistoricalOrdersPage({ ...page, items: [{ ...row, order_id: "😀".repeat(64), items_summary: "😀".repeat(500) }] }).items.length, 1);
});
test("PII-shaped city and extra sensitive fields never pass the safe projection", () => {
  for (const city of ["臺北市 合成街1號", "synthetic@example.invalid", "台北市", "Taipei", "", 42])
    assert.throws(() => parseHistoricalOrdersPage({ ...page, items: [{ ...row, city }] }), /unavailable/);
  for (const extra of [{ email: "synthetic@example.invalid" }, { address: "synthetic street" }, { phone: "synthetic phone" }, { payment_method: "synthetic" }, { id: store }])
    assert.throws(() => parseHistoricalOrdersPage({ ...page, items: [{ ...row, ...extra }] }), /unavailable/);
  assert.throws(() => parseHistoricalOrdersPage({ ...page, tenant_id: store }), /unavailable/);
});
test("all canonical Go cities pass; malformed types/bounds/order/time/money/cursor fail", () => {
  const go = readFileSync("internal/twcity/twcity.go", "utf8").split("var Cities = []string{")[1].split("}")[0];
  const cities = [...go.matchAll(/"([^"]+)"/g)].map((m) => m[1]); assert.equal(cities.length, 22);
  for (const city of cities) assert.equal(parseHistoricalOrdersPage({ ...page, items: [{ ...row, city }] }).items[0].city, city);
  for (const override of [{ order_id: "" }, { order_id: "x".repeat(65) }, { ordered_at: "yesterday" },
    { ordered_at: "2026-02-31T00:00:00Z" }, { ordered_at: "2026-10-07T24:00:00Z" }, { status: "" },
    { status: "x".repeat(41) }, { total_minor: -1 }, { total_minor: 1.5 }, { total_minor: 1_000_000_000_001 },
    { total_minor: "128000" }, { currency: "USD" }, { items_summary: "x".repeat(501) }, { items_summary: null }])
    assert.throws(() => parseHistoricalOrdersPage({ ...page, items: [{ ...row, ...override }] }), /unavailable/);
  for (const override of [{ next_cursor: "a=b" }, { next_cursor: 1 }, { total: 0 }, { total: 2001 }, { total: "51" }, { items: null }, { items: [row, row] }])
    assert.throws(() => parseHistoricalOrdersPage({ ...page, ...override }), /unavailable/);
  assert.throws(() => parseHistoricalOrdersPage({ ...page, items: [], next_cursor: "Abc" }), /unavailable/);
  assert.throws(() => parseHistoricalOrdersPage({ ...page, items: Array.from({ length: 51 }, (_, n) => ({ ...row, order_id: `SL-${n}` })) }, 50), /unavailable/);
});
test("read grammar is exact limit/after, not the stale brief's cursor name", () => {
  const url = `http://localhost:3100/api/stores/${store}/customers/${store}/historical-orders`;
  for (const [query, limit] of [["", 50], ["?limit=50", 50], ["?limit=100&after=Abc-_1", 100], ["?after=Abc", 50]] as const)
    assert.equal(historicalOrdersQuery(url + query), limit);
  for (const query of ["?", "?cursor=Abc", "?tenant_id=x", "?limit=0", "?limit=101", "?limit=050", "?limit=50&limit=25", "?after=", "?after=a=b", "?after=a+b", "?limit=5&"]) {
    assert.equal(historicalOrdersQuery(url + query), null, query);
  }
  assert.equal(historicalOrdersURL(store, store, "Abc-_1"), `/api/stores/${store}/customers/${store}/historical-orders?limit=50&after=Abc-_1`);
  assert.throws(() => historicalOrdersURL("wrong", store), /invalid_request/);
  assert.throws(() => historicalOrdersURL(store, store, "x=y"), /invalid_request/);
  assert.equal(validHistoricalOrdersRequest(new Request(url), store, store), true);
  for (const headers of [{ "Idempotency-Key": "synthetic-key" }, { "content-length": "1" }, { "transfer-encoding": "chunked" }])
    assert.equal(validHistoricalOrdersRequest(new Request(url, { headers }), store, store), false);
  assert.equal(validHistoricalOrdersRequest(new Request(url, { method: "POST", body: "{}" }), store, store), false);
});
test("three locales retain truthful history-not-revenue basis and copy parity", () => {
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    assert.deepEqual(Object.keys(importHistoryCopy[locale]).sort(), Object.keys(importHistoryCopy.en).sort());
    assert.match(importHistoryCopy[locale].title, /SHOPLINE/); assert.ok(importHistoryCopy[locale].count(50, 51).includes("50"));
  }
  assert.match(importHistoryCopy.en.basis, /read-only.*excluded from revenue and reports/);
  assert.match(importHistoryCopy["zh-TW"].basis, /只供查看，不計入營收與報表/);
});
