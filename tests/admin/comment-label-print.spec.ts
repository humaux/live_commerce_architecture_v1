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
const ledger: { locale: string; width: number; page: string; control: string; operation: string; expected: string; actual: string; result: "PASS" | "FAIL" }[] = [];
function record(locale: string, width: number, control: string, operation: string, expected: string) {
  ledger.push({ locale, width, page: "/studio/console", control, operation, expected, actual: expected, result: "PASS" });
}
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

test.afterEach(async ({}, info) => {
  // Playwright restarts its worker after a failure. Per-case files retain all
  // successful controls instead of letting a restarted worker overwrite them.
  const rows = ledger.splice(0);
  if (info.status !== "passed") rows.push({ locale: "en", width: 1586, page: "/studio/console", control: info.title, operation: "real-click contract case", expected: "All unchanged contract assertions pass", actual: `Test status: ${info.status}; see error-context.md`, result: "FAIL" });
  await writeFile(`${evidence}/label-click-ledger-${info.testId.replace(/[^\w-]/g, "_")}.json`, JSON.stringify(rows, null, 2));
});
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
    // Operate both states, not merely a checkbox's initially unchecked path.
    await page.getByTestId(`comment-label-select-${data.print_refs[0]}`).uncheck();
    await expect(page.getByTestId("comment-label-preview")).toContainText("(2)");
    await page.getByTestId(`comment-label-select-${data.print_refs[0]}`).check();
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
    // Fixture-only synthetic names: a mask over the underlying stream would cover the modal too.
    await page.screenshot({ path: `${evidence}/labels-${locale}-${width}-preview.png`, fullPage: false });
    await page.getByTestId("comment-label-print").click();
    await expect.poll(() => writes.length).toBe(3);
    expect(writes.map((write) => write.ref).sort()).toEqual([...data.print_refs].sort());
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
    await page.getByTestId("comment-label-paper").selectOption("a4");
    await page.getByTestId("comment-label-close").click();
    for (const w of writes) await expect(page.getByTestId(`comment-label-count-${w.ref}`)).toContainText(`×${w.count}`);
    // Marks must survive refresh through real A2/PG, not just local optimistic state.
    await page.reload(); await page.getByTestId("comment-filter-keyword").click();
    for (const w of writes) await expect(page.getByTestId(`comment-label-count-${w.ref}`)).toContainText(`×${w.count}`);
    await page.getByTestId(`comment-label-single-${data.print_refs[0]}`).click();
    await expect(page.getByTestId("comment-label-paper")).toHaveValue("a4");
    await page.getByTestId("comment-label-close").click();
    record(locale, width, "keyword filter", "click", "Three actual claimed keyword comments visible");
    record(locale, width, "three checkboxes", "check; uncheck first; recheck first", "Selection counts 3→2→3");
    record(locale, width, "preview labels", "click", "Three exact synthetic names/keywords/quantities/time/session labels");
    record(locale, width, "paper selector", "A4→label→A4; reload and reopen", "Print media 60×40mm / A4 three columns; A4 preference survives reload");
    record(locale, width, "print", "click", "Three exact {} A3 requests, three unique keys, confirmed badges survive A2 reload");
    record(locale, width, "close preview", "click", "Dialog closes; console remains usable");
    record(locale, width, "single label", "click", "One label opens with restored paper preference");
  });
}

test("W3U3_404 real revoked store closes private labels before native print", async ({ page, request }) => {
  await page.addInitScript(() => { (window as unknown as { printCalls: number }).printCalls = 0; window.print = () => { (window as unknown as { printCalls: number }).printCalls++; }; });
  await open(page, "en");
  await page.getByTestId(`comment-label-single-${data.print_refs[0]}`).click();
  await expect(page.getByTestId("comment-label")).toHaveCount(1);
  // Hold only background reads so the REAL scoped A3 404 owns this regression,
  // rather than A1/A2 first expiring authority and hiding the original defect.
  let resume!: () => void;
  const held = new Promise<void>((resolve) => { resume = resolve; });
  await page.route("**/api/stores/**", async (route) => {
    if (route.request().method() === "GET") await held;
    await route.continue();
  });
  const grant = async (mode: string) => {
    const result = await request.post(`${required("LC_BROWSER_API_ORIGIN")}/__test/live-console/fault`, { headers: { "X-Console-Control": required("LC_BROWSER_CONSOLE_CONTROL") }, data: { scene: data.session, mode } });
    expect(result.status()).toBe(200);
  };
  try {
    await grant("grant_revoke");
    const denied = page.waitForResponse((r) => r.request().method() === "POST" && new URL(r.url()).pathname.endsWith("/print"), { timeout: 10000 });
    await page.getByTestId("comment-label-print").click();
    expect((await denied).status()).toBe(404);
    await expect(page.getByTestId("comment-label")).toHaveCount(0);
    expect(await page.evaluate(() => (window as unknown as { printCalls: number }).printCalls)).toBe(0);
    await privateStorage(page);
    record("en", 1586, "print after scoped grant revocation", "click", "Real A3 404 clears private labels; native print calls=0");
  } finally { await grant("grant_restore"); resume(); await page.unrouteAll({ behavior: "wait" }); }
});

test("W3U3 lost A3 acknowledgement retries the same fact key", async ({ page }) => {
  await page.addInitScript(() => { (window as unknown as { printCalls: number }).printCalls = 0; window.print = () => { (window as unknown as { printCalls: number }).printCalls++; }; });
  await open(page, "en");
  const ref = data.print_refs[0], keys: string[] = [], counts: number[] = [];
  await page.getByTestId(`comment-label-single-${ref}`).click();
  await page.route(`**/comments/${ref}/print`, async (route) => {
    keys.push(route.request().headers()["idempotency-key"]);
    const actual = await route.fetch(); expect(actual.status()).toBe(200);
    counts.push((await actual.json()).print_count);
    // The first actual transaction commits; only its acknowledgement is lost.
    if (keys.length === 1) await route.abort("connectionreset"); else await route.fulfill({ response: actual });
  });
  await page.getByTestId("comment-label-print").click();
  await expect(page.getByTestId("comment-label-status")).toContainText("not recorded");
  await page.getByTestId("comment-label-print").click();
  await expect.poll(() => counts.length).toBe(2);
  expect(keys[1]).toBe(keys[0]); expect(counts[1]).toBe(counts[0]);
  await expect(page.getByTestId("comment-label-status")).toHaveCount(0);
  await page.getByTestId("comment-label-close").click();
  await expect(page.getByTestId(`comment-label-count-${ref}`)).toContainText(`×${counts[0]}`);
  await privateStorage(page);
  record("en", 1586, "print with lost acknowledgement", "click print twice explicitly", "Committed fact replays same key and print_count; no extra fact");
});

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
  record("en", 1586, "single print; close; single preview", "click with aborted network then reset epoch", "Physical print remains available, no false new badge; preview clears on real reset");
});
