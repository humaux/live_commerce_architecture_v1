// AT5 merchant half: six genuine storefront bank-transfer Begin orders were created by ads-attribution.mjs.
// The Go runner supplies their draft/date/count/net facts after authoritative PG reads. No session or network mock is used.
import { expect, test, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
import { attributionCopy } from "../../apps/admin/lib/attribution-copy";
import { money } from "../../packages/format/src/index";
import type { Locale } from "../../packages/i18n/src/index";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_STORE");
const evidence = required("LC_BROWSER_EVIDENCE");
type CheckoutFixture = {
  draft_id: string;
  orders: number;
  net_minor: number;
  pending_orders: number;
  pending_minor: number;
  from: string;
  to?: string;
  path?: "ad_click";
};
const fixture: CheckoutFixture = JSON.parse(
  required("LC_ATTRIBUTION_CHECKOUT_FIXTURE"),
);
if (
  !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(fixture.draft_id) ||
  fixture.orders !== 0 ||
  fixture.net_minor !== 0 ||
  fixture.pending_orders !== 0 ||
  fixture.pending_minor !== 0 ||
  !/^\d{4}-\d{2}-\d{2}$/.test(fixture.from) ||
  (fixture.to !== undefined && !/^\d{4}-\d{2}-\d{2}$/.test(fixture.to)) ||
  (fixture.path !== undefined && fixture.path !== "ad_click")
)
  throw new Error(
    "AT5 requires six genuine UNPAID bank-transfer origins verified by PG, with collected orders/net and pending COD orders/amount all zero, and an explicit date window",
  );
const to = fixture.to ?? fixture.from;
test.describe.configure({ mode: "serial" }); // One evidence manifest writer, like the existing offline gate.
test.use({
  baseURL: origin,
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});

async function checkoutDraftFacts(page: Page, locale: Locale) {
  const c = attributionCopy[locale];
  const draft = page.getByTestId("attribution-draft-panel");
  const ours = draft.getByTestId("attribution-ours");
  await expect(draft).toBeVisible();
  await expect(
    ours.getByRole("heading", { name: c.ordersTitle, exact: true }),
  ).toBeVisible();
  const row = ours.getByTestId("attribution-path-ad_click");
  await expect(row).toHaveCount(1);
  await expect(row.getByRole("rowheader")).toHaveText(c.adClick);
  await expect(row.locator("td").nth(0)).toHaveText(String(fixture.orders));
  await expect(row.locator("td").nth(1)).toHaveText(
    money(locale, "TWD", fixture.net_minor),
  );
  await expect(row.locator("td").nth(2)).toHaveText(
    String(fixture.pending_orders),
  );
  await expect(row.locator("td").nth(3)).toHaveText(
    money(locale, "TWD", fixture.pending_minor),
  );
  await expect(ours).not.toContainText(c.metaTitle);
  await expect(
    draft
      .getByTestId("attribution-meta")
      .getByRole("heading", { name: c.metaTitle, exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId("attribution-window")).toContainText(
    `${fixture.from} – ${to}`,
  );
}

async function checkoutReportShot(page: Page, locale: Locale, width: number) {
  const file = `attribution-checkout-${locale}-${width}.png`;
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

for (const locale of ["zh-TW", "zh-CN", "en"] as const)
  for (const width of [390, 1586]) {
    test(`AT5 genuine checkout draft report ${locale} ${width}`, async ({
      page,
    }) => {
      const ledger: {
        control: string;
        action: string;
        expected: string;
        actual: string;
      }[] = [];
      await page.setViewportSize({ width, height: 992 });
      try {
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
          expected: "same-store report",
          actual: "PASS",
        });
        await page.getByTestId("attribution-from").fill(fixture.from);
        await page.getByTestId("attribution-to").fill(to);
        await page.getByTestId("attribution-apply").click();
        await expect(page.getByTestId("attribution-window")).toContainText(
          `${fixture.from} – ${to}`,
        );
        ledger.push({
          control: "from/to/apply",
          action: "fill/fill/click",
          expected: "authoritative checkout date window",
          actual: "PASS",
        });
        await page
          .getByTestId("attribution-draft")
          .selectOption(fixture.draft_id);
        await expect(page.getByTestId("attribution-draft")).toHaveValue(
          fixture.draft_id,
        );
        await expect(page).toHaveURL(new RegExp(`draft=${fixture.draft_id}`));
        await checkoutDraftFacts(page, locale);
        ledger.push({
          control: "attribution-draft",
          action: "selectOption/read DOM",
          expected:
            "six real UNPAID origins; collected orders/net and pending COD orders/amount all zero; separate Meta totals",
          actual: "PASS",
        });
        await page.reload();
        await expect(page.getByTestId("attribution-draft")).toHaveValue(
          fixture.draft_id,
        );
        await checkoutDraftFacts(page, locale);
        ledger.push({
          control: "URL filter recovery",
          action: "reload",
          expected:
            "same draft; six UNPAID origins never count as collected or pending COD",
          actual: "PASS",
        });
        // READ/MEASURE only; no DOM mutation, request interception, direct API shortcut or session prerequisite.
        expect(
          await page.evaluate(
            () => document.documentElement.scrollWidth - innerWidth,
          ),
        ).toBeLessThanOrEqual(1);
        await checkoutReportShot(page, locale, width);
        await page.getByTestId("attribution-back").click();
        await expect(page.getByTestId("merchant-ads")).toBeVisible();
        ledger.push({
          control: "attribution-back",
          action: "click",
          expected: "same-store ads",
          actual: "PASS",
        });
      } finally {
        await writeFile(
          path.join(
            evidence,
            `attribution-checkout-clicks-${locale}-${width}.json`,
          ),
          JSON.stringify(ledger, null, 2),
        );
      }
    });
  }
