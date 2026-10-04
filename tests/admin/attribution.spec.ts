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
  unknown_draft_id: string;
  truncated: true;
  breakdowns_unavailable: { day: string; dimensions: string[] }[];
  unknown_breakdown: { dimension: "hourly"; bucket: string };
  // Runner controls name actual PG sessions; they never modify the report API DTO.
  state_sessions: { insufficient: string; not_authorized: string };
  provisional: true;
  audience_read?: "queued" | "forbidden";
  live_audience: {
    status: "available" | "insufficient" | "not_authorized";
    views: number | null;
    peak_concurrent: number | null;
    total_view_time_ms: number | null;
    age_gender: { bucket: string; view_time_ms: number }[];
    regions: { bucket: string; view_time_ms: number }[];
  };
  forbidden_private?: string[];
  meta_account_timezone: "America/Los_Angeles";
  hourly?: {
    day: string;
    timezone_name: string;
    bucket: string;
    hour_start: string;
    spend_minor: number;
    reach: number | null;
    impressions: number | null;
    clicks: number | null;
    engagements: number | null;
    comments: number | null;
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
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
if (
  !uuid.test(fixture.unknown_draft_id ?? "") ||
  fixture.unknown_draft_id === fixture.draft_id ||
  fixture.truncated !== true ||
  !fixture.breakdowns_unavailable?.length ||
  fixture.unknown_breakdown?.dimension !== "hourly" ||
  !fixture.unknown_breakdown.bucket ||
  fixture.provisional !== true ||
  fixture.meta_account_timezone !== "America/Los_Angeles" ||
  fixture.live_audience?.status !== "available" ||
  !uuid.test(fixture.state_sessions?.insufficient ?? "") ||
  !uuid.test(fixture.state_sessions?.not_authorized ?? "") ||
  new Set([
    fixture.session_id,
    fixture.state_sessions.insufficient,
    fixture.state_sessions.not_authorized,
  ]).size !== 3
)
  throw new Error(
    "R11 requires real PG unknown draft, capped report, unavailable dimension, omitted hourly metrics, and three audience states",
  );

// Independent acceptance strings: a changed translation must fail, not change the expected value with the UI.
const requiredLabels = {
  en: {
    insufficient: "Audience too small; Meta did not provide a profile.",
    notAuthorized: "Reconnect Facebook to authorize audience insights",
    sessionSpend:
      "Ad spend promoting this live post within the selected period",
    truncated: "Only the first 100 records are shown",
    unavailable: "Breakdowns unavailable",
    zone: "Meta account time zone: America/Los_Angeles",
    provisional: "Meta data may still change",
    boosted: "From promoted post",
    collected: "Collected orders",
  },
  "zh-TW": {
    insufficient: "觀眾數不足，Meta 未提供輪廓",
    notAuthorized: "需重新連接 Facebook 以授權觀眾數據",
    sessionSpend: "所選期間內推廣此直播貼文的廣告花費",
    truncated: "僅顯示前 100 筆",
    unavailable: "無法取得分類資料",
    zone: "Meta 帳戶時區: America/Los_Angeles",
    provisional: "Meta 數據可能仍會更新",
    boosted: "受推廣貼文帶來",
    collected: "已收款訂單",
  },
  "zh-CN": {
    insufficient: "观众数不足，Meta 未提供轮廓",
    notAuthorized: "需重新连接 Facebook 以授权观众数据",
    sessionSpend: "所选期间内推广此直播帖文的广告花费",
    truncated: "仅显示前 100 条",
    unavailable: "无法取得分类数据",
    zone: "Meta 账户时区: America/Los_Angeles",
    provisional: "Meta 数据可能仍会更新",
    boosted: "受推广帖文带来",
    collected: "已收款订单",
  },
} as const;
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
  await expect(
    session
      .getByTestId("attribution-session-facts")
      .getByText(requiredLabels[locale].collected, { exact: true }),
  ).toBeVisible();
  await fact(c.net, money(locale, "TWD", expected.net_minor));
  await fact(c.pending, num(expected.pending_orders));
  await fact(c.pendingValue, money(locale, "TWD", expected.pending_minor));
  await fact(
    requiredLabels[locale].sessionSpend,
    money(locale, "TWD", expected.spend_minor),
  );
  await fact(
    c.roas,
    expected.spend_minor === 0
      ? "—"
      : `${new Intl.NumberFormat(locale, {
          minimumFractionDigits: 2,
          maximumFractionDigits: 2,
        }).format(expected.net_minor / expected.spend_minor)}×`,
  );
  const draftROAS = draft
    .locator("dl div")
    .filter({
      has: page.getByText(c.roas, { exact: true }),
    })
    .locator("dd");
  await expect(draftROAS).toHaveText(/^[0-9,]+\.[0-9]{2}×$/);
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
  await expect(draft.getByTestId("attribution-meta-zone")).toHaveText(
    requiredLabels[locale].zone,
  );
  await expect(draft.getByTestId("attribution-provisional")).toHaveText(
    requiredLabels[locale].provisional,
  );
  await expect(
    draft.getByTestId("attribution-path-boosted_post").getByRole("rowheader"),
  ).toHaveText(requiredLabels[locale].boosted);
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
    for (const [i, value] of [
      hour.reach,
      hour.impressions,
      hour.clicks,
      hour.engagements,
      hour.comments,
    ].entries())
      await expect(row.locator("td").nth(5 + i)).toHaveText(
        value === null ? c.unknown : String(value),
      );
    await expect(row.locator("td").nth(10)).toHaveText(
      hour.purchases === null ? c.unknown : String(hour.purchases),
    );
    await expect(row.locator("td").nth(11)).toHaveText(
      hour.purchase_value_minor === null
        ? c.unknown
        : money(locale, "TWD", hour.purchase_value_minor),
    );
  }
  await expect(page.getByTestId("attribution-truncated")).toHaveText(
    requiredLabels[locale].truncated,
  );
  const dimensions: Record<string, string> = {
    age_gender: c.ageGender,
    region: c.region,
    placement: c.placement,
    device: c.device,
    hourly: c.hourly,
  };
  await expect(
    draft.getByTestId("attribution-breakdowns-unavailable"),
  ).toHaveText(
    fixture.breakdowns_unavailable.map(
      (item) =>
        `${item.day} · ${requiredLabels[locale].unavailable}: ${item.dimensions.map((name) => dimensions[name] ?? name).join(", ")}`,
    ),
  );
  const unknownRow = draft
    .getByTestId("attribution-breakdowns")
    .locator("tbody tr")
    .filter({
      has: page.getByRole("rowheader", {
        name: fixture.unknown_breakdown.bucket,
        exact: true,
      }),
    });
  await expect(unknownRow).toHaveCount(1);
  for (let i = 5; i <= 9; i++)
    await expect(unknownRow.locator("td").nth(i)).toHaveText(c.unknown);
}

async function audienceFacts(page: Page, locale: Locale) {
  const expected = fixture.live_audience;
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

async function audienceStateClicks(page: Page, locale: Locale, width: number) {
  const selection = page.getByTestId("attribution-session");
  const panel = page.getByTestId("attribution-live-audience");
  const assertUnknownSpend = async () => {
    // These real PG sessions have distinct posts without any associated insight rows.
    const facts = page.getByTestId("attribution-session-facts");
    for (const label of [
      requiredLabels[locale].sessionSpend,
      attributionCopy[locale].roas,
    ]) {
      await expect(
        facts
          .locator("div")
          .filter({ has: page.getByText(label, { exact: true }) })
          .locator("dd"),
      ).toHaveText("—");
    }
  };
  const assertInsufficient = async () => {
    await expect(selection).toHaveValue(fixture.state_sessions.insufficient);
    await expect(panel.getByTestId("attribution-insufficient")).toHaveText(
      requiredLabels[locale].insufficient,
    );
    await expect(panel.getByTestId("attribution-not-authorized")).toHaveCount(
      0,
    );
    await expect(panel.locator("dl dd")).toHaveText([
      attributionCopy[locale].unknown,
      attributionCopy[locale].unknown,
      attributionCopy[locale].unknown,
    ]);
    await expect(panel.getByRole("table")).toHaveCount(0);
    await assertUnknownSpend();
  };
  await selection.selectOption(fixture.state_sessions.insufficient);
  await expect(page).toHaveURL(
    new RegExp(`session=${fixture.state_sessions.insufficient}`),
  );
  await assertInsufficient();
  await page.reload();
  await assertInsufficient();
  await reportShot(page, locale, width, "insufficient");

  const assertNotAuthorized = async () => {
    await expect(selection).toHaveValue(fixture.state_sessions.not_authorized);
    await expect(panel.getByTestId("attribution-not-authorized")).toHaveText(
      requiredLabels[locale].notAuthorized,
    );
    await expect(panel.getByTestId("attribution-insufficient")).toHaveCount(0);
    await expect(panel.locator("dd")).toHaveCount(0);
    await expect(panel.getByRole("table")).toHaveCount(0);
    await assertUnknownSpend();
  };
  await selection.selectOption(fixture.state_sessions.not_authorized);
  await expect(page).toHaveURL(
    new RegExp(`session=${fixture.state_sessions.not_authorized}`),
  );
  await assertNotAuthorized();
  await page.reload();
  await assertNotAuthorized();
  await reportShot(page, locale, width, "not-authorized");

  const reportURL = page.url();
  await panel.getByTestId("attribution-reconnect").click();
  await expect(page).toHaveURL(
    new RegExp(`/${locale}/settings\\?store=${store}$`),
  );
  await expect(page.getByTestId("metaconnect-card")).toBeVisible();
  await page.reload();
  await expect(page.getByTestId("metaconnect-card")).toBeVisible();
  await page.goto(reportURL); // Return to the report after the reconnect entry's actual click + reload.
  await assertNotAuthorized();

  await selection.selectOption(fixture.session_id);
  await expect(page).toHaveURL(new RegExp(`session=${fixture.session_id}`));
  await visibleFacts(page, locale);
}

async function unknownDraftClicks(page: Page, locale: Locale, width: number) {
  const selection = page.getByTestId("attribution-draft");
  const assertUnknown = async () => {
    await expect(selection).toHaveValue(fixture.unknown_draft_id);
    const facts = page
      .getByTestId("attribution-draft-panel")
      .locator("dl")
      .first();
    await expect(facts.locator("dd")).toHaveText(["—", "—"]);
    await expect(page.getByTestId("attribution-truncated")).toHaveText(
      requiredLabels[locale].truncated,
    );
  };
  await selection.selectOption(fixture.unknown_draft_id);
  await expect(page).toHaveURL(new RegExp(`draft=${fixture.unknown_draft_id}`));
  await assertUnknown();
  await page.reload();
  await assertUnknown();
  await reportShot(page, locale, width, "unknown-draft");
  await selection.selectOption(fixture.draft_id);
  await expect(page).toHaveURL(new RegExp(`draft=${fixture.draft_id}`));
  await visibleFacts(page, locale);
}

async function tableLayout(page: Page, locale: Locale, width: number) {
  const orders = page.getByTestId("attribution-order-paths").first();
  await expect(orders).toBeVisible();
  const headers = await page
    .locator(".attribution-scroll table thead tr")
    .evaluateAll((rows) =>
      rows.map((row) =>
        Array.from(row.children).map((cell) => {
          const range = document.createRange();
          range.selectNodeContents(cell);
          const rect = range.getBoundingClientRect();
          return {
            text: cell.textContent,
            left: rect.left,
            right: rect.right,
            width: rect.width,
            height: rect.height,
          };
        }),
      ),
    );
  await writeFile(
    path.join(evidence, `table-layout-${locale}-${width}.json`),
    JSON.stringify(headers, null, 2),
  );
  expect(headers.length).toBeGreaterThan(0);
  for (const cells of headers) {
    expect(cells.length).toBeGreaterThan(1);
    for (const [index, cell] of cells.entries()) {
      expect(cell.text?.trim().length).toBeGreaterThan(0);
      expect(
        [cell.left, cell.right, cell.width, cell.height].every(Number.isFinite),
      ).toBe(true);
      expect(cell.width).toBeGreaterThan(0);
      expect(cell.height).toBeGreaterThan(0);
      if (index > 0)
        expect(
          cells[index - 1].right,
          `Header text overlaps: ${cells[index - 1].text} / ${cell.text}`,
        ).toBeLessThanOrEqual(cell.left);
    }
  }
  if (width === 390) {
    const region = orders.locator("..");
    expect(
      await region.evaluate((el) => el.scrollWidth - el.clientWidth),
    ).toBeGreaterThan(0);
    expect(await region.evaluate((el) => el.scrollLeft)).toBe(0);
    await region.click({ position: { x: 8, y: 8 } });
    await expect(region).toBeFocused();
    await region.press("ArrowRight");
    await expect
      .poll(() => region.evaluate((el) => el.scrollLeft))
      .toBeGreaterThan(0);
    await region.press("ArrowLeft");
    await expect.poll(() => region.evaluate((el) => el.scrollLeft)).toBe(0);
  }
}

async function reportShot(
  page: Page,
  locale: Locale,
  width: number,
  state?: string,
) {
  const file = `attribution-${locale}-${width}${state ? `-${state}` : ""}.png`;
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
        await audienceStateClicks(page, locale, width);
        await unknownDraftClicks(page, locale, width);
        ledger.push({
          control:
            "unknown draft, capped report, unavailable breakdowns, Facebook reconnect entry",
          action: "selectOption/reload/selectOption/click/reload",
          expected:
            "spend and ROAS remain — after reload; omitted metrics stay unknown; cap and unavailable labels remain visible; settings Page-connect entry opens",
          actual: "PASS",
        });
        ledger.push({
          control:
            "insufficient/not_authorized sessions + Los Angeles/provisional/promoted-post labels + ROAS",
          action: "selectOption/reload/selectOption/reload/selectOption",
          expected:
            "both exact localized states survive reload; unknown metrics stay unknown; primary report facts restored",
          actual: "PASS",
        });
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
        await expect(
          page.getByTestId("ads-attribution").getByRole("alert"),
        ).toHaveText(c.invalid);
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
        await tableLayout(page, locale, width);
        ledger.push({
          control: "report tables",
          action:
            width === 390 ? "measure/click/ArrowRight/ArrowLeft" : "measure",
          expected:
            "nonempty header text does not overlap; mobile local overflow is keyboard reachable",
          actual: "PASS",
        });
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
