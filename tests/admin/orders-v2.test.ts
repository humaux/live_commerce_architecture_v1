import assert from "node:assert/strict";
import { test } from "node:test";
import { buckets, emptyFilters, parseOrderListV2, validOrdersV2Query, validOrderFilters } from "../../apps/admin/lib/orders-v2.ts";
import { ordersV2Copy } from "../../apps/admin/lib/orders-v2-copy.ts";

const counts = Object.fromEntries(buckets.map(k => [k, 0]));
test("orders-v2 accepts bounded filters and Taipei inclusive dates; rejects ambiguity", () => {
  assert(validOrdersV2Query("view=v2&limit=10&state=active&payment_mode=cash_on_delivery&delivery=home&from=2026-01-01&to=2026-01-02"));
  assert.equal(validOrdersV2Query("view=v2&q=1234"), false, "search terms are body-only, never URL");
  for (const query of ["view=v2&q=", "view=v2&q=a&q=b", "view=v2&tenant_id=abc", "view=v2&store_id=abc", "view=v2&from=2026-02-30", "view=v2&from=2026-01-02&to=2026-01-01", "view=v2&q=%00", "view=v2&q=%ZZ", "view=v2&limit=010", "view=v2&bucket=notreal", "view=v2&delivery=evil", "view=v2&session_id=wrong"]) assert.equal(validOrdersV2Query(query), false, query);
  assert.equal(validOrderFilters({ ...emptyFilters, q: "a".repeat(81) }), false);
  assert.equal(validOrderFilters({ ...emptyFilters, q: " 1234" }), false);
});
test("orders-v2 rejects client count approximations, unbounded or extra response data", () => {
  const empty = { items: [], next_cursor: "", total: 0, counts, sessions: [] };
  assert.equal(parseOrderListV2(empty).total, 0);
  for (const value of [{ ...empty, counts: {} }, { ...empty, total: -1 }, { ...empty, counts: { ...counts, unpaid: 0.5 } }, { ...empty, phone: "synthetic" }, { ...empty, sessions: [{ id: "bad", name: "foreign" }] }, { ...empty, items: null }]) assert.throws(() => parseOrderListV2(value));
});
test("all three locales label every task queue, payment and delivery mode", () => {
  for (const c of Object.values(ordersV2Copy)) {
    assert.deepEqual(Object.keys(c.tabs), [...buckets]);
    assert(c.modes.cash_on_delivery); assert(c.deliveries.cvs_familymart); assert(c.completedNote);
  }
});
