// Purpose: Independent W6 report real-click acceptance from reporting-v2 §§1–6, with persisted export audits in Go.
// Depends on: Playwright, reports-copy labels only, genuine browser_w6_customers_reports_test.go stack and LC_W6UI_*.
// Used by: TestBrowserW6Reports / --browser-reports; no API writes or JSON mocks substitute user clicks.
// Invariants: I01/I05/I11/I18; signed MOCK OIDC, REAL_PG synthetic corpus; provider SANDBOX/LIVE NOT_RUN.
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { reportsCopy } from "../../apps/admin/lib/reports-copy";

const required = (key: string) => { const value = process.env[key]; if (!value) throw new Error(`${key} required`); return value; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE"), store = required("LC_W6UI_STORE");
const control = required("LC_W6UI_CONTROL_URL"), gateKey = required("LC_W6UI_CONTROL_KEY");
type LedgerRow = { page: string; control: string; operation: string; expected: string; actual: string; result: "pass" | "fail" };
const ledger: LedgerRow[] = [];
test.describe.configure({ mode: "serial" });
test.afterEach(async () => { await writeFile(path.join(evidence, "w6ui-click-ledger.json"), JSON.stringify(ledger, null, 2)); });

async function step(page: Page, controlName: string, expected: string, action: () => Promise<void>, operation = "click") {
  const row: LedgerRow = { page: new URL(page.url()).pathname, control: controlName, operation, expected, actual: "", result: "fail" };
  ledger.push(row);
  try { await action(); row.result = "pass"; row.actual = expected; } catch (error) { row.actual = error instanceof Error ? error.message.slice(0, 240) : "assertion failed"; throw error; }
}
async function ctl(resource: string, mode = "") {
  // Fixture preparation/fault arming only. The report action itself always follows a real UI click.
  const response = await fetch(`${control}/${resource}`, { method: "POST", headers: { "X-W6UI-Gate-Key": gateKey, "Content-Type": "application/json" }, body: JSON.stringify({ mode }) });
  expect(response.status, `fixture ${resource}`).toBe(204);
}
async function signed(browser: Browser, locale: "en" | "zh-TW", width: number, actor = "owner") {
  await ctl("actor", actor);
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width, height: width === 390 ? 844 : 900 } });
  const page = await context.newPage();
  await page.goto(`${origin}/en/`);
  await step(page, "Sign in with identity service", "signed OIDC callback opens the merchant workspace", async () => {
    await page.getByRole("button", { name: "Sign in with identity service" }).click();
    await page.getByTestId("shell-store-selector").waitFor(); // sign-out sits in the collapsed Account popover (WorkspaceFrame <details>), never visible here
  });
  const cookies = await context.cookies();
  expect(cookies.some((c) => c.name.startsWith("__Host-") && c.httpOnly && c.secure)).toBe(true);
  await page.goto(`${origin}/${locale}/finance/reports?store=${store}&from=2026-09-01&to=2026-09-30`);
  return { context, page };
}
async function fill(page: Page, field: Locator, value: string, name: string) {
  await step(page, name, `input displays ${value}`, async () => { await field.fill(value); await expect(field).toHaveValue(value); }, "fill");
}
async function fits(page: Page) {
  // READ/MEASURE only: never writes DOM, storage or app state.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(1);
}
function csvRows(text: string): Record<string, string>[] {
  // RFC4180 read-only oracle, independent of the report writer; downloaded content can contain quoted commas/newlines.
  const rows: string[][] = []; let row: string[] = [], cell = "", quoted = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch === '"') { if (quoted && text[i + 1] === '"') { cell += '"'; i++; } else quoted = !quoted; }
    else if (ch === "," && !quoted) { row.push(cell); cell = ""; }
    else if (ch === "\n" && !quoted) { row.push(cell.replace(/\r$/, "")); rows.push(row); row = []; cell = ""; }
    else cell += ch;
  }
  if (row.length || cell) { row.push(cell.replace(/\r$/, "")); rows.push(row); }
  expect(quoted, "downloaded CSV has closed quotes").toBe(false);
  const header = rows.shift() ?? [];
  return rows.filter((r) => r.some(Boolean)).map((r) => { expect(r.length).toBe(header.length); return Object.fromEntries(header.map((h, i) => [h, r[i]])); });
}
async function exportTab(page: Page, tab: string) {
  let text = "";
  await step(page, `CSV ${tab}`, "click downloads one report with correct filename and CSV grammar", async () => {
    const download = page.waitForEvent("download");
    await page.getByTestId(`reports-csv-${tab}`).click();
    const file = await download;
    expect(file.suggestedFilename()).toBe(`report-${tab.replaceAll("-", "_")}-2026-09-01-2026-09-30.csv`);
    const saved = await file.path(); expect(saved).not.toBeNull(); text = await readFile(saved!, "utf8");
    expect(text).not.toContain("W6-ARCHIVE-ONLY");
  });
  return csvRows(text);
}

test("RPUI calibration-sensitive response: genuine products read renders data", async ({ browser }) => {
  const { context, page } = await signed(browser, "en", 1440);
  try {
    if (process.env.LC_W6UI_CALIBRATION === "reports-truncate-response") {
      await ctl("fault", "truncate-report"); await page.reload();
    }
    await step(page, "initial products read", "RPUI-RED-REPORT-DATA: real product table is visible", async () => {
      await expect(page.getByTestId("reports-products-table"), "RPUI-RED-REPORT-DATA").toBeVisible();
      await expect(page.getByTestId("reports-products-table")).toContainText("=Alpha");
    }, "observe");
  } finally { await context.close(); }
});

for (const locale of ["en", "zh-TW"] as const) for (const width of [1440, 390]) {
  test(`RPUI all report controls, CSV and persistence ${locale}/${width}`, async ({ browser }) => {
    const { context, page } = await signed(browser, locale, width), c = reportsCopy[locale];
    try {
      await expect(page.getByTestId("reports-money-basis")).toHaveText(c.moneyBasis);
      await expect(page.getByTestId("reports-page")).toContainText(c.rangeBasis);
      await expect(page.getByTestId("reports-products-table").locator("tbody tr")).toHaveCount(5);
      const table = page.getByTestId("reports-products-table");
      // Expectations pin independent hand arithmetic, with each currency/environment group kept separate.
      const alpha = table.locator("tbody tr").filter({ hasText: "=Alpha" });
      await expect(alpha).toContainText("173.33"); await expect(alpha.locator("td").nth(5)).toContainText("150");
      for (const label of [c.product, c.units, c.captured, c.refunded, c.net, c.offlineUnits, c.offline]) {
        const header = table.locator("thead th").filter({ has: page.getByRole("button", { name: new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`) }) });
        for (let n = 0; n < 2; n++) await step(page, `sort ${label}`, "sort changes aria direction and rows remain separate", async () => {
          const before = await header.getAttribute("aria-sort"); await header.getByRole("button").click();
          await expect(header).not.toHaveAttribute("aria-sort", before ?? "none");
          await expect(table.locator("tbody tr")).toHaveCount(5);
          const groups = await table.locator("tbody tr").evaluateAll((rows) => rows.map((row) => [row.children[7]?.textContent, row.children[8]?.textContent].join("/")));
          for (const group of new Set(groups)) { const indexes = groups.flatMap((value, i) => value === group ? [i] : []); expect(indexes.at(-1)! - indexes[0] + 1).toBe(indexes.length); }
        });
      }
      const productCSV = await exportTab(page, "products");
      expect(productCSV).toHaveLength(5);
      const facts = productCSV.map((r) => [r.code, r.currency, r.environment, r.units, r.captured_minor, r.refunded_minor, r.net_minor, r.offline_minor]);
      expect(facts).toEqual(expect.arrayContaining([
        ["AAA", "TWD", "LIVE", "3", "23333", "6000", "17333", "15000"],
        ["BBB", "TWD", "LIVE", "3", "13433", "3000", "10433", "8000"],
        ["CCC", "TWD", "LIVE", "2", "8335", "0", "8335", "0"],
        ["BBB", "TWD", "SANDBOX", "3", "30000", "0", "30000", "0"],
        ["USD", "USD", "SANDBOX", "1", "700", "0", "700", "0"],
      ]));
      expect(productCSV.find((r) => r.code === "AAA")?.name).toBe("'=Alpha");

      await step(page, "Channels tab", "four channel bars and separate currency/environment money render", async () => {
        await page.getByTestId("reports-tab-channels").click();
        await expect(page.getByTestId("reports-channels-chart")).toBeVisible();
        for (const name of Object.values(c.channels)) await expect(page.getByTestId("reports-channels-chart")).toContainText(name);
        await expect(page.getByTestId("reports-channels-chart").getByRole("img")).toHaveCount(5);
      });
      const channels = await exportTab(page, "channels");
      const sum = (currency: string, environment: string, key: string) => channels.filter((r) => r.currency === currency && r.environment === environment).reduce((n, r) => n + Number(r[key]), 0);
      expect(sum("TWD", "LIVE", "captured_minor")).toBe(45101); expect(sum("TWD", "LIVE", "net_minor")).toBe(36101);
      expect(sum("TWD", "SANDBOX", "net_minor")).toBe(30000); expect(sum("USD", "SANDBOX", "net_minor")).toBe(700);
      expect(channels.find((r) => r.channel === "facebook_live" && r.currency === "TWD")?.orders).toBe("1");

      await step(page, "Funnel tab", "SANDBOX cohort stages are nested 6/4/2/1 and missing-link orders stay separate", async () => {
        await page.getByTestId("reports-tab-funnel").click();
        const stages = page.getByTestId("reports-funnel-chart").locator("ol li"); await expect(stages).toHaveCount(4);
        for (const [index, value] of [6, 4, 2, 1].entries()) await expect(stages.nth(index).locator("div span")).toHaveText(String(value));
        await expect(page.getByTestId("reports-funnel-chart")).toContainText(`${c.unlinked}: 1`);
      });
      const funnel = await exportTab(page, "funnel");
      expect(funnel).toHaveLength(1); expect([funnel[0].claimed, funnel[0].link_sent, funnel[0].ordered, funnel[0].paid]).toEqual(["6", "4", "2", "1"]);
      await step(page, "Manual orders tab", "manual orders show honest unavailable staff/session labels and offline columns", async () => {
        await page.getByTestId("reports-tab-manual-orders").click();
        await expect(page.getByTestId("reports-manual-table")).toBeVisible();
        await expect(page.getByTestId("reports-page")).toContainText(c.manualBasis);
        await expect(page.getByTestId("reports-manual-table")).toContainText(c.unknownCreator);
      });
      const manual = await exportTab(page, "manual-orders");
      expect(manual.reduce((n, r) => n + Number(r.offline_minor), 0)).toBe(23000);
      await step(page, "tab keyboard", "Home/ArrowRight/End select and focus real report tabs", async () => {
        await page.getByTestId("reports-tab-manual-orders").focus(); await page.keyboard.press("Home");
        await expect(page.getByTestId("reports-tab-products")).toHaveAttribute("aria-selected", "true");
        await page.keyboard.press("ArrowRight"); await expect(page.getByTestId("reports-tab-channels")).toBeFocused();
        await page.keyboard.press("End"); await expect(page.getByTestId("reports-tab-manual-orders")).toHaveAttribute("aria-selected", "true");
      }, "keyboard");
      await step(page, "Select Channels", "the Channels tab is selected and recorded in the address", async () => {
        await page.getByTestId("reports-tab-channels").click();
        await expect(page.getByTestId("reports-tab-channels")).toHaveAttribute("aria-selected", "true");
        await expect(page).toHaveURL(/report=channels/);
      });
      await fill(page, page.getByTestId("reports-from"), "2026-09-02", c.from);
      await step(page, "Edited range not applied", "CSV stays disabled until Show applies the edited range", async () => {
        await expect(page.getByTestId("reports-csv-products")).toBeDisabled();
      });
      await step(page, "Show report", "date change navigates and survives refresh", async () => {
        await page.getByTestId("reports-show").click(); await expect(page).toHaveURL(/from=2026-09-02/);
        await page.reload(); await expect(page.getByTestId("reports-from")).toHaveValue("2026-09-02");
        // Codex review P2 (PR #3): the selected report tab survives Show and refresh instead of resetting to Products.
        await expect(page).toHaveURL(/report=channels/);
        await expect(page.getByTestId("reports-tab-channels")).toHaveAttribute("aria-selected", "true");
        await expect(page.getByTestId("reports-tab-products")).toHaveAttribute("aria-selected", "false");
      });
      await fits(page);
    } finally { await context.close(); }
  });
}

test("RPUI 92/93 days, reversed dates, real truncated/failed reads and empty success", async ({ browser }) => {
  const { context, page } = await signed(browser, "en", 1440), c = reportsCopy.en;
  try {
    await fill(page, page.getByTestId("reports-from"), "2026-07-01", c.from);
    await fill(page, page.getByTestId("reports-to"), "2026-09-30", c.to);
    await step(page, "92-day Show", "inclusive 92 Taipei days are admitted", async () => { await expect(page.getByTestId("reports-show")).toBeEnabled(); await page.getByTestId("reports-show").click(); await expect(page).toHaveURL(/from=2026-07-01/); });
    await fill(page, page.getByTestId("reports-to"), "2026-10-01", c.to);
    await expect(page.getByTestId("reports-range-invalid")).toHaveText(c.rangeInvalid); await expect(page.getByTestId("reports-show")).toBeDisabled();
    await fill(page, page.getByTestId("reports-from"), "2026-10-02", c.from);
    await expect(page.getByTestId("reports-show")).toBeDisabled();
    await page.goto(`${origin}/en/finance/reports?store=${store}&from=2026-09-01&to=2026-09-30`);
    for (const fault of ["read-503", "truncate-report"]) {
      await ctl("fault", fault);
      await step(page, `fault ${fault}`, "actual failed response shows unavailable; Retry reads genuine data", async () => {
        await page.reload(); await expect(page.getByTestId("reports-page")).toContainText(c.unavailable);
        await page.getByRole("button", { name: c.retry, exact: true }).click(); await expect(page.getByTestId("reports-products-table")).toBeVisible();
      });
    }
    await fill(page, page.getByTestId("reports-from"), "2025-01-01", c.from); await fill(page, page.getByTestId("reports-to"), "2025-01-02", c.to);
    await step(page, "empty range Show", "empty 200 is distinct from unavailable and forbidden", async () => { await page.getByTestId("reports-show").click(); await expect(page.getByTestId("reports-page")).toContainText(c.empty); });
  } finally { await context.close(); }
});

test("RPUI reader/no-orders signed identities: export disabled and funnel permission refusal", async ({ browser }) => {
  const c = reportsCopy.en;
  for (const actor of ["reader", "none"]) {
    const { context, page } = await signed(browser, "en", 390, actor);
    try {
      if (actor === "reader") {
        await expect(page.getByTestId("reports-products-table")).toBeVisible();
        await expect(page.getByTestId("reports-csv-products")).toBeDisabled(); await expect(page.getByTestId("reports-page")).toContainText(c.csvForbidden);
        await step(page, "reader Funnel", "missing live:read is an explicit permission refusal", async () => { await page.getByTestId("reports-tab-funnel").click(); await expect(page.getByTestId("reports-page")).toContainText(c.forbidden); });
      } else { await expect(page.getByTestId("reports-page")).toContainText(c.forbidden); await expect(page.getByTestId("reports-products-table")).toHaveCount(0); }
    } finally { await context.close(); }
  }
});
