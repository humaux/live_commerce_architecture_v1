// Purpose: accept the owner-signed simplified customer journeys with real clicks in three locales.
// Depends on: Playwright, node:fs/promises, node:path; the existing CB11 signed-identity browser fixture.
// Used by: tests/foundation/browser_customers_billing_test.go, --browser-customers-billing in GitHub CI.
// Invariants: MOCK responses retain the existing DTO; no API/contract changes, DOM writes, force clicks or live data.
import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import path from "node:path";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_STORE");
const customer = required("LC_BROWSER_CUSTOMER");
const evidence = required("LC_BROWSER_EVIDENCE");
const name = "示範林一"; // Synthetic signed-frame fixture, never a real buyer.
const copy = {
  "zh-TW": { title: "顧客", search: "姓名開頭／電話末碼", submit: "搜尋", clear: "清除搜尋", view: "查看顧客", back: "返回搜尋結果", orders: "訂單", consent: "同意", privacy: "隱私", download: "下載顧客資料", confirmDownload: "確認下載", erase: "抹除顧客", confirmErase: "確認抹除", checked: "已核對處理與保留範圍", verify: "先核實身分", protected: "收件資料受保護；最多200筆訂單，妥善保管", permissions: "需隱私及顧客／訂單讀取權限", pending: "查看待處理要求", pendingBack: "返回待處理要求", viewData: "查看顧客資料", exportSubmitted: "下載要求已送出", eraseSubmitted: "抹除要求已送出" },
  "zh-CN": { title: "客户", search: "姓名开头／手机号尾号", submit: "搜索", clear: "清空搜索", view: "查看客户", back: "返回搜索结果", orders: "订单", consent: "授权记录", privacy: "隐私", download: "下载客户资料", confirmDownload: "确认下载", erase: "清除客户资料", confirmErase: "确认清除", checked: "已核对清除与保留范围", verify: "先核实申请人身份", protected: "收件信息受保护；最多200笔订单，请妥善保管", permissions: "需要隐私操作及客户／订单查看权限", pending: "查看待处理申请", pendingBack: "返回待处理申请", viewData: "查看客户资料", exportSubmitted: "下载申请已提交", eraseSubmitted: "资料清除申请已提交" },
  en: { title: "Customers", search: "Name prefix / last phone digits", submit: "Search", clear: "Clear search", view: "View customer", back: "Back to results", orders: "Orders", consent: "Consent", privacy: "Privacy", download: "Download customer data", confirmDownload: "Confirm download", erase: "Erase customer data", confirmErase: "Confirm erasure", checked: "I checked what will change and what will be kept", verify: "Verify the requester's identity first", protected: "Protected delivery details; up to 200 orders. Keep the file secure", permissions: "Requires privacy and customer/order read permissions", pending: "View pending request", pendingBack: "Back to pending request", viewData: "View customer data", exportSubmitted: "Download request submitted", eraseSubmitted: "Erasure request submitted" },
} as const; // Expectations transcribed from signed DELIVERY; never imported from implementation copy.
type Locale = keyof typeof copy;
type Ledger = { page: string; control: string; operation: string; expected: string; actual: string; result: "pass" | "fail" }[];
const ledgers = new WeakMap<Page, Ledger>();
test.afterEach(async ({ page }, info) => {
  await writeFile(path.join(evidence, `people-v2-${info.title.replace(/[^a-zA-Z0-9-]/g, "-")}-clicks.json`), JSON.stringify(ledgers.get(page) ?? [], null, 2));
});
async function act(page: Page, control: string, expected: string, action: () => Promise<unknown>, verify: () => Promise<unknown>, operation = "click") {
  const ledger = ledgers.get(page) ?? [];
  ledgers.set(page, ledger);
  const row: Ledger[number] = { page: new URL(page.url()).pathname, control, operation, expected, actual: "", result: "fail" };
  ledger.push(row);
  await test.step(`${control}: ${expected}`, async () => {
    try { await action(); await verify(); row.result = "pass"; row.actual = expected; }
    catch (error) { row.actual = error instanceof Error ? error.message : String(error); throw error; }
  });
}
async function signed(page: Page, width: number) {
  await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
  await page.goto(`${origin}/en/`);
  await act(page, "Sign in with identity service", "signed merchant workspace opens", () => page.getByRole("button", { name: "Sign in with identity service" }).click(), () => expect(page.getByTestId("shell-store-selector")).toBeVisible());
}

async function fixture(page: Page, fault: "none" | "export" | "erase" | "blocked" = "none") {
  // MOCK fixture/fault preparation only. Actual privacy requests below originate solely from UI clicks.
  const row = {
    customer_id: customer, first_seen_at: "2026-09-12T00:00:00Z", last_activity_at: "2026-10-25T00:00:00Z",
    display_name: name, phone_last3: "805", orders_count: 3, paid_orders_count: 3,
    captured_minor: 384000, refunded_minor: 0, currency: "TWD", claims_count: 0,
    platforms: ["facebook"], consents: { marketing_messages: false, ads_personalization: true },
    active: true, imported: false, tags: [],
  };
  const orders = ["11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"].map((id) => ({
    order_id: id, created_at: "2026-10-25T00:00:00Z", updated_at: "2026-10-25T00:00:00Z",
    currency: "TWD", total_minor: 128000, commercial_state: "CONFIRMED", fulfillment_state: "MANUAL_UNASSIGNED",
    payment_state: "CAPTURED", test_mode: true, work_state: "READY", refunded_minor: 0, refund_pending_minor: 0,
    pickup_source: null, payment_mode: "card", collection_state: null, cod_surcharge_minor: null, cod_collect_minor: null,
  }));
  const detail = {
    ...row, orders, claims: [], tags_revision: "a".repeat(64), notes: [],
    consent_history: [
      { purpose: "marketing_messages", channel: "meta_dm", granted: false, source: "buyer_settings", policy_version: "lc-2026-10", occurred_at: "2026-10-20T00:00:00Z" },
      { purpose: "ads_personalization", channel: "meta_ads", granted: true, source: "buyer_settings", policy_version: "lc-2026-10", occurred_at: "2026-10-01T00:00:00Z" },
    ],
    privacy_actions: [] as { kind: string; via: string; completed_at: string; summary: Record<string, number> | null }[],
  };
  const posts: { resource: string; key: string | undefined; body: string | null }[] = [];
  await page.route(`**/api/stores/${store}/customers**`, async (route) => {
    const request = route.request(), url = new URL(request.url());
    const resource = url.pathname.split(`/customers`)[1];
    const json = (value: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", headers: { "Cache-Control": "no-store" }, body: JSON.stringify(value) });
    if (request.method() === "GET") {
      if (resource === "/tags") return json({ items: [] });
      if (resource === "") {
        const q = url.searchParams.get("q") ?? "";
        return json({ items: !q || name.startsWith(q) || "805".endsWith(q) ? [row] : [], next_cursor: "" });
      }
      if (resource === `/${customer}`) return json(detail);
      return route.continue();
    }
    posts.push({ resource, key: request.headers()["idempotency-key"], body: request.postData() });
    if ((fault === "export" && resource.endsWith("/exports")) || (fault === "erase" && resource.endsWith("/erasure"))) return route.abort("failed");
    if (fault === "blocked" && resource.endsWith("/erasure")) return json({ error: { code: "erasure_blocked" } }, 409);
    if (resource.endsWith("/exports")) {
      detail.privacy_actions.push({ kind: "EXPORT", via: "merchant", completed_at: "2026-10-29T00:00:00Z", summary: null });
      return json({ schema: "lc.customer-export.v1", customer: row, orders, consent_history: detail.consent_history });
    }
    if (resource.endsWith("/erasure")) {
      detail.active = false; detail.consents = { marketing_messages: false, ads_personalization: false };
      const summary = { consents_withdrawn: 1, sessions_revoked: 1, snapshots_redacted: 0, bundles_relabelled: 0 };
      detail.privacy_actions.push({ kind: "ERASURE", via: "merchant", completed_at: "2026-10-29T00:00:00Z", summary });
      return json(summary);
    }
    return route.continue();
  });
  return { detail, posts };
}
const mainURL = (locale: Locale) => `${origin}/${locale}/customers?store=${store}`;
const detailURL = (locale: Locale) => `${origin}/${locale}/customers/${customer}?store=${store}`;
async function fits(page: Page) {
  // READ/MEASURE only; no DOM or application state changes.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
async function eraseReady(page: Page) {
  await act(page, "Erase customer data", "signed erasure confirmation opens", () => page.getByTestId("customer-erase").click(), () => expect(page.getByTestId("erase-dialog")).toBeVisible());
  const dialog = page.getByTestId("erase-dialog"), submit = page.getByTestId("erase-submit");
  await expect(submit, "PEOPLE-V2-ERASE-NAME-AND-CHECK: empty confirmation is disabled").toBeDisabled();
  await act(page, "Customer name", "wrong customer name keeps erasure disabled", () => page.getByTestId("erase-typed").fill("other customer"), () => expect(submit).toBeDisabled(), "fill");
  await act(page, "Customer name", "correct name alone does not authorize erasure", () => page.getByTestId("erase-typed").fill(name), () => expect(submit).toBeDisabled(), "fill");
  const scope = dialog.getByRole("checkbox", { name: copy.en.checked, exact: true });
  await expect(scope, "PEOPLE-V2-ERASE-SCOPE: signed confirmation requires an explicit scope checkbox").toBeVisible();
  await act(page, copy.en.checked, "correct name and checked scope enable confirmation", () => scope.check(), () => expect(submit).toBeEnabled(), "check");
  await act(page, copy.en.checked, "unchecking scope disables erasure again", () => scope.uncheck(), () => expect(submit).toBeDisabled(), "uncheck");
  await act(page, copy.en.checked, "checking scope restores the ready state", () => scope.check(), () => expect(submit).toBeEnabled(), "check");
  await expect(dialog).toContainText("Withdraw consent, sign out devices and deactivate the customer.");
  await expect(dialog).toContainText("Handle social data separately.");
  await expect(dialog).toContainText("Completed erasure cannot be undone.");
}

for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const width of [1440, 390]) {
  test(`PEOPLE-V2 signed main path ${locale}-${width}`, async ({ page }) => {
    const c = copy[locale];
    await signed(page, width); await fixture(page); await page.goto(mainURL(locale));
    await expect(page.getByTestId(`customer-open-${customer}`)).toBeVisible(); // Existing baseline reaches new assertion.
    await expect(page.getByTestId("customers-search"), "PEOPLE-V2-SIGNED-SEARCH: signed concise search prompt").toHaveAttribute("placeholder", c.search);
    await fits(page);
    await act(page, c.search, "customer search draft is visible", () => page.getByTestId("customers-search").fill("示範林"), () => expect(page.getByTestId("customers-search")).toHaveValue("示範林"), "fill");
    await act(page, c.submit, "search results retain the customer and query", () => page.getByTestId("customers-search-submit").click(), async () => { await expect(page).toHaveURL(/q=/); await expect(page.getByTestId(`customer-open-${customer}`)).toContainText(name); });
    await act(page, "Refresh", "search persists after reload", () => page.reload(), () => expect(page.getByTestId("customers-search")).toHaveValue("示範林"), "reload");
    await act(page, c.view, "customer profile opens", () => page.getByTestId(`customer-open-${customer}`).click(), () => expect(page.getByTestId("customer-detail")).toContainText(name));
    await expect(page.getByTestId("customer-detail")).toContainText("805");
    await expect(page.getByTestId("customer-detail")).toContainText(customer.slice(0, 8));
    await expect(page.getByTestId("customer-detail")).toContainText(/3,840/);
    await expect(page.getByTestId("customer-detail")).toContainText("2026-09-12");
    await act(page, c.orders, "three server orders are visible", () => page.getByRole("link", { name: c.orders, exact: true }).click(), () => expect(page.getByTestId("customer-orders").locator("tbody tr")).toHaveCount(3));
    await act(page, c.consent, "consent facts remain visible", () => page.getByRole("link", { name: c.consent, exact: true }).click(), () => expect(page.getByTestId("customer-consent")).toContainText("2026-10-20"));
    await act(page, c.privacy, "privacy section opens", () => page.getByRole("link", { name: c.privacy, exact: true }).click(), () => expect(page.getByTestId("customer-download")).toHaveText(c.download));
    await fits(page);
    await act(page, c.back, "return preserves search context", () => page.getByRole("link", { name: c.back, exact: true }).click(), () => expect(page.getByTestId("customers-search")).toHaveValue("示範林"));
    await act(page, c.clear, "clear restores all store customers", () => page.getByRole("button", { name: c.clear, exact: true }).click(), () => expect(page.getByTestId("customers-search")).toHaveValue(""));
    await act(page, c.search, "phone suffix is accepted", () => page.getByTestId("customers-search").fill("805"), () => expect(page.getByTestId("customers-search")).toHaveValue("805"), "fill");
    await act(page, c.submit, "phone search returns the matching customer", () => page.getByTestId("customers-search-submit").click(), () => expect(page.getByTestId(`customer-open-${customer}`)).toContainText(name));
  });
}

for (const width of [1440, 390]) {
  test(`PEOPLE-V2 export confirmation and persistence en-${width}`, async ({ page }) => {
    await signed(page, width); const f = await fixture(page); await page.goto(detailURL("en"));
    await expect(page.getByTestId("customer-download")).toBeVisible();
    await act(page, "Download data", "PEOPLE-V2-EXPORT-CONFIRM: review opens before any POST", () => page.getByTestId("customer-download").click(), () => expect(page.getByRole("button", { name: copy.en.confirmDownload, exact: true })).toBeVisible());
    expect(f.posts).toHaveLength(0);
    for (const fact of [name, "805", copy.en.verify, copy.en.protected, copy.en.permissions]) await expect(page.getByTestId("customer-detail")).toContainText(fact);
    const download = page.waitForEvent("download");
    await act(page, copy.en.confirmDownload, "synchronous export delivers one file", () => page.getByRole("button", { name: copy.en.confirmDownload, exact: true }).click(), async () => { expect((await download).suggestedFilename()).toMatch(/\.json$/); expect(f.posts).toHaveLength(1); });
    await act(page, "Refresh", "completed export remains in server history", () => page.reload(), () => expect(page.getByTestId("privacy-actions")).toContainText("Data export"), "reload");
    expect(f.posts).toHaveLength(1); expect(f.posts[0].key).toBeTruthy(); await fits(page);
  });
  test(`PEOPLE-V2 erasure name checkbox and persistence en-${width}`, async ({ page }) => {
    await signed(page, width); const f = await fixture(page); await page.goto(detailURL("en"));
    await expect(page.getByTestId("customer-erase")).toBeVisible(); await eraseReady(page);
    expect(f.posts).toHaveLength(0);
    await act(page, copy.en.confirmErase, "erasure success reflects server deactivation", () => page.getByTestId("erase-submit").click(), () => expect(page.getByTestId("customer-erased")).toBeVisible());
    expect(f.posts).toHaveLength(1); expect(f.posts[0].body).toBe(JSON.stringify({ confirm: "ERASE" }));
    await act(page, "Refresh", "server erasure and retained orders survive reload", () => page.reload(), async () => { await expect(page.getByTestId("customer-erased")).toBeVisible(); await expect(page.getByTestId("customer-orders").locator("tbody tr")).toHaveCount(3); }, "reload");
    expect(f.posts).toHaveLength(1); await fits(page);
  });
  test(`PEOPLE-V2 blocked erasure retains customer and orders en-${width}`, async ({ page }) => {
    await signed(page, width); const f = await fixture(page, "blocked"); await page.goto(detailURL("en"));
    await expect(page.getByTestId("customer-erase")).toBeVisible(); await eraseReady(page);
    await act(page, copy.en.confirmErase, "server refusal is visible without claiming erasure", () => page.getByTestId("erase-submit").click(), () => expect(page.getByTestId("erase-problem")).toContainText(/payment|order|refund/i));
    expect(f.posts).toHaveLength(1); expect(f.detail.active).toBe(true);
    await expect(page.getByTestId("customer-erased")).toHaveCount(0);
    await act(page, "Refresh", "refused erasure keeps all three server orders", () => page.reload(), async () => { await expect(page.getByTestId("customer-orders").locator("tbody tr")).toHaveCount(3); await expect(page.getByTestId("customer-erased")).toHaveCount(0); }, "reload");
    expect(f.posts).toHaveLength(1);
  });
  for (const kind of ["export", "erase"] as const) {
    test(`PEOPLE-V2 ${kind} uncertain read-only return en-${width}`, async ({ page }) => {
      await signed(page, width); const f = await fixture(page, kind); await page.goto(detailURL("en"));
      if (kind === "erase") { await expect(page.getByTestId("customer-erase")).toBeVisible(); await eraseReady(page); }
      else {
        await expect(page.getByTestId("customer-download")).toBeVisible();
        await act(page, copy.en.download, "PEOPLE-V2-EXPORT-CONFIRM: opens confirmation", () => page.getByTestId("customer-download").click(), () => expect(page.getByRole("button", { name: copy.en.confirmDownload, exact: true })).toBeVisible());
      }
      await act(page, kind === "erase" ? copy.en.confirmErase : copy.en.confirmDownload, "unconfirmed result offers only the signed read-only path", () => kind === "erase" ? page.getByTestId("erase-submit").click() : page.getByRole("button", { name: copy.en.confirmDownload, exact: true }).click(), () => expect(page.getByRole("button", { name: copy.en.pending, exact: true })).toBeVisible());
      expect(f.posts).toHaveLength(1);
      await act(page, copy.en.pending, "existing unconfirmed request entry is visible", () => page.getByRole("button", { name: copy.en.pending, exact: true }).click(), () => expect(page.getByRole("button", { name: copy.en.viewData, exact: true })).toBeVisible());
      await act(page, copy.en.viewData, "customer stays readable with both privacy commands locked", () => page.getByRole("button", { name: copy.en.viewData, exact: true }).click(), async () => { await expect(page.getByTestId("customer-download")).toBeDisabled(); await expect(page.getByTestId("customer-erase")).toBeDisabled(); });
      await act(page, "Refresh", "unconfirmed request is not silently replayed or unlocked", () => page.reload(), async () => { await expect(page.getByTestId("customer-download")).toBeDisabled(); await expect(page.getByTestId("customer-erase")).toBeDisabled(); }, "reload");
      expect(f.posts).toHaveLength(1);
      await act(page, copy.en.pendingBack, "read-only return preserves customer identity", () => page.getByRole("button", { name: copy.en.pendingBack, exact: true }).click(), () => expect(page.getByTestId("customer-detail")).toContainText(name));
      expect(f.posts).toHaveLength(1); await fits(page);
    });
  }
}
