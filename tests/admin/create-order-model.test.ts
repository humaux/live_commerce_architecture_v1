// Purpose: LC-U3 A15/A16 exact DTO, private prefill and quote-authority counterexamples.
// Depends on: actual create-order-model and unchanged merchant-tools-model parsers.
// Used by: focused Node gate; synthetic records only.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  parseOrderPrefill,
  parseForBuyerResult,
  targetQuery,
  forBuyerErrors,
  forBuyerError,
  validPrefillQuery,
  validForBuyerBody,
  prefillForm,
  makeForBuyerBody,
} from "../../apps/admin/lib/create-order-model.ts";
import {
  parseManualResult,
  toolsRoute,
} from "../../apps/admin/lib/merchant-tools-model.ts";
const id = "11111111-1111-4111-8111-111111111111",
  optionKey = `${id}|TW|home`;
const prefill = () => ({
  items: [
    {
      sku_id: id,
      offer_id: id,
      keyword: "A1",
      name: "Synthetic item",
      variant: "SKU",
      quantity: 2,
      live_price_minor: 100,
      catalog_price_minor: 200,
      sellable: true,
      live_quantity_remaining: 1,
    },
  ],
  bundles: [id],
  customer: null,
  last_delivery: null,
  suggested_option_key: null,
  live_price_eligible: false,
  live_price_reason: "no_conversation",
});
const body = () => ({
  items: [{ sku_id: id, quantity: 2 }],
  customer: { name: "Synthetic buyer", phone: "0912345678", email: "" },
  delivery: {
    option_key: optionKey,
    home_address: {
      region: "",
      city: "Synthetic city",
      postal_code: "",
      line1: "Synthetic address",
      line2: "",
    },
    cvs: null,
  },
  payment_mode: "bank_transfer",
  locale: "en",
  for: { bundle_ids: [id], conversation_id: null },
  send_payment_link: false,
});
const result = () => ({
  order_id: id,
  commercial_state: "AWAITING_TRANSFER",
  payment_mode: "bank_transfer",
  total_minor: 400,
  currency: "TWD",
  expires_at: "2030-01-01T00:00:00Z",
  buyer_link: null,
  link_state: "configured",
  source: "merchant_manual",
  live_price: "not_applied",
  live_price_reason: "no_conversation",
  send: { state: "not_sent", reason: "not_requested" },
});
test("A15 exact selector XOR and method resources", () => {
  assert.equal(targetQuery({ bundleId: id }), `?bundle_id=${id}`);
  assert.equal(targetQuery({ conversationId: id }), `?conversation_id=${id}`);
  for (const v of [
    {},
    { bundleId: id, conversationId: id },
    { bundleId: "bad" },
    { bundleId: id, extra: 1 },
  ])
    assert.throws(() => targetQuery(v));
  for (const q of [
    "",
    "?",
    `?bundle_id=${id}&bundle_id=${id}`,
    `?bundle_id=${id}&conversation_id=${id}`,
    `?bundle_id=${id}&text=secret`,
    `?bundle_id=${id}&`,
  ])
    assert.equal(validPrefillQuery(q), false);
  assert.equal(validPrefillQuery(`?bundle_id=${id}`), true);
  assert.equal(toolsRoute("GET", "inbox/order-prefill"), "order-prefill");
  assert.equal(toolsRoute("POST", "orders/for-buyer"), "for-buyer");
  assert.equal(toolsRoute("GET", "orders/for-buyer"), null);
});
test("A15 closed fields, inactive delivery omitted, no inferred customer or price sums", () => {
  const p = parseOrderPrefill(prefill());
  const form = prefillForm(p, "en", "TWD");
  assert.equal(form.name, "");
  assert.equal(form.optionKey, "");
  assert.equal(form.home.line1, "");
  assert.equal(form.lines[0].quantity, 2);
  assert.equal(
    form.lines[0].price,
    "NT$2",
  );
  const raw: any = prefill();
  raw.items.push({ ...raw.items[0], quantity: 3 });
  assert.equal(
    prefillForm(parseOrderPrefill(raw), "en", "TWD").lines[0].quantity,
    5,
  );
  for (const mutate of [
    (x: any) => (x.extra = 1),
    (x: any) => (x.items[0].quantity = 0),
    (x: any) => (x.items[0].total_minor = 200),
    (x: any) =>
      (x.last_delivery = {
        option_key: optionKey,
        cvs: { store_code: "1", store_name: "Shop", store_address: "Address" },
        home_address: null,
      }),
  ]) {
    const x = prefill();
    mutate(x);
    assert.throws(() => parseOrderPrefill(x));
  }
  const linked: any = prefill();
  linked.customer = {
    name: "Synthetic buyer",
    phone: "0912345678",
    email: null,
  };
  linked.last_delivery = {
    option_key: optionKey,
    home_address: body().delivery.home_address,
  };
  linked.suggested_option_key = optionKey;
  assert.equal(
    prefillForm(parseOrderPrefill(linked), "zh-TW", "TWD").home.line1,
    "Synthetic address",
  );
  const orphan: any = prefill();
  orphan.last_delivery = linked.last_delivery;
  orphan.suggested_option_key = optionKey;
  assert.equal(
    prefillForm(parseOrderPrefill(orphan), "en", "TWD").optionKey,
    "",
  );
  assert.equal(
    parseOrderPrefill({
      ...prefill(),
      live_price_reason: "PRIVATE_UPSTREAM_MESSAGE",
    }).live_price_reason,
    "unknown",
  );
});
test("A16 exact nested body and builder never carries prices or truncates bundles", () => {
  assert.equal(validForBuyerBody(body()), true);
  for (const mutate of [
    (x: any) => (x.items[0].price_minor = 1),
    (x: any) => x.items.push(x.items[0]),
    (x: any) => (x.customer.extra = "secret"),
    (x: any) => delete x.delivery.cvs,
    (x: any) => (x.for.bundle_ids = Array(6).fill(id)),
    (x: any) => (x.for.conversation_id = "bad"),
    (x: any) => (x.send_payment_link = 1),
    (x: any) => (x.delivery.home_address.line1 = ""),
  ]) {
    const x = body();
    mutate(x);
    assert.equal(validForBuyerBody(x), false);
  }
  const p = parseOrderPrefill(prefill());
  const values = prefillForm(p, "en", "TWD");
  Object.assign(values, {
    name: "Synthetic buyer",
    phone: "0912345678",
    optionKey,
    mode: "bank_transfer",
    home: body().delivery.home_address,
  });
  const available: any = [
    {
      option_key: optionKey,
      delivery_kind: "home",
      payment_modes: ["bank_transfer"],
    },
  ];
  const built = makeForBuyerBody(values, available, p, { bundleId: id }, false);
  assert.deepEqual(built, body());
  const many = {
    ...p,
    bundles: Array.from(
      { length: 6 },
      (_, i) => `00000000-0000-4000-8000-${String(i).padStart(12, "0")}`,
    ),
  };
  assert.throws(() =>
    makeForBuyerBody(values, available, many, { conversationId: id }, false),
  );
});
test("A16 replay null link is opt-in; send and price states stay closed", () => {
  assert.equal(parseForBuyerResult(result()).buyer_link, null);
  const { live_price, live_price_reason, send, ...manual } = result();
  assert.throws(() => parseManualResult(manual));
  assert.equal(
    parseForBuyerResult({
      ...result(),
      send: { state: "queued", operation_id: id },
    }).send.state,
    "queued",
  );
  for (const mutate of [
    (x: any) => (x.send.extra = 1),
    (x: any) => (x.send = { state: "SUCCEEDED", operation_id: id }),
    (x: any) => (x.send = { state: "queued", operation_id: "bad" }),
    (x: any) => (x.extra = 1),
    (x: any) => (x.live_price = "maybe"),
  ]) {
    const x = result();
    mutate(x);
    assert.throws(() => parseForBuyerResult(x));
  }
});

test("A16 coded errors close status/code pairs and preserve only the permitted order pointer", () => {
  for (const [status, codes] of Object.entries(forBuyerErrors))
    for (const code of codes) {
      const raw = {
        code,
        message: "PRIVATE_DIAGNOSTIC",
        details: { order_id: id, extra: "PRIVATE_DIAGNOSTIC" },
      };
      const parsed = forBuyerError(Number(status), raw);
      assert.deepEqual(
        parsed,
        code === "bundle_already_ordered" ? { code, order_id: id } : { code },
      );
      assert.equal(
        JSON.stringify(parsed).includes("PRIVATE_DIAGNOSTIC"),
        false,
      );
    }
  assert.equal(
    forBuyerError(200, {
      code: "bundle_already_ordered",
      details: { order_id: id },
    }),
    null,
  );
  assert.equal(
    forBuyerError(409, {
      code: "bundle_already_ordered",
      details: { order_id: "bad" },
    }),
    null,
  );
  assert.equal(forBuyerError(409, { code: "unknown_code" }), null);
});
