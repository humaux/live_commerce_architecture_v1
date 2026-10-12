# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: comment-label-print.spec.ts >> W3U3 three label real A3 print zh-TW-390
- Location: tests/admin/comment-label-print.spec.ts:37:3

# Error details

```
Test timeout of 60000ms exceeded.
```

```
Error: locator.check: Test timeout of 60000ms exceeded.
Call log:
  - waiting for getByTestId('comment-label-select-219731783316578_8973835304')

```

# Test source

```ts
  1   | // Purpose: W3-U3 real-click label selection, real A3 writes and private print DOM acceptance.
  2   | // Depends on: signed Next/Go/PG console fixture, synthetic Graph rows and native browser print media.
  3   | // Used by: --browser-live-console labels subtest; no physical printer or LIVE provider acceptance.
  4   | import { expect, type Page } from "@playwright/test";
  5   | import { test } from "./inbox-private-evidence";
  6   | import { writeFile } from "node:fs/promises";
  7   | 
  8   | const required = (key: string) => { const value = process.env[key]; if (!value) throw new Error(`${key} missing`); return value; };
  9   | const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), store = required("LC_BROWSER_CONSOLE_STORE");
  10  | const data: { session: string; print_refs: string[] } = JSON.parse(required("LC_BROWSER_CONSOLE_COMMENTS"));
  11  | const evidence = required("LC_BROWSER_EVIDENCE");
  12  | const ledger: { locale: string; width: number; action: string; count: number }[] = [];
  13  | test.use({ baseURL: origin, trace: "off", screenshot: "off", video: "off" });
  14  | test.setTimeout(60000);
  15  | 
  16  | async function open(page: Page, locale: string) {
  17  |   await page.goto(`${origin}/en/`);
  18  |   await page.getByRole("button", { name: "Sign in with identity service" }).click();
  19  |   await expect(page.getByTestId("shell-store-selector")).toBeAttached();
  20  |   await page.goto(`${origin}/${locale}/studio/console?store=${store}&scene=${data.session}`);
  21  |   await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(50);
  22  |   await page.getByTestId("comment-filter-keyword").click();
  23  |   await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(3);
  24  | }
  25  | async function privateStorage(page: Page) {
  26  |   // Read-only privacy assertion; no DOM/API response manipulation.
  27  |   const values = await page.evaluate(() => [...Object.entries(localStorage), ...Object.entries(sessionStorage)]);
  28  |   const serialized = JSON.stringify(values);
  29  |   expect(serialized).not.toContain("LC-U2a synthetic author");
  30  |   expect(serialized).not.toContain("LC-U2a row");
  31  |   for (const ref of data.print_refs) expect(serialized).not.toContain(ref);
  32  |   expect(values.find(([key]) => key === "comment-label-paper")?.[1]).toMatch(/^(label|a4)$/);
  33  | }
  34  | 
  35  | test.afterAll(async () => { await writeFile(`${evidence}/label-click-ledger.json`, JSON.stringify(ledger, null, 2)); });
  36  | for (const locale of ["zh-TW", "zh-CN", "en"]) for (const width of [1586, 390]) {
  37  |   test(`W3U3 three label real A3 print ${locale}-${width}`, async ({ page }) => {
  38  |     await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
  39  |     // Only the physical print dialog is intercepted. API, app handlers and print CSS remain real.
  40  |     await page.addInitScript(() => { (window as unknown as { printCalls: number }).printCalls = 0; window.print = () => { (window as unknown as { printCalls: number }).printCalls++; }; });
  41  |     await open(page, locale);
  42  |     const writes: { ref: string; key: string; body: string; status: number; count: number }[] = [];
  43  |     page.on("response", async (response) => {
  44  |       if (!new URL(response.url()).pathname.endsWith("/print")) return;
  45  |       const request = response.request(), body = await response.json();
  46  |       writes.push({ ref: request.url().split("/").at(-2)!, key: request.headers()["idempotency-key"], body: request.postData()!, status: response.status(), count: body.print_count });
  47  |     });
> 48  |     for (const ref of data.print_refs) await page.getByTestId(`comment-label-select-${ref}`).check();
      |                                                                                              ^ Error: locator.check: Test timeout of 60000ms exceeded.
  49  |     await page.getByTestId("comment-label-preview").click();
  50  |     await expect(page.getByTestId("comment-label")).toHaveCount(3);
  51  |     for (const [i, ref] of data.print_refs.entries()) {
  52  |       const label = page.getByTestId("comment-label").filter({ has: page.locator(`[data-label-ref="${ref}"]`) });
  53  |       await expect(label).toContainText("LC-U2a synthetic author");
  54  |       await expect(label).toContainText(`A1 × ${i + 1}`);
  55  |       await expect(label).toContainText(data.session.slice(0, 8));
  56  |       await expect(label.locator("time")).not.toBeEmpty();
  57  |     }
  58  |     await page.getByTestId("comment-label-paper").selectOption("a4");
  59  |     await page.getByTestId("comment-label-print").click();
  60  |     await expect.poll(() => writes.length).toBe(3);
  61  |     expect(new Set(writes.map((w) => w.key)).size).toBe(3);
  62  |     for (const w of writes) { expect(w.status).toBe(200); expect(w.body).toBe("{}"); expect(w.key).toMatch(/^[0-9a-f-]{36}$/); }
  63  |     await expect.poll(() => page.evaluate(() => (window as unknown as { printCalls: number }).printCalls)).toBe(1);
  64  |     await privateStorage(page);
  65  |     await page.emulateMedia({ media: "print" });
  66  |     await expect(page.getByTestId("comment-label")).toHaveCount(3);
  67  |     await expect(page.getByTestId("comment-label-print")).toBeHidden();
  68  |     await expect(page.getByTestId("comment-stream")).toBeHidden();
  69  |     await expect(page.locator("h1")).toBeHidden();
  70  |     await expect(page.getByTestId("comment-label-sheet")).toHaveAttribute("data-paper", "a4");
  71  |     await page.screenshot({ path: `${evidence}/labels-${locale}-${width}-a4.png`, fullPage: true });
  72  |     await page.emulateMedia({ media: "screen" });
  73  |     await page.getByTestId("comment-label-paper").selectOption("label");
  74  |     await page.emulateMedia({ media: "print" });
  75  |     const box = await page.getByTestId("comment-label").first().boundingBox();
  76  |     expect(box).not.toBeNull(); expect(box!.width).toBeCloseTo(60 * 96 / 25.4, 0); expect(box!.height).toBeCloseTo(40 * 96 / 25.4, 0);
  77  |     await page.emulateMedia({ media: "screen" });
  78  |     await page.getByTestId("comment-label-close").click();
  79  |     for (const w of writes) await expect(page.getByTestId(`comment-label-count-${w.ref}`)).toContainText(`×${w.count}`);
  80  |     // Marks must survive refresh through real A2/PG, not just local optimistic state.
  81  |     await page.reload(); await page.getByTestId("comment-filter-keyword").click();
  82  |     for (const w of writes) await expect(page.getByTestId(`comment-label-count-${w.ref}`)).toContainText(`×${w.count}`);
  83  |     ledger.push({ locale, width, action: "select-three-preview-a4-print-small-close-refresh", count: 3 });
  84  |   });
  85  | }
  86  | 
  87  | test("W3U3 failed record still prints without claiming success; preview closes on reset", async ({ page, request }) => {
  88  |   await page.addInitScript(() => { (window as unknown as { printCalls: number }).printCalls = 0; window.print = () => { (window as unknown as { printCalls: number }).printCalls++; }; });
  89  |   await open(page, "en");
  90  |   const ref = data.print_refs[0], oldCount = await page.getByTestId(`comment-label-count-${ref}`).textContent();
  91  |   await page.getByTestId(`comment-label-single-${ref}`).click();
  92  |   await expect(page.getByTestId("comment-label")).toHaveCount(1);
  93  |   // Network fault only: the print fact is not committed; browser printing must remain available.
  94  |   await page.route("**/comments/*/print", (route) => route.abort("connectionreset"));
  95  |   await page.getByTestId("comment-label-print").click();
  96  |   await expect(page.getByTestId("comment-label-status")).toContainText("not recorded");
  97  |   await expect.poll(() => page.evaluate(() => (window as unknown as { printCalls: number }).printCalls)).toBe(1);
  98  |   await page.getByTestId("comment-label-close").click();
  99  |   await expect(page.getByTestId(`comment-label-count-${ref}`)).toHaveText(oldCount!);
  100 |   await page.getByTestId(`comment-label-single-${ref}`).click();
  101 |   const response = await request.post(`${required("LC_BROWSER_API_ORIGIN")}/__test/live-console/fault`, { headers: { "X-Console-Control": required("LC_BROWSER_CONSOLE_CONTROL") }, data: { scene: data.session, mode: "comments_reset" } });
  102 |   expect(response.status()).toBe(200);
  103 |   await expect(page.getByTestId("comment-label")).toHaveCount(0);
  104 |   await privateStorage(page);
  105 | });
  106 | 
```