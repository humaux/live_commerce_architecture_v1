// Purpose: OC-09a — owner-approved plain-language COD copy (output/oc-09/OWNER-APPROVED-COPY.md) on the order page and the pre-order note,
//   and the order-page amount staying the server cod_collect_minor. The checkout bottom bar (OC-09b) is deferred and not asserted here.
// Depends on: node:test, apps/storefront/lib/cod-copy.ts (types-only imports), CodOrderStatus.tsx source text. No browser.
// Used by: scripts/dev/test-node.sh (registration requested in the PR; shared file, not edited by OC-09a).
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { codCopy } from "../../apps/storefront/lib/cod-copy.ts";

const want = {
  "zh-TW": { recorded: "到貨時要付的金額", pending: "包裹送達時，付現金給物流人員", note: "包裹送達時付現金給物流人員，網站不會先扣款。" },
  "zh-CN": { recorded: "到货时要付的金额", pending: "包裹送达时，付现金给物流人员", note: "包裹送达时付现金给物流人员，网站不会先扣款。" },
  en: { recorded: "Amount to pay on delivery", pending: "Pay the delivery driver in cash when the parcel arrives", note: "Pay the delivery driver in cash when the parcel arrives. The site does not charge you beforehand." },
} as const;

for (const [locale, w] of Object.entries(want)) {
  test(`order-page and pre-order COD copy, ${locale}`, () => {
    const c = codCopy[locale as keyof typeof want];
    assert.equal(c.recorded, w.recorded);
    assert.equal(c.orderStates.PENDING, w.pending);
    assert.equal(c.codNote, w.note);
  });
}

test("old wording is gone", () => {
  const all = JSON.stringify(codCopy);
  for (const old of ["原訂單代收金額", "原订单代收金额", "Original collection amount", "向貨運公司", "向货运公司", "Pay the carrier"]) assert.ok(!all.includes(old), old);
});

test("order-page amount is the server cod_collect_minor, unchanged; no arithmetic in the component", () => {
  const src = readFileSync(new URL("../../apps/storefront/components/CodOrderStatus.tsx", import.meta.url), "utf8");
  assert.match(src, /formatMoney\(locale, order\.cod_collect_minor \?\? 0, "TWD"\)/);
  assert.doesNotMatch(src, /cod_collect_minor[^)]*[+\-*]/);
});
