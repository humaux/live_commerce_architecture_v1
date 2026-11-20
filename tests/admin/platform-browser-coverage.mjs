// Shared geometry and coverage contract for the platform browser runner and its strict release consumer.
import assert from "node:assert/strict";
import { platformLocales, platformPages } from "../../apps/admin/lib/company.ts";

export { platformLocales, platformPages };
export const platformViewports = [390, 1586];
export const platformMessagingPages = ["privacy", "terms"];

export function platformBrowserCoverage(result, rows) {
  const key = (locale, width, page) => JSON.stringify([locale, width, page]);
  const cases = pages => new Set(platformLocales.flatMap(locale =>
    platformViewports.flatMap(width => pages.map(page => key(locale, width, page)))));
  const expected = cases(platformPages);
  const copy = cases(platformMessagingPages);
  const coverage = { render: new Set(), reload: new Set(), "messaging-copy": new Set() };
  const clicked = new Set();
  let clicks = 0;
  for (const { locale, width, page, action, result: status } of rows) {
    const triple = key(locale, width, page);
    assert.ok(expected.has(triple) && status === "PASS" && typeof action === "string" && action.length,
      "invalid platform browser ledger row");
    if (Object.hasOwn(coverage, action)) {
      assert.ok(!coverage[action].has(triple), "duplicate platform coverage row");
      coverage[action].add(triple);
    } else {
      clicked.add(triple);
      clicks++;
    }
  }
  for (const action of ["render", "reload"]) assert.deepEqual(coverage[action], expected, `missing platform ${action} case`);
  assert.deepEqual(coverage["messaging-copy"], copy, "incomplete platform messaging-copy coverage");
  assert.deepEqual(clicked, expected, "missing platform click case");
  const ssr = platformLocales.length * platformPages.length;
  const totals = { pass: expected.size, fail: 0, clicks, messagingCopyCases: copy.size,
    reloads: expected.size, ssrCases: ssr, tagCases: ssr + 1 };
  assert.deepEqual(result, totals, "platform result counts disagree with the current matrix/ledger");
  return totals;
}
