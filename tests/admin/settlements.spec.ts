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
import { parseSettlementDetail, periodLabel } from "../../apps/admin/lib/card-payments-settlements-model";
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

const en = cardPaymentsSettlementsCopy.en;
// the seeded balance-transaction / dispute ids carry the W4U1 tag; real Stripe ids start with the digit 1 ("txn_1Abc..."); "txn_date" is only a JSON key
const leak = /(?:acct_|sk_(?:live|test)_|rk_(?:live|test)_|pk_(?:live|test)_|whsec_|txn_W4U1|dp_W4U1|\btxn_1|\bdp_1|\bre_1)/;

type Line = { order_number: string; kind: string; store_minor: number; fee_store_minor: number; txn_date: string };
type Statement = {
  statement_id: string; period_start: string; period_end: string; currency: string; captured_minor: number; refunded_minor: number;
  dispute_minor: number; stripe_fee_minor: number; platform_fee_minor: number; carried_in_minor: number; net_payable_minor: number;
  returned_in_minor: number; payout_returned: boolean; payout_returned_at?: string;
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
// The server's own answer through the BFF the page uses. An in-page fetch: the Secure __Host- session cookie is sent by the browser, not by the
// APIRequestContext jar (page.request) over http, which answers 401 (as customers-billing.spec.ts / orders-ui.spec.ts do).
async function readBff(page: Page, resource: string): Promise<unknown> {
  // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change)
  const response = await page.evaluate(async (target) => {
    const r = await fetch(target, { credentials: "same-origin", cache: "no-store" });
    return { status: r.status, body: await r.json().catch(() => null) };
  }, `/api/stores/${store}/${resource}`);
  expect(response.status, resource).toBe(200);
  return response.body;
}
async function listOf(page: Page): Promise<Statement[]> {
  return ((await readBff(page, "settlements")) as { statements: Statement[] }).statements;
}
async function detailOf(page: Page, id: string): Promise<Statement> {
  return ((await readBff(page, `settlements/${id}`)) as { statement: Statement }).statement;
}
// What the store receives (review P1-1), restated from the 0150 identity (net = captured - refunded - dispute + stripe_fee - platform_fee + carried_in)
// and independent of the page's helper: + is added to the payout, - is deducted.
const contributions = (s: Statement) => [s.captured_minor, -s.refunded_minor, -s.dispute_minor, s.stripe_fee_minor, -s.platform_fee_minor, s.carried_in_minor, s.returned_in_minor].map((v) => v + 0);
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

// Preserve the original real-chain serial group. The new independent MOCK DTO cases below must still execute
// if SA-1 has not merged yet and the old server cannot emit the required fields; they never make that failure PASS.
test.describe("STL1–STL5 real settlement ledger", () => {
test.describe.configure({ mode: "serial" });
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
    // What is on screen adds up: all seven contributions equal the same server net; old signs stay intact.
    const shown = [];
    for (let index = 1; index <= 8; index++) shown.push(minorOf(flat(await cells.nth(index).textContent())));
    expect(shown.slice(0, 7).reduce((a, b) => a + b, 0), `${s.statement_id}: displayed columns vs net`).toBe(shown[7]);
    expect(shown[7]).toBe(s.net_payable_minor);
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
    const detailKeys = ["captured", "refunded", "dispute", "stripe-fee", "platform-fee", "carried-in", "returned-in"];
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
});

// SA-2: frozen §6.9 DTO fixture, explicitly MOCK at the BFF edge. This does not claim the
// not-yet-merged SA-1 return definer executed. STL1–STL5 retain their existing real Go/PG chain.
type ReturnedStatement = Statement & {
  platform_fee_bps: number;
  payout_minor?: number;
  returned_in_minor: number;
  payout_returned: boolean;
  payout_returned_at?: string;
};
const returnedCopy = {
  "zh-TW": { message: "撥付已退回 · 退回款會計入之後的結算單", amount: "退回款帶入（加計）", status: "撥付已退回" },
  "zh-CN": { message: "拨付已退回 · 退回款会计入之后的结算单", amount: "退回款带入（加计）", status: "拨付已退回" },
  en: { message: "Payout returned · Returned funds will be included in a later settlement", amount: "Returned funds carried in (added)", status: "Payout returned" },
} as const;
type Sa2Locale = keyof typeof returnedCopy;
const clicks: { page: string; control: string; action: string; expected: string; actual: string; result: "PASS"; locale: string; width: number }[] = [];
async function clickProof(page: Page, locale: Sa2Locale, control: string, expected: string, verify: () => Promise<void>) {
  await page.getByTestId(control).click();
  await verify();
  const detail = page.getByTestId("settlement-detail");
  const actual = await detail.count() ? await detail.innerText() : await page.getByTestId("settlements-page").innerText();
  clicks.push({ page: `/${locale}/settings/settlements`, control, action: "click", expected, actual, result: "PASS", locale, width: page.viewportSize()!.width });
  await writeFile(path.join(evidence, "sa2-click-ledger.json"), JSON.stringify(clicks, null, 2));
}
async function returnedFixture(page: Page): Promise<ReturnedStatement[]> {
  const { statements } = JSON.parse(await readFile(new URL("./fixtures/card-payments-wire/settlement-returned.json", import.meta.url), "utf8")) as { statements: ReturnedStatement[] };
  await page.route(`**/api/stores/${store}/settlements**`, async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    const base = `/api/stores/${store}/settlements`;
    if (pathname === base) {
      const list = statements.map(({ lines: _lines, ...row }) => row);
      await route.fulfill({ status: 200, contentType: "application/json", headers: { "cache-control": "no-store" }, body: JSON.stringify({ statements: list }) });
      return;
    }
    const row = statements.find((s) => pathname === `${base}/${s.statement_id}`);
    if (row) await route.fulfill({ status: 200, contentType: "application/json", headers: { "cache-control": "no-store" }, body: JSON.stringify({ statement: row }) });
    else await route.continue();
  });
  return statements;
}

for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  for (const width of [1586, 390]) {
    test(`STL6 returned payout and carried funds survive reload (${locale}, ${width}px, frozen DTO MOCK)`, async ({ page }) => {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
      await signedLogin(page);
      const rows = await returnedFixture(page);
      const source = rows.find((s) => s.payout_returned)!;
      const carrying = rows.find((s) => s.returned_in_minor > 0)!;
      const quiet = rows.find((s) => s.line_count === 0)!;
      const copy = returnedCopy[locale];
      await page.goto(url(locale));
      await expect(page.getByTestId(`settlement-status-${source.statement_id}`)).toHaveText(copy.status);
      await expect(page.getByTestId(`settlement-returned-message-${source.statement_id}`)).toHaveText(copy.message);
      await expect(page.getByRole("columnheader", { name: copy.amount, exact: true })).toBeVisible();
      await expect(page.getByTestId(`settlement-row-${carrying.statement_id}`)).toContainText("+" + money(locale, "TWD", 125000));
      await clickProof(page, locale, `settlement-open-${source.statement_id}`, "returned state, explanation and original payout retained", async () => {
        await expect(page.getByTestId("settlement-detail-status")).toHaveAttribute("data-state", "returned");
        await expect(page.getByTestId(`settlement-returned-message-${source.statement_id}`)).toHaveText(copy.message);
        await expect(page.getByTestId("settlement-returned-at")).toBeVisible();
        await expect(page.getByTestId("settlement-payout-ref")).toHaveText(source.payout_ref!);
      });
      await clickProof(page, locale, "settlement-back", "statement list restored", async () => {
        await expect(page.getByTestId(`settlement-row-${carrying.statement_id}`)).toBeVisible();
      });
      await clickProof(page, locale, `settlement-open-${carrying.statement_id}`, "returned funds add once to the carrying statement", async () => {
        await expect(page.getByTestId("settlement-detail-returned-in")).toHaveText("+" + money(locale, "TWD", 125000));
        await expect(page.getByTestId("settlement-detail-net")).toHaveText(money(locale, "TWD", 173200));
        await expect(page.getByTestId("settlement-detail-status")).toHaveAttribute("data-state", "pending");
        await expect(page.getByTestId("settlement-returned-at")).toHaveCount(0);
      });
      await page.reload();
      await expect(page.getByTestId(`settlement-status-${source.statement_id}`)).toHaveText(copy.status);
      await clickProof(page, locale, `settlement-open-${carrying.statement_id}`, "reload keeps server returned amount and net unchanged", async () => {
        await expect(page.getByTestId("settlement-detail-returned-in")).toHaveText("+" + money(locale, "TWD", 125000));
        await expect(page.getByTestId("settlement-detail-net")).toHaveText(money(locale, "TWD", 173200));
      });
      await shot(page, "sa2-returned-detail", locale, width === 390 ? "mobile" : "desktop");
      await clickProof(page, locale, "settlement-back", "can open a later quiet statement without carrying the return twice", async () => {
        await expect(page.getByTestId(`settlement-row-${quiet.statement_id}`)).toBeVisible();
      });
      await clickProof(page, locale, `settlement-open-${quiet.statement_id}`, "later quiet statement has zero returned funds and net", async () => {
        await expect(page.getByTestId("settlement-detail-returned-in")).toHaveText(money(locale, "TWD", 0));
        await expect(page.getByTestId("settlement-detail-net")).toHaveText(money(locale, "TWD", 0));
      });
      // The real parser also rejects forged return metadata; no permissive legacy defaults or extra private fields.
      expect(parseSettlementDetail({ statement: carrying }).returned_in_minor).toBe(125000);
      for (const patch of [
        { returned_in_minor: -1 }, { returned_in_minor: 1.5 }, { returned_in_minor: 100000000001 },
        { payout_returned: "true" }, { payout_returned_at: "later" }, { return_ref: "PRIVATE-RETURN" },
        { payout_returned: false, payout_returned_at: source.payout_returned_at },
        { payout_returned_at: "2026-09-16T12:00:00+08:00" },
      ]) expect(() => parseSettlementDetail({ statement: { ...source, ...patch } })).toThrow();
      for (const key of ["returned_in_minor", "payout_returned", "payout_returned_at"] as const) {
        const invalid: Record<string, unknown> = { ...source };
        delete invalid[key];
        expect(() => parseSettlementDetail({ statement: invalid })).toThrow();
      }
      await noLeaks(page);
    });

    test(`STL7 settings navigation opens settlements (${locale}, ${width}px)`, async ({ page }) => {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
      await signedLogin(page);
      const rows = await returnedFixture(page);
      await page.goto(`/${locale}/settings/payments/card?store=${store}`);
      const menu = page.locator('button[aria-controls="workspace-navigation"]');
      if (await menu.isVisible()) await menu.click();
      await expect(page.getByTestId("nav-settlements")).toBeVisible();
      await clickProof(page, locale, "nav-settlements", "permission-checked settlement route opens from settings navigation", async () => {
        await expect(page).toHaveURL(new RegExp(`/${locale}/settings/settlements\\?store=${store}$`));
        await expect(page.getByTestId(`settlement-row-${rows[0].statement_id}`)).toBeVisible();
      });
      await shot(page, "sa2-settings-entry", locale, width === 390 ? "mobile" : "desktop");
    });
  }
}
