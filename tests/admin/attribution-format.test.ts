// Pure display tests only: paid amounts and ratios still come from the server report.
import assert from "node:assert/strict";
import { test } from "node:test";
import { formatROAS } from "../../apps/admin/lib/attribution-format.ts";

for (const [locale, unknown] of [
  ["en", "Not provided"],
  ["zh-TW", "未提供"],
  ["zh-CN", "未提供"],
] as const) {
  test(`R10 ROAS has exactly two decimals in ${locale}`, () => {
    assert.equal(formatROAS(locale, 2.5, unknown), "2.50×");
    assert.equal(formatROAS(locale, 0, unknown), "0.00×");
    assert.equal(formatROAS(locale, 1900 / 1730, unknown), "1.10×");
    assert.equal(formatROAS(locale, 1.234, unknown), "1.23×");
    assert.equal(formatROAS(locale, 1.236, unknown), "1.24×");
    assert.equal(formatROAS(locale, 1234.5, unknown), "1,234.50×");
  });
  test(`R10 unknown ROAS is never fabricated as zero in ${locale}`, () => {
    for (const value of [null, NaN, Infinity, -Infinity]) {
      assert.equal(formatROAS(locale, value, unknown), unknown);
    }
  });
}
