// w4-u1-payment-activation-ui: strict parsers for the frozen platform-Stripe card DTOs
// (apps/admin/lib/card-payments-model.ts, card-payments-settlements-model.ts) against
// internal/payments/platformstripe + internal/payments/settlement (contract stripe-platform-account-v1).
// Synthetic fixtures only; no Stripe account ids, keys or approval data may ever appear (integrator ruling).
import assert from "node:assert/strict";
import { test } from "node:test";
import { canOpen, matchRoute, routes, visibleGroups } from "../../apps/admin/src/routes.ts";
import { shellCopy } from "../../apps/admin/src/shell-copy.ts";
import {
  buildCardInput,
  cardView,
  descriptorBase,
  descriptorPreview,
  descriptorSuffixOf,
  parseCardResult,
  parseCardSummary,
  suffixBudget,
  validDescriptorSuffix,
} from "../../apps/admin/lib/card-payments-model.ts";
import {
  parseSettlementDetail,
  parseSettlementList,
  payoutState,
  periodLabel,
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
  // descriptorBase recovers the platform base even from a DISABLED store's retained-suffix preview.
  assert.equal(descriptorBase("LCPLATFORM"), "LCPLATFORM");
  assert.equal(descriptorBase("LCPLATFORM* SHOP"), "LCPLATFORM");
});

// Signs follow migrations/0150: a fee is a cost (<= 0), refunds/disputes are negative store amounts on their lines,
// and the statement nets captured - refunded - dispute + stripe_fee - platform_fee + carried_in.
const line = () => ({
  order_number: `LC-${"AB12".repeat(8)}`,
  kind: "CHARGE",
  store_minor: 12000,
  fee_store_minor: -400,
  txn_date: "2026-09-10",
});
const statement = () => ({
  statement_id: "11111111-2222-3333-4444-555555555555",
  period_start: "2026-09-07",
  period_end: "2026-09-14", // exclusive: start + 7 (migrations/0150 CHECK)
  currency: "TWD",
  captured_minor: 50000,
  refunded_minor: 3000,
  dispute_minor: 0,
  stripe_fee_minor: -1600,
  platform_fee_bps: 150,
  platform_fee_minor: 750,
  carried_in_minor: 0,
  net_payable_minor: 44650,
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
    payout_minor: 44650,
    paid_at: "2026-09-16T02:00:00Z",
  };
  assert.equal(parseSettlementList({ statements: [paid] })[0].payout_ref, "po_2026wk37-store");
});

// A detail whose totals are exactly the sums of its lines (close_settlement's arithmetic), with no platform fee or carry.
const detailOf = (lines: ReturnType<typeof line>[]) => {
  const sum = (kinds: string[], pick: (l: ReturnType<typeof line>) => number) => lines.filter((l) => kinds.includes(l.kind)).reduce((n, l) => n + pick(l), 0);
  const captured = sum(["CHARGE"], (l) => l.store_minor);
  const refunded = -sum(["REFUND", "REFUND_FAILURE"], (l) => l.store_minor);
  const dispute = -sum(["DISPUTE", "DISPUTE_REVERSAL"], (l) => l.store_minor);
  const fee = lines.reduce((n, l) => n + l.fee_store_minor, 0);
  return {
    ...statement(),
    captured_minor: captured,
    refunded_minor: refunded,
    dispute_minor: dispute,
    stripe_fee_minor: fee,
    platform_fee_bps: 0,
    platform_fee_minor: 0,
    carried_in_minor: 0,
    net_payable_minor: captured - refunded - dispute + fee,
    line_count: lines.length,
    lines,
  };
};

test("settlement detail carries lines whose count and sums match the totals", () => {
  const detail = parseSettlementDetail({ statement: detailOf([line()]) });
  assert.equal(detail.lines?.length, 1);
  assert.equal(detail.lines?.[0].kind, "CHARGE");
  const signs: Record<string, number> = { CHARGE: 1, REFUND: -1, REFUND_FAILURE: 1, DISPUTE: -1, DISPUTE_REVERSAL: 1 };
  for (const kind of Object.keys(signs))
    assert.equal(
      parseSettlementDetail({ statement: detailOf([{ ...line(), kind, store_minor: signs[kind] * 3000 }]) }).lines?.[0].kind,
      kind,
    );
  // REFUND lines are negative in store currency; the statement's refunded total is positive
  const refund = parseSettlementDetail({ statement: detailOf([{ ...line(), kind: "REFUND", store_minor: -3000, fee_store_minor: 0 }]) });
  assert.equal(refund.lines?.[0].store_minor, -3000);
  assert.equal(refund.refunded_minor, 3000);
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
    { carried_in_minor: 100, net_payable_minor: 44750 }, // 0150 CHECK carried_in <= 0
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
    { paid: true, payout_ref: "BANK-REF-1", payout_minor: 44649, paid_at: "2026-09-16T02:00:00Z" }, // payout must equal net
    { net_payable_minor: 44651 }, // identity: captured - refunded - dispute + fee - platform_fee + carried_in
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
    [{ ...line(), kind: "CHARGE", store_minor: -1 }], // 0150: CHARGE / REFUND_FAILURE / DISPUTE_REVERSAL are >= 0
    [{ ...line(), kind: "REFUND", store_minor: 1 }], // REFUND / DISPUTE are <= 0
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


// ---- real-ledger signs and the totals identity (migrations/0150 settlement_statements CHECKs) ----

test("fees, refunds and disputes are signed exactly as the ledger stores them", () => {
  // A later week with a dispute reversal (store_minor >= 0, so dispute_minor = -sum is NEGATIVE) and a returned
  // refund failure (refunded_minor NEGATIVE) must display, not be refused as "unavailable".
  const week = {
    ...statement(),
    captured_minor: 10000,
    refunded_minor: -500,
    dispute_minor: -2500,
    stripe_fee_minor: -300,
    platform_fee_minor: 0,
    platform_fee_bps: 0,
    carried_in_minor: -1000,
    net_payable_minor: 10000 + 500 + 2500 - 300 - 0 - 1000,
    line_count: 0,
  };
  const got = parseSettlementList({ statements: [week] })[0];
  assert.equal(got.refunded_minor, -500);
  assert.equal(got.dispute_minor, -2500);
  assert.equal(got.stripe_fee_minor, -300);
  // an unpaid negative net is a carry, never a payout
  const debt = { ...week, captured_minor: 0, refunded_minor: 3000, dispute_minor: 0, stripe_fee_minor: -100, carried_in_minor: 0, net_payable_minor: -3100 };
  assert.equal(parseSettlementList({ statements: [debt] })[0].net_payable_minor, -3100);
  assert.throws(() => parseSettlementList({ statements: [{ ...debt, paid: true, payout_ref: "BANK-REF-1", payout_minor: -3100, paid_at: "2026-09-16T02:00:00Z" }] }));
});

test("a returned fee is a positive line fee; the detail's lines must add up to the statement totals", () => {
  const refund = { order_number: `LC-${"CD34".repeat(8)}`, kind: "REFUND", store_minor: -3000, fee_store_minor: 0, txn_date: "2026-09-11" };
  const dispute = { order_number: `LC-${"EF56".repeat(8)}`, kind: "DISPUTE", store_minor: -2000, fee_store_minor: -100, txn_date: "2026-09-12" };
  const reversal = { order_number: dispute.order_number, kind: "DISPUTE_REVERSAL", store_minor: 2000, fee_store_minor: 100, txn_date: "2026-09-13" };
  const lines = [line(), refund, dispute, reversal];
  const totals = {
    ...statement(),
    captured_minor: 12000,
    refunded_minor: 3000,
    dispute_minor: 0, // -(-2000 + 2000)
    stripe_fee_minor: -400 + 0 - 100 + 100,
    platform_fee_minor: 0,
    platform_fee_bps: 0,
    carried_in_minor: 0,
    net_payable_minor: 12000 - 3000 - 0 - 400,
    line_count: 4,
  };
  assert.equal(parseSettlementDetail({ statement: { ...totals, lines } }).lines?.length, 4);
  // one number off in any total, or a line that is not part of it, is an unreadable statement
  assert.throws(() => parseSettlementDetail({ statement: { ...totals, captured_minor: 12001, net_payable_minor: 12000 - 3000 - 400 + 1, lines } }));
  assert.throws(() => parseSettlementDetail({ statement: { ...totals, lines: [line(), refund, dispute, { ...reversal, fee_store_minor: 99 }] } }));
});

test("closed_at/paid_at accept the offsets PostgreSQL emits as well as Z", () => {
  for (const at of ["2026-09-14T00:00:05Z", "2026-09-14T08:00:05.123456+08:00", "2026-09-14T00:00:05+00:00"]) {
    const list = parseSettlementList({ statements: [{ ...statement(), closed_at: at }] });
    assert.equal(list[0].closed_at, at);
  }
  for (const at of ["2026-09-14T00:00:05", "2026-09-14T25:00:05Z", "2026-09-14T00:00:05+8"])
    assert.throws(() => parseSettlementList({ statements: [{ ...statement(), closed_at: at }] }), at);
});

test("a payout reference shaped like a Stripe key or account id is never displayed", () => {
  const paid = (payout_ref: string) => ({ ...statement(), paid: true, payout_ref, payout_minor: 44650, paid_at: "2026-09-16T02:00:00Z" });
  assert.equal(parseSettlementList({ statements: [paid("BANK-REF-2026-0907")] })[0].payout_ref, "BANK-REF-2026-0907");
  for (const ref of ["acct_1Abcdef", "sk_test_abcdef", "rk_live_abcdef", "pk_test_abcdef", "whsec_abcdef"])
    assert.throws(() => parseSettlementList({ statements: [paid(ref)] }), ref);
});

// ---- the display state machine (what the page shows, per platform and store state) ----

const view = (patch: Record<string, unknown>, manage = true) => cardView(parseCardSummary({ ...summary(), ...patch }), manage);

test("platform not OPEN: 'not open yet' and no enable control, for every non-OPEN state", () => {
  for (const platform_state of ["NONE", "DESIGNATED", "CLOSED", "REVOKED"])
    for (const store_state of ["NONE", "DISABLED"]) {
      const v = view({ platform_state, store_state });
      assert.equal(v.notOpen, true, `${platform_state}/${store_state}`);
      assert.equal(v.canEnable, false, `${platform_state}/${store_state}`);
      assert.equal(v.canDisable, false, `${platform_state}/${store_state}`);
    }
  // contract 3.3: disable is always allowed, so an enabled store can still turn itself off while the platform is CLOSED
  const closed = view({ platform_state: "CLOSED", store_state: "ENABLED" });
  assert.deepEqual([closed.notOpen, closed.canEnable, closed.canDisable], [true, false, true]);
});

test("allowlisted + OPEN: enable from NONE/DISABLED, disable from ENABLED; not allowlisted hides the enable control", () => {
  assert.deepEqual(
    [view({ store_state: "NONE" }).canEnable, view({ store_state: "DISABLED" }).canEnable, view({ store_state: "ENABLED" }).canEnable],
    [true, true, false],
  );
  assert.deepEqual([view({ store_state: "ENABLED" }).canDisable, view({ store_state: "NONE" }).canDisable, view({ store_state: "DISABLED" }).canDisable], [true, false, false]);
  const denied = view({ store_state: "NONE", allowed: false });
  assert.deepEqual([denied.notAllowed, denied.canEnable], [true, false]);
  assert.equal(view({ store_state: "NONE" }).notAllowed, false);
  // the terms are the thing being accepted: no terms_version, no enable
  assert.equal(view({ store_state: "NONE", terms_version: null }).canEnable, false);
});

test("BLOCKED: the suspended note, disable only, never enable (whatever the platform says)", () => {
  const v = view({ store_state: "BLOCKED" });
  assert.deepEqual([v.blocked, v.canEnable, v.canDisable], [true, false, true]);
  const closed = view({ store_state: "BLOCKED", platform_state: "CLOSED" });
  assert.deepEqual([closed.blocked, closed.canEnable, closed.canDisable], [true, false, true]);
  assert.equal(view({ store_state: "ENABLED" }).blocked, false);
});

test("without billing:manage nothing can be changed, but the state is still readable", () => {
  for (const store_state of ["NONE", "ENABLED", "DISABLED", "BLOCKED"]) {
    const v = view({ store_state }, false);
    assert.deepEqual([v.canEnable, v.canDisable], [false, false], store_state);
  }
});

// ---- the PUT body the page sends (exactly the frozen CAS shape) ----

test("enable carries the platform's current terms, the suffix and the version read; NONE starts at 0", () => {
  const none = parseCardSummary({ ...summary(), store_state: "NONE", accepted_terms_version: null, descriptor_preview: "LCPLATFORM", version: 0 });
  assert.deepEqual(buildCardInput(none, true, "SHOP"), { enabled: true, terms_version: "pf-2026-10", descriptor_suffix: "SHOP", expected_version: 0 });
  assert.deepEqual(buildCardInput(none, true, null), { enabled: true, terms_version: "pf-2026-10", descriptor_suffix: null, expected_version: 0 });
});

test("disable never sends a suffix and quotes the terms the store accepted (the platform's may have moved on)", () => {
  const enabled = parseCardSummary({ ...summary(), terms_version: "pf-2026-11", accepted_terms_version: "pf-2026-10", version: 7 });
  assert.deepEqual(buildCardInput(enabled, false, "IGNORED"), { enabled: false, terms_version: "pf-2026-10", descriptor_suffix: null, expected_version: 7 });
  // a platform that has no terms at all (NONE) still yields a well-formed body for a disable
  const bare = parseCardSummary({ ...summary(), terms_version: null, accepted_terms_version: null, store_state: "BLOCKED" });
  assert.equal(buildCardInput(bare, false, null).terms_version.length > 0, true);
});

test("a retained suffix is recovered from the final preview so a re-enable can keep it", () => {
  assert.equal(descriptorSuffixOf("LCPLATFORM"), null);
  assert.equal(descriptorSuffixOf("LCPLATFORM* SHOP"), "SHOP");
  assert.equal(descriptorSuffixOf("LCPLATFORM* "), null);
  assert.equal(descriptorSuffixOf(null), null);
  assert.equal(descriptorSuffixOf("LCPLATFORM* bad_suffix"), null); // not a valid suffix: never pre-filled
});

test("the period label shows the last day inside the (exclusive-end) week", () => {
  assert.equal(periodLabel("2026-09-07", "2026-09-14"), "2026-09-07 – 2026-09-13");
  assert.equal(periodLabel("2026-12-28", "2027-01-04"), "2026-12-28 – 2027-01-03");
  assert.equal(periodLabel("2026-02-23", "2026-03-02"), "2026-02-23 – 2026-03-01");
});

test("payout state: paid, pending only when money is due, none when the week nets to zero or less", () => {
  assert.equal(payoutState({ paid: true, net_payable_minor: 100 }), "paid");
  assert.equal(payoutState({ paid: false, net_payable_minor: 100 }), "pending");
  assert.equal(payoutState({ paid: false, net_payable_minor: 0 }), "none");
  assert.equal(payoutState({ paid: false, net_payable_minor: -900 }), "none");
});

// ---- shell registry: the pages are reachable by permission only and add no nav button (W3-U5 incident) ----

test("card-payments and settlements are nav:false routes, so no group loses or gains a nav button", () => {
  for (const id of ["card-payments", "settlements"]) {
    const route = routes.find((r) => r.id === id);
    assert.ok(route, id);
    assert.equal(route.nav, false, `${id}: a second nav:true route would replace the group's own nav-<group> button`);
    assert.equal(route.group, "settings");
    assert.ok(shellCopy.en[route.labelKey] && shellCopy["zh-TW"][route.labelKey] && shellCopy["zh-CN"][route.labelKey], id);
  }
  // the settings group still shows exactly its three pre-existing entries
  const owner = { role: "owner", permissions: [] };
  assert.deepEqual(
    visibleGroups(owner).find((g) => g.id === "settings")?.routes.map((r) => r.id),
    ["settings", "team", "billing"],
  );
});

test("settlements need billing:manage, the card page integration:read; the owner opens both", () => {
  const settlements = matchRoute("/settings/settlements")!;
  const card = matchRoute("/settings/payments/card")!;
  assert.deepEqual([settlements.permission, card.permission], ["billing:manage", "integration:read"]);
  const staff = (...permissions: string[]) => ({ role: "staff", permissions });
  assert.equal(canOpen(settlements, staff("integration:read")), false, "no billing:manage: the settlements page is hidden");
  assert.equal(canOpen(settlements, staff("billing:manage")), true);
  assert.equal(canOpen(card, staff("billing:manage")), false);
  assert.equal(canOpen(card, staff("integration:read")), true);
  assert.equal(canOpen(settlements, { role: "owner", permissions: [] }) && canOpen(card, { role: "owner", permissions: [] }), true);
  assert.equal(canOpen(settlements, null), false);
});
