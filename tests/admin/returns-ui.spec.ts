// W3-U5 returns/cancel admin UI (contracts/returns-v1.md §2/§3/§6/§7; unit brief docs/delivery/units/w3-u5-returns-ui.md).
// BFF routes exercised through real clicks: GET|POST /api/stores/{store}/orders/{id}/returns, POST .../orders/{id}/cancel,
// POST .../returns/{rma}/{receive|inspect|close|cancel}, GET .../returns[?state=], GET .../orders/cancel-refund-gaps
// -> Go internal/httpapi/returns.go. Driven by tests/foundation/browser_returns_ui_test.go (TestBrowserReturnsUI);
// until that harness lands this spec is NOT_RUN (the test-local.sh mode fails closed on the missing Go file).
// Label: BROWSER(MOCK) — orders are paid/shipped through the fakes; no provider or deployment acceptance.
//
// Fixture contract (the Go harness seeds through the real merchant/buyer paths and exports):
//   LC_BROWSER_PUBLIC_ORIGIN   admin origin (https)
//   LC_BROWSER_EVIDENCE        evidence directory for screenshots.json
//   LC_BROWSER_STORE           store id (uuid)
//   LC_BROWSER_ORDER_SHIPPED   fulfillment MERCHANT_SHIPPED, payment CAPTURED, one SKU line quantity >= 2, no live RMA
//   LC_BROWSER_ORDER_PAID      commercial CONFIRMED, payment CAPTURED, no live RMA (cancel -> 409 refund_first)
//   LC_BROWSER_ORDER_IN_FLIGHT commercial AWAITING_PAYMENT with a payment attempt in flight (cancel -> 409 payment_in_flight)
//   LC_BROWSER_GAP_ORDER       cancelled card order still listed by cancel-refund-gaps (cancel_refund_failed)
//   LC_BROWSER_RESTRICTED_TOKEN staff session WITHOUT fulfillment:write / inventory:write
import { expect, test, type BrowserContext, type Locator, type Page } from "@playwright/test";
import { createHash, randomBytes } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const shippedOrder = required("LC_BROWSER_ORDER_SHIPPED");
const paidOrder = required("LC_BROWSER_ORDER_PAID");
const inFlightOrder = required("LC_BROWSER_ORDER_IN_FLIGHT");
const gapOrder = required("LC_BROWSER_GAP_ORDER");
const restrictedToken = required("LC_BROWSER_RESTRICTED_TOKEN");
const cookieName = "__Host-commerce_session";

const ui = {
  register: /register a return|登記退貨|登记退货/i,
  receive: /confirm receipt|確認收貨|确认收货/i,
  inspect: /inspect|驗貨|验货/i,
  closeRma: /close rma|關閉退貨單|关闭退货单/i,
  withdraw: /withdraw|撤銷登記|撤销登记/i,
  cancelOrder: /cancel order|取消訂單|取消订单/i,
  refundFirst: /refund the captured amount in full first|請先完成全額退款|请先完成全额退款/,
  paymentInFlight: /payment is in flight|付款正在進行中|付款正在进行中/,
  refundHint: /use this order's|請使用本訂單的|请使用本订单的/,
  submitted: /return registered|退貨已登記|退货已登记/,
  closed: /rma closed|退貨單已關閉|退货单已关闭/,
  withdrawn: /withdrawn|已撤銷|已撤销/,
};

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
}
/** Opens the orders page deep-linked at one order; the page expands ?order= itself. */
async function openOrder(page: Page, locale: string, orderId: string): Promise<Locator> {
  await page.goto(new URL(`/${locale}/orders?store=${store}&state=all&order=${orderId}`, origin).toString());
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  const detail = page.getByTestId("order-detail");
  await expect(detail).toHaveAttribute("aria-label", new RegExp(orderId));
  return detail;
}
async function asToken(context: BrowserContext, token: string) {
  await context.clearCookies();
  // The admin UI's session fence hashes the readable companion cookie (settings-client.sessionBoundary), so a
  // swapped-in session needs its own __Host-commerce_csrf value exactly as the real callback would set it.
  const csrf = randomBytes(32).toString("base64url");
  const url = origin.replace(/^http:/, "https:");
  await context.addCookies([
    { name: cookieName, value: token, url, secure: true, httpOnly: true, sameSite: "Lax" },
    { name: "__Host-commerce_csrf", value: csrf, url, secure: true, httpOnly: false, sameSite: "Lax" },
  ]);
}
const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false });
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
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

type Post = { url: string; key: string | null; body: string | null };
/** Records every keyed POST the UI sends to the returns/cancel routes (observation only, never a stub). */
function watchPosts(page: Page): Post[] {
  const posts: Post[] = [];
  page.on("request", (r) => {
    const pathName = new URL(r.url()).pathname;
    if (r.method() === "POST" && (/(^|\/)orders\/[0-9a-f-]{36}\/(returns|cancel)$/.test(pathName) || /(^|\/)returns\/[0-9a-f-]{36}\/(receive|inspect|close|cancel)$/.test(pathName)))
      posts.push({ url: pathName, key: r.headers()["idempotency-key"] ?? null, body: r.postData() });
  });
  return posts;
}

/** The RMA article (not its action buttons, whose testids share the rma- prefix). */
const rmaCards = (detail: Locator, state: string) => detail.locator(`article[data-testid^="rma-"][data-state="${state}"]`);

test("W3-U5 full RMA walk: register -> receive -> inspect -> close, one keyed POST each, refresh persistence", async ({ page }) => {
  await signedLogin(page);
  const posts = watchPosts(page);
  const detail = await openOrder(page, "zh-TW", shippedOrder);
  const returns = detail.getByTestId("order-returns");
  await expect(returns).toBeVisible();

  // 登記退貨: reason enum + note, quantity 1 on the first line
  await returns.getByTestId("return-register").click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("return-reason").selectOption("damaged");
  await dialog.getByTestId("return-note").fill("外箱壓毀");
  await dialog.locator('input[data-testid^="return-qty-"]').first().fill("1");
  await dialog.getByTestId("return-submit").click();
  await expect(dialog).toHaveCount(0);
  await expect(returns.getByTestId("return-notice")).toContainText(ui.submitted);
  const registered = rmaCards(detail, "REGISTERED").first();
  await expect(registered).toBeVisible();
  const rmaId = (await registered.getAttribute("data-testid"))!.slice("rma-".length);

  // register POST: exact body shape {reason, lines}, one fresh idempotency key
  const registerPost = posts.find((p) => p.url.endsWith(`/orders/${shippedOrder}/returns`));
  expect(registerPost?.key).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  const registerBody = JSON.parse(registerPost?.body ?? "{}") as { reason: string; lines: { sku_id: string; quantity: number }[] };
  expect(Object.keys(registerBody).sort()).toEqual(["lines", "reason"]);
  expect(registerBody.reason).toBe("damaged: 外箱壓毀");
  expect(registerBody.lines).toHaveLength(1);
  expect(registerBody.lines[0].quantity).toBe(1);
  expect(registerBody.lines[0]).not.toHaveProperty("warehouse_id"); // the server resolves the warehouse

  // 確認收貨: the dialog defaults to the registered quantities; submit as-is
  await registered.getByTestId(`rma-receive-${rmaId}`).click();
  await expect(page.locator("dialog[open]")).toBeVisible();
  await page.locator("dialog[open]").getByTestId("return-submit").click();
  await expect(page.locator("dialog[open]")).toHaveCount(0);
  await expect(rmaCards(detail, "RECEIVED").first()).toBeVisible();
  const receivePost = posts.find((p) => p.url.endsWith(`/returns/${rmaId}/receive`));
  expect(receivePost?.key).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  expect(receivePost?.key).not.toBe(registerPost?.key); // one key per distinct command
  const receiveBody = JSON.parse(receivePost?.body ?? "{}") as Record<string, unknown>;
  expect(Object.keys(receiveBody).sort()).toEqual(["expected_version", "lines"]);
  expect(receiveBody.expected_version).toBe(1);

  // 驗貨與處置: restock defaults to the received quantity, scrap 0; the client enforces restock + scrap = received
  await rmaCards(detail, "RECEIVED").first().getByTestId(`rma-inspect-${rmaId}`).click();
  const inspectDialog = page.locator("dialog[open]");
  await expect(inspectDialog).toBeVisible();
  // a mismatched disposition disables submission instead of sending an invalid body
  const restock = inspectDialog.locator('input[data-testid^="inspect-restock-"]').first();
  await restock.fill("0");
  const scrap = inspectDialog.locator('input[data-testid^="inspect-scrap-"]').first();
  await scrap.fill("0");
  await expect(inspectDialog.getByTestId("return-submit")).toBeDisabled();
  await restock.fill("1");
  await expect(inspectDialog.getByTestId("return-submit")).toBeEnabled();
  await inspectDialog.getByTestId("return-submit").click();
  await expect(page.locator("dialog[open]")).toHaveCount(0);
  const inspected = rmaCards(detail, "INSPECTED").first();
  await expect(inspected).toBeVisible();
  // Integrator 裁决: the returns panel never refunds — it points at the order's refund section
  await expect(inspected).toContainText(ui.refundHint);
  await expect(inspected.locator('a[href="#order-refunds"]')).toBeVisible();
  const inspectBody = JSON.parse(posts.find((p) => p.url.endsWith(`/returns/${rmaId}/inspect`))?.body ?? "{}") as Record<string, unknown>;
  expect(inspectBody.expected_version).toBe(2);

  // 關閉: the confirmation restates the restock units before anything is sent
  await inspected.getByTestId(`rma-close-${rmaId}`).click();
  const closeDialog = page.locator("dialog[open]");
  await expect(closeDialog.getByTestId("return-close-confirm")).toContainText("1");
  await closeDialog.getByTestId("return-submit").click();
  await expect(page.locator("dialog[open]")).toHaveCount(0);
  await expect(returns.getByTestId("return-notice")).toContainText(ui.closed);
  await expect(rmaCards(detail, "CLOSED").first()).toBeVisible();
  expect(JSON.parse(posts.find((p) => p.url.endsWith(`/returns/${rmaId}/close`))?.body ?? "{}")).toMatchObject({ expected_version: 3 });

  // refresh persistence: the closed RMA is still there after a full reload
  await page.reload();
  const again = await openOrder(page, "zh-TW", shippedOrder);
  await expect(rmaCards(again, "CLOSED").first()).toBeVisible();
});

test("W3-U5 register then withdraw: REGISTERED -> CANCELLED with the release confirmation", async ({ page }) => {
  await signedLogin(page);
  const posts = watchPosts(page);
  const detail = await openOrder(page, "zh-TW", shippedOrder);
  const returns = detail.getByTestId("order-returns");
  await returns.getByTestId("return-register").click();
  const dialog = page.locator("dialog[open]");
  await dialog.getByTestId("return-reason").selectOption("buyer_request");
  await dialog.locator('input[data-testid^="return-qty-"]').first().fill("1");
  await dialog.getByTestId("return-submit").click();
  await expect(dialog).toHaveCount(0);
  const registered = rmaCards(detail, "REGISTERED").first();
  await expect(registered).toBeVisible();
  const rmaId = (await registered.getAttribute("data-testid"))!.slice("rma-".length);

  await registered.getByTestId(`rma-withdraw-${rmaId}`).click();
  const withdrawDialog = page.locator("dialog[open]");
  await expect(withdrawDialog.getByTestId("return-withdraw-confirm")).toBeVisible();
  await withdrawDialog.getByTestId("return-submit").click();
  await expect(page.locator("dialog[open]")).toHaveCount(0);
  await expect(returns.getByTestId("return-notice")).toContainText(ui.withdrawn);
  await expect(rmaCards(detail, "CANCELLED").first()).toBeVisible();
  expect(posts.some((p) => p.url.endsWith(`/returns/${rmaId}/cancel`))).toBe(true);
  // a withdrawn RMA offers no further action
  await expect(rmaCards(detail, "CANCELLED").first().getByRole("button")).toHaveCount(0);
});

test("W3-U5 cancel a paid order: refund_first refusal keeps the dialog open and never cancels", async ({ page }) => {
  await signedLogin(page);
  const posts = watchPosts(page);
  const detail = await openOrder(page, "zh-TW", paidOrder);
  const cancel = detail.getByTestId("order-cancel");
  await expect(cancel).toBeVisible();
  await cancel.getByTestId("order-cancel-open").click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(/釋放已保留的庫存|释放已保留的库存|releases the reserved stock/);
  await dialog.getByTestId("order-cancel-reason").fill("買家反悔");
  await dialog.getByTestId("order-cancel-submit").click();
  // the refusal renders next to the form; the dialog stays open; the order is untouched
  await expect(dialog.getByTestId("order-cancel-problem")).toContainText(ui.refundFirst);
  await expect(dialog).toBeVisible();
  const cancelPost = posts.find((p) => p.url.endsWith(`/orders/${paidOrder}/cancel`));
  expect(JSON.parse(cancelPost?.body ?? "{}")).toEqual({ expected_state: "CONFIRMED", reason: "買家反悔" });
  await expect(detail.locator('[data-state="CONFIRMED"]').first()).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("dialog[open]")).toHaveCount(0);
  await expect(cancel.getByTestId("order-cancel-notice")).toHaveCount(0);
});

test("W3-U5 cancel with a payment in flight: payment_in_flight refusal copy", async ({ page }) => {
  await signedLogin(page);
  const detail = await openOrder(page, "zh-TW", inFlightOrder);
  const cancel = detail.getByTestId("order-cancel");
  await cancel.getByTestId("order-cancel-open").click();
  const dialog = page.locator("dialog[open]");
  await dialog.getByTestId("order-cancel-reason").fill("測試取消");
  await dialog.getByTestId("order-cancel-submit").click();
  await expect(dialog.getByTestId("order-cancel-problem")).toContainText(ui.paymentInFlight);
  await page.keyboard.press("Escape");
  await expect(page.locator("dialog[open]")).toHaveCount(0);
});

test("W3-U5 returns page: RMA list, state filter links and the refund-gap row survive a reload", async ({ page }) => {
  await signedLogin(page);
  await page.goto(new URL(`/zh-TW/returns?store=${store}`, origin).toString());
  await expect(page.getByTestId("returns-page")).toBeVisible();
  await expect(page.getByTestId("returns-list")).toBeVisible();
  // the RMAs from the flows above are listed, newest first
  await expect(page.locator('[data-testid^="returns-row-"]').first()).toBeVisible();
  // state filter: real links, refresh-persistent
  await page.getByTestId("returns-filter-closed").click();
  await expect(page).toHaveURL(/state=CLOSED/);
  await expect(page.locator('tr[data-testid^="returns-row-"]').first()).toBeVisible();
  for (const row of await page.locator('tr[data-testid^="returns-row-"]').all())
    await expect(row.locator('[data-state="CLOSED"]')).toBeVisible();
  await page.reload();
  await expect(page).toHaveURL(/state=CLOSED/);
  await expect(page.getByTestId("returns-list")).toBeVisible();
  // 退款失敗待處理: the seeded gap links to the cancelled order
  const gaps = page.getByTestId("refund-gaps");
  await expect(gaps).toBeVisible();
  const gapRow = page.getByTestId(`refund-gap-${gapOrder}`);
  await expect(gapRow).toBeVisible();
  await gapRow.getByTestId(`refund-gap-open-${gapOrder}`).click();
  await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label", new RegExp(gapOrder));
});

test("W3-U5 a member without fulfillment:write sees the returns but no action; the server refuses a forged write", async ({ page, context }) => {
  await signedLogin(page);
  await asToken(context, restrictedToken);
  const detail = await openOrder(page, "en", shippedOrder);
  const returns = detail.getByTestId("order-returns");
  await expect(returns).toBeVisible();
  await expect(rmaCards(detail, "CLOSED").first()).toBeVisible(); // orders:read still lists the history
  await expect(returns.getByTestId("return-register")).toHaveCount(0);
  await expect(returns.locator('button[data-testid^="rma-receive-"]')).toHaveCount(0);
  await expect(returns.locator('button[data-testid^="rma-withdraw-"]')).toHaveCount(0);
  await expect(detail.getByTestId("order-cancel")).toHaveCount(0);
  // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI,
  // must refuse (UI click paths of the same route are covered by the flow tests above).
  const status = await page.evaluate(async ({ store: s, order: o }) => {
    const r = await fetch(`/api/stores/${s}/orders/${o}/returns`, { method: "POST", headers: { "content-type": "application/json", "Idempotency-Key": "w3u5-restricted-key", "X-CSRF-Token": document.cookie.split("; ").find((c) => c.startsWith("__Host-commerce_csrf="))?.slice(21) ?? "" }, body: JSON.stringify({ reason: "other", lines: [] }) });
    return r.status;
  }, { store, order: shippedOrder });
  expect([401, 403]).toContain(status); // the server, not the hidden button, is the authority
});

for (const locale of ["zh-TW", "en"] as const) {
  test(`W3-U5 ${locale}: returns + cancel sections readable and no overflow at desktop and 390 px`, async ({ page }) => {
    await signedLogin(page);
    const detail = await openOrder(page, locale, shippedOrder);
    await expect(detail.getByTestId("order-returns")).toBeVisible();
    await page.setViewportSize({ width: 1586, height: 992 });
    await shot(page, "returns-order", locale, "desktop");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(detail.getByTestId("order-returns")).toBeVisible();
    await shot(page, "returns-order", locale, "mobile");
    await page.goto(new URL(`/${locale}/returns?store=${store}`, origin).toString());
    await expect(page.getByTestId("returns-page")).toBeVisible();
    await shot(page, "returns-list", locale, "mobile");
    await page.setViewportSize({ width: 1586, height: 992 });
    await shot(page, "returns-list", locale, "desktop");
  });
}
