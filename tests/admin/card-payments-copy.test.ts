// w4-u1-payment-activation-ui: copy parity for the card-payments and settlements pages plus the
// storefront collector disclosure. The merchant terms (contract stripe-platform-account-v1 §5) and the two
// ruling-fixed strings (「信用卡收款尚未開放」, 「已被平台暫停，請聯絡客服」) are pinned verbatim; no string may
// ever contain a Stripe account id, key or approval id.
import assert from "node:assert/strict";
import { test } from "node:test";
import { cardPaymentsCopy } from "../../apps/admin/lib/card-payments-copy.ts";
import { cardPaymentsSettlementsCopy } from "../../apps/admin/lib/card-payments-settlements-copy.ts";
import { paymentCopy } from "../../apps/storefront/lib/payment-copy.ts";

const locales = ["en", "zh-CN", "zh-TW"] as const;

function keysOf(value: unknown, prefix = ""): string[] {
  if (!value || typeof value !== "object") return [prefix];
  return Object.entries(value as Record<string, unknown>).flatMap(([key, item]) =>
    item && typeof item === "object" ? keysOf(item, `${prefix}${key}.`) : [`${prefix}${key}`],
  );
}
function stringsOf(value: unknown): string[] {
  if (typeof value === "string") return [value];
  if (!value || typeof value !== "object") return [];
  return Object.values(value as Record<string, unknown>).flatMap(stringsOf);
}

test("card-payments copy: identical key sets in all three locales", () => {
  const en = keysOf(cardPaymentsCopy.en).sort();
  for (const locale of locales) assert.deepEqual(keysOf(cardPaymentsCopy[locale]).sort(), en, locale);
  const sEn = keysOf(cardPaymentsSettlementsCopy.en).sort();
  for (const locale of locales) assert.deepEqual(keysOf(cardPaymentsSettlementsCopy[locale]).sort(), sEn, locale);
});

test("merchant terms are quoted verbatim (contract §5) in the enable dialog", () => {
  assert.equal(
    cardPaymentsCopy["zh-TW"].terms,
    "款項由平台代收，按結算週期以銀行轉帳撥付；退款與爭議款會從你的結算中扣除",
  );
  // The other locales translate the same sentence; they must not drift into a different promise.
  for (const locale of locales) {
    const terms = cardPaymentsCopy[locale].terms;
    assert.ok(terms.length >= 20, locale);
    assert.ok(!terms.includes("{"), locale);
  }
});

test("ruling-fixed strings: platform not OPEN and BLOCKED note", () => {
  assert.equal(cardPaymentsCopy["zh-TW"].notOpen, "信用卡收款尚未開放");
  assert.equal(cardPaymentsCopy["zh-TW"].blockedNote, "已被平台暫停，請聯絡客服");
});

test("error map covers every frozen refusal code from migrations/0137", () => {
  const frozen = [
    "platform_stripe_not_allowed",
    "platform_stripe_blocked",
    "platform_stripe_unavailable",
    "platform_stripe_closed",
    "terms_version_stale",
    "version_changed",
    "stripe_store_has_own_account",
    "descriptor_suffix_too_long",
    "invalid_descriptor_suffix",
    "currency_unsupported",
    "no_payment_market",
    "unavailable",
    "retry_later",
    "unauthorized",
    "default",
  ];
  for (const locale of locales)
    for (const code of frozen) assert.ok(cardPaymentsCopy[locale].errors[code], `${locale}:${code}`);
});

test("no Stripe account id, key or approval id shape appears in any copy string", () => {
  const all = [
    ...locales.flatMap((locale) => stringsOf(cardPaymentsCopy[locale])),
    ...locales.flatMap((locale) => stringsOf(cardPaymentsSettlementsCopy[locale])),
    ...locales.flatMap((locale) => stringsOf(paymentCopy[locale])),
  ];
  for (const text of all) assert.ok(!/(?:acct_|sk_(?:live|test)_|rk_(?:live|test)_)/.test(text), text);
});

test("storefront collector disclosure matches contract §5 templates in three languages", () => {
  assert.equal(
    paymentCopy["zh-TW"].collector,
    "本筆信用卡款項由 {display_name} 代 {store_name} 收取，信用卡帳單顯示「{descriptor_preview}」。",
  );
  assert.equal(
    paymentCopy["zh-CN"].collector,
    "本笔信用卡款项由 {display_name} 代 {store_name} 收取，信用卡账单显示「{descriptor_preview}」。",
  );
  assert.equal(
    paymentCopy.en.collector,
    'Card payment collected by {display_name} on behalf of {store_name}. Your card statement shows "{descriptor_preview}".',
  );
  for (const locale of locales)
    for (const slot of ["{display_name}", "{store_name}", "{descriptor_preview}"])
      assert.ok(paymentCopy[locale].collector.includes(slot), `${locale}:${slot}`);
});
