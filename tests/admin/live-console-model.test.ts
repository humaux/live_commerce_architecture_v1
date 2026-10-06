// Purpose: LC-U1 frozen console, lifecycle, results and copy parser counterexamples (MOCK).
// Depends on: console-model.ts; node:test and node:assert.
// Used by: LC-U1 transport verification and the integrator's independent acceptance.
import assert from "node:assert/strict";
import { test } from "node:test";
import { parseConsole, parseConsoleOffer, parseCopyResult, parseLifecycleResult, parseRecommendResult, parseSessionResults } from "../../apps/admin/src/features/live/console-model.ts";

const sid = "22222222-2222-4222-8222-222222222222";
const oid = "33333333-3333-4333-8333-333333333333";
const sku = "44444444-4444-4444-8444-444444444444";
const at = "2026-10-06T00:00:00Z";
const offer = {
  offer_id: oid, keyword: "A1", sku_id: sku, product_name: "Synthetic product", variant_label: "Blue", active: true,
  version: 4, live_price_minor: 1200, sku_price_minor: 1500,
  stock: { tracked: true, sellable: 3, reserved: 2, warehouse_id: sku, balance_version: 4 },
  claimed: { buyers: 2, quantity: 3 }, ordered_qty: 2, paid_qty: 1, paid_amount_minor: 1200, sold_out: false, low_stock: true,
};
const window = { session_id: sid, state: "OPEN", generation: 2, version: 3, opened_at: at, closed_at: null, match_mode: "CONTAINS" };
const snapshot = {
  session: { id: sid, title: "Synthetic session", lifecycle: "live", version: 3, started_at: at, ended_at: null },
  window: { state: "OPEN", generation: 2, opened_at: at, match_mode: "CONTAINS" },
  stats: { comments: { total: null, source: "unavailable" }, keyword_comments: 2, buyers: 1,
    orders: { count: 1, amount_minor: 1500 }, paid: { count: 0, amount_minor: 0 }, currency: "TWD", as_of: at },
  offers: [offer], capabilities: { facebook: { read_comment: { state: "unknown", reason: "not_checked", evidence: "DESIGN", checked_at: null } } },
  stream: { state: "unavailable", poll_interval_ms: 5000, last_ok_at: null, lag_ms: null, source_platform: "facebook", video_embeddable: false }, recommended: null,
};

test("A1 strict MOCK snapshot preserves unavailable counts, capability unknown and server stock", () => {
  assert.deepEqual(parseConsole(snapshot, sid), snapshot);
  assert.deepEqual(parseConsoleOffer(offer), offer);
  for (const value of [
    { ...snapshot, tenant_id: sid }, { ...snapshot, session: { ...snapshot.session, id: sku } },
    { ...snapshot, stream: { ...snapshot.stream, state: "LIVE" } },
    { ...snapshot, capabilities: { facebook: { read_comment: { ...snapshot.capabilities.facebook.read_comment, state: "supported" } } } },
    { ...snapshot, stats: { ...snapshot.stats, paid: { count: 1, amount_minor: Number.MAX_SAFE_INTEGER + 1 } } },
    { ...snapshot, offers: [offer, offer] },
  ]) assert.throws(() => parseConsole(value, sid));
  for (const value of [null, [], { ...offer, token: "forbidden" }, { ...offer, version: 0 },
    { ...offer, stock: { ...offer.stock, reserved: -1 } }, { ...offer, active: "true" },
    { ...offer, claimed: { buyers: 1.1, quantity: 1 } }]) assert.throws(() => parseConsoleOffer(value));
});

test("A7 returns a separate lifecycle version and closed full CONTAINS window", () => {
  const result = { lifecycle: "live", version: 8, window };
  assert.deepEqual(parseLifecycleResult(result, sid), result);
  for (const value of [{ ...result, lifecycle: "DRAFT" }, { ...result, version: 0 },
    { ...result, window: { ...window, session_id: sku } }, { ...result, window: { ...window, secret: "forbidden" } }])
    assert.throws(() => parseLifecycleResult(value, sid));
});

test("A5 results retain separate environments/currencies and bind exact requested ids/order", () => {
  const item = { session_id: sid, orders: 2, paid_orders: 1, multi_session_orders: 1,
    money: [{ currency: "TWD", order_minor: 3000, paid_minor: 1000, sandbox_paid_minor: 2000 }] };
  const result = { as_of: at, items: [item] };
  assert.deepEqual(parseSessionResults(result, [sid]), result);
  for (const value of [{ ...result, secret: "forbidden" }, { ...result, items: [] },
    { ...result, items: [{ ...item, session_id: sku }] }, { ...result, items: [{ ...item, money: [item.money[0], item.money[0]] }] },
    { ...result, items: [{ ...item, money: [{ ...item.money[0], paid_minor: -1 }] }] }])
    assert.throws(() => parseSessionResults(value, [sid]));
});

test("A5 copy parses real Go envelope, copied offer prices and CONTAINS window", () => {
  const session = { session_id: sid, program_id: sku, title: "Copied", scheduled_at: null, aspect_ratio: "9:16",
    state: "DRAFT", version: 1, created_at: at, updated_at: at };
  const created = { offer_id: oid, session_id: sid, keyword: "A1", sku_id: sku, sku_code: "S1", product_name: "Synthetic product",
    max_quantity_per_claim: 5, active: true, version: 1, activated_at: at, updated_at: at, sku_price_minor: 1500, currency: "TWD", live_price_minor: 1200 };
  const result = { session, window: { ...window, state: "CLOSED", generation: 0, version: 0, opened_at: null },
    created: [created], conflicts: [], source_version: 6 };
  assert.deepEqual(parseCopyResult(result), result);
  for (const value of [{ ...result, source_version: 0 }, { ...result, created: [{ ...created, session_id: sku }] },
    { ...result, window: { ...result.window, state: "OPEN", opened_at: at } }, { ...result, source_id: sku }])
    assert.throws(() => parseCopyResult(value));
});

test("A6 accepts optional operation UUID, never guessed send success", () => {
  for (const value of [{ recommended_at: at }, { recommended_at: at, operation_id: oid }]) assert.deepEqual(parseRecommendResult(value), value);
  for (const value of [{ recommended_at: at, send_state: "sent" }, { recommended_at: at, operation_id: null }, { recommended_at: "yesterday" }])
    assert.throws(() => parseRecommendResult(value));
});
