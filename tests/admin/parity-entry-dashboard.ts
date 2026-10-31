// Purpose: Exercise signed dashboard states on the real root route after the password gate creates a store.
// Depends on: Playwright; the caller's real signed store; password-auth evidence directory.
// Used by: password-auth.spec.ts full-chain case (registered password-auth browser gate).
// Invariants: no alternate gallery, DOM mutations, forced clicks, invented metrics, or direct API writes.
import { expect, type Page } from "@playwright/test";
import { appendFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

/** Checks the page content; PARITY-SHELL owns the surrounding navigation. */
export async function dashboardParity(page: Page, evidence: string) {
  mkdirSync(join(evidence, "parity"), { recursive: true });
  const root = new URL(page.url());
  // FIXTURE/SETUP: synthetic existing-response documents make nonempty design,
  // configured-delivery and Meta capability states reachable without provider calls.
  // Each write below is still issued by a real UI click and checked as a CAS.
  let draft = {
    version: 1, updated_at: "2026-10-30T00:00:00Z", published_version: null,
    document: {
      profile: { name: "Browser Store", tagline: null, logo_image_id: null, favicon_image_id: null, accent_color: "#247965", announcement: null,
        contact: { email: null, phone: null, address: null, line_url: null, facebook_url: null, instagram_url: null } },
      nav: { header: [{ label: "所有商品", kind: "all_products", target: null }], footer: [] },
      home: { sections: [] }, pages: [],
    },
  };
  const dashboardURL = "**/api/stores/*/tools/dashboard";
  // FIXTURE only: an existing closed dashboard response, including a historical offline order; never a real order write.
  const order = { order_id: "33333333-3333-4333-8333-333333333333", created_at: "2026-10-30T01:00:00.000000Z", updated_at: "2026-10-30T01:00:00.000000Z",
    currency: "TWD", total_minor: 2500, commercial_state: "AWAITING_TRANSFER", fulfillment_state: "MANUAL_UNASSIGNED", payment_state: "NOT_STARTED",
    test_mode: false, work_state: "NONE", refunded_minor: 0, refund_pending_minor: 0, pickup_source: null, payment_mode: "bank_transfer",
    collection_state: null, cod_surcharge_minor: null, cod_collect_minor: null, source: "merchant_manual" };
  await page.route(dashboardURL, route => route.fulfill({ json: { generated_at: "2026-10-30T02:00:00Z", timezone: "Asia/Taipei",
    orders: { today: 2, last_7_days: 5 }, gmv: [],
    todos: { awaiting_transfer_confirmation: 1, to_ship: 2, cvs_awaiting_label: 1, low_stock_skus: 0, open_refunds: 0 }, latest_orders: [order],
  } }));
  const designURL = "**/api/stores/*/design/draft";
  await page.route(designURL, async route => {
    if (route.request().method() === "PUT") {
      const sent = route.request().postDataJSON();
      expect(sent.expected_version).toBe(draft.version);
      expect(sent.document.profile).toEqual(draft.document.profile);
      expect(sent.document.home).toEqual(draft.document.home);
      expect(sent.document.pages).toEqual(draft.document.pages);
      expect(route.request().headers()["x-csrf-token"]).toBeTruthy();
      draft = { ...draft, version: draft.version + 1, document: sent.document };
    } else expect(route.request().method()).toBe("GET");
    await route.fulfill({ json: draft });
  });
  const market = { id: "11111111-1111-4111-8111-111111111111", code: "taiwan", name: "Taiwan", currency: "TWD", version: 1, active: true };
  const services = ["home", "cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"].map((kind, index) => ({
    market_id: market.id, country: "TW", code: kind, version: 1, policy_method: kind, policy_version: 1, currency: "TWD",
    name_hans: kind, name_hant: kind, name_en: kind, delivery_kind: kind, mode: "MANUAL", enabled: index < 2, visible: true, sort_order: index,
  }));
  const marketsURL = /\/api\/stores\/[^/]+\/markets(?:\?|$)/;
  const servicesURL = /\/api\/stores\/[^/]+\/markets\/[^/]+\/countries\/TW\/delivery-services(?:\/[^/?]+(?:\/policy)?)?(?:\?.*)?$/;
  await page.route(marketsURL, route => route.fulfill({ json: { items: [market], next_cursor: "" } }));
  await page.route(servicesURL, async route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("delivery-services")) { await route.fulfill({ json: { items: services, next_cursor: "" } }); return; }
    const service = services.find(item => path.split("/").includes(item.code));
    expect(service).toBeDefined();
    if (path.endsWith("/policy")) {
      await route.fulfill({ json: { market_id: market.id, country: "TW", method: service!.policy_method, currency: "TWD", shipping_mode: "country_flat", tax_mode: "none", tax_basis: "goods", version: 1, shipping_minor: 0, tax_rate_bps: 0, quote_ttl_seconds: 600, enabled: true } });
      return;
    }
    if (route.request().method() === "PUT") {
      const sent = route.request().postDataJSON();
      expect(sent.expected_version).toBe(service!.version);
      expect(route.request().headers()["idempotency-key"]).toBeTruthy();
      service!.enabled = sent.enabled;
      service!.version++;
    }
    await route.fulfill({ json: service });
  });
  const healthURL = "**/api/stores/*/meta/health";
  const recheckURL = "**/api/stores/*/meta/health/recheck";
  await page.route(healthURL, route => route.fulfill({ json: { severity: "none", pages: [{ page_id: "123456", page_name: "Synthetic page", status: "active", severity: "none", capabilities: [
    { binding_id: "22222222-2222-4222-8222-222222222222", provider: "facebook", capability: "read_comment", state: "ok", reason: "probe_ok", evidence: "MOCK" },
  ] }] } }));
  await page.route(recheckURL, route => {
    expect(route.request().method()).toBe("POST");
    expect(route.request().postDataJSON()).toEqual({ page_id: "123456" });
    return route.fulfill({ status: 202, json: { next_check_at: "2026-10-30T00:01:00Z" } });
  });
  const location = (view = "", locale = "zh-TW") => {
    const url = new URL(root);
    url.pathname = `/${locale}/`;
    if (view) url.searchParams.set("view", view);
    else url.searchParams.delete("view");
    return url.toString();
  };
  const receipt = (frame: string, control: string, result: string) => appendFileSync(join(evidence, "parity-clicks.jsonl"), JSON.stringify({ frame, route: page.url(), control, operation: "click", expected: result, actual: "assertion passed", environment: "MOCK" }) + "\n");
  const screenshot = async (id: string) => {
    await page.screenshot({ path: join(evidence, "parity", `${id.replaceAll(":", "-")}.png`) });
    // READ/MEASURE only: capture current viewport text density without changing DOM.
    const density = await page.getByTestId("dashboard-page").evaluate(element => {
      const blocks = [...element.querySelectorAll("h1, h2, h3, p, dt, dd, label, button, a")]
        .filter(node => { const r = node.getBoundingClientRect(); return r.width > 0 && r.height > 0 && r.top < innerHeight && r.bottom > 0; })
        .map(node => node.textContent?.trim() ?? "").filter(Boolean);
      return { blocks: blocks.length, characters: blocks.reduce((n, text) => n + text.length, 0) };
    });
    appendFileSync(join(evidence, "parity-density.jsonl"), JSON.stringify({ frame: id, ...density }) + "\n");
    // READ/MEASURE only: layout measurement, never a DOM write.
    expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0);
  };
  for (const mobile of [false, true]) {
    await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
    const id = mobile ? "4:12615" : "4:12467";
    await page.goto(location());
    const content = page.getByTestId("dashboard-page");
    await expect(content.getByRole("heading", { name: "工作台", exact: true })).toBeVisible();
    await expect(content.getByRole("heading", { name: "總覽", exact: true })).toHaveCount(0);
    await expect(content.getByText("今日訂單", { exact: true })).toBeVisible();
    await expect(content.getByRole("heading", { name: "最近訂單", exact: true })).toBeVisible();
    await expect(content.getByRole("heading", { name: "待辦", exact: true })).toBeVisible();
    await expect(content.getByRole("heading", { name: "各付款方式的銷售額", exact: true })).toHaveCount(0);
    await expect(content.getByRole("link", { name: "匯入或匯出商品", exact: true })).toHaveCount(0);
    await expect(content.getByText("近 7 天", { exact: true })).toHaveCount(0);
    await expect(content.getByTestId("orders-today")).toHaveText("2");
    await expect(content.getByRole("link", { name: "33333333", exact: true })).toBeVisible();
    await screenshot(id);
    await content.getByRole("link", { name: /全部訂單|^全部$/ }).click();
    await expect(page).toHaveURL(/\/zh-TW\/orders(?:\?|$)/);
    receipt(id, "全部訂單", "orders route");
    await page.goto(location());
    await page.getByTestId("dashboard-page").getByRole("link", { name: "新建商品", exact: true }).click();
    await expect(page).toHaveURL(/\/zh-TW\/products\/new(?:\?|$)/);
    receipt(id, "新建商品", "product creation route");
    await page.goto(location());
    await page.getByTestId("dashboard-page").getByRole("link", { name: "開始直播", exact: true }).click();
    await expect(page).toHaveURL(/\/zh-TW\/studio(?:[/?]|$)/);
    receipt(id, "開始直播", "studio route");

    await page.goto(location("help"));
    const help = mobile ? "21:1063" : "21:762";
    await expect(page.getByRole("heading", { name: "使用說明", exact: true })).toBeVisible();
    await expect(page.getByText("開店、直播收單與訂單處理的操作入口。", { exact: true })).toBeVisible();
    const entries = page.getByRole("link", { name: "查看操作入口", exact: true });
    await expect(entries).toHaveCount(3);
    await screenshot(help);
    for (const [index, destination] of ["settings", "studio", "orders"].entries()) {
      await page.goto(location("help"));
      await page.getByRole("link", { name: "查看操作入口", exact: true }).nth(index).click();
      await expect(page).toHaveURL(new RegExp(`/zh-TW/${destination}(?:[/?]|$)`));
      receipt(help, `查看操作入口 ${index + 1}`, destination);
    }

    // The attention frame's persisted product-import result is DEFERRED, never replaced by a fabricated count.
    await page.goto(location("attention"));
    await expect(page.getByRole("heading", { name: "待處理事項", exact: true })).toBeVisible();
    await expect(page.getByText("需要確認的連接、操作與匯入結果集中顯示。", { exact: true })).toBeVisible();
    await expect(page.getByText("2 筆待修正", { exact: true })).toHaveCount(0);
    await screenshot(mobile ? "21:1089" : "21:822");

    await page.goto(location("navigation"));
    const navigation = mobile ? "94:2341" : "94:2263";
    await expect(page.getByRole("heading", { name: "商店導覽", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "導覽項目", exact: true })).toBeVisible();
    await expect(page.getByText(/原型，未真實寫入|示範選單尚未儲存/)).toHaveCount(0);
    await expect(page.getByRole("button", { name: "儲存選單", exact: true })).toBeVisible();
    await screenshot(navigation);
    // Exercise the existing design CAS API through a genuine UI save; reload is the persistence assertion.
    const names = page.getByRole("textbox", { name: "顯示名稱", exact: true });
    const original = await names.first().inputValue();
    const edited = original + " · 測試";
    await names.first().fill(edited);
    await page.getByRole("button", { name: "儲存選單", exact: true }).click();
    await expect(page.getByRole("status").filter({ hasText: "已儲存" })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("textbox", { name: "顯示名稱", exact: true }).first()).toHaveValue(edited);
    receipt(navigation, "顯示名稱 / 儲存選單 / refresh", "saved value survives refresh");
    await page.getByRole("textbox", { name: "顯示名稱", exact: true }).first().fill(original);
    await page.getByRole("button", { name: "儲存選單", exact: true }).click();
    await expect(page.getByRole("status").filter({ hasText: "已儲存" })).toBeVisible();

    await page.goto(location("delivery"));
    const delivery = mobile ? "1222:25445" : "1222:25443";
    await expect(page.getByRole("heading", { name: "選擇要開放的配送方式", exact: true })).toBeVisible();
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeVisible();
    await expect(page.getByRole("checkbox", { name: /超商取貨/ })).toBeVisible();
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeEnabled();
    await expect(page.getByRole("checkbox", { name: /超商取貨/ })).toBeEnabled();
    await screenshot(delivery);
    const home = page.getByRole("checkbox", { name: /宅配/ });
    const selected = await home.isChecked();
    await home.click();
    await expect(home).toBeChecked({ checked: !selected });
    await page.getByRole("button", { name: "下一步：上架第一個商品", exact: true }).click();
    await expect(page.getByRole("heading", { name: "上架第一個商品", exact: true })).toBeVisible();
    await page.goto(location("delivery"));
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeEnabled();
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeChecked({ checked: !selected });
    await page.reload();
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeEnabled();
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeChecked({ checked: !selected });
    await page.getByRole("checkbox", { name: /宅配/ }).click();
    await expect(page.getByRole("checkbox", { name: /宅配/ })).toBeChecked({ checked: selected });
    await page.getByRole("button", { name: "下一步：上架第一個商品", exact: true }).click();
    await expect(page.getByRole("heading", { name: "上架第一個商品", exact: true })).toBeVisible();
    await screenshot(mobile ? "1222:25446" : "1222:25444");
    await page.getByRole("link", { name: "新增第一個商品", exact: true }).click();
    await expect(page).toHaveURL(/\/zh-TW\/products\/new(?:\?|$)/);
    receipt(delivery, "delivery checkboxes / next / 新增第一個商品", "product creation route");

    if (mobile) {
      await page.goto(location("capabilities"));
      await expect(page.getByRole("heading", { name: "功能檢查", exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: "先確認功能能否使用", exact: true })).toBeVisible();
      await expect(page.getByText("每則留言最多一次私密回覆；會話私訊仍須符合 Meta 的訊息窗口與目前權限。", { exact: true })).toBeVisible();
      await screenshot("50:1357");
      await page.getByRole("button", { name: "重新檢查", exact: true }).click();
      await expect(page.getByRole("status").filter({ hasText: /已安排檢查|等待結果更新/ })).toBeVisible();
      receipt("50:1357", "重新檢查", "scheduled receipt, not a claimed platform success");
    }
  }
  for (const [locale, heading] of [["zh-TW", "工作台"], ["zh-CN", "工作台"], ["en", "Dashboard"]]) {
    await page.goto(location("", locale));
    await expect(page.getByTestId("dashboard-page").getByRole("heading", { name: heading, exact: true })).toBeVisible();
  }
  await page.goto(location());
  // Only our fixture interceptions are removed; the caller retains its session and other routes.
  for (const route of [dashboardURL, designURL, marketsURL, servicesURL, healthURL, recheckURL]) await page.unroute(route);
}
