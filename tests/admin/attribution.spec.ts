// AT7/AT9 browser UI half: REAL_PG + real Go/BFF, Meta MOCK in the root-owned runner.
// No response interception or completed-order synthesis. The runner supplies authoritative fixture totals.
// Actual buyer ad URL -> checkout (AT5) is added by the root's storefront/checkout acceptance path.
import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { attributionCopy } from "../../apps/admin/lib/attribution-copy";
import { money } from "../../packages/format/src/index";
import type { Locale } from "@live-commerce/i18n";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"),
  store = required("LC_BROWSER_STORE"),
  evidence = required("LC_BROWSER_EVIDENCE");
type Fixture = {
  audience_read?: "queued" | "forbidden";
  from: string;
  to: string;
  draft_id: string;
  session_id: string;
  expected: {
    orders: number;
    net_minor: number;
    pending_orders: number;
    pending_minor: number;
    spend_minor: number;
    meta_purchases: number | null;
    meta_value_minor: number | null;
    comments: number;
    claims: number;
    checkout_links: number;
    paid_orders: number;
    ambiguous_orders: number;
    new_buyers: number;
    returning_buyers: number;
    counties: { name: string; orders: number; net_minor: number }[];
    top_products: { name: string; quantity: number }[];
  };
};
const fixture: Fixture = JSON.parse(required("LC_ATTRIBUTION_FIXTURE"));
test.use({
  baseURL: origin,
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});

async function login(page: Page) {
  await page.goto("/en/");
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  await expect(page.getByTestId("nav-group-marketing")).toBeVisible();
}
async function visibleFacts(page: Page, locale: Locale) {
  const c = attributionCopy[locale],
    expected = fixture.expected;
  const session = page.getByTestId("attribution-session-panel"),
    draft = page.getByTestId("attribution-draft-panel");
  await expect(session).toBeVisible();
  await expect(draft).toBeVisible();
  const fact = async (label: string, value: string) => {
    const row = session
      .getByTestId("attribution-session-facts")
      .locator("div")
      .filter({ has: page.getByText(label, { exact: true }) });
    await expect(row.locator("dd")).toHaveText(value);
  };
  const num = (n: number) => String(n);
  await fact(c.orders, num(expected.orders));
  await fact(c.net, money(locale, "TWD", expected.net_minor));
  await fact(c.pending, num(expected.pending_orders));
  await fact(c.pendingValue, money(locale, "TWD", expected.pending_minor));
  await fact(c.spend, money(locale, "TWD", expected.spend_minor));
  await expect(
    session.getByTestId("attribution-funnel").locator("tbody td"),
  ).toHaveText(
    [
      expected.comments,
      expected.claims,
      expected.checkout_links,
      expected.paid_orders,
    ].map(String),
  );
  if (expected.ambiguous_orders)
    await expect(session.getByTestId("attribution-ambiguous")).toHaveText(
      `${c.ambiguous}: ${num(expected.ambiguous_orders)}`,
    );
  const meta = draft.getByTestId("attribution-meta");
  await expect(meta.locator("dd")).toHaveText([
    expected.meta_purchases === null ? c.unknown : num(expected.meta_purchases),
    expected.meta_value_minor === null
      ? c.unknown
      : money(locale, "TWD", expected.meta_value_minor),
  ]);
  await expect(draft.getByTestId("attribution-ours")).not.toContainText(
    c.metaTitle,
  );
  const buyers = session.getByTestId("attribution-buyers");
  await expect(
    buyers
      .locator("dt")
      .filter({ hasText: c.newBuyers })
      .locator("..")
      .locator("dd"),
  ).toHaveText(num(expected.new_buyers));
  await expect(
    buyers
      .locator("dt")
      .filter({ hasText: c.returning })
      .locator("..")
      .locator("dd"),
  ).toHaveText(num(expected.returning_buyers));
  for (const county of expected.counties) {
    const row = buyers
      .getByTestId("attribution-counties")
      .locator("tbody tr")
      .filter({ hasText: county.name });
    await expect(row.locator("td")).toHaveText([
      num(county.orders),
      money(locale, "TWD", county.net_minor),
    ]);
  }
  for (const product of expected.top_products)
    await expect(
      buyers
        .getByTestId("attribution-products")
        .locator("tbody tr")
        .filter({ hasText: product.name })
        .locator("td"),
    ).toHaveText(num(product.quantity));
  await expect(session.getByTestId("attribution-timeline")).toBeVisible();
}

for (const locale of ["en", "zh-TW", "zh-CN"] as const)
  for (const width of [390, 1586]) {
    test(`AT7/AT9 actual report clicks ${locale} ${width}`, async ({
      page,
    }) => {
      const c = attributionCopy[locale],
        ledger: {
          control: string;
          action: string;
          expected: string;
          actual: string;
        }[] = [];
      await page.setViewportSize({ width, height: 992 });
      await login(page);
      await page.goto(`/${locale}/ads?store=${store}`);
      await page.getByTestId("ads-attribution-link").click();
      await expect(page.getByTestId("ads-attribution")).toBeVisible();
      ledger.push({
        control: "ads-attribution-link",
        action: "click",
        expected: "report page",
        actual: "PASS",
      });
      await page.getByTestId("attribution-from").fill(fixture.from);
      await page.getByTestId("attribution-to").fill(fixture.to);
      await page.getByTestId("attribution-apply").click();
      await expect(page.getByTestId("attribution-window")).toContainText(
        `${fixture.from} – ${fixture.to}`,
      );
      ledger.push({
        control: "from/to/apply",
        action: "fill/fill/click",
        expected: "fixture date window",
        actual: "PASS",
      });
      await page
        .getByTestId("attribution-draft")
        .selectOption(fixture.draft_id);
      await expect(page).toHaveURL(new RegExp(`draft=${fixture.draft_id}`));
      await page
        .getByTestId("attribution-session")
        .selectOption(fixture.session_id);
      await expect(page).toHaveURL(new RegExp(`session=${fixture.session_id}`));
      await visibleFacts(page, locale);
      ledger.push({
        control: "draft/session",
        action: "selectOption/selectOption",
        expected: "exact fixture facts",
        actual: "PASS",
      });
      const audiencePath = `/api/stores/${store}/ads/sessions/${fixture.session_id}/audience-read`;
      const observeRead = () =>
        page.waitForResponse(
          (r) =>
            r.request().method() === "POST" &&
            new URL(r.url()).pathname === audiencePath,
        );
      const firstRead = observeRead();
      await page.getByTestId("attribution-audience-refresh").click();
      const firstResponse = await firstRead;
      const firstKey = firstResponse.request().headers()["idempotency-key"];
      expect(firstKey).toMatch(/^audience-read-[0-9a-f-]+$/);
      if (fixture.audience_read === "forbidden") {
        expect(firstResponse.status()).toBe(403);
        await expect(
          page.getByTestId("attribution-audience-problem"),
        ).toHaveText(c.audienceForbidden);
      } else {
        expect(firstResponse.ok()).toBe(true);
        expect((await firstResponse.json()).state).toBe("READY");
        await expect(
          page.getByTestId("attribution-audience-queued"),
        ).toHaveText(c.audienceQueued);
      }
      ledger.push({
        control: "audience-refresh",
        action: "click",
        expected: fixture.audience_read ?? "queued READY, not fetched",
        actual: "PASS",
      });
      await page.reload();
      await expect(page.getByTestId("attribution-draft")).toHaveValue(
        fixture.draft_id,
      );
      await expect(page.getByTestId("attribution-session")).toHaveValue(
        fixture.session_id,
      );
      await visibleFacts(page, locale);
      if (fixture.audience_read !== "forbidden") {
        await expect(
          page.getByTestId("attribution-audience-queued"),
        ).toHaveText(c.audienceQueued);
        const nextRead = observeRead();
        await page.getByTestId("attribution-audience-refresh").click();
        const nextResponse = await nextRead;
        expect(nextResponse.ok()).toBe(true);
        expect(nextResponse.request().headers()["idempotency-key"]).not.toBe(
          firstKey,
        );
        await expect(
          page.getByTestId("attribution-audience-queued"),
        ).toHaveText(c.audienceQueued);
        ledger.push({
          control: "audience-refresh new read",
          action: "reload/click",
          expected: "queued survives reload, new explicit request gets new key",
          actual: "PASS",
        });
      }
      ledger.push({
        control: "URL filters",
        action: "reload",
        expected: "same window and selection",
        actual: "PASS",
      });
      await page.getByTestId("attribution-from").fill("2020-01-01");
      await page.getByTestId("attribution-apply").click();
      await expect(page.getByRole("alert")).toHaveText(c.invalid);
      ledger.push({
        control: "invalid range",
        action: "fill/click",
        expected: "bounded-range error",
        actual: "PASS",
      });
      await page.getByTestId("attribution-from").fill(fixture.from);
      await page.getByTestId("attribution-apply").click();
      await visibleFacts(page, locale);
      // READ/MEASURE only: checks layout; never changes DOM, values or backend state.
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - innerWidth,
        ),
      ).toBeLessThanOrEqual(1);
      await page.screenshot({
        path: path.join(evidence, `attribution-${locale}-${width}.png`),
        fullPage: true,
      });
      await page.getByTestId("attribution-back").click();
      await expect(page.getByTestId("merchant-ads")).toBeVisible();
      ledger.push({
        control: "attribution-back",
        action: "click",
        expected: "same store ads",
        actual: "PASS",
      });
      await writeFile(
        path.join(evidence, `attribution-clicks-${locale}-${width}.json`),
        JSON.stringify(ledger, null, 2),
      );
    });
  }
