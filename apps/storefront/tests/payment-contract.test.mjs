import test from "node:test";
import assert from "node:assert/strict";
import {
  validOrderPayment,
  validPaymentPrepared,
  validHostedHandoff,
} from "../lib/payment-contract.ts";

const orderID = "12345678-1234-1234-1234-123456789abc";
const expiry = "2026-09-25T01:02:03.123456789Z";
const method = {
  code: "payuni_credit",
  version: 1,
  name_hans: "测试",
  name_hant: "測試",
  name_en: "Mock",
};
const view = {
  order_id: orderID,
  currency: "TWD",
  total_minor: 2500,
  commercial_state: "DRAFT",
  payment_state: "NOT_STARTED",
  handoff_state: "NONE",
  handoff_expires_at: null,
  test_mode: true,
  methods: [method],
};
const prepared = {
  order_id: orderID,
  state: "PAYMENT_PENDING",
  currency: "TWD",
  amount_minor: 2500,
};
const form = {
  action: "https://sandbox-api.payuni.com.tw/api/upp",
  fields: {
    Version: "2.0",
    MerID: "synthetic_merchant",
    EncryptInfo: "ab".repeat(8),
    HashInfo: "A".repeat(64),
  },
};
const handoff = {
  order_id: orderID,
  disposition: "ISSUED",
  expires_at: expiry,
  form,
};

test("BPT03 exact valid projection and Go nanosecond UTC timestamps", () => {
  assert.equal(validOrderPayment(view, orderID), true);
  assert.equal(validPaymentPrepared(prepared, orderID), true);
  assert.equal(validHostedHandoff(handoff, orderID), true);
  assert.equal(
    validHostedHandoff(
      { order_id: orderID, disposition: "ALREADY_ISSUED", expires_at: expiry },
      orderID,
    ),
    true,
  );
  for (const state of ["PENDING", "AUTHORIZED", "CAPTURED", "REVIEW_REQUIRED"])
    assert.equal(
      validOrderPayment({ ...view, payment_state: state, methods: [] }, orderID),
      true,
    );
  assert.equal(
    validOrderPayment(
      { ...view, payment_state: "PENDING", handoff_state: "UNAVAILABLE", handoff_expires_at: null, methods: [] },
      orderID,
    ),
    true,
  );
});

test("BPT03 view rejects private fields, inconsistent state and malformed names", () => {
  for (const [name, mutation] of [
    ["private attempt", { attempt_id: orderID }],
    ["wrong order", { order_id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" }],
    ["currency", { currency: "twD" }],
    ["negative amount", { total_minor: -1 }],
    ["unsafe amount", { total_minor: Number.MAX_SAFE_INTEGER }],
    ["state", { payment_state: "PAID" }],
    ["handoff state", { handoff_state: "SUCCESS" }],
    ["missing page timestamp", { handoff_state: "PREPARED" }],
    ["invented timestamp", { handoff_expires_at: expiry }],
    ["invalid calendar date", { handoff_state: "PREPARED", handoff_expires_at: "2026-02-30T00:00:00Z" }],
    ["local timestamp", { handoff_state: "PREPARED", handoff_expires_at: "2026-09-25T01:02:03+08:00" }],
    ["methods after pending", { payment_state: "PENDING" }],
    ["two methods", { methods: [method, method] }],
    ["method private field", { methods: [{ ...method, connection_id: orderID }] }],
    ["method code", { methods: [{ ...method, code: "payuni_atm" }] }],
    ["method version", { methods: [{ ...method, version: 0 }] }],
    ["blank name", { methods: [{ ...method, name_en: "  \t " }] }],
    ["control name", { methods: [{ ...method, name_en: "Mock\n" }] }],
    ["121 Unicode points", { methods: [{ ...method, name_en: "😀".repeat(121) }] }],
  ]) {
    const candidate = name === "private attempt" ? { ...view, ...mutation } : { ...view, ...mutation };
    assert.equal(validOrderPayment(candidate, orderID), false, name);
  }
  assert.equal(
    validOrderPayment({ ...view, methods: [{ ...method, name_en: "😀".repeat(120) }] }, orderID),
    true,
    "120 astral Unicode code points must not be counted as 240 UTF-16 units",
  );
});

test("BPT03 prepared receipt enforces exact original credit-card amount", () => {
  for (const candidate of [
    { ...prepared, attempt_id: orderID },
    { ...prepared, order_id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" },
    { ...prepared, state: "CAPTURED" },
    { ...prepared, currency: "USD" },
    { ...prepared, amount_minor: 0 },
    { ...prepared, amount_minor: 2501 },
    { ...prepared, amount_minor: 20000000 },
    { ...prepared, amount_minor: 1.5 },
  ]) assert.equal(validPaymentPrepared(candidate, orderID), false);
  assert.equal(validPaymentPrepared({ ...prepared, amount_minor: 100 }, orderID), true);
  assert.equal(validPaymentPrepared({ ...prepared, amount_minor: 19999900 }, orderID), true);
});

test("BPT03 hosted form and replay are exact, bounded and action-allowlisted", () => {
  for (const [name, candidate] of [
    ["wrong order", { ...handoff, order_id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" }],
    ["bad disposition", { ...handoff, disposition: "PENDING" }],
    ["bad calendar", { ...handoff, expires_at: "2026-02-30T00:00:00Z" }],
    ["evil action", { ...handoff, form: { ...form, action: "https://evil.example/api/upp" } }],
    ["prefix action", { ...handoff, form: { ...form, action: form.action + "/evil" } }],
    ["form extra", { ...handoff, form: { ...form, target: "_blank" } }],
    ["field extra", { ...handoff, form: { ...form, fields: { ...form.fields, Credential: "secret" } } }],
    ["bad merid", { ...handoff, form: { ...form, fields: { ...form.fields, MerID: "bad id" } } }],
    ["short cipher", { ...handoff, form: { ...form, fields: { ...form.fields, EncryptInfo: "ab".repeat(7) } } }],
    ["odd cipher", { ...handoff, form: { ...form, fields: { ...form.fields, EncryptInfo: "a".repeat(17) } } }],
    ["uppercase cipher", { ...handoff, form: { ...form, fields: { ...form.fields, EncryptInfo: "AB".repeat(8) } } }],
    ["long cipher", { ...handoff, form: { ...form, fields: { ...form.fields, EncryptInfo: "ab".repeat(12289) } } }],
    ["lowercase hash", { ...handoff, form: { ...form, fields: { ...form.fields, HashInfo: "a".repeat(64) } } }],
    ["short hash", { ...handoff, form: { ...form, fields: { ...form.fields, HashInfo: "A".repeat(63) } } }],
    ["replay form", { ...handoff, disposition: "ALREADY_ISSUED" }],
    ["missing issued form", { order_id: orderID, disposition: "ISSUED", expires_at: expiry }],
  ]) assert.equal(validHostedHandoff(candidate, orderID), false, name);
});
