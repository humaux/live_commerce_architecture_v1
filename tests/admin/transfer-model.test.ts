// checkout-offline: strict parsers and body builders for the admin bank-transfer DTOs (apps/admin/lib/transfer-model.ts, contracts/storefront-v2.md §C).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  canConfirm,
  canRefundOffline,
  canReject,
  confirmBody,
  refundBody,
  parseTransferDetail,
  parseTransferSettings,
  rejectBody,
  transferSettingsBody,
} from "../../apps/admin/lib/transfer-model.ts";
import { transferCopy } from "../../apps/admin/lib/transfer-copy.ts";
import { parseFinanceSummary } from "../../apps/admin/lib/customers-model.ts";

const ID = "11111111-1111-4111-8111-111111111111";
const at = "2026-10-01T04:00:00.000000Z";
const settings = { version: 2, enabled: true, allow_cvs: false, bank_name: "Taiwan Bank", branch: "", account_name: "Shop Ltd", account_number: "123-456-7890", window_hours: 72 };
const detail = (over: Record<string, unknown> = {}) => ({
  order_id: ID, state: "AWAITING", window_hours: 72, deadline_at: at, currency: "TWD", amount_minor: 90000,
  bank: { bank_name: "Taiwan Bank", branch: "", account_name: "Shop Ltd", account_number: "123-456-7890" },
  proof: null, reject_reason: null, confirmed_at: null, refunded_at: null, ...over,
});
const proof = { last5: "12345", amount_minor: 90000, paid_at: at, submitted_at: at, count: 1 };

test("XFM01 settings parse: exact keys, ranges and the enabled-needs-details rule", () => {
  assert.deepEqual(parseTransferSettings(settings), settings);
  assert.equal(parseTransferSettings({ ...settings, version: 0, enabled: false, bank_name: "", account_name: "", account_number: "" }).enabled, false);
  for (const [name, bad] of [
    ["extra key", { ...settings, iban: "x" }], ["missing key", (({ branch, ...r }) => r)(settings)],
    ["window 5", { ...settings, window_hours: 5 }], ["window 169", { ...settings, window_hours: 169 }],
    ["enabled without bank", { ...settings, bank_name: "" }], ["enabled without number", { ...settings, account_number: "" }],
    ["letters in number", { ...settings, account_number: "12AB5678" }], ["control char", { ...settings, bank_name: "a\nb" }],
    ["string version", { ...settings, version: "2" }],
  ] as const)
    assert.throws(() => parseTransferSettings(bad), /unavailable/, name);
});

test("XFM02 settings body builder: whole hours 6..168, trimmed fields, the exact frozen keys", () => {
  const f = { expectedVersion: 2, enabled: true, allowCvs: true, bankName: " Taiwan Bank ", branch: "", accountName: "Shop Ltd", accountNumber: " 123-456-7890 ", windowHours: "72" };
  assert.deepEqual(transferSettingsBody(f), {
    expected_version: 2, enabled: true, allow_cvs: true, bank_name: "Taiwan Bank", branch: "", account_name: "Shop Ltd", account_number: "123-456-7890", window_hours: 72,
  });
  for (const bad of [
    { ...f, windowHours: "5" }, { ...f, windowHours: "169" }, { ...f, windowHours: "7.5" }, { ...f, windowHours: "" },
    { ...f, bankName: " " }, { ...f, accountName: "" }, { ...f, accountNumber: "abc" }, { ...f, accountNumber: "12" }, { ...f, expectedVersion: -1 },
  ])
    assert.equal(transferSettingsBody(bad), null, JSON.stringify(bad));
  assert.notEqual(transferSettingsBody({ ...f, enabled: false, bankName: "", accountName: "", accountNumber: "" }), null, "off needs no details");
});

test("XFM03 order transfer detail: closed shape and the state/field agreement", () => {
  assert.equal(parseTransferDetail(detail(), ID).state, "AWAITING");
  assert.equal(parseTransferDetail(detail({ state: "SUBMITTED", proof }), ID).proof?.last5, "12345");
  assert.equal(parseTransferDetail(detail({ state: "REJECTED", proof, reject_reason: "amount differs" }), ID).reject_reason, "amount differs");
  assert.equal(parseTransferDetail(detail({ state: "CONFIRMED", confirmed_at: at }), ID).confirmed_at, at);
  assert.equal(parseTransferDetail(detail({ state: "REFUNDED_OFFLINE", confirmed_at: at, refunded_at: at }), ID).refunded_at, at);
  assert.equal(parseTransferDetail(detail({ state: "EXPIRED" }), ID).state, "EXPIRED");
  for (const [name, bad] of [
    ["another order", detail({ order_id: "22222222-2222-4222-8222-222222222222" })],
    ["unknown state", detail({ state: "PAID" })], ["submitted without proof", detail({ state: "SUBMITTED" })],
    ["rejected without reason", detail({ state: "REJECTED", proof })], ["reason while awaiting", detail({ reject_reason: "x" })],
    ["confirmed without time", detail({ state: "CONFIRMED" })], ["bad last5", detail({ state: "SUBMITTED", proof: { ...proof, last5: "12a45" } })],
    ["extra key", { ...detail(), buyer_email: "a@b.co" }], ["bank missing", detail({ bank: null })],
    ["fractional amount", detail({ amount_minor: 1.5 })], ["window 3", detail({ window_hours: 3 })],
  ] as const)
    assert.throws(() => parseTransferDetail(bad, ID), /unavailable/, name);
});

test("XFM04 decisions: which button each state offers, and the reject body", () => {
  assert.deepEqual(["AWAITING", "SUBMITTED", "REJECTED", "CONFIRMED", "EXPIRED", "REFUNDED_OFFLINE"].map((s) => canConfirm(s as never)), [true, true, true, false, false, false]);
  assert.deepEqual(["AWAITING", "SUBMITTED", "REJECTED", "CONFIRMED", "EXPIRED", "REFUNDED_OFFLINE"].map((s) => canReject(s as never)), [false, true, false, false, false, false]);
  assert.deepEqual(["AWAITING", "SUBMITTED", "REJECTED", "CONFIRMED", "EXPIRED", "REFUNDED_OFFLINE"].map((s) => canRefundOffline(s as never)), [false, false, false, true, false, false]);
  assert.equal(confirmBody(), "{}");
  // K3-02: the refund always states the stock choice explicitly; restock is opt-in.
  assert.equal(refundBody(false), '{"restock":false}');
  assert.equal(refundBody(true), '{"restock":true}');
  assert.equal(rejectBody("  amount does not match  "), JSON.stringify({ reason: "amount does not match" }));
  for (const bad of ["", "   ", "x".repeat(201), "line\nbreak"]) assert.equal(rejectBody(bad), null, JSON.stringify(bad));
  assert.notEqual(rejectBody("x".repeat(200)), null);
});

test("XFM05 copy: the three locales carry every key, state and refusal text", () => {
  const keys = Object.keys(transferCopy.en).sort().join();
  for (const locale of ["zh-CN", "zh-TW"] as const) assert.equal(Object.keys(transferCopy[locale]).sort().join(), keys, locale);
  for (const [locale, c] of Object.entries(transferCopy)) {
    for (const state of ["AWAITING", "SUBMITTED", "REJECTED", "CONFIRMED", "EXPIRED", "REFUNDED_OFFLINE"]) assert.ok((c.states as Record<string, string>)[state], `${locale} ${state}`);
    for (const code of ["already_confirmed", "already_refunded", "transfer_not_open", "transfer_window_closed", "transfer_not_submitted", "transfer_not_confirmed", "already_shipped", "restock_unavailable", "not_bank_transfer", "invalid_reason", "default"])
      assert.ok(c.errors[code]?.length > 0, `${locale} ${code}`);
    assert.match(c.confirmText("NT$900"), /900/);
    for (const key of ["confirmNoProofWarning", "refundRestockLabel", "refundRestockHint", "refundRestockShipped"] as const) assert.ok(c[key].length > 0, `${locale} ${key}`);
  }
});

test("XFM06 finance rows: the 11-key shape parses, and a transfer pair without the pickup pair is refused", () => {
  const row = (day: string) => ({ day, currency: "TWD", environment: "LIVE", captured_count: 0, captured_minor: 0, refunded_minor: 0, net_minor: 0,
    pickup_collected_count: 0, pickup_collected_minor: 0, bank_transfer_confirmed_count: 2, bank_transfer_confirmed_minor: 180000 });
  const total = { ...row(""), day: "" };
  const parsed = parseFinanceSummary({ from: "2026-10-01", to: "2026-10-01", timezone: "Asia/Taipei", rows: [row("2026-10-01")], totals: [total] });
  assert.equal(parsed.rows[0].bank_transfer_confirmed_minor, 180000);
  // the 7- and 9-key rows of an older Go still parse with zeroed transfer columns
  const old = { day: "2026-10-01", currency: "TWD", environment: "LIVE", captured_count: 1, captured_minor: 100, refunded_minor: 0, net_minor: 100 };
  assert.equal(parseFinanceSummary({ from: "2026-10-01", to: "2026-10-01", timezone: "Asia/Taipei", rows: [old], totals: [{ ...old, day: "" }] }).rows[0].bank_transfer_confirmed_count, 0);
  const noPickup = (({ pickup_collected_count, pickup_collected_minor, ...r }) => r)(row("2026-10-01"));
  assert.throws(() => parseFinanceSummary({ from: "2026-10-01", to: "2026-10-01", timezone: "Asia/Taipei", rows: [noPickup], totals: [] }), /unavailable/);
  const half = (({ bank_transfer_confirmed_minor, ...r }) => r)(row("2026-10-01"));
  assert.throws(() => parseFinanceSummary({ from: "2026-10-01", to: "2026-10-01", timezone: "Asia/Taipei", rows: [half], totals: [] }), /unavailable/);
});
