// ops-polish independent model/copy gates (docs/delivery/units/ops-polish.md OP3, OP4), no browser, no Docker. Written from the unit brief and the
// BD7 amendment (customers-billing-v1: rows carry pickup_collected_count / pickup_collected_minor, separate from captured and net), not from the code.
import assert from "node:assert/strict";
import { test } from "node:test";
import { parseFinanceSummary } from "../../apps/admin/lib/customers-model.ts";
import { studioCopy } from "../../apps/admin/lib/studio-copy.ts";

const row = (extra: Record<string, unknown> = {}) => ({
  day: "2026-09-30", currency: "TWD", environment: "LIVE", captured_count: 1, captured_minor: 2500, refunded_minor: 500, net_minor: 2000, ...extra,
});
const total = (extra: Record<string, unknown> = {}) => ({ ...row(extra), day: "" });
const doc = (rows: unknown[], totals: unknown[]) => ({ from: "2026-09-01", to: "2026-09-30", timezone: "Asia/Taipei", rows, totals });

test("finance rows with the two pay-at-pickup keys parse and stay separate from captured and net", () => {
  const parsed = parseFinanceSummary(doc([row({ pickup_collected_count: 2, pickup_collected_minor: 5000 })], [total({ pickup_collected_count: 2, pickup_collected_minor: 5000 })]));
  assert.equal(parsed.rows[0].pickup_collected_count, 2);
  assert.equal(parsed.rows[0].pickup_collected_minor, 5000);
  assert.equal(parsed.rows[0].captured_minor, 2500);
  assert.equal(parsed.rows[0].net_minor, 2000, "net is captured minus refunded only");
  assert.equal(parsed.totals[0].pickup_collected_minor, 5000);
});

test("a pickup-only day (no captured money) is a valid row whose captured and net stay zero", () => {
  const only = row({ captured_count: 0, captured_minor: 0, refunded_minor: 0, net_minor: 0, pickup_collected_count: 1, pickup_collected_minor: 2500 });
  const parsed = parseFinanceSummary(doc([only], [{ ...only, day: "" }]));
  assert.equal(parsed.rows[0].net_minor, 0);
  assert.equal(parsed.rows[0].pickup_collected_minor, 2500);
});

test("the older 7-key row (before the column) still parses with zero pickup money", () => {
  const parsed = parseFinanceSummary(doc([row()], [total()]));
  assert.equal(parsed.rows[0].pickup_collected_count, 0);
  assert.equal(parsed.rows[0].pickup_collected_minor, 0);
});

test("a half pair, a negative or a non-integer pickup value is refused, never shown", () => {
  for (const bad of [
    { pickup_collected_count: 1 },
    { pickup_collected_minor: 100 },
    { pickup_collected_count: -1, pickup_collected_minor: 0 },
    { pickup_collected_count: 1, pickup_collected_minor: -5 },
    { pickup_collected_count: 1.5, pickup_collected_minor: 100 },
    { pickup_collected_count: "1", pickup_collected_minor: 100 },
    { pickup_collected_count: 1, pickup_collected_minor: 100, extra_key: 1 },
  ]) assert.throws(() => parseFinanceSummary(doc([row(bad)], [])), /unavailable/, JSON.stringify(bad));
});

test("Studio subtitle is neutral production wording in all three locales", () => {
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    const subtitle = studioCopy[locale].subtitle;
    assert.ok(subtitle.length > 8, `${locale} subtitle is empty`);
    assert.doesNotMatch(subtitle, /MOCK|rehears|local|演练|演練|模拟|模擬|本地|本機|仅供|僅供|Nothing here publishes/i, `${locale}: ${subtitle}`);
  }
  assert.notEqual(studioCopy.en.subtitle, studioCopy["zh-TW"].subtitle);
  assert.notEqual(studioCopy["zh-CN"].subtitle, studioCopy["zh-TW"].subtitle);
});
