// Purpose: PARITY-STATES browser assertions against real cart/order/link/lookup routes and signed state copy.
// Depends on: @playwright/test, browser-engine, the production storefront started by shop-real-gate.
// Used by: shop-real-gate.mjs / --browser-storefront CI, with explicitly MOCK BFF reads in isolated contexts.
// Invariants: fixtures never alter DOM or application storage; recovery uses real clicks; unknown only re-reads the same order.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";
import { ctxOpts } from "./browser-engine.mjs";

// Independent expectations transcribed from ux-states/DELIVERY.md; never imported from production copy.
const copy = {
  "zh-TW": {
    empty: "購物車還沒有商品", emptyBody: "先挑選想買的商品，再回到購物車確認數量與配送方式。", emptyNote: "選好商品後，可在購物車確認數量。", browse: "瀏覽商品",
    forbidden: "目前無法開啟此頁", forbiddenBody: "此連結無法開啟，或目前購物連線無法使用這項功能。請從商店重新開始。", forbiddenNote: "如需查看訂單，請使用商店的訂單查詢。", home: "返回商店",
    offline: "暫時無法載入購物車", offlineBody: "請檢查網路連線，再重新載入。這次只讀取購物車，不會送出訂單。", offlineNote: "商品與數量以原購物連線重新讀取的結果為準。", retry: "重新載入", failed: "目前仍無法連線。請保留原頁，稍後重新載入。",
    expired: "無法開啟這個連結", expiredBody: "請從訂單查詢重新開始，並準備訂單編號與電子郵件。", expiredNote: "連結不適用時，請勿再次送出原訂單。", lookup: "前往訂單查詢",
    unknown: "付款結果尚未確認", unknownBody: "請先查詢這筆訂單的付款狀態。確認前請勿再次付款。", check: "查詢狀態", unknownFailed: "目前無法讀取最新狀態。請稍後查詢同一筆訂單。",
    lookupSubmit: "查詢", lookupFailed: "暫時無法查詢，請稍後再試。", lookupNoMatch: "找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。",
  },
  "zh-CN": {
    empty: "购物车还没有商品", emptyBody: "先挑选想买的商品，再回到购物车确认数量与配送方式。", emptyNote: "选好商品后，可在购物车确认数量。", browse: "浏览商品",
    forbidden: "暂时无法打开此页面", forbiddenBody: "此链接无法打开，或当前购物会话无法使用此功能。请从商店重新开始。", forbiddenNote: "如需查看订单，请使用商店的订单查询。", home: "返回商店",
    offline: "暂时无法加载购物车", offlineBody: "请检查网络连接，再重新加载。本次只读取购物车，不会提交订单。", offlineNote: "商品与数量以当前购物会话重新读取的结果为准。", retry: "重新加载", failed: "暂时仍无法连接。请保留当前页面，稍后重新加载。",
    expired: "无法打开此链接", expiredBody: "请从订单查询重新开始，并准备订单编号与邮箱。", expiredNote: "链接不可用时，请勿重新提交原订单。", lookup: "前往订单查询",
    unknown: "付款结果尚未确认", unknownBody: "请先查询这笔订单的付款状态。确认前请勿再次付款。", check: "查询状态", unknownFailed: "暂时无法读取最新状态。请稍后查询同一笔订单。",
    lookupSubmit: "查询", lookupFailed: "暂时无法查询，请稍后再试。", lookupNoMatch: "找不到匹配的订单，请确认订单编号与邮箱或手机号码。",
  },
  en: {
    empty: "Your cart is empty", emptyBody: "Choose products first, then return to your cart to confirm quantities and delivery.", emptyNote: "You can check quantities in your cart after choosing products.", browse: "Browse products",
    forbidden: "This page cannot be opened", forbiddenBody: "This link cannot be opened, or your shopping session cannot use this feature. Start again from the store.", forbiddenNote: "Use the store’s order lookup to view an order.", home: "Return to store",
    offline: "Your cart could not be loaded", offlineBody: "Check your connection and reload. This only reads your cart and does not place an order.", offlineNote: "Products and quantities come from re-reading your original shopping session.", retry: "Reload", failed: "We still cannot connect. Keep this page open and reload later.",
    expired: "This link cannot be opened", expiredBody: "Start again from order lookup, with your order number and email ready.", expiredNote: "Do not place the original order again if the link cannot be used.", lookup: "Go to order lookup",
    unknown: "Your payment result is not confirmed", unknownBody: "Check this order’s payment status first. Do not pay again until it is confirmed.", check: "Check status", unknownFailed: "The latest status could not be loaded. Check the same order again later.",
    lookupSubmit: "Find order", lookupFailed: "Lookup is unavailable right now. Please try again later.", lookupNoMatch: "We could not find a matching order. Check the order number and the email or phone number.",
  },
};
const uid = n => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
const orderID = uid(61);
const active = { state: "active", context: "a".repeat(43), expires_at: "2099-01-01T00:00:00Z" };
const cart = { id: uid(62), currency: "TWD", version: 1, items: [] };
// Contract-shaped synthetic order: no real recipient, address, contact, token, key or provider URL.
const order = {
  order_id: orderID, cart_id: cart.id, cart_version: 1, commercial_state: "AWAITING_PAYMENT", fulfillment_state: "MANUAL_UNASSIGNED",
  snapshot: {
    quote: { currency: "TWD", lines: [{ sku_id: uid(63), name: "Synthetic state item", code: "STATE-TEST", quantity: 1, unit_price_minor: 262000 }], amount: { subtotal_minor: 262000, discount_minor: 0, shipping_minor: 0, shipping_tax_minor: 0, tax_minor: 0, total_minor: 262000 } },
    destination: { kind: "home", country: "TW", recipient_name: "Synthetic Recipient", phone: "+886900000091", home_address: { region: "", city: "Synthetic City", postal_code: "", line1: "Synthetic Street", line2: "" } },
    service: { code: "home", name_hans: "测试配送", name_hant: "測試配送", name_en: "Synthetic delivery", delivery_kind: "home", mode: "MANUAL" },
  },
};
const payment = { order_id: orderID, currency: "TWD", total_minor: 262000, commercial_state: "AWAITING_PAYMENT", test_mode: true, payment_state: "PENDING", handoff_state: "CREATING", handoff_expires_at: "2099-01-01T00:00:00Z", methods: [], cancel_requested: false };

export async function checkBuyerParityStates({ browser, origin, evidence }) {
  const dir = path.join(evidence, "parity");
  await mkdir(dir, { recursive: true });
  const ledger = [], failures = [];
  for (const [locale, c] of Object.entries(copy)) {
    for (const width of [1440, 390]) {
      for (const kind of ["empty", "offline", "forbidden", "cart-forbidden", "order-offline", "expired", "unknown", "lookup"]) {
        const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height: width === 1440 ? 900 : 844 }, locale }));
        const page = await context.newPage();
        const label = `${kind}-${locale}-${width}`;
        const writes = [], reads = [];
        let cartFailed = kind === "offline", paymentFailed = false, lookupFailed = true;
        page.on("request", request => {
          if (request.url().includes("/api/buyer/")) {
            if (request.method() === "GET") reads.push(new URL(request.url()).pathname);
            else writes.push({ method: request.method(), path: new URL(request.url()).pathname });
          }
        });
        // MOCK fixture setup at existing BFF boundaries. These are response data, never user actions or DOM changes.
        await context.route("**/api/buyer/session", route => route.fulfill({ json: active }));
        await context.route("**/api/buyer/cart", route => kind === "cart-forbidden" ? route.fulfill({ status: 403, json: { code: "forbidden" } }) : cartFailed ? route.fulfill({ status: 503, json: { code: "unavailable" } }) : route.fulfill({ json: cart }));
        await context.route(`**/api/buyer/orders/${orderID}`, route => kind === "forbidden" ? route.fulfill({ status: 404, json: { code: "not_found" } }) : kind === "order-offline" ? route.fulfill({ status: 503, json: { code: "unavailable" } }) : route.fulfill({ json: order }));
        await context.route(`**/api/buyer/orders/${orderID}/payment`, route => paymentFailed ? route.fulfill({ status: 503, json: { code: "unavailable" } }) : route.fulfill({ json: payment }));
        await context.route("**/api/buyer/orders/lookup", route => route.fulfill({ status: lookupFailed ? 503 : 404, json: { code: lookupFailed ? "unavailable" : "not_found" } }));
        const shot = async frame => {
          if (locale === "zh-TW") await page.screenshot({ path: path.join(dir, `${frame}.png`) });
        };
        try {
          if (kind === "lookup") {
            await page.goto(`${origin}/${locale}/orders/lookup`);
            await page.getByTestId("lookup-ref").fill("0123-ABCD-4567");
            await page.getByTestId("lookup-contact").fill("state-gate@example.invalid");
            await page.getByRole("button", { name: c.lookupSubmit, exact: true }).click();
            const error = page.getByTestId("lookup-problem");
            await expect(error).toContainText(c.lookupFailed);
            await expect(error).toHaveJSProperty("tagName", "DIV"); // InlineError replaces the legacy bare paragraph.
            await expect(error.getByRole("alert")).toBeVisible();
            await expect(page.getByTestId("lookup-ref")).toHaveValue("0123-ABCD-4567");
            lookupFailed = false;
            await page.getByRole("button", { name: c.lookupSubmit, exact: true }).click();
            await expect(error).toContainText(c.lookupNoMatch);
            await page.reload();
            await expect(page.getByTestId("lookup-ref")).toHaveValue("");
            await expect(page.getByTestId("lookup-contact")).toHaveValue("");
          } else if (kind === "cart-forbidden") {
            await page.goto(`${origin}/${locale}/cart`);
            const panel = page.locator('section[data-variant="forbidden"]');
            await expect(panel.getByRole("heading", { name: c.forbidden, exact: true })).toBeVisible();
            await expect(page.getByRole("heading", { name: c.empty, exact: true })).toHaveCount(0);
            await panel.getByRole("link", { name: c.home, exact: true }).click();
            await expect(page).toHaveURL(new RegExp(`/${locale}/?$`));
          } else if (kind === "order-offline") {
            await page.goto(`${origin}/${locale}/orders/${orderID}`);
            const panel = page.locator('section[data-variant="offline"]');
            await expect(panel).toBeVisible();
            await panel.getByRole("button", { name: c.retry, exact: true }).click();
            await expect(page.getByRole("alert")).toBeVisible();
            await expect(page).toHaveURL(`${origin}/${locale}/orders/${orderID}`);
            await page.reload();
            await expect(panel).toBeVisible();
          } else {
            const route = kind === "empty" || kind === "offline" ? "cart" : kind === "expired" ? "order-link" : `orders/${orderID}`;
            await page.goto(`${origin}/${locale}/${route}`);
            const panel = page.locator(`section[data-variant="${kind}"]`);
            await expect(panel.getByRole("heading", { name: c[kind], exact: true })).toBeVisible();
            await expect(panel).toContainText(c[`${kind}Body`]);
            if (c[`${kind}Note`]) await expect(panel).toContainText(c[`${kind}Note`]);
            await expect(page.locator(".sf-empty")).toHaveCount(0);
            await expect(page.getByText(/403\s*·/)).toHaveCount(0);
            const frames = { empty: ["1429-35150", "1429-35170"], forbidden: ["1429-35190", "1429-35210"], offline: ["1429-35230", "1429-35249"], expired: ["1429-35268", "1429-35288"], unknown: ["1429-35308", "1429-35331"] };
            await shot(frames[kind][width === 1440 ? 0 : 1]);
            if (kind === "offline") {
              const retry = page.getByRole("button", { name: c.retry, exact: true });
              await retry.click();
              await expect(page.getByRole("alert")).toContainText(c.failed);
              await shot(width === 1440 ? "1429-35448" : "1429-35466");
              cartFailed = false;
              await retry.click();
              await expect(page.getByRole("heading", { name: c.empty, exact: true })).toBeVisible();
              await page.reload();
              await expect(page.getByRole("heading", { name: c.empty, exact: true })).toBeVisible();
            } else if (kind === "unknown") {
              await expect(page.getByTestId("order-page")).toContainText("2,620");
              const before = reads.filter(url => url.endsWith("/payment")).length;
              paymentFailed = true;
              await panel.getByRole("button", { name: c.check, exact: true }).click();
              await expect(page.getByRole("alert")).toContainText(c.unknownFailed);
              assert.ok(reads.filter(url => url.endsWith("/payment")).length > before, "Check status reads the same payment");
              await expect(page.getByTestId("order-page")).toContainText("2,620");
              await expect(page.getByTestId("pay-order")).toHaveCount(0);
              await shot(width === 1440 ? "1429-35484" : "1429-35506");
              paymentFailed = false;
              await page.reload();
              await expect(panel).toBeVisible();
            } else {
              await page.reload();
              await expect(panel).toBeVisible();
              const actionName = kind === "empty" ? c.browse : kind === "expired" ? c.lookup : c.home;
              const suffix = kind === "empty" ? "/products" : kind === "expired" ? "/orders/lookup" : "";
              await panel.getByRole("link", { name: actionName, exact: true }).click();
              await expect(page).toHaveURL(new RegExp(`/${locale}${suffix}/?$`));
            }
          }
          assert.deepEqual(writes.filter(write => kind !== "lookup" || write.path !== "/api/buyer/orders/lookup"), [], "recovery must not replay any command or payment signal");
          ledger.push({ route: kind, locale, width, operation: "real click / fill / reload", expected: "signed copy; safe recovery; no replay; refresh result retained where applicable", result: "PASS", reads, writes });
        } catch (error) {
          failures.push(`${label}: ${error.message}`);
          ledger.push({ route: kind, locale, width, result: "FAIL", error: error.message });
          await page.screenshot({ path: path.join(dir, `failed-${label}.png`) }).catch(() => {});
        } finally { await context.close(); }
      }
    }
  }
  await writeFile(path.join(dir, "buyer-click-ledger.json"), JSON.stringify(ledger, null, 2));
  assert.deepEqual(failures, [], "PARITY-STATES: signed panels and real recovery controls on buyer routes");
  console.log("PARITY-STATES: cart/order/link/lookup real clicks, all three locales, desktop+mobile; read fixtures MOCK");
}
