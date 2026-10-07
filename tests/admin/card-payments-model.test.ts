// w4-u1-payment-activation-ui: strict parsers for the frozen platform-Stripe card DTOs
// (apps/admin/lib/card-payments-model.ts, card-payments-settlements-model.ts) against
// internal/payments/platformstripe + internal/payments/settlement (contract stripe-platform-account-v1).
// Synthetic fixtures only; no Stripe account ids, keys or approval data may ever appear (integrator ruling).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  descriptorPreview,
  parseCardResult,
  parseCardSummary,
  suffixBudget,
  validDescriptorSuffix,
} from "../../apps/admin/lib/card-payments-model.ts";
import {
  parseSettlementDetail,
  parseSettlementList,
} from "../../apps/admin/lib/card-payments-settlements-model.ts";

const summary = () => ({
  platform_state: "OPEN",
  store_state: "ENABLED",
  allowed: true,
  terms_version: "pf-2026-10",
  accepted_terms_version: "pf-2026-10",
  display_name: "Platform Test",
  descriptor_preview: "LCPLATFORM* SHOP",
  currency: "TWD",
  min_minor: 100,
  max_minor: 500000,
  version: 3,
});

test("card summary accepts the frozen shape incl. all-null platform NONE", () => {
  const parsed = parseCardSummary(summary());
  assert.equal(parsed.store_state, "ENABLED");
  assert.equal(parsed.version, 3);
  const none = {
    platform_state: "NONE",
    store_state: "NONE",
    allowed: false,
    terms_version: null,
    accepted_terms_version: null,
    display_name: null,
    descriptor_preview: null,
    currency: null,
    min_minor: null,
    max_minor: null,
    version: 0,
  };
  assert.equal(parseCardSummary(none).descriptor_preview, null);
  for (const platform of ["NONE", "DESIGNATED", "OPEN", "CLOSED", "REVOKED"])
    assert.equal(parseCardSummary({ ...none, platform_state: platform }).platform_state, platform);
  for (const store of ["NONE", "ENABLED", "DISABLED", "BLOCKED"])
    assert.equal(parseCardSummary({ ...none, store_state: store }).store_state, store);
});

test("card summary rejects unknown keys, bad enums, bad money, bad version", () => {
  const bad: Record<string, unknown>[] = [
    { extra: 1 },
    { platform_state: "OPEN " },
    { platform_state: "open" },
    { store_state: "PAUSED" },
    { allowed: "yes" },
    { currency: "twd" },
    { currency: "TW" },
    { min_minor: -1 },
    { max_minor: 1.5 },
    { min_minor: 10, max_minor: 5 },
    { version: -1 },
    { version: 1.2 },
    { terms_version: "" },
    { descriptor_preview: "x".repeat(65) },
    { display_name: "has\nnewline" },
  ];
  for (const patch of bad) assert.throws(() => parseCardSummary({ ...summary(), ...patch }), JSON.stringify(patch));
  const { version: _v, ...missing } = summary();
  assert.throws(() => parseCardSummary(missing));
  assert.throws(() => parseCardSummary(null));
  assert.throws(() => parseCardSummary([summary()]));
});

test("card result is exactly {state, version, max_minor, currency, descriptor_preview}", () => {
  const result = {
    state: "ENABLED",
    version: 4,
    max_minor: 500000,
    currency: "TWD",
    descriptor_preview: "LCPLATFORM* SHOP",
  };
  assert.equal(parseCardResult(result).state, "ENABLED");
  assert.equal(
    parseCardResult({ state: "DISABLED", version: 5, max_minor: null, currency: null, descriptor_preview: null }).state,
    "DISABLED",
  );
  assert.throws(() => parseCardResult({ ...result, account: "acct_1" }));
  assert.throws(() => parseCardResult({ ...result, state: "READY" }));
  assert.throws(() => parseCardResult({ ...result, max_minor: -1 }));
});

test("descriptor suffix rule: charset, 2..10 chars, must contain a letter", () => {
  for (const ok of ["SHOP", "A1", "1A", "S H-9.Z", "ABCDEFGHIJ", "SHOP "]) assert.equal(validDescriptorSuffix(ok), true, ok);
  for (const no of ["", "S", "12", "ABCDEFGHIJK", " SHOP", "SH_OP", "SH*OP", "店", "SHOP\n"])
    assert.equal(validDescriptorSuffix(no), false, JSON.stringify(no));
});

test("suffix budget derives from descriptor_preview as the base (L + 2 + suffix <= 22, charset caps at 10)", () => {
  assert.equal(suffixBudget(null), null);
  assert.equal(suffixBudget("LCPLATFORM"), 10);
  assert.equal(suffixBudget("A".repeat(20)), 0);
  assert.equal(suffixBudget("A".repeat(22)), 0);
  assert.equal(suffixBudget("SHORT"), 10);
});

test("descriptor preview mirrors the contract projection (base + '* ' + suffix)", () => {
  assert.equal(descriptorPreview("LCPLATFORM", null), "LCPLATFORM");
  assert.equal(descriptorPreview("LCPLATFORM", "SHOP"), "LCPLATFORM* SHOP");
});

const line = () => ({
  order_number: `LC-${"AB12".repeat(8)}`,
  kind: "CHARGE",
  store_minor: 12000,
  fee_store_minor: 408,
  txn_date: "2026-09-10",
});
const statement = () => ({
  statement_id: "11111111-2222-3333-4444-555555555555",
  period_start: "2026-09-07",
  period_end: "2026-09-13",
  currency: "TWD",
  captured_minor: 50000,
  refunded_minor: 3000,
  dispute_minor: 0,
  stripe_fee_minor: 1639,
  platform_fee_bps: 150,
  platform_fee_minor: 750,
  carried_in_minor: 0,
  net_payable_minor: 44611,
  line_count: 1,
  closed_at: "2026-09-14T00:00:05Z",
  paid: false,
});

test("settlement list accepts statements without payout/lines keys (omitempty)", () => {
  const list = parseSettlementList({ statements: [statement()] });
  assert.equal(list.length, 1);
  assert.equal(list[0].payout_ref, null);
  assert.equal(list[0].lines, null);
  assert.equal(parseSettlementList({ statements: [] }).length, 0);
  const paid = {
    ...statement(),
    paid: true,
    payout_ref: "po_2026wk37-store",
    payout_minor: 44611,
    paid_at: "2026-09-16T02:00:00Z",
  };
  assert.equal(parseSettlementList({ statements: [paid] })[0].payout_ref, "po_2026wk37-store");
});

test("settlement detail carries lines whose count matches line_count", () => {
  const detail = parseSettlementDetail({ statement: { ...statement(), lines: [line()] } });
  assert.equal(detail.lines?.length, 1);
  assert.equal(detail.lines?.[0].kind, "CHARGE");
  for (const kind of ["CHARGE", "REFUND", "REFUND_FAILURE", "DISPUTE", "DISPUTE_REVERSAL"])
    assert.equal(
      parseSettlementDetail({ statement: { ...statement(), lines: [{ ...line(), kind }] } }).lines?.[0].kind,
      kind,
    );
  // REFUND lines are negative in store currency; fees never are.
  assert.equal(
    parseSettlementDetail({ statement: { ...statement(), lines: [{ ...line(), kind: "REFUND", store_minor: -3000 }] } })
      .lines?.[0].store_minor,
    -3000,
  );
});

test("settlement parsers reject malformed reads", () => {
  assert.throws(() => parseSettlementList({ statements: [statement(), ...Array.from({ length: 52 }, () => statement())] }));
  const badList: Record<string, unknown>[] = [
    { extra: 1 },
    { statement_id: "not-a-uuid" },
    { period_start: "2026-13-01" },
    { period_start: "2026-09-13", period_end: "2026-09-07" },
    { currency: "twd" },
    { captured_minor: -1 },
    { stripe_fee_minor: 1.5 },
    { carried_in_minor: Number.MAX_SAFE_INTEGER },
    { platform_fee_bps: 10001 },
    { platform_fee_bps: -1 },
    { line_count: -1 },
    { closed_at: "2026-09-14" },
    { paid: "no" },
    { paid: false, payout_ref: "po_x" },
    { paid: false, paid_at: "2026-09-16T02:00:00Z" },
    { paid: true, payout_ref: "bad ref!" },
    { paid: true, payout_ref: "po" },
    { paid: true, payout_minor: -5 },
    { paid: true, paid_at: "soon" },
    { lines: [] }, // line_count is 1
    { lines: [line(), line()] },
  ];
  for (const patch of badList)
    assert.throws(() => parseSettlementList({ statements: [{ ...statement(), ...patch }] }), JSON.stringify(patch));
  const badLines: Record<string, unknown>[] = [
    [{ ...line(), order_number: "ORDER-1" }],
    [{ ...line(), kind: "ADJUSTMENT" }],
    [{ ...line(), store_minor: 1.5 }],
    [{ ...line(), fee_store_minor: -1 }],
    [{ ...line(), txn_date: "2026-02-30" }],
    [{ ...line(), extra: 1 }],
  ];
  for (const lines of badLines)
    assert.throws(
      () => parseSettlementDetail({ statement: { ...statement(), lines } }),
      JSON.stringify(lines),
    );
  assert.throws(() => parseSettlementDetail({ statement: statement(), extra: 1 }));
  assert.throws(() => parseSettlementDetail(statement()));
  assert.throws(() => parseSettlementList(null));
});
