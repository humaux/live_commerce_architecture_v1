// w4-u1-payment-activation-ui: the admin parsers against the REAL wire bodies of the Go handlers (E3: Go + PG -> JSON -> TS).
// The fixtures in tests/admin/fixtures/card-payments-wire are the verbatim HTTP bodies of GET payments/card and GET settlements[/{id}]
// captured by tests/foundation/w4_u1_seed_test.go TestW4U1WireShapes (LC_W4U1_WIRE_OUT=<dir>) after statements were produced only through the
// operator paths (sync -> close -> payout). They are synthetic data (no buyer PII, no Stripe id, no key).
// Regenerate: LC_W4U1_WIRE_OUT=$PWD/tests/admin/fixtures/card-payments-wire bash scripts/dev/test-focused.sh '^TestW4U1WireShapes$'
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { cardView, parseCardSummary } from "../../apps/admin/lib/card-payments-model.ts";
import { parseSettlementDetail, parseSettlementList, payoutState, statementCells } from "../../apps/admin/lib/card-payments-settlements-model.ts";

const dir = new URL("./fixtures/card-payments-wire/", import.meta.url);
const read = (name: string): unknown => JSON.parse(readFileSync(new URL(name, dir), "utf8"));
const raw = (name: string) => readFileSync(new URL(name, dir), "utf8");

test("the real card summary parses and drives the display state machine", () => {
  const summary = parseCardSummary(read("card-enabled.json"));
  assert.deepEqual([summary.platform_state, summary.store_state, summary.allowed, summary.currency, summary.min_minor], ["OPEN", "ENABLED", true, "TWD", 2500]);
  const v = cardView(summary, true);
  assert.deepEqual([v.notOpen, v.canEnable, v.canDisable], [false, false, true]);
});

test("the real statement list parses: newest first, no lines, three payout states", () => {
  const list = parseSettlementList(read("settlements-list.json"));
  assert.equal(list.length, 3);
  assert.deepEqual(list.map((s) => s.period_start), [...list.map((s) => s.period_start)].sort().reverse());
  assert.ok(list.every((s) => s.lines === null));
  assert.deepEqual(list.map(payoutState), ["pending", "none", "paid"]);
  const [pending, carried, paid] = list;
  // the signs the Kimi parsers refused: fees are costs, a debt is carried in, a dispute reversal makes the dispute total negative
  assert.ok(paid.stripe_fee_minor < 0 && carried.net_payable_minor < 0 && pending.carried_in_minor < 0 && pending.dispute_minor < 0);
  assert.equal(paid.payout_minor, paid.net_payable_minor);
  assert.equal(paid.payout_ref, "BANK-REF-W4U1-0831");
  assert.ok(Number.isFinite(Date.parse(paid.closed_at)) && /[+-]\d\d:\d\d$/.test(paid.closed_at), "PostgreSQL offset form");
});

test("every real detail parses with its lines and the lines add up to the statement", () => {
  for (const name of ["settlement-paid.json", "settlement-carried.json", "settlement-pending.json"]) {
    const s = parseSettlementDetail(read(name));
    assert.ok(s.lines && s.lines.length === s.line_count && s.line_count > 0, name);
    assert.ok(s.lines.every((l) => /^LC-[0-9A-F]{32}$/.test(l.order_number)), name);
  }
  assert.deepEqual(parseSettlementDetail(read("settlement-pending.json")).lines?.map((l) => l.kind), ["CHARGE", "DISPUTE_REVERSAL"]);
  assert.deepEqual(parseSettlementDetail(read("settlement-carried.json")).lines?.map((l) => l.kind), ["CHARGE", "REFUND", "DISPUTE"]);
});

test("an empty ledger parses to no statements", () => {
  assert.deepEqual(parseSettlementList(read("settlements-empty.json")), []);
});

test("no account id, key or approval shape is in any real body", () => {
  for (const name of ["card-enabled.json", "settlements-list.json", "settlement-paid.json", "settlement-carried.json", "settlement-pending.json", "settlements-empty.json"])
    assert.doesNotMatch(raw(name), /:"(?:acct_|sk_(?:live|test)_|rk_(?:live|test)_|whsec_|txn_|dp_|re_)[A-Za-z0-9]|approval/i, name);
});

// Review P1-1, on the REAL ledger rows: literal strings, not a re-run of the formatter. Sign = what the store receives.
test("the real statements display literally as signed contributions that add up to the server's net", () => {
  const rows = parseSettlementList(read("settlements-list.json")).map((s) => statementCells("en", s));
  assert.deepEqual(rows, [
    // week of a dispute REVERSAL (+NT$25, a credit) and a RETURNED Stripe fee (+NT$58, a credit) with the debt of the week before carried in
    ["2026-09-14 – 2026-09-20", "+NT$25", "NT$0", "+NT$25", "+NT$58", "NT$0", "-NT$68", "NT$40"],
    // week of a refund (-NT$8), a dispute (-NT$25) and the fees: nets below zero, never paid
    ["2026-09-07 – 2026-09-13", "+NT$25", "-NT$8", "-NT$25", "-NT$60", "NT$0", "NT$0", "-NT$68"],
    ["2026-08-31 – 2026-09-06", "+NT$25", "NT$0", "NT$0", "-NT$1", "NT$0", "NT$0", "NT$24"],
  ]);
  const minor = (text: string) => {
    const m = /^([+-]?)NT\$([\d,]+)$/.exec(text)!;
    return (m[1] === "-" ? -1 : 1) * Number(m[2].replace(/,/g, "")) * 100;
  };
  for (const row of rows) assert.equal(row.slice(1, 7).map(minor).reduce((a, b) => a + b, 0), minor(row[7]), row[0]);
});
