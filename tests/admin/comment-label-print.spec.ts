// Purpose: W3-U3 real-click label selection, real A3 writes and private print DOM acceptance.
// Depends on: signed Next/Go/PG console fixture, synthetic Graph rows and native browser print media.
// Used by: --browser-live-console labels subtest; no physical printer or LIVE provider acceptance.
import { expect, type Page } from "@playwright/test";
import { test } from "./inbox-private-evidence";
import { writeFile } from "node:fs/promises";

const required = (key: string) => { const value = process.env[key]; if (!value) throw new Error(`${key} missing`); return value; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), store = required("LC_BROWSER_CONSOLE_STORE");
const data: { session: string; print_refs: string[] } = JSON.parse(required("LC_BROWSER_CONSOLE_COMMENTS"));
const evidence = required("LC_BROWSER_EVIDENCE");
const ledger: { locale: string; width: number; action: string; count: number }[] = [];
test.use({ baseURL: origin, trace: "off", screenshot: "off", video: "off" });
test.setTimeout(60000);

async function open(page: Page, locale: string) {
  await page.goto(`${origin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("shell-store-selector")).toBeAttached();
  await page.goto(`${origin}/${locale}/studio/console?store=${store}&scene=${data.session}`);
  await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(50);
  await page.getByTestId("comment-filter-keyword").click();
  await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(3);
}
async function privateStorage(page: Page) {
  // Read-only privacy assertion; no DOM/API response manipulation.
  const values = await page.evaluate(() => [...Object.entries(localStorage), ...Object.entries(sessionStorage)]);
  const serialized = JSON.stringify(values);
  expect(serialized).not.toContain("LC-U2a synthetic author");
  expect(serialized).not.toContain("LC-U2a row");
  for (const ref of data.print_refs) expect(serialized).not.toContain(ref);
  const preference = values.find(([key]) => key === "comment-label-paper");
  if (preference) expect(preference[1]).toMatch(/^(label|a4)$/);
}

test.afterAll(async () => { await writeFile(`${evidence}/label-click-ledger.json`, JSON.stringify(ledger, null, 2)); });
for (const locale of ["zh-TW", "zh-CN", "en"]) for (const width of [1586, 390]) {
  test(`W3U3 three label real A3 print ${locale}-${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
    // Only the physical print dialog is intercepted. API, app handlers and print CSS remain real.
    await page.addInitScript(() => { (window as unknown as { printCalls: number }).printCalls = 0; window.print = () => { (window as unknown as { printCalls: number }).printCalls++; }; });
    await open(page, locale);
    const writes: { ref: string; key: string; body: string; status: number; count: number }[] = [];
    page.on("response", async (response) => {
      if (!new URL(response.url()).pathname.endsWith("/print")) return;
      const request = response.request(), body = await response.json();
      writes.push({ ref: request.url().split("/").at(-2)!, key: request.headers()["idempotency-key"], body: request.postData()!, status: response.status(), count: body.print_count });
    });
    for (const ref of data.print_refs) await page.getByTestId(`comment-label-select-${ref}`).check();
    await page.getByTestId("comment-label-preview").click();
    await expect(page.getByTestId("comment-label")).toHaveCount(3);
    for (const [i, ref] of data.print_refs.entries()) {
      const label = page.getByTestId("comment-label").filter({ has: page.locator(`[data-label-ref="${ref}"]`) });
      await expect(label).toContainText("LC-U2a synthetic author");
      await expect(label).toContainText(`A1 × ${i + 1}`);
      await expect(label).toContainText(data.session.slice(0, 8));
      await expect(label.locator("time")).not.toBeEmpty();
    }
    await page.getByTestId("comment-label-paper").selectOption("a4");
    await page.screenshot({ path: `${evidence}/labels-${locale}-${width}-preview.png`, fullPage: true, mask: [page.getByTestId("comment-rows"), page.getByTestId("buyer-panel")] });
    await page.getByTestId("comment-label-print").click();
    await expect.poll(() => writes.length).toBe(3);
    expect(new Set(writes.map((w) => w.key)).size).toBe(3);
    for (const w of writes) { expect(w.status).toBe(200); expect(w.body).toBe("{}"); expect(w.key).toMatch(/^[0-9a-f-]{36}$/); }
    await expect.poll(() => page.evaluate(() => (window as unknown as { printCalls: number }).printCalls)).toBe(1);
    await privateStorage(page);
    await page.emulateMedia({ media: "print" });
    await expect(page.getByTestId("comment-label")).toHaveCount(3);
    await expect(page.getByTestId("comment-label-print")).toBeHidden();
    await expect(page.getByTestId("comment-stream")).toBeHidden();
    await expect(page.locator("h1")).toBeHidden();
    await expect(page.getByTestId("comment-label-sheet")).toHaveAttribute("data-paper", "a4");
    await page.screenshot({ path: `${evidence}/labels-${locale}-${width}-a4.png`, fullPage: true });
    await page.emulateMedia({ media: "screen" });
    await page.getByTestId("comment-label-paper").selectOption("label");
    await page.emulateMedia({ media: "print" });
    const box = await page.getByTestId("comment-label").first().boundingBox();
    expect(box).not.toBeNull(); expect(box!.width).toBeCloseTo(60 * 96 / 25.4, 0); expect(box!.height).toBeCloseTo(40 * 96 / 25.4, 0);
    await page.screenshot({ path: `${evidence}/labels-${locale}-${width}-small.png`, fullPage: true });
    await page.emulateMedia({ media: "screen" });
    await page.getByTestId("comment-label-close").click();
    for (const w of writes) await expect(page.getByTestId(`comment-label-count-${w.ref}`)).toContainText(`×${w.count}`);
    // Marks must survive refresh through real A2/PG, not just local optimistic state.
    await page.reload(); await page.getByTestId("comment-filter-keyword").click();
    for (const w of writes) await expect(page.getByTestId(`comment-label-count-${w.ref}`)).toContainText(`×${w.count}`);
    ledger.push({ locale, width, action: "select-three-preview-a4-print-small-close-refresh", count: 3 });
  });
}

test("W3U3 failed record still prints without claiming success; preview closes on reset", async ({ page, request }) => {
  await page.addInitScript(() => { (window as unknown as { printCalls: number }).printCalls = 0; window.print = () => { (window as unknown as { printCalls: number }).printCalls++; }; });
  await open(page, "en");
  const ref = data.print_refs[0], oldCount = await page.getByTestId(`comment-label-count-${ref}`).textContent();
  await page.getByTestId(`comment-label-single-${ref}`).click();
  await expect(page.getByTestId("comment-label")).toHaveCount(1);
  // Network fault only: the print fact is not committed; browser printing must remain available.
  await page.route("**/comments/*/print", (route) => route.abort("connectionreset"));
  await page.getByTestId("comment-label-print").click();
  await expect(page.getByTestId("comment-label-status")).toContainText("not recorded");
  await expect.poll(() => page.evaluate(() => (window as unknown as { printCalls: number }).printCalls)).toBe(1);
  await page.getByTestId("comment-label-close").click();
  await expect(page.getByTestId(`comment-label-count-${ref}`)).toHaveText(oldCount!);
  await page.getByTestId(`comment-label-single-${ref}`).click();
  const response = await request.post(`${required("LC_BROWSER_API_ORIGIN")}/__test/live-console/fault`, { headers: { "X-Console-Control": required("LC_BROWSER_CONSOLE_CONTROL") }, data: { scene: data.session, mode: "comments_reset" } });
  expect(response.status()).toBe(200);
  await expect(page.getByTestId("comment-label")).toHaveCount(0);
  await privateStorage(page);
});
