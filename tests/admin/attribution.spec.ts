// AT7/AT9 browser UI half: REAL_PG + real Go/BFF, Meta MOCK in the root-owned runner.
// No response interception or completed-order synthesis. The runner supplies authoritative fixture totals.
// Actual buyer ad URL -> checkout (AT5) is added by the root's storefront/checkout acceptance path.
import { expect, test, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
import { attributionCopy } from "../../apps/admin/lib/attribution-copy";
import { displayTime, money } from "../../packages/format/src/index";
import type { Locale } from "../../packages/i18n/src/index";

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
  live_audience?: {
    status: "available" | "insufficient" | "not_authorized";
    views: number | null;
    peak_concurrent: number | null;
    total_view_time_ms: number | null;
    age_gender: { bucket: string; view_time_ms: number }[];
    regions: { bucket: string; view_time_ms: number }[];
  };
  forbidden_private?: string[];
  meta_account_timezone?: string;
  hourly?: {
    day: string;
    timezone_name: string;
    bucket: string;
    hour_start: string;
    spend_minor: number;
    reach: number;
    impressions: number;
    clicks: number;
    engagements: number;
    comments: number;
    purchases: number | null;
    purchase_value_minor: number | null;
  }[];
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
test.describe.configure({ mode: "serial" }); // Preserve one manifest writer in the existing runner.

async function login(page: Page, width: number) {
  await page.goto("/en/");
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  if (width === 390)
    await page
      .getByRole("button", { name: "Open navigation", exact: true })
      .click();
  await expect(page.getByTestId("nav-group-marketing")).toBeVisible();
  if (width === 390) {
    await page
      .locator("[data-shell-rail]")
      .getByRole("button", { name: "Close navigation", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Open navigation", exact: true }),
    ).toHaveAttribute("aria-expanded", "false");
    await expect(page.locator("[data-shell-rail]")).toBeHidden();
  }
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
  await audienceFacts(page, locale);
  for (const privateString of fixture.forbidden_private ?? []) {
    expect(privateString.length).toBeGreaterThan(0);
    await expect(page.locator("body")).not.toContainText(privateString);
  }
  if (fixture.meta_account_timezone)
    await expect(draft).toContainText(
      `${c.metaZone}: ${fixture.meta_account_timezone}`,
    );
  for (const hour of fixture.hourly ?? []) {
    const row = draft
      .getByTestId("attribution-breakdowns")
      .locator("tbody tr")
      .filter({
        has: page.getByRole("rowheader", { name: hour.bucket, exact: true }),
      })
      .filter({ hasText: hour.day });
    await expect(row).toHaveCount(1);
    await expect(row.locator("td").nth(0)).toHaveText(hour.day);
    await expect(row.locator("td").nth(1)).toHaveText(hour.timezone_name);
    await expect(row.locator("td").nth(2)).toHaveText(c.hourly);
    await expect(row.locator("td").nth(3)).toHaveText(
      displayTime(locale, hour.hour_start),
    );
    await expect(row.locator("td").nth(4)).toHaveText(
      money(locale, "TWD", hour.spend_minor),
    );
    await expect(row.locator("td").nth(5)).toHaveText(String(hour.reach));
    await expect(row.locator("td").nth(6)).toHaveText(String(hour.impressions));
    await expect(row.locator("td").nth(7)).toHaveText(String(hour.clicks));
    await expect(row.locator("td").nth(8)).toHaveText(String(hour.engagements));
    await expect(row.locator("td").nth(9)).toHaveText(String(hour.comments));
    await expect(row.locator("td").nth(10)).toHaveText(
      hour.purchases === null ? c.unknown : String(hour.purchases),
    );
    await expect(row.locator("td").nth(11)).toHaveText(
      hour.purchase_value_minor === null
        ? c.unknown
        : money(locale, "TWD", hour.purchase_value_minor),
    );
  }
}

async function audienceFacts(page: Page, locale: Locale) {
  const expected = fixture.live_audience;
  if (!expected) return; // Existing runners may omit this additional expectation; supplied facts are exact.
  const c = attributionCopy[locale];
  const panel = page.getByTestId("attribution-live-audience");
  await expect(panel).toBeVisible();
  if (expected.status === "not_authorized") {
    await expect(panel.getByTestId("attribution-not-authorized")).toHaveText(
      c.notAuthorized,
    );
    await expect(panel.locator("dd")).toHaveCount(0);
    await expect(panel.getByRole("table")).toHaveCount(0);
    return;
  }
  await expect(panel.getByTestId("attribution-not-authorized")).toHaveCount(0);
  // Exact values make nulls distinguishable from fabricated zeros.
  await expect(panel.locator("dl dt")).toHaveText([
    c.views,
    c.peak,
    c.totalTime,
  ]);
  await expect(panel.locator("dl dd")).toHaveText(
    [expected.views, expected.peak_concurrent, expected.total_view_time_ms].map(
      (value) => (value === null ? c.unknown : String(value)),
    ),
  );
  if (
    expected.status === "insufficient" ||
    (!expected.age_gender.length && !expected.regions.length)
  ) {
    await expect(panel.getByTestId("attribution-insufficient")).toHaveText(
      c.insufficient,
    );
    await expect(panel.getByRole("table")).toHaveCount(0);
    return;
  }
  await expect(panel.getByTestId("attribution-insufficient")).toHaveCount(0);
  await expect(panel.getByRole("table")).toHaveCount(
    Number(expected.age_gender.length > 0) +
      Number(expected.regions.length > 0),
  );
  for (const [label, buckets] of [
    [c.ageGender, expected.age_gender],
    [c.region, expected.regions],
  ] as const) {
    const table = panel.getByRole("table").filter({
      has: page.getByRole("columnheader", { name: label, exact: true }),
    });
    if (!buckets.length) await expect(table).toHaveCount(0);
    else {
      await expect(table.locator("thead th")).toHaveText([label, c.viewTime]);
      await expect(table.locator("tbody tr")).toHaveCount(buckets.length);
      await expect(table.locator("tbody th")).toHaveText(
        buckets.map((bucket) => bucket.bucket),
      );
      await expect(table.locator("tbody td")).toHaveText(
        buckets.map((bucket) => String(bucket.view_time_ms)),
      );
    }
  }
}

async function reportShot(page: Page, locale: Locale, width: number) {
  const file = `attribution-${locale}-${width}.png`;
  await page.screenshot({ path: path.join(evidence, file), fullPage: true });
  const manifestPath = path.join(evidence, "screenshots.json");
  let manifest: {
    File: string;
    Sha256: string;
    Locale: string;
    Viewport: string;
  }[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
  manifest.push({
    File: file,
    Sha256: createHash("sha256")
      .update(await readFile(path.join(evidence, file)))
      .digest("hex"),
    Locale: locale,
    Viewport: String(width),
  });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
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
      try {
        await page.setViewportSize({ width, height: 992 });
        await login(page, width);
        ledger.push({
          control: "identity sign-in / mobile navigation",
          action: width === 390 ? "click/open/assert/close" : "click/assert",
          expected:
            "authenticated marketing navigation visible; mobile drawer closed",
          actual: "PASS",
        });
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
        await expect(page).toHaveURL(
          new RegExp(`session=${fixture.session_id}`),
        );
        await visibleFacts(page, locale);
        ledger.push({
          control: "audience/hourly/privacy facts",
          action: "read DOM",
          expected: "exact runner values, nulls unknown, no private strings",
          actual: "PASS",
        });
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
            expected:
              "queued survives reload, new explicit request gets new key",
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
        await expect(page.getByTestId("ads-attribution").getByRole("alert")).toHaveText(c.invalid);
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
        await reportShot(page, locale, width);
        await page.getByTestId("attribution-back").click();
        await expect(page.getByTestId("merchant-ads")).toBeVisible();
        ledger.push({
          control: "attribution-back",
          action: "click",
          expected: "same store ads",
          actual: "PASS",
        });
      } finally {
        await writeFile(
          path.join(evidence, `attribution-clicks-${locale}-${width}.json`),
          JSON.stringify(ledger, null, 2),
        );
      }
    });
  }
