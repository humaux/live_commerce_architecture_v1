// stop-bleed D02 + M06 (UI architecture v2 §10.7, REPORT-admin-vqa): the admin's ONE money rule and ONE time rule. Pure logic, no browser.
//   money: every merchant input is MAJOR units; TWD takes whole dollars only (x100 on the wire, "60" -> 6000, "60.5" refused); every display
//   drops the zero decimals of a whole amount ("NT$60", never "NT$60.00" or a bare "$60.00"); no copy says "minor units" any more.
//   time: the admin shows Asia/Taipei and labels it, never UTC.
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { test } from "node:test";
import { currencySign, money } from "../../apps/admin/lib/client.ts";
import { fromMinor, toMinor } from "../../apps/admin/lib/catalog-v2-model.ts";
import { parseLivePrice } from "../../apps/admin/lib/claims-model.ts";
import { displayTime, STORE_TIME_ZONE } from "../../apps/admin/lib/orders-model.ts";
import { createBody, emptyForm, instantToTaipei } from "../../apps/admin/lib/promotions-model.ts";

test("D02 TWD input: whole dollars only, x100 on the wire", () => {
  assert.equal(toMinor("60", "TWD"), 6000);
  assert.equal(toMinor("0", "TWD"), 0);
  assert.equal(toMinor(" 1200 ", "TWD"), 120000);
  for (const bad of ["60.5", "60.50", "60.00", "0.5", "60.", ".5", "-1", "1e3", "", "abc"]) assert.equal(toMinor(bad, "TWD"), null, `TWD ${bad}`);
  assert.equal(toMinor("12.5", "USD"), 1250); // other currencies keep their own decimals
  assert.equal(toMinor("12.345", "USD"), null);
  assert.equal(toMinor("500", "JPY"), 500);
  assert.equal(toMinor("1.5", "JPY"), null);
  assert.equal(toMinor("10000000000", "TWD"), 1_000_000_000_000); // the upper bound the Go routes take (1e12 minor)
  assert.equal(toMinor("10000000001", "TWD"), null);
});

test("D02 TWD edit text: whole dollars have no decimals; a legacy amount with cents is never shown as another price", () => {
  assert.equal(fromMinor(6000, "TWD"), "60");
  assert.equal(fromMinor(0, "TWD"), "0");
  assert.equal(fromMinor(50, "TWD"), "0.50"); // the owner's NT$0.50 pilot price: shown exactly, refused by toMinor until retyped whole
  assert.equal(toMinor(fromMinor(50, "TWD"), "TWD"), null);
  assert.equal(fromMinor(1250, "USD"), "12.50");
  assert.equal(fromMinor(500, "JPY"), "500");
  for (const minor of [0, 100, 6000, 123400, 1_000_000_000_000]) assert.equal(toMinor(fromMinor(minor, "TWD"), "TWD"), minor);
});

test("D02 display: NT$ without decimals in every language, cents only when there are cents", () => {
  for (const locale of ["zh-CN", "zh-TW", "en"]) {
    assert.equal(money(locale, "TWD", 6000), "NT$60", locale); // zh-TW used to print "$60.00", the others "NT$60.00"
    assert.equal(money(locale, "TWD", 0), "NT$0", locale);
    assert.equal(money(locale, "TWD", 123456000), "NT$1,234,560", locale);
    assert.equal(money(locale, "TWD", 50), "NT$0.50", locale); // legacy cents are shown, never rounded to NT$1
    assert.doesNotMatch(money(locale, "TWD", 6000), /\.00/, locale);
  }
  assert.equal(money("en", "USD", 1250), "$12.50");
  assert.equal(money("en", "USD", 1200), "$12");
  assert.equal(currencySign("TWD"), "NT$");
  assert.equal(currencySign("USD"), "USD");
});

test("D02 live-price input (Studio claims): TWD whole dollars, other currencies their decimals", () => {
  assert.equal(parseLivePrice("60", "TWD"), 6000);
  assert.equal(parseLivePrice("60.5", "TWD"), null);
  assert.equal(parseLivePrice("0", "TWD"), null); // a live price must be positive
  assert.equal(parseLivePrice("12.5", "USD"), 1250);
});

test("D02 discount code amounts: TWD whole dollars, a decimal is refused before it is sent", () => {
  const fixed = { ...emptyForm, code: "TEN-OFF", kind: "fixed" as const, value: "100" };
  assert.equal(createBody(fixed, "TWD")?.fixed_minor, 10000);
  assert.equal(createBody({ ...fixed, value: "100.5" }, "TWD"), undefined);
  assert.equal(createBody({ ...fixed, minSubtotal: "500.25" }, "TWD"), undefined);
  assert.equal(createBody({ ...fixed, value: "100.5" }, "USD")?.fixed_minor, 10050);
});

test("D02 no admin copy or component says 'minor units' to the merchant", () => {
  const root = new URL("../../apps/admin/", import.meta.url);
  const forbidden = /最小货币单位|最小貨幣單位|minor units?\b|smallest currency/i;
  const bad: string[] = [];
  for (const dir of ["lib", "components"])
    for (const file of readdirSync(new URL(`${dir}/`, root)).filter((name) => /\.(ts|tsx)$/.test(name))) {
      const text = readFileSync(new URL(`${dir}/${file}`, root), "utf8");
      // comments may explain the wire (price_minor); only user-facing strings count
      for (const line of text.split("\n")) if (!/^\s*(\/\/|\*|\/\*)/.test(line) && forbidden.test(line.replace(/\/\/.*$/, ""))) bad.push(`${dir}/${file}: ${line.trim().slice(0, 90)}`);
    }
  assert.deepEqual(bad, []);
});

test("M06 time: Asia/Taipei everywhere, labelled, never UTC", () => {
  assert.equal(STORE_TIME_ZONE, "Asia/Taipei");
  // 2026-10-01T16:30Z is 2026-10-02 00:30 in Taipei: the day rolls over, which is exactly what a UTC display got wrong
  assert.match(displayTime("en", "2026-10-01T16:30:00Z"), /10\/02\/2026\D+00:30/);
  assert.match(displayTime("zh-TW", "2026-10-01T16:30:00Z"), /2026\/10\/02\D+00:30/);
  assert.match(displayTime("zh-CN", "2026-10-01T16:30:00+00:00"), /2026\/10\/02\D+00:30/);
  assert.equal(instantToTaipei("2026-10-01T16:30:00Z"), "2026-10-02T00:30"); // the helper the Studio schedule field shares with promotions
  const bad: string[] = [];
  for (const file of readdirSync(new URL("../../apps/admin/lib/", import.meta.url)).filter((name) => name.endsWith("-copy.ts"))) {
    const text = readFileSync(new URL(`../../apps/admin/lib/${file}`, import.meta.url), "utf8");
    // "(UTC+8)" is a statement about the calendar day, not a displayed time; every other UTC label is a stale display claim
    for (const line of text.split("\n")) if (/UTC(?!\+8)/.test(line)) bad.push(`${file}: ${line.trim().slice(0, 90)}`);
  }
  assert.deepEqual(bad, []);
});
