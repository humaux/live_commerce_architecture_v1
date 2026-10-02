// merchant-tools: strict parsers of the dashboard / import / manual-order answers (apps/admin/lib/merchant-tools-model.ts), the manual-order
// request builder (no price field can exist), the BFF grammar, and copy parity of the three admin locales (lib/merchant-tools-copy.ts).
// Synthetic fixtures only. Run: node --test --experimental-strip-types tests/admin/merchant-tools-model.test.ts
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  csvFileProblem, draftProblem, manualBody, parseDashboard, parseImportResult, parseManualOptions, parseManualResult, parseRegenerateResult, rowErrorCodes, toolsRoute,
  validManualBody, validRegenerateBody, MAX_CSV_BYTES, type ManualDraft, type ManualOption,
} from "../../apps/admin/lib/merchant-tools-model.ts";
import { toolsCopy } from "../../apps/admin/lib/merchant-tools-copy.ts";

const id = "11111111-1111-4111-8111-111111111111";
const market = "22222222-2222-4222-8222-222222222222";
const sku = "33333333-3333-4333-8333-333333333333";
const order = {
  order_id: id, created_at: "2026-10-01T00:00:00.000000Z", updated_at: "2026-10-01T00:00:00.000000Z", currency: "TWD", total_minor: 2500,
  commercial_state: "AWAITING_TRANSFER", fulfillment_state: "MANUAL_UNASSIGNED", payment_state: "NOT_STARTED", test_mode: false, work_state: "NONE",
  refunded_minor: 0, refund_pending_minor: 0, pickup_source: null, payment_mode: "bank_transfer", collection_state: null, cod_surcharge_minor: null, cod_collect_minor: null, source: "merchant_manual",
};
const dashboard = () => ({
  generated_at: "2026-10-01T02:00:00Z", timezone: "Asia/Taipei", orders: { today: 2, last_7_days: 5 },
  gmv: [{ currency: "TWD", environment: "LIVE", today: { card_minor: 0, bank_transfer_minor: 2500, pay_at_pickup_minor: 0, cod_minor: 0 }, last_7_days: { card_minor: 100, bank_transfer_minor: 2500, pay_at_pickup_minor: 30, cod_minor: 0 } }],
  todos: { awaiting_transfer_confirmation: 1, to_ship: 2, cvs_awaiting_label: 1, low_stock_skus: 0, open_refunds: 0 },
  latest_orders: [order],
});

test("dashboard: accepts the frozen shape, rejects drift", () => {
  assert.equal(parseDashboard(dashboard()).orders.last_7_days, 5);
  assert.equal(parseDashboard({ ...dashboard(), gmv: [], latest_orders: [] }).gmv.length, 0);
  for (const mutate of [
    (d: any) => { d.extra = 1; },
    (d: any) => { d.timezone = "UTC"; },
    (d: any) => { d.orders.today = 9; }, // today > 7 days
    (d: any) => { d.orders.today = -1; },
    (d: any) => { d.todos.to_ship = 1.5; },
    (d: any) => { delete d.todos.open_refunds; },
    (d: any) => { d.gmv[0].today.card_minor = 500; }, // today above the 7-day window
    (d: any) => { d.gmv[0].environment = "STAGING"; },
    (d: any) => { d.gmv.push({ ...d.gmv[0] }); }, // duplicate (currency, environment)
    (d: any) => { d.latest_orders[0].order_id = "x"; },
    (d: any) => { d.latest_orders = Array(11).fill(order); },
  ]) {
    const d = dashboard();
    mutate(d);
    assert.throws(() => parseDashboard(d), /unavailable/);
  }
});

const imported = () => ({
  file_sha256: "a".repeat(64), rows: 3, created_products: 2, updated_products: 0, created_skus: 3, updated_skus: 0, stock_adjustments: 2, unchanged_rows: 0,
  errors: [], errors_truncated: false, committed: true, replayed: false,
});

test("import result: strict keys, closed error codes, committed never carries errors", () => {
  assert.equal(parseImportResult(imported()).created_products, 2);
  assert.equal(parseImportResult({ ...imported(), committed: false, errors: [{ row: 2, column: "handle", code: "invalid_value" }] }).errors[0].row, 2);
  assert.equal(parseImportResult({ ...imported(), committed: false, errors: [{ row: 0, column: "", code: "limit" }] }).errors[0].code, "limit");
  for (const mutate of [
    (r: any) => { r.extra = 1; },
    (r: any) => { r.file_sha256 = "zz"; },
    (r: any) => { r.rows = -1; },
    (r: any) => { r.errors = [{ row: 1, column: "x", code: "made_up" }]; r.committed = false; },
    (r: any) => { r.errors = [{ row: 1, column: "x", code: "required" }]; }, // committed with errors
    (r: any) => { r.committed = "yes"; },
    (r: any) => { r.errors = [{ row: 99999, column: "x", code: "required" }]; r.committed = false; },
    (r: any) => { r.errors = Array(201).fill({ row: 1, column: "x", code: "required" }); r.committed = false; },
  ]) {
    const r = imported();
    mutate(r);
    assert.throws(() => parseImportResult(r), /unavailable/);
  }
  assert.ok(rowErrorCodes.includes("sku_in_other_product"));
});

test("csv file guard mirrors the Go bounds", () => {
  assert.equal(csvFileProblem("a.csv", 10), null);
  assert.equal(csvFileProblem("A.CSV", 10), null);
  assert.equal(csvFileProblem("a.xlsx", 10), "not_csv");
  assert.equal(csvFileProblem("a.csv", 0), "empty");
  assert.equal(csvFileProblem("a.csv", MAX_CSV_BYTES + 1), "too_large");
  assert.equal(csvFileProblem("a.csv", MAX_CSV_BYTES), null);
});

const option = (over: Partial<ManualOption> = {}): ManualOption => ({
  option_key: `${market}|TW|home1`, market_id: market, country: "TW", delivery_code: "home1", delivery_kind: "home", mode: "MANUAL",
  name_hans: "快递", name_hant: "宅配", name_en: "Home", currency: "TWD", service_version: 1, allocation_version: 1,
  payment_modes: ["bank_transfer"], pickup_selection: null, ...over,
});

test("manual options: only bank_transfer / pay_at_pickup, key must equal market|country|code", () => {
  assert.equal(parseManualOptions({ options: [option()] })[0].delivery_kind, "home");
  assert.equal(parseManualOptions({ options: [] }).length, 0);
  for (const bad of [
    { ...option(), payment_modes: ["card"] },
    { ...option(), payment_modes: [] },
    { ...option(), option_key: `${market}|TW|other` },
    { ...option(), delivery_kind: "pigeon" },
    { ...option(), pickup_selection: "map" },
    { ...option(), extra: 1 },
  ]) assert.throws(() => parseManualOptions({ options: [bad] }), /unavailable/);
});

const result = (over: Record<string, unknown> = {}) => ({
  order_id: id, commercial_state: "AWAITING_TRANSFER", payment_mode: "bank_transfer", total_minor: 2500, currency: "TWD", expires_at: "2026-10-04T00:00:00Z",
  buyer_link: `https://shop.example.test/zh-TW/order-link#o=${id}&t=${"A".repeat(43)}`, link_state: "configured", source: "merchant_manual", ...over,
});

test("manual result: link is https with the order id and a 43-char capability in the fragment only", () => {
  assert.equal(parseManualResult(result()).order_id, id);
  assert.equal(parseManualResult(result({ buyer_link: null, link_state: "storefront_unavailable" })).buyer_link, null);
  assert.equal(parseManualResult(result({ commercial_state: "CONFIRMED", payment_mode: "pay_at_pickup" })).commercial_state, "CONFIRMED");
  for (const bad of [
    result({ buyer_link: `http://shop.example.test/x#o=${id}&t=${"A".repeat(43)}` }),
    result({ buyer_link: `https://shop.example.test/x?t=${"A".repeat(43)}#o=${id}&t=${"A".repeat(43)}` }),
    result({ buyer_link: `https://shop.example.test/x#o=${id}&t=short` }),
    result({ buyer_link: `https://shop.example.test/x#o=${market}&t=${"A".repeat(43)}` }),
    result({ buyer_link: "https://shop.example.test/x#o=" + id + "&t=" + "A".repeat(43), link_state: "storefront_unavailable" }),
    result({ link_state: "configured", buyer_link: null }),
    result({ source: "storefront" }),
    result({ commercial_state: "CONFIRMED" }), // CONFIRMED at placement only for pay_at_pickup
    result({ payment_mode: "card" }),
  ]) assert.throws(() => parseManualResult(bad), /unavailable/);
});

const draft = (over: Partial<ManualDraft> = {}): ManualDraft => ({
  lines: [{ sku_id: sku, quantity: 2 }], name: "Wang", phone: "0912-345-678", email: "", option: option(), mode: "bank_transfer",
  home: { region: "", city: "Taipei", postal_code: "", line1: "1 Road", line2: "" }, cvs: { store_code: "", store_name: "", store_address: "" }, locale: "zh-TW", ...over,
});

test("draft: first problem, and the body has no price-like field at all", () => {
  assert.equal(draftProblem(draft()), null);
  assert.equal(draftProblem(draft({ lines: [] })), "lines");
  assert.equal(draftProblem(draft({ lines: [{ sku_id: sku, quantity: 0 }] })), "lines");
  assert.equal(draftProblem(draft({ lines: [{ sku_id: sku, quantity: 1 }, { sku_id: sku, quantity: 1 }] })), "lines");
  assert.equal(draftProblem(draft({ name: "  " })), "name");
  assert.equal(draftProblem(draft({ phone: "123" })), "phone");
  assert.equal(draftProblem(draft({ email: "nope" })), "email");
  assert.equal(draftProblem(draft({ option: null })), "delivery");
  assert.equal(draftProblem(draft({ mode: "pay_at_pickup" })), "mode");
  assert.equal(draftProblem(draft({ home: { region: "", city: "", postal_code: "", line1: "x", line2: "" } })), "address");
  const cvs = option({ delivery_kind: "cvs_711", payment_modes: ["bank_transfer", "pay_at_pickup"], pickup_selection: "buyer_entered" });
  assert.equal(draftProblem(draft({ option: cvs })), "store");
  assert.equal(draftProblem(draft({ option: cvs, cvs: { store_code: "123456", store_name: "S", store_address: "Taipei City" }, mode: "pay_at_pickup" })), null);
  const body = manualBody(draft());
  assert.ok(validManualBody(body));
  assert.equal((body.delivery as any).cvs, null);
  assert.equal(JSON.stringify(body).includes("price"), false);
  assert.equal(validManualBody({ ...body, total_minor: 1 }), false);
  assert.equal(validManualBody({ ...body, items: [{ sku_id: sku, quantity: 1, unit_price_minor: 1 }] }), false);
  assert.equal(validManualBody({ ...body, tenant_id: id }), false);
  assert.equal(validManualBody([]), false);
  assert.equal(validManualBody({ ...body, delivery: { ...(body.delivery as object), shipping_fee: 0 } }), false);
});

test("BFF grammar: exactly the seven tools resources", () => {
  assert.equal(toolsRoute("GET", "dashboard"), "dashboard");
  assert.equal(toolsRoute("GET", "products/export.csv"), "export");
  assert.equal(toolsRoute("GET", "orders/manual/options"), "manual-options");
  assert.equal(toolsRoute("POST", "products/import/preview"), "import-preview");
  assert.equal(toolsRoute("POST", "products/import/commit"), "import-commit");
  assert.equal(toolsRoute("POST", "orders/manual"), "manual-place");
  assert.equal(toolsRoute("POST", "orders/manual/regenerate-link"), "manual-regenerate");
  for (const [method, path] of [["POST", "dashboard"], ["GET", "orders/manual"], ["PUT", "orders/manual"], ["GET", "products"], ["GET", "dashboard/x"], ["DELETE", "dashboard"], ["GET", "../dashboard"], ["GET", "orders/manual/regenerate-link"], ["PUT", "orders/manual/regenerate-link"]])
    assert.equal(toolsRoute(method, path), null, `${method} ${path}`);
});

test("regenerate body/result: exactly {order_id, locale} and a fresh https link naming that order", () => {
  const body = { order_id: id, locale: "zh-TW" };
  assert.ok(validRegenerateBody(body));
  assert.equal(validRegenerateBody({ ...body, locale: "fr" }), false);
  assert.equal(validRegenerateBody({ ...body, order_id: "nope" }), false);
  assert.equal(validRegenerateBody({ ...body, extra: 1 }), false);
  assert.equal(validRegenerateBody({ order_id: id }), false);
  assert.equal(validRegenerateBody([]), false);
  const result = parseRegenerateResult({ order_id: id, buyer_link: `https://shop.example.test/zh-TW/order-link#o=${id}&t=${"A".repeat(43)}`, source: "merchant_manual" });
  assert.equal(result.order_id, id);
  assert.equal(result.source, "merchant_manual");
  assert.throws(() => parseRegenerateResult({ order_id: id, buyer_link: `https://shop.example.test/zh-TW/order-link#o=${id}&t=${"A".repeat(43)}`, source: "storefront" }));
  assert.throws(() => parseRegenerateResult({ order_id: id, buyer_link: `https://shop.example.test/zh-TW/order-link#o=${id}&t=short`, source: "merchant_manual" }));
});

test("copy parity: every locale has every key, import rules and error codes", () => {
  const shape = (value: unknown, path = ""): string[] =>
    value && typeof value === "object" && !Array.isArray(value)
      ? Object.entries(value).flatMap(([key, v]) => shape(v, `${path}.${key}`))
      : [path + (Array.isArray(value) ? `[${value.length}]` : "")];
  const reference = shape(toolsCopy.en).sort();
  for (const locale of ["zh-CN", "zh-TW"] as const) assert.deepEqual(shape(toolsCopy[locale]).sort(), reference, locale);
  for (const locale of ["zh-CN", "zh-TW", "en"] as const) {
    for (const code of rowErrorCodes) assert.ok(toolsCopy[locale].importer.codes[code], `${locale} import code ${code}`);
    for (const code of ["invalid_request", "conflict", "insufficient_inventory", "idempotency_conflict", "bank_transfer_unavailable", "pay_at_pickup_unavailable", "pay_at_pickup_amount_exceeds", "cvs_entry_unavailable", "default"])
      assert.ok(toolsCopy[locale].manual.errors[code], `${locale} manual error ${code}`);
    for (const kind of ["home", "cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"]) assert.ok(toolsCopy[locale].manual.kinds[kind], `${locale} kind ${kind}`);
  }
  // The 375 px layout relies on short nav labels.
  for (const locale of ["zh-CN", "zh-TW", "en"] as const) assert.ok(toolsCopy[locale].navDashboard.length <= 12);
});
