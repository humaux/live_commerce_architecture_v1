// Money display of the storefront (display only; Go decides every amount). One formatter, lib/money.ts formatMoney, serves the shop
// window, the cart, the checkout bar and quotation, the order screens and the claim page, so one amount never reads "TWD 2,560" on a
// line and "TWD 2,560.00" on the total (visual QA finding 4). The source guard fails if a component builds its own currency formatter again.
import test from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { formatMoney } from "../lib/money.ts";

const nb = (s) => s.replace(/ /g, " ");

test("TWD whole amounts show no decimals in every locale and at every magnitude (line, subtotal, total, zero)", () => {
  for (const [locale, prefix] of [["zh-TW", "TWD "], ["zh-CN", "NT$"], ["en", "NT$"]]) {
    assert.equal(nb(formatMoney(locale, 256000, "TWD")), `${prefix}2,560`);
    assert.equal(nb(formatMoney(locale, 128000, "TWD")), `${prefix}1,280`);
    assert.equal(nb(formatMoney(locale, 0, "TWD")), `${prefix}0`);
    for (const minor of [0, 100, 8000, 68000, 256000, 144000, 100000000]) assert(!/\.\d/.test(formatMoney(locale, minor, "TWD")), `${locale} ${minor} must not carry decimals`);
  }
});

test("cents appear only when there are cents, then always as two digits; the minor-unit arithmetic is untouched", () => {
  assert.equal(nb(formatMoney("zh-TW", 1250, "TWD")), "TWD 12.50");
  assert.equal(nb(formatMoney("zh-TW", 1205, "TWD")), "TWD 12.05");
  // The caller multiplies in minor units; the formatter only renders. 2 x 1280.00 and 1280.50 + 1280.50 agree with the line total.
  const unit = 128000, qty = 2;
  assert.equal(formatMoney("zh-TW", unit * qty, "TWD"), formatMoney("zh-TW", 256000, "TWD"));
  assert.equal(nb(formatMoney("zh-TW", 128050 * 2, "TWD")), "TWD 2,561");
});

test("digits come from the currency: JPY none, USD two when there are cents", () => {
  assert.equal(nb(formatMoney("en", 500, "JPY")), "¥500");
  assert.equal(nb(formatMoney("en", 1999, "USD")), "$19.99");
  assert.equal(nb(formatMoney("en", 2000, "USD")), "$20");
});

test("one formatter: no component, page or lib builds a currency Intl.NumberFormat outside lib/money.ts", () => {
  const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
  const offenders = [];
  const walk = (dir) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory()) { if (entry.name !== "node_modules" && entry.name !== ".next") walk(file); continue; }
      if (!/\.(tsx?|mjs)$/.test(entry.name) || file.endsWith(path.join("lib", "money.ts"))) continue;
      if (/style:\s*["']currency["']/.test(readFileSync(file, "utf8"))) offenders.push(path.relative(root, file));
    }
  };
  for (const dir of ["app", "components", "lib"]) walk(path.join(root, dir));
  assert.deepEqual(offenders, [], "route these through formatMoney(locale, minor, currency) from lib/money.ts");
});
