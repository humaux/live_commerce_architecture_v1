// Purpose: PARITY-STATES signed panels on real admin routes, including shared loading/error/empty and session recovery.
// Depends on: Playwright, existing orders-ui signed-login fixture, browser-evidence, three independent locale expectations.
// Used by: orders-ui.spec.ts / browser-merchant-orders-ui CI; not a gallery or a standalone server.
// Invariants: MOCK read responses only; all recovery actions are real clicks; no assertion or fixture in the parent gate is replaced.
import { test, expect, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { browserEvidenceDirectory } from "../browser-evidence.mjs";

const locales = {
  "zh-TW": { title: "目前無法開啟此頁", body: "此連結無法開啟，或目前帳號沒有此操作權限。請返回可用的功能。", note: "若需這項功能，請向店鋪擁有者確認權限。", home: "返回工作台", retry: "重試", expired: "登入已過期，請重新登入後繼續。", signIn: "重新登入" },
  "zh-CN": { title: "暂时无法打开此页面", body: "此链接无法打开，或当前账号没有操作权限。请返回可用的功能。", note: "如需使用此功能，请向店铺所有者确认权限。", home: "返回工作台", retry: "重试", expired: "登录已过期，请重新登录后继续。", signIn: "重新登录" },
  en: { title: "This page cannot be opened", body: "This link cannot be opened, or your account cannot use this feature. Return to an available feature.", note: "Ask the store owner if you need access to this feature.", home: "Back to workspace", retry: "Try again", expired: "Your session has expired. Sign in again to continue.", signIn: "Sign in again" },
};

export function registerParityStates(signedLogin: (page: Page) => Promise<void>) {
  for (const [locale, c] of Object.entries(locales)) {
    for (const width of [1440, 390]) {
      const height = width === 1440 ? 900 : 844;
      test(`PARITY-STATES admin signed forbidden ${locale} ${width}`, async ({ page }) => {
        await signedLogin(page);
        await page.setViewportSize({ width, height });
        const store = process.env.LC_BROWSER_ORDER_STORE!;
        // MOCK fixture: only the existing read envelope is denied, never a DOM/ACL rewrite.
        await page.route(`**/api/stores/${store}/orders?*`, route => route.fulfill({ status: 403, json: { code: "forbidden" } }));
        await page.goto(`${process.env.LC_BROWSER_PUBLIC_ORIGIN}/${locale}/orders?store=${store}`);
        const panel = page.locator('[data-variant="forbidden"]');
        await expect(panel.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
        await expect(panel).toContainText(c.body);
        await expect(panel).toContainText(c.note);
        await expect(page.getByText(/403\s*·/)).toHaveCount(0);
        await expect(page.getByTestId("orders-table")).toHaveCount(0);
        const action = panel.getByRole("link", { name: c.home, exact: true });
        await expect(action).toHaveAttribute("href", `/${locale}`);
        if (locale === "zh-TW") {
          const dir = browserEvidenceDirectory("parity");
          await mkdir(dir, { recursive: true });
          await page.screenshot({ path: `${dir}/${width === 1440 ? "1429-34986" : "1429-35006"}.png` });
        }
        await page.reload();
        await expect(panel.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
        await action.click();
        await expect(page).toHaveURL(new RegExp(`/${locale}/?$`));
        const dir = browserEvidenceDirectory("parity");
        await mkdir(dir, { recursive: true });
        await writeFile(`${dir}/admin-forbidden-${locale}-${width}-clicks.json`, JSON.stringify([{ route: `/${locale}/orders`, control: c.home, action: "click", expected: `/${locale}`, actual: page.url(), result: "PASS", refresh: "denial persists" }], null, 2));
      });

      test(`PARITY-STATES admin shared read states ${locale} ${width}`, async ({ page }) => {
        await signedLogin(page);
        await page.setViewportSize({ width, height });
        const store = process.env.LC_BROWSER_ORDER_STORE!;
        let state: "loading" | "failed" | "empty" = "loading";
        let release!: () => void;
        const held = new Promise<void>(resolve => { release = resolve; });
        const reads: string[] = [];
        await page.route(`**/api/stores/${store}/orders?*`, async route => {
          reads.push(route.request().method());
          if (state === "loading") await held;
          if (state === "failed") await route.fulfill({ status: 503, json: { code: "unavailable" } });
          else await route.fulfill({ status: 200, headers: { "cache-control": "private, no-store" }, json: { items: [], next_cursor: "", total: 0, counts: { all: 0, unpaid: 0, transfer_review: 0, ready_to_ship: 0, ready_to_consign: 0, shipped: 0, completed: 0, cancelled: 0 }, sessions: [] } });
        });
        await page.goto(`${process.env.LC_BROWSER_PUBLIC_ORIGIN}/${locale}/orders?store=${store}`);
        try {
          const loading = page.getByTestId("merchant-orders").getByRole("status");
          await expect(loading).toBeVisible();
          await expect(loading.locator('[aria-hidden="true"]')).toBeVisible();
          state = "failed";
          release();
          const panel = page.getByTestId("merchant-orders").locator('[data-variant="offline"]');
          await expect(panel).toBeVisible();
          const retry = panel.getByRole("button", { name: c.retry, exact: true });
          await expect(retry).toBeEnabled();
          await retry.click();
          await expect(panel.getByRole("alert")).toBeVisible();
          state = "empty";
          await retry.click();
          await expect(page.getByTestId("merchant-orders").locator('[data-variant="empty"]')).toBeVisible();
          await expect(panel).toHaveCount(0);
          await page.reload();
          await expect(page.getByTestId("merchant-orders").locator('[data-variant="empty"]')).toBeVisible();
          expect(reads.every(method => method === "GET")).toBe(true);
          const dir = browserEvidenceDirectory("parity");
          await mkdir(dir, { recursive: true });
          await writeFile(`${dir}/admin-read-${locale}-${width}-clicks.json`, JSON.stringify([{ control: c.retry, action: "click", expected: "failure remains with InlineError", result: "PASS" }, { control: c.retry, action: "click", expected: "successful empty response; survives refresh; GET only", result: "PASS", reads }], null, 2));
        } finally { release(); }
      });

      test(`PARITY-STATES admin session expired ${locale} ${width}`, async ({ page }) => {
        await signedLogin(page);
        await page.setViewportSize({ width, height });
        // MOCK session expiry at the shell's existing BFF boundary; no token changes.
        await page.route("**/api/stores", route => route.fulfill({ status: 401, json: { code: "unauthorized" } }));
        await page.goto(`${process.env.LC_BROWSER_PUBLIC_ORIGIN}/${locale}/orders?store=${process.env.LC_BROWSER_ORDER_STORE}`);
        const panel = page.getByTestId("shell-session-expired");
        await expect(panel.locator('[data-variant="expired"]')).toBeVisible();
        await expect(panel).toContainText(c.expired);
        await expect(page.getByText(/403\s*·/)).toHaveCount(0);
        await panel.getByRole("link", { name: c.signIn, exact: true }).click();
        await expect(page).toHaveURL(new RegExp(`/${locale}/?$`));
      });

      test(`PARITY-STATES admin shell removes raw diagnostic ${locale} ${width}`, async ({ page }) => {
        await signedLogin(page);
        await page.setViewportSize({ width, height });
        // MOCK fixture: the server projection remains authoritative in production.
        // Narrow this disposable identity's displayed read grants to exercise the real shell branch.
        await page.route("**/api/stores", async route => {
          const response = await route.fetch();
          const body = await response.json();
          await route.fulfill({ response, json: { ...body, items: body.items.map((item: { permissions?: string[] }) => ({ ...item, permissions: (item.permissions ?? []).filter(permission => permission !== "orders:read") })) } });
        });
        await page.goto(`${process.env.LC_BROWSER_PUBLIC_ORIGIN}/${locale}/orders?store=${process.env.LC_BROWSER_ORDER_STORE}`);
        const denied = page.getByTestId("route-forbidden");
        await expect(denied.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
        await expect(page.getByText(/403\s*·/)).toHaveCount(0);
        await denied.getByRole("link", { name: c.home, exact: true }).click();
        await expect(page).toHaveURL(new RegExp(`/${locale}/?(?:\\?store=|$)`));
      });
    }
  }
}
