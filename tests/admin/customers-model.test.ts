// customers-billing-ui: strict parsers for the frozen customers/finance DTOs (apps/admin/lib/customers-model.ts).
// Fixtures are synthetic (no real buyer data). The real Go shapes are proved by customers-billing-tests (CB09/CB11).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  financeDay,
  isSandbox,
  parseCustomer,
  parseCustomerDetail,
  parseCustomerList,
  parseErasureSummary,
  parseFinanceSummary,
} from "../../apps/admin/lib/customers-model.ts";

const id = "11111111-1111-4111-8111-111111111111";
const customer = () => ({
  customer_id: id, first_seen_at: "2026-09-01T00:00:00Z", last_activity_at: "2026-09-29T10:20:30.123456Z",
  display_name: "Test Buyer", phone_last3: "123", orders_count: 3, paid_orders_count: 2,
  captured_minor: 300000, refunded_minor: 50000, currency: "TWD", claims_count: 1, platforms: ["facebook", "manual"],
  consents: { marketing_messages: true, ads_personalization: false }, active: true, imported: false, // W5-02B: Go customerKeys carries imported
  tags: [{ id: "55555555-5555-4555-8555-555555555555", name: "VIP", color: "red" }],
});
const summary = () => ({
  order_id: "22222222-2222-4222-8222-222222222222", created_at: "2026-09-20T00:00:00.000000Z",
  updated_at: "2026-09-20T00:00:00.000000Z", currency: "TWD", total_minor: 1000, commercial_state: "AWAITING_PAYMENT",
  fulfillment_state: "MANUAL_UNASSIGNED", payment_state: "PENDING", test_mode: true, work_state: "NONE",
  refunded_minor: 0, refund_pending_minor: 0,
  pickup_source: null, payment_mode: "card", collection_state: null, cod_surcharge_minor: null, cod_collect_minor: null, // merchantorders.Summary C4 keys + home-cod R5 projection
});
const detail = () => ({
  ...customer(), orders: [summary()],
  claims: [{ session_id: "33333333-3333-4333-8333-333333333333", platform: "facebook", bound_at: "2026-09-10T00:00:00Z", line_count: 2 }],
  consent_history: [{ purpose: "marketing_messages", channel: "meta_dm", granted: true, source: "buyer_checkout",
    policy_version: "lc-2026-10", occurred_at: "2026-09-10T00:00:00Z" }],
  privacy_actions: [{ kind: "EXPORT", via: "merchant", completed_at: "2026-09-11T00:00:00Z", summary: null }],
  tags_revision: "a".repeat(64), notes: [{ id: "66666666-6666-4666-8666-666666666666", body: "only 7-11\nfixed", author_id: "77777777-7777-4777-8777-777777777777", created_at: "2026-09-12T00:00:00.000000Z", edited_at: null, version: 1 }],
});

test("customer list accepts the frozen row and rejects unknown keys, dup ids, bad cursor, oversize", () => {
  assert.equal(parseCustomerList({ items: [customer()], next_cursor: "abc_-" }).items[0].display_name, "Test Buyer");
  assert.deepEqual(parseCustomerList({ items: [], next_cursor: "" }), { items: [], next_cursor: "" });
  assert.throws(() => parseCustomerList({ items: [{ ...customer(), actor_key: "x" }], next_cursor: "" }));
  assert.throws(() => parseCustomerList({ items: [customer(), customer()], next_cursor: "" }));
  assert.throws(() => parseCustomerList({ items: [], next_cursor: "a b" }));
  assert.throws(() => parseCustomerList({ items: [], next_cursor: "", extra: 1 }));
  // W5-02B: an imported customer parses; the flag is required and boolean (Go internal/customers/types.go customerKeys).
  assert.equal(parseCustomerList({ items: [{ ...customer(), imported: true }], next_cursor: "" }).items[0].imported, true);
  const { imported: _drop, ...noImported } = customer();
  assert.throws(() => parseCustomerList({ items: [noImported], next_cursor: "" }));
  assert.throws(() => parseCustomerList({ items: [{ ...customer(), imported: "yes" }], next_cursor: "" }));
  const many = Array.from({ length: 101 }, (_, n) => ({ ...customer(), customer_id: `11111111-1111-4111-8111-${String(n).padStart(12, "0")}` }));
  assert.throws(() => parseCustomerList({ items: many, next_cursor: "" }));
});

test("customer row invariants: nulls for bundle-only/erased, counts, money, consents", () => {
  const bundleOnly = { ...customer(), display_name: null, phone_last3: null, currency: null, orders_count: 0,
    paid_orders_count: 0, captured_minor: 0, refunded_minor: 0 };
  assert.equal(parseCustomer(bundleOnly).display_name, null);
  const bad: Record<string, unknown>[] = [
    { paid_orders_count: 4 }, { refunded_minor: 300001 }, { captured_minor: -1 }, { captured_minor: 1.5 },
    { phone_last3: "12" }, { currency: "twd" }, { platforms: ["facebook", "facebook"] }, { platforms: ["Facebook"] },
    { customer_id: id.toUpperCase().replace(/[0-9]/g, "A") }, { first_seen_at: "yesterday" },
    { active: false }, // an erased owner cannot still hold a granted consent
    { consents: { marketing_messages: true } }, { display_name: "" }, { display_name: "a\u0000b" },
  ];
  for (const patch of bad) assert.throws(() => parseCustomer({ ...customer(), ...patch }), JSON.stringify(patch));
  const erased = { ...customer(), active: false, display_name: null, phone_last3: null,
    consents: { marketing_messages: false, ads_personalization: false } };
  assert.equal(parseCustomer(erased).active, false);
});

test("customer detail: bounded lists, order summaries reuse the orders parser, id must match", () => {
  const parsed = parseCustomerDetail(detail(), id);
  assert.equal(parsed.orders.length, 1);
  assert.equal(parsed.consent_history[0].purpose, "marketing_messages");
  assert.throws(() => parseCustomerDetail(detail(), "22222222-2222-4222-8222-222222222222"));
  assert.throws(() => parseCustomerDetail({ ...detail(), orders: Array(51).fill(summary()) }, id));
  assert.throws(() => parseCustomerDetail({ ...detail(), orders: [{ ...summary(), payment_state: "NOPE" }] }, id));
  assert.throws(() => parseCustomerDetail({ ...detail(), orders: [summary(), summary()] }, id));
  assert.throws(() => parseCustomerDetail({ ...detail(), claims: [{ session_id: id, platform: "facebook", bound_at: "2026-09-10T00:00:00Z", line_count: 2, actor_key: "x" }] }, id));
  assert.throws(() => parseCustomerDetail({ ...detail(), consent_history: [{ ...detail().consent_history[0], channel: "meta_ads" }] }, id));
  assert.throws(() => parseCustomerDetail({ ...detail(), privacy_actions: [{ ...detail().privacy_actions[0], summary: [] }] }, id));
  assert.throws(() => parseCustomerDetail({ ...detail(), privacy_actions: [{ ...detail().privacy_actions[0], kind: "export" }] }, id));
  const { privacy_actions: _drop, ...missing } = detail();
  assert.throws(() => parseCustomerDetail(missing, id));
});

test("erasure summary is four counts only", () => {
  const ok = { consents_withdrawn: 2, sessions_revoked: 1, snapshots_redacted: 3, bundles_relabelled: 0 };
  assert.deepEqual(parseErasureSummary(ok), ok);
  assert.throws(() => parseErasureSummary({ ...ok, extra: 1 }));
  assert.throws(() => parseErasureSummary({ ...ok, sessions_revoked: -1 }));
});

const row = (over: Record<string, unknown> = {}) => ({
  day: "2026-09-29", currency: "TWD", environment: "SANDBOX", captured_count: 2, captured_minor: 200000,
  refunded_minor: 50000, net_minor: 150000, ...over,
});
test("finance summary: net = captured - refunded (I05), totals carry an empty day, sandbox flag", () => {
  const ok = { from: "2026-09-01", to: "2026-09-30", timezone: "Asia/Taipei", rows: [row()], totals: [row({ day: "" })] };
  const parsed = parseFinanceSummary(ok);
  assert.equal(parsed.totals[0].net_minor, 150000);
  assert.equal(isSandbox(parsed.rows[0]), true);
  assert.equal(isSandbox({ ...parsed.rows[0], environment: "LIVE" }), false);
  // A refund of an earlier day's capture makes a day's net negative: allowed.
  assert.equal(parseFinanceSummary({ ...ok, rows: [row({ captured_minor: 0, refunded_minor: 100, net_minor: -100, captured_count: 0 })] }).rows[0].net_minor, -100);
  assert.throws(() => parseFinanceSummary({ ...ok, rows: [row({ net_minor: 149999 })] }));
  assert.throws(() => parseFinanceSummary({ ...ok, rows: [row({ day: "" })] }));
  assert.throws(() => parseFinanceSummary({ ...ok, totals: [row()] }));
  assert.throws(() => parseFinanceSummary({ ...ok, rows: [row({ currency: "NT$" })] }));
  assert.throws(() => parseFinanceSummary({ ...ok, extra: 1 }));
});

test("financeDay is the UTC+8 day", () => {
  assert.equal(financeDay(new Date("2026-09-29T16:30:00Z")), "2026-09-30");
  assert.equal(financeDay(new Date("2026-09-29T15:30:00Z")), "2026-09-29");
  assert.equal(financeDay(new Date("2026-09-29T15:30:00Z"), -30), "2026-08-30");
});

// W6-01B: the backend now always sends `tags` on rows and `tags_revision` + `notes` on the detail.
test("tags and notes are parsed strictly (W6-01B)", () => {
  assert.equal(parseCustomer(customer()).tags[0].name, "VIP");
  assert.deepEqual(parseCustomer({ ...customer(), tags: [] }).tags, []);
  const tagBad: unknown[] = [
    null, [{ id, name: "VIP", color: "pink" }], [{ id, name: "", color: "red" }], [{ id, name: "x".repeat(21), color: "red" }],
    [{ id, name: "VIP", color: "red", extra: 1 }], [{ id, name: "VIP", color: "red" }, { id, name: "B", color: "red" }],
    Array.from({ length: 21 }, (_, n) => ({ id: `11111111-1111-4111-8111-${String(n).padStart(12, "0")}`, name: `t${n}`, color: "red" })),
  ];
  for (const tags of tagBad) assert.throws(() => parseCustomer({ ...customer(), tags }), JSON.stringify(tags));
  const { tags: _drop, ...noTags } = customer();
  assert.throws(() => parseCustomer(noTags), "a row without tags is drift");
  const d = parseCustomerDetail(detail(), id);
  assert.equal(d.notes[0].body, "only 7-11\nfixed");
  assert.equal(d.tags_revision.length, 64);
  const n0 = detail().notes[0];
  const noteBad: unknown[] = [
    null, [{ ...n0, body: "" }], [{ ...n0, body: " \n " }], [{ ...n0, body: "x".repeat(1001) }], [{ ...n0, body: "a\u0000b" }],
    [{ ...n0, version: 0 }], [{ ...n0, edited_at: "yesterday" }], [{ ...n0, owner_id: id }], Array.from({ length: 51 }, () => n0),
  ];
  for (const notes of noteBad) assert.throws(() => parseCustomerDetail({ ...detail(), notes }, id), JSON.stringify(notes).slice(0, 80));
  assert.throws(() => parseCustomerDetail({ ...detail(), tags_revision: "abc" }, id));
  const { notes: _n, ...noNotes } = detail();
  assert.throws(() => parseCustomerDetail(noNotes, id), "a detail without notes is drift");
  const { tags_revision: _r, ...noRev } = detail();
  assert.throws(() => parseCustomerDetail(noRev, id), "a detail without tags_revision is drift");
});
