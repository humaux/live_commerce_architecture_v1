// Purpose: DS-STATES-V2 real-click acceptance of every StatePanel/InlineError action and the denial copy boundary.
// Depends on: Playwright Page and the existing gallery runner's step ledger; three-locale recovery copy.
// Used by: ui-gallery-clicks.mjs, under the existing --browser-ui-gallery CI gate.
// Invariants: all interactions are clicks; evaluate is measurement only; synthetic reads target one operation, no writes.
import assert from "node:assert/strict";
import { stateRecoveryCopy } from "../../packages/i18n/src/state-recovery.ts";
import { galleryCopy } from "../../apps/admin/src/ui-gallery-copy.ts";

export async function recoveryClicks({ page, step }) {
  const origin = new URL(page.url()).origin;
  const requests = [];
  const writes = [];
  const observe = request => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method())) writes.push({ method: request.method(), path: new URL(request.url()).pathname });
  };
  page.on("request", observe);
  // FIXTURE: a synthetic HTTP status/list read. No production endpoint or API write is called.
  const pattern = "**/api/__gallery/recovery?**";
  const handler = async route => {
    const request = route.request();
    const url = new URL(request.url());
    const kind = url.searchParams.get("kind");
    requests.push({ method: request.method(), operation: url.searchParams.get("operation"), kind });
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ operation: "ds-states-example", status: kind === "unknown" ? "pending" : "loaded" }) });
  };
  await page.route(pattern, handler);
  const fixture = kind => page.locator(`[data-recovery="${kind}"]`);
  const visible = async (scope, text) => {
    await scope.getByText(text, { exact: true }).waitFor({ state: "visible" });
  };
  const target = async control => {
    const box = await control.boundingBox(); // [READ/MEASURE] rendered action's actual hit rectangle.
    assert.ok(box && box.width >= 44 && box.height >= 44, `44px target, got ${JSON.stringify(box)}`);
  };
  try {
    for (const locale of ["zh-TW", "zh-CN", "en"]) for (const width of [1440, 390]) {
      const c = stateRecoveryCopy[locale];
      await page.setViewportSize({ width, height: 900 });
      const home = `${origin}/${locale}/ui-gallery`;
      await page.goto(home, { waitUntil: "load" });
      await page.locator('[data-recovery-ready="true"]').waitFor();
      await step(`DS-STATES ${locale}/${width} forbidden copy`, "read rendered text", "localized denial; no resource identifier or raw diagnostic", async () => {
        const text = await fixture("forbidden").innerText();
        for (const value of [c.forbidden, c.forbiddenBody, c.forbiddenNote, c.back]) assert.ok(text.includes(value));
        assert.doesNotMatch(text, /private-order-123|Access denied to/);
      });
      for (const kind of ["offline", "unknown", "inline-callback"]) {
        await step(`DS-STATES ${locale}/${width} ${kind}`, "click recovery, then reload", "one GET on the same operation; visible result survives reload", async () => {
          const before = requests.length;
          const control = fixture(kind).getByRole("button", { name: kind === "unknown" ? c.check : c.retry, exact: true });
          assert.equal(await fixture(kind).locator("button, a").count(), 1);
          await target(control);
          await Promise.all([
            page.waitForResponse(response => response.url().includes("/api/__gallery/recovery?") && new URL(response.url()).searchParams.get("kind") === (kind === "inline-callback" ? "inline" : kind)),
            control.click(),
          ]);
          await visible(fixture(kind), kind === "unknown" ? c.checked : c.loaded);
          assert.deepEqual(requests.slice(before), [{ method: "GET", operation: "ds-states-example", kind: kind === "inline-callback" ? "inline" : kind }]);
          await page.reload({ waitUntil: "load" });
          await page.locator('[data-recovery-ready="true"]').waitFor();
          await visible(fixture(kind), kind === "unknown" ? c.checked : c.loaded);
          assert.equal(requests.length, before + 1, "refresh does not replay the callback");
          assert.deepEqual(writes, [], "recovery must never issue a write request");
          if (kind === "unknown") {
            await visible(fixture(kind), c.unknown);
            await visible(fixture(kind), c.unknownBody);
            assert.equal(await fixture(kind).locator('[role="status"][aria-live="polite"]').count(), 1);
          }
        });
      }
      for (const kind of ["empty", "forbidden", "expired", "inline-link"]) {
        await step(`DS-STATES ${locale}/${width} ${kind}`, "click primary link, then reload", "destination tab and URL persist", async () => {
          const control = fixture(kind).getByRole("link", { name: c.back, exact: true });
          assert.equal(await fixture(kind).locator("button, a").count(), 1);
          await target(control);
          await control.click();
          await page.waitForURL(`**/${locale}/ui-gallery?tab=b#sec-switch`);
          const selected = page.locator("#gt-tab-b");
          await selected.waitFor({ state: "visible" });
          assert.ok((await selected.innerText()).includes(galleryCopy[locale].tabB));
          assert.equal(await selected.getAttribute("aria-selected"), "true");
          await page.reload({ waitUntil: "load" });
          assert.equal(await selected.getAttribute("aria-selected"), "true");
          assert.equal(new URL(page.url()).searchParams.get("tab"), "b");
          await page.goto(home, { waitUntil: "load" });
          await page.locator('[data-recovery-ready="true"]').waitFor();
        });
      }
    }
    assert.ok(requests.length > 0, "acceptance must exercise callbacks");
    assert.ok(requests.every(r => r.method === "GET" && r.operation === "ds-states-example"), "UNKNOWN never submits or changes operation");
  } finally {
    page.off("request", observe);
    await page.unroute(pattern, handler);
  }
}
