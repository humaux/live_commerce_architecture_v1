// w4-u1-payment-activation-ui: the platform collector disclosure on the buyer payment view
// (stripe-platform-account-v1 §5). validOrderPayment must admit the Go `collector` projection
// (derived connections) and keep rejecting identifier-shaped or malformed additions; the rendered
// line substitutes display_name / store_name / descriptor_preview into the contract templates.
import test from "node:test";
import assert from "node:assert/strict";
import { validOrderPayment } from "../lib/payment-contract.ts";
import { paymentCopy } from "../lib/payment-copy.ts";

const orderID = "12345678-1234-1234-1234-123456789abc";
const stripeMethod = {
  code: "stripe_checkout",
  version: 2,
  name_hans: "信用卡",
  name_hant: "信用卡",
  name_en: "Card",
};
const stripeView = {
  order_id: orderID,
  currency: "TWD",
  total_minor: 2500,
  commercial_state: "DRAFT",
  payment_state: "NOT_STARTED",
  handoff_state: "NONE",
  handoff_expires_at: null,
  test_mode: true,
  // A fresh Stripe view offers the method and carries no cancel_requested (isStripeView's second leg).
  methods: [stripeMethod],
};
const collector = {
  display_name: "Platform Test",
  descriptor_preview: "LCPLATFORM* SHOP",
};

test("collector projection: admitted for derived connections, absent/null for primary", () => {
  assert.equal(validOrderPayment({ ...stripeView, collector }, orderID), true);
  assert.equal(validOrderPayment({ ...stripeView, collector: null }, orderID), true);
  // A primary-connection view (no collector key at all) stays byte-identical and valid.
  assert.equal(validOrderPayment(stripeView, orderID), true);
});

test("collector rejects identifiers, extra keys and malformed text", () => {
  for (const bad of [
    { ...collector, account: "acct_1x" },
    { display_name: "Platform Test" },
    { display_name: "", descriptor_preview: "LCPLATFORM* SHOP" },
    { display_name: "Platform Test", descriptor_preview: "" },
    { display_name: "line\nbreak", descriptor_preview: "LCPLATFORM" },
    "LCPLATFORM* SHOP",
    42,
  ])
    assert.equal(validOrderPayment({ ...stripeView, collector: bad }, orderID), false, JSON.stringify(bad));
});

test("disclosure renders the contract §5 line with all three slots substituted", () => {
  const render = (locale) =>
    paymentCopy[locale].collector
      .replace("{display_name}", collector.display_name)
      .replace("{store_name}", "Demo Store")
      .replace("{descriptor_preview}", collector.descriptor_preview);
  assert.equal(
    render("zh-TW"),
    "本筆信用卡款項由 Platform Test 代 Demo Store 收取，信用卡帳單顯示「LCPLATFORM* SHOP」。",
  );
  assert.equal(
    render("zh-CN"),
    "本笔信用卡款项由 Platform Test 代 Demo Store 收取，信用卡账单显示「LCPLATFORM* SHOP」。",
  );
  assert.equal(
    render("en"),
    'Card payment collected by Platform Test on behalf of Demo Store. Your card statement shows "LCPLATFORM* SHOP".',
  );
  // No placeholder may survive a render.
  for (const locale of ["en", "zh-CN", "zh-TW"]) assert.ok(!render(locale).includes("{"), locale);
});
