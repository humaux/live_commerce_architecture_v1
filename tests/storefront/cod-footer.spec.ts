// Purpose: OC-09a — owner-approved plain-language COD copy (output/oc-09/OWNER-APPROVED-COPY.md) on the order page and the pre-order note,
//   and the order-page amount staying the server cod_collect_minor; OC-09b — the checkout bottom bar 「到貨需付」 shows the server quote's
//   cod_due_minor (never a client sum) only while cash on delivery is the selected mode.
// Depends on: node:test, apps/storefront/lib/cod-copy.ts (runtime imports: codCopy, codAmountLabel, codFooter), apps/storefront/lib/money.ts
//   (formatMoney), and the source text of CodOrderStatus.tsx, CheckoutFlow.tsx and OrderFlow.tsx. No browser.
// Used by: scripts/dev/test-node.sh (one integrator-granted registration line).
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { codAmountLabel, codCopy } from "../../apps/storefront/lib/cod-copy.ts";
import * as codModule from "../../apps/storefront/lib/cod-copy.ts";
import { formatMoney } from "../../apps/storefront/lib/money.ts";

const want = {
  "zh-TW": { pending_label: "到貨時要付的金額", recorded: "原訂單代收金額", pending: "包裹送達時，付現金給物流人員", note: "包裹送達時付現金給物流人員，網站不會先扣款。" },
  "zh-CN": { pending_label: "到货时要付的金额", recorded: "原订单代收金额", pending: "包裹送达时，付现金给物流人员", note: "包裹送达时付现金给物流人员，网站不会先扣款。" },
  en: { pending_label: "Amount to pay on delivery", recorded: "Original collection amount", pending: "Pay the delivery driver in cash when the parcel arrives", note: "Pay the delivery driver in cash when the parcel arrives. The site does not charge you beforehand." },
} as const;

for (const [locale, w] of Object.entries(want)) {
  test(`order-page and pre-order COD copy, ${locale}`, () => {
    const c = codCopy[locale as keyof typeof want];
    assert.equal(c.recordedPending, w.pending_label); // PENDING only
    assert.equal(c.recorded, w.recorded); // every other state keeps the previous wording
    assert.equal(c.orderStates.PENDING, w.pending);
    assert.equal(c.codNote, w.note);
  });
}

test("old wording is gone", () => {
  const all = JSON.stringify(codCopy);
  for (const old of ["向貨運公司", "向货运公司", "Pay the carrier"]) assert.ok(!all.includes(old), old);
});

test("order-page amount is the server cod_collect_minor, unchanged; no arithmetic in the component", () => {
  const src = readFileSync(new URL("../../apps/storefront/components/CodOrderStatus.tsx", import.meta.url), "utf8");
  assert.match(src, /formatMoney\(locale, order\.cod_collect_minor \?\? 0, "TWD"\)/);
  assert.doesNotMatch(src, /cod_collect_minor[^)]*[+\-*]/);
  // the rendered label comes from codAmountLabel (behaviour asserted below), not an inline choice
  assert.match(src, /label=\{codAmountLabel\(locale, collection\)\}/);
});

for (const [locale, w] of Object.entries(want)) {
  test(`amount label per collection state, ${locale}: approved wording only while PENDING`, () => {
    const states = Object.keys(codCopy[locale as keyof typeof want].orderStates) as (keyof typeof codCopy.en.orderStates)[];
    assert.ok(states.length >= 6 && states.includes("PENDING"), states.join());
    for (const st of states) assert.equal(codAmountLabel(locale as keyof typeof want, st), st === "PENDING" ? w.pending_label : w.recorded, st);
  });
}

// OC-09b bottom bar. Looked up on the module object so a missing export fails these tests, not the whole file.
type Footer = { label: string; amount: string; note: string } | null;
const codFooter = (codModule as Record<string, unknown>).codFooter as
  | ((locale: keyof typeof want, codDueMinor: number | undefined, fee: number | null, money: (minor: number) => string) => Footer)
  | undefined;

for (const locale of Object.keys(want) as (keyof typeof want)[]) {
  test(`checkout bottom bar, ${locale}: COD shows the server cod_due_minor as given; card and a missing due hide it`, () => {
    assert.equal(typeof codFooter, "function", "cod-copy.ts exports codFooter");
    const money = (minor: number) => formatMoney(locale, minor, "TWD");
    const c = codCopy[locale];
    assert.deepEqual(codFooter!(locale, 7500, 5000, money), { label: c.due, amount: money(7500), note: c.includesFee(money(5000)) });
    // the amount is the server value verbatim: a due that is not total + fee is shown as given, nothing is added
    assert.equal(codFooter!(locale, 1234500, 5000, money)?.amount, money(1234500));
    assert.equal(codFooter!(locale, 7500, 0, money)?.note, ""); // no fee: the bracket is omitted (owner rule)
    assert.equal(codFooter!(locale, 7500, null, money), null); // card (or COD not offered): hidden
    assert.equal(codFooter!(locale, undefined, 5000, money), null); // server omitted cod_due_minor: hidden, never computed
  });
}

test("approved footer label: 到貨需付 / 到货需付 / Due on delivery", () => {
  assert.equal(codCopy["zh-TW"].due, "到貨需付");
  assert.equal(codCopy["zh-CN"].due, "到货需付");
  assert.equal(codCopy.en.due, "Due on delivery");
});

test("bottom bar source: CheckoutFlow renders codFooter from quote.cod_due_minor; no COD sum in OrderFlow", () => {
  const checkout = readFileSync(new URL("../../apps/storefront/components/CheckoutFlow.tsx", import.meta.url), "utf8");
  assert.match(checkout, /codFooter\(locale, quote\.cod_due_minor, codFee, /);
  assert.doesNotMatch(checkout, /cod_due_minor[^)\n]*[+\-*]/);
  const order = readFileSync(new URL("../../apps/storefront/components/OrderFlow.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(order, /total_minor \+ \(option\.cod_surcharge_minor/);
  assert.match(order, /quote\.cod_due_minor/);
});
