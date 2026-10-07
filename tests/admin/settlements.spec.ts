// Purpose: real-click browser gate of the read-only settlement statements page (list, detail with lines, payout states, empty state, 390px) against real PG, Go and the production admin Next build.
// Depends on: @playwright/test, node:crypto, node:fs/promises, node:path, ../../apps/admin/lib/card-payments-settlements-copy, ../../apps/admin/lib/card-payments-settlements-model, ../../apps/admin/src/shell-copy, ../../packages/format/src/index; harness env: LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_EVIDENCE, LC_BROWSER_STORE, LC_BROWSER_STMT_PAID, LC_BROWSER_STMT_CARRIED, LC_BROWSER_STMT_PENDING, LC_BROWSER_PAYOUT_REF
// Used by: apps/admin/src/features/settings/routes.ts (settlements), scripts/dev/test-local.sh (--browser-card-payments), tests/foundation/browser_card_payments_test.go
// W4-U1 (contracts/stripe-platform-account-v1.md §6.4; W4-S2 delivery): BFF GET /api/stores/{store}/settlements[/{id}] -> Go read_store_settlements.
// The statements are produced by tests/foundation/w4_u1_seed_test.go only through the operator paths (sync -> close -> payout): one paid, one that
// netted below zero (never paid, carried into the next) and one pending after the carry (its dispute reversal makes the dispute total negative).
// Evidence class: BROWSER, MOCK balance transactions (no Stripe). The amounts on screen are compared with the server's own JSON, formatted by the shared money() function.
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { cardPaymentsSettlementsCopy } from "../../apps/admin/lib/card-payments-settlements-copy";
import { periodLabel } from "../../apps/admin/lib/card-payments-settlements-model";
import { shellCopy } from "../../apps/admin/src/shell-copy";
import { money } from "../../packages/format/src/index";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const paid = required("LC_BROWSER_STMT_PAID");
const carried = required("LC_BROWSER_STMT_CARRIED");
const pending = required("LC_BROWSER_STMT_PENDING");
const payoutRef = required("LC_BROWSER_PAYOUT_REF");

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.describe.configure({ mode: "serial" });

const en = cardPaymentsSettlementsCopy.en;
// the seeded balance-transaction / dispute ids carry the W4U1 tag; real Stripe ids start with the digit 1 ("txn_1Abc..."); "txn_date" is only a JSON key
const leak = /(?:acct_|sk_(?:live|test)_|rk_(?:live|test)_|pk_(?:live|test)_|whsec_|txn_W4U1|dp_W4U1|\btxn_1|\bdp_1|\bre_1)/;

type Line = { order_number: string; kind: string; store_minor: number; fee_store_minor: number; txn_date: string };
type Statement = {
  statement_id: string; period_start: string; period_end: string; currency: string; captured_minor: number; refunded_minor: number;
  dispute_minor: number; stripe_fee_minor: number; platform_fee_minor: number; carried_in_minor: number; net_payable_minor: number;
  line_count: number; paid: boolean; payout_ref?: string; paid_at?: string; lines?: Line[];
};

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await page.getByTestId("nav-orders").waitFor({ state: "attached" });
  const menu = page.locator('button[aria-controls="workspace-navigation"]');
  const drawer = await menu.isVisible();
  if (drawer) await menu.click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  if (drawer) await page.keyboard.press("Escape");
}
const url = (locale: string) => `/${locale}/settings/settlements?store=${store}`;
// The server's own answer through the BFF the page uses (a GET: no state change).
async function listOf(page: Page): Promise<Statement[]> {
  const response = await page.request.get(`/api/stores/${store}/settlements`);
  expect(response.status()).toBe(200);
  return ((await response.json()) as { statements: Statement[] }).statements;
}
async function detailOf(page: Page, id: string): Promise<Statement> {
  const response = await page.request.get(`/api/stores/${store}/settlements/${id}`);
  expect(response.status()).toBe(200);
  return ((await response.json()) as { statement: Statement }).statement;
}
// What the store receives (review P1-1), restated from the 0150 identity (net = captured - refunded - dispute + stripe_fee - platform_fee + carried_in)
// and independent of the page's helper: + is added to the payout, - is deducted.
const contributions = (s: Statement) => [s.captured_minor, -s.refunded_minor, -s.dispute_minor, s.stripe_fee_minor, -s.platform_fee_minor, s.carried_in_minor].map((v) => v + 0);
const signed = (currency: string, minor: number) => (minor > 0 ? "+" : "") + money("en", currency, minor === 0 ? 0 : minor);
// "+NT$1,234.50" / "-NT$8" / "NT$0" back to minor units, to add up what is literally displayed
const minorOf = (text: string): number => {
  const m = /^([+-]?)NT\$([\d,]+)(?:\.(\d{2}))?$/.exec(text);
  expect(m, `not a money string: ${text}`).not.toBeNull();
  const value = Number(m![2].replace(/,/g, "")) * 100 + Number(m![3] ?? 0);
  return m![1] === "-" ? -value : value;
};
const flat = (text: string | null) => (text ?? "").replace(/ /g, " ").trim();
async function noLeaks(page: Page) {
  const html = await page.content();
  expect(html).not.toMatch(leak);
  // G-UI8 audit [READ/MEASURE]: scans client storage for credentials (read only)
  const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
  expect(stored).not.toMatch(leak);
}
const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow of the PAGE (the wide table scrolls inside its own frame)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}

test("STL1 list: every statement row shows the server's numbers (signed) and the right payout state", async ({ page }) => {
  await signedLogin(page);
  await page.goto(url("en"));
  await expect(page.getByTestId("settlements-page")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(shellCopy.en.settlements);
  const statements = await listOf(page);
  expect(statements.map((s) => s.statement_id).sort()).toEqual([paid, carried, pending].sort());
  await expect(page.locator('[data-testid^="settlement-row-"]')).toHaveCount(3);
  const state = { [paid]: "paid", [carried]: "none", [pending]: "pending" } as Record<string, string>;
  const label = { paid: en.paid, none: en.noPayout, pending: en.pending } as Record<string, string>;
  for (const s of statements) {
    const cells = page.getByTestId(`settlement-row-${s.statement_id}`).locator("td");
    const expected = [
      periodLabel(s.period_start, s.period_end),
      ...contributions(s).map((minor) => signed(s.currency, minor)),
      money("en", s.currency, s.net_payable_minor), // the net is the server's own number, never a displayed sum
      label[state[s.statement_id]],
    ];
    for (const [index, text] of expected.entries()) expect(flat(await cells.nth(index).textContent()), `${s.statement_id} column ${index}`).toBe(text);
    await expect(page.getByTestId(`settlement-status-${s.statement_id}`)).toHaveAttribute("data-state", state[s.statement_id]);
    // what is on screen adds up: the six signed columns equal the net cell (review P1-1)
    const shown = [];
    for (let index = 1; index <= 7; index++) shown.push(minorOf(flat(await cells.nth(index).textContent())));
    expect(shown.slice(0, 6).reduce((a, b) => a + b, 0), `${s.statement_id}: displayed columns vs net`).toBe(shown[6]);
    expect(shown[6]).toBe(s.net_payable_minor);
  }
  await expect(page.getByTestId("settlements-legend")).toContainText(en.legend);
  await expect(page.getByTestId("settlements-formula")).toHaveText(en.formula);
  // the three sign cases are really on screen: a negative net, a negative dispute total (reversal), a carried debt
  expect(statements.find((s) => s.statement_id === carried)!.net_payable_minor).toBeLessThan(0);
  expect(statements.find((s) => s.statement_id === pending)!.dispute_minor).toBeLessThan(0);
  expect(statements.find((s) => s.statement_id === pending)!.carried_in_minor).toBeLessThan(0);
  expect(statements.find((s) => s.statement_id === paid)!.stripe_fee_minor).toBeLessThan(0);
  await noLeaks(page);
  await shot(page, "settlements-list", "en", "desktop");
});

test("STL2 detail: lines, payout reference of the paid statement, none for the others; back returns to the list", async ({ page }) => {
  await signedLogin(page);
  await page.goto(url("en"));
  for (const id of [paid, carried, pending]) {
    const s = await detailOf(page, id);
    await page.getByTestId(`settlement-open-${id}`).click();
    await expect(page.getByTestId("settlement-detail")).toBeVisible();
    await expect(page.getByTestId("settlement-detail-period")).toHaveText(periodLabel(s.period_start, s.period_end));
    const detailKeys = ["captured", "refunded", "dispute", "stripe-fee", "platform-fee", "carried-in"];
    for (const [index, minor] of contributions(s).entries())
      await expect(page.getByTestId(`settlement-detail-${detailKeys[index]}`)).toHaveText(signed(s.currency, minor));
    await expect(page.getByTestId("settlement-detail-net")).toHaveText(money("en", s.currency, s.net_payable_minor));
    await expect(page.getByTestId("settlement-detail").getByTestId("settlements-formula")).toHaveText(en.formula);
    const lines = page.locator('[data-testid^="settlement-line-"]');
    await expect(lines).toHaveCount(s.line_count);
    for (const [index, line] of (s.lines ?? []).entries()) {
      const cells = lines.nth(index).locator("td");
      expect(flat(await cells.nth(0).textContent())).toBe(line.order_number);
      expect(flat(await cells.nth(1).textContent())).toBe(en[`kind${line.kind}` as keyof typeof en]);
      expect(flat(await cells.nth(2).textContent())).toBe(signed(s.currency, line.store_minor));
      expect(flat(await cells.nth(3).textContent())).toBe(signed(s.currency, line.fee_store_minor));
      expect(flat(await cells.nth(4).textContent())).toBe(line.txn_date);
    }
    if (id === paid) {
      await expect(page.getByTestId("settlement-detail-status")).toHaveAttribute("data-state", "paid");
      await expect(page.getByTestId("settlement-payout-ref")).toHaveText(payoutRef);
      await expect(page.getByTestId("settlement-paid-at")).toBeVisible();
      await shot(page, "settlement-detail-paid", "en", "desktop");
    } else {
      await expect(page.getByTestId("settlement-detail-status")).toHaveAttribute("data-state", id === carried ? "none" : "pending");
      await expect(page.getByTestId("settlement-payout-ref")).toHaveCount(0);
      await expect(page.getByTestId("settlement-paid-at")).toHaveCount(0);
    }
    // order numbers are the only order identifier: no buyer name, phone, e-mail or Stripe id anywhere in the statement
    const text = await page.getByTestId("settlement-detail").innerText();
    expect(text).not.toMatch(/@|\+?886\d{6,}|\bpi_|\bch_|\bcs_/);
    await noLeaks(page);
    await page.getByTestId("settlement-back").click();
    await expect(page.getByTestId("settlement-detail")).toHaveCount(0);
    await expect(page.locator('[data-testid^="settlement-row-"]')).toHaveCount(3);
  }
});

test("STL3 zh-TW and zh-CN: the same statements in the other languages", async ({ page }) => {
  await signedLogin(page);
  for (const locale of ["zh-TW", "zh-CN"] as const) {
    const copy = cardPaymentsSettlementsCopy[locale];
    await page.goto(url(locale));
    await expect(page.getByTestId(`settlement-status-${paid}`)).toHaveText(copy.paid);
    await expect(page.getByTestId(`settlement-status-${carried}`)).toHaveText(copy.noPayout);
    await expect(page.getByTestId(`settlement-status-${pending}`)).toHaveText(copy.pending);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(shellCopy[locale].settlements);
    if (locale === "zh-TW") await shot(page, "settlements-list", locale, "desktop");
  }
});

test("STL4 empty ledger (BFF answer stubbed in the browser): the empty state, no table", async ({ page }) => {
  await signedLogin(page);
  await page.route(`**/api/stores/${store}/settlements`, (route) =>
    route.fulfill({ status: 200, contentType: "application/json", headers: { "cache-control": "no-store" }, body: JSON.stringify({ statements: [] }) }));
  await page.goto(url("en"));
  await expect(page.getByTestId("settlements-empty")).toContainText(en.empty);
  await expect(page.locator('[data-testid^="settlement-row-"]')).toHaveCount(0);
});

test("STL5 phone width: list and detail fit 390px in en and zh-TW", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await signedLogin(page);
  for (const locale of ["en", "zh-TW"] as const) {
    await page.goto(url(locale));
    await expect(page.getByTestId(`settlement-row-${paid}`)).toBeVisible();
    await shot(page, "settlements-list", locale, "mobile");
    await page.getByTestId(`settlement-open-${paid}`).click();
    await expect(page.getByTestId("settlement-detail")).toBeVisible();
    await shot(page, "settlement-detail", locale, "mobile");
  }
});
