// Purpose: render actual storefront producers at a Taipei midnight and year boundary.
// Depends on: Next SWC for TSX loading and the real @live-commerce/format implementation.
// Used by: storefront node tests; React hooks are presentation fixtures, not format stubs.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import * as format from "../../../packages/format/src/index.ts";
import { historyCopy } from "../lib/history-copy.ts";
import { orderCopy } from "../lib/order-copy.ts";
import { claimCopy } from "../lib/claim-copy.ts";
import { bankTransferCopy } from "../lib/bank-transfer-copy.ts";
import { browseCopy } from "../lib/browse-copy.ts";
import { shopCopy } from "../lib/shop-copy.ts";
import * as bankTransferContract from "../lib/bank-transfer-contract.ts";

const require = createRequire(import.meta.url);
const swc = require("next/dist/build/swc");
const instant = "2026-12-31T16:30:00Z"; // 2027-01-01 00:30 Asia/Taipei.
const fakeDate = class extends Date { constructor(...args) { super(...(args.length ? args : [instant])); } static now() { return Date.parse(instant); } };
const jsx = (type, props) => ({ type, props });
const react = (states) => {
  let i = 0;
  return { useState: (initial) => [i < states.length ? states[i++] : (typeof initial === "function" ? initial() : initial), () => {}], useRef: (value) => ({ current: value }), useEffect() {}, useCallback: (fn) => fn };
};
function load(name, states, modules = {}) {
  const source = readFileSync(new URL(`../components/${name}.tsx`, import.meta.url), "utf8");
  const code = swc.transformSync(source, { filename: `${name}.tsx`, jsc: { parser: { syntax: "typescript", tsx: true }, target: "es2022", transform: { react: { runtime: "automatic" } } }, module: { type: "commonjs" } }).code;
  const exports = {};
  runInNewContext(code, { exports, Date: fakeDate, window: {}, require: (id) => {
    if (id === "react") return react(states);
    if (id === "react/jsx-runtime") return { jsx, jsxs: jsx, Fragment: "fragment" };
    if (id === "../../../packages/format/src/index") return format;
    assert.ok(Object.hasOwn(modules, id), `${name}: unlisted dependency ${id}`);
    return modules[id];
  } });
  return exports;
}
function text(node) {
  if (Array.isArray(node)) return node.map(text).join(" ");
  if (node == null || typeof node === "boolean") return "";
  if (typeof node !== "object") return String(node);
  return text(node.props?.children);
}
const fmt = format.displayTime("en", instant);

test("OrderHistory renders server order instant in Taipei", () => {
  const order = { order_id: "order-1", created_at: instant, commercial_state: "CONFIRMED", total_minor: 100, currency: "TWD" };
  const component = load("OrderHistory", [[order], "", null, false, false, false], {
    "../lib/history-copy": { historyCopy }, "../lib/order-copy": { orderCopy },
    "../lib/purchase": {}, "./OrderFlow": { OrderDetails() {} }, "../lib/buyer-client": { BuyerClientError: Error },
  }).default;
  const result = component({ context: "test", locale: "en", money: () => "NT$1", onError() {}, onPaymentBusy() {} });
  assert.match(text(result), /01\/01\/2027|1\/1\/2027|Jan 01, 2027|Jan 1, 2027/);
  assert.ok(text(result).includes(fmt), `expected shared formatter output ${fmt}`);
});

test("ClaimLink renders claim expiry in Taipei", () => {
  const preview = { expires_at: instant, lines: [] };
  const component = load("ClaimLink", ["en", "ready", "context", preview, { items: [] }, false, null, false], {
    "../lib/buyer-client": { BuyerClientError: Error }, "../lib/purchase": { knownOrderID: () => null, pendingPurchase: () => null },
    "../lib/routes": {}, "../lib/claim-contract": {}, "../lib/claim-copy": { claimCopy }, "../lib/money": { formatMoney: () => "NT$1" },
  }).default;
  const shown = text(component({ locale: "en" }));
  assert.match(shown, /01\/01\/2027|1\/1\/2027/);
  assert.match(shown, /(?:00|24):30/);
  assert.ok(shown.includes(fmt));
});

test("BankTransfer renders deadline and proof instant in Taipei", () => {
  const view = { state: "SUBMITTED", deadline_at: instant, currency: "TWD", amount_minor: 100, bank: null, proof: { last5: "12345", amount_minor: 100, paid_at: instant } };
  const component = load("BankTransfer", [view, false, Date.parse(instant), "", "", "", false, "", null], {
    "../lib/buyer-client": { BuyerClientError: Error }, "../lib/purchase": {}, "../lib/bank-transfer-copy": { bankTransferCopy },
    "../lib/browse-copy": { browseCopy }, "../lib/bank-transfer-contract": bankTransferContract,
  }).default;
  const shown = text(component({ context: "test", orderID: "order-1", locale: "en", money: () => "NT$1", refreshToken: 0 }));
  assert.match(shown, /01\/01\/2027|1\/1\/2027/);
  assert.match(shown, /(?:00|24):30/);
  assert.equal(shown.split(fmt).length - 1, 2);
});

test("ShopFooter uses Taipei year across UTC year boundary", () => {
  const stub = () => null;
  const component = load("ShopChrome", [], {
    "next/link": { default: stub }, "../lib/design": { resolveNav: () => [], safeExternal: () => null },
    "../lib/legal-copy": { legalFooterLinks: () => [] }, "../lib/routes": { storeImage: () => "" },
    "../lib/shop-copy": { shopCopy }, "./CartDrawer": { HeaderCart: stub },
    "./icons": Object.fromEntries(["ChatIcon", "FacebookIcon", "InstagramIcon", "MailIcon", "PhoneIcon", "PinIcon", "SearchIcon"].map((name) => [name, stub])),
    "./LocaleSwitch": { default: stub }, "./MobileMenu": { default: stub },
  }).ShopFooter;
  const design = { profile: { name: "Shop", contact: {} }, nav: { footer: [] } };
  assert.match(text(component({ locale: "en", design, preview: null })), /©\s+2027\s+Shop/);
});
