// Purpose: CVS-PICK-IMPL signed buyer picker acceptance using real clicks in the existing CVS stack.
// Depends on: Playwright; the parent runner's real Next/BFF/Go/PG fixture and public store snapshot.
// Used by: cvs-buyer.mjs, TestBrowserTaiwanCvs MOCK / WebKit in GitHub CI.
// Invariants: no per-keystroke backend writes, no automatic choice, canonical server facts persist.
import { confirmCheckoutAddress } from "./shop-helpers.mjs";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { mkdir, readFile } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";

const localeCopy = {
  "zh-TW": { title: "選擇取貨門市", query: "地址、門市名稱或編號", empty: "找不到符合的門市", confirm: "確認取貨門市", confirmTotal: "確認新金額", source: "資料：綠界門市清單 · 更新 2026-10-28" },
  "zh-CN": { title: "选择自提门店", query: "地址、门店名称或编号", empty: "未找到匹配的门店", confirm: "确认自提门店", confirmTotal: "确认新金额", source: "数据：绿界门店清单 · 更新 2026-10-28" },
  en: { title: "Choose pickup store", query: "Address, store name or code", empty: "No matching stores", confirm: "Confirm pickup store", confirmTotal: "Confirm new total", source: "Source: ECPay stores · Updated 2026-10-28" },
};

const signedCopy = {
  "zh-TW": { browse: "依縣市／區找門市", close: "關閉", helper: "輸入門市地址、名稱或編號", hint: "請核對地址或切換品牌。", best: "最符合", change: "更換", review: "核對取貨門市", back: "返回", retry: "重試" },
  "zh-CN": { browse: "按县市／区查门店", close: "关闭", helper: "输入自提门店的地址、名称或编号", hint: "请核对地址，或换一个品牌。", best: "最匹配", change: "更换", review: "核对自提门店", back: "返回", retry: "重试" },
  en: { browse: "Browse by city / district", close: "Close", helper: "Enter a pickup store address, name or code", hint: "Check the address or switch brand.", best: "Best match", change: "Change", review: "Review pickup store", back: "Back", retry: "Retry" },
};

// PARITY-CVS: the real checkout route, never /ui-gallery. Network interception below
// prepares only an outage fixture; every state transition uses real user controls.
async function exerciseSignedCvsParity({ page, locale, ledger, densities }) {
  const c = localeCopy[locale], s = signedCopy[locale];
  const originalViewport = page.viewportSize();
  const picker = page.getByTestId("cvs-picker");
  const query = picker.getByRole("textbox", { name: c.query, exact: true });
  const posts = [];
  const watch = (request) => {
    if (new URL(request.url()).pathname === "/api/buyer/cvs-stores" && request.method() === "POST") posts.push(request);
  };
  const frames = {
    desktop: ["1429:14065", "1429:14294", "1429:14519", "1429:14738", "1431:71232"],
    mobile: ["1429:14122", "1429:14359", "1429:14578", "1429:14800", "1431:71249"],
  };
  const capture = async (viewport, state) => {
    if (locale !== "zh-TW") return;
    if (viewport === "mobile") await density(picker, densities, `signed-${state}`);
    const directory = path.join(process.env.LC_CVS_EVIDENCE, "parity");
    await mkdir(directory, { recursive: true });
    // GitHub artifacts reject colons; retain the signed frame ID in frames above.
    const filename = `${frames[viewport][state].replace(":", "-")}.png`;
    await page.screenshot({ path: path.join(directory, filename) });
  };
  const facts = async () => {
    const card = picker.getByTestId("cvs-picker-chosen");
    await expect(card).toContainText("7-ELEVEN · 樂隆");
    await expect(card.getByTestId("cvs-picker-chosen-code")).toHaveText("121275");
    await expect(card).toContainText("台北市大安區敦化南路二段331巷14號");
    await expect(picker.getByRole("button", { name: s.change, exact: true })).toBeVisible();
    // READ/MEASURE only: the signed card gives the code visual priority.
    const codeSize = await card.getByTestId("cvs-picker-chosen-code").evaluate((node) => parseFloat(getComputedStyle(node).fontSize));
    assert.ok(codeSize >= 32, "signed chosen store code is at least 32px");
  };
  page.on("request", watch);
  try {
    // Keep the owner-mandated manual fallback and official link reachable outside the sheet.
    await expect(page.getByTestId("cvs-entered-code")).toHaveAttribute("maxlength", "6");
    await expect(page.getByTestId("cvs-entered-code")).toHaveAttribute("inputmode", "numeric");
    await expect(page.getByTestId("cvs-search-link")).toHaveAttribute("href", /^https:\/\//);
    await expect(page.getByTestId("cvs-search-link")).toContainText("7-ELEVEN");
    for (const [viewport, size] of [["desktop", { width: 1440, height: 900 }], ["mobile", { width: 390, height: 844 }]]) {
      await page.setViewportSize(size);
      await page.getByTestId("cvs-open-picker").click();
      await expect(picker.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
      await expect(query, "PARITY-CVS: signed first view is search-first").toBeVisible();
      await expect(picker.getByRole("button", { name: s.browse, exact: true })).toBeVisible();
      await expect(picker.getByTestId("cvs-picker-region-tab")).toHaveCount(0);
      await expect(picker.getByTestId("cvs-picker-search-tab")).toHaveCount(0);
      await expect(picker.getByRole("button", { name: s.close, exact: true })).toHaveText(s.close);
      await query.fill("");
      await expect(picker.getByText(s.helper, { exact: true })).toBeVisible();
      await capture(viewport, 0);
      await query.fill("敦化南路");
      const best = picker.getByTestId("cvs-candidate-121275");
      await expect(picker.locator('[data-testid^="cvs-candidate-"]').first()).toContainText(s.best);
      await expect(picker.getByTestId("cvs-candidate-127620")).toBeVisible();
      await capture(viewport, 1);
      await query.fill("不存在的火星門市XYZ");
      await expect(picker.getByText(c.empty, { exact: true })).toBeVisible();
      await expect(picker.getByText(s.hint, { exact: true })).toBeVisible();
      await expect(query).toHaveValue("不存在的火星門市XYZ");
      await capture(viewport, 2);
      await query.fill("敦化南路");
      await best.click();
      await facts();
      await expect(picker.locator('[data-testid^="cvs-candidate-"]')).toHaveCount(0);
      assert.equal(posts.length, 0, "a radio-row click cannot confirm a store");
      await capture(viewport, 3);
      await picker.getByRole("button", { name: s.change, exact: true }).click();
      await expect(query).toHaveValue("敦化南路");
      await best.click();
      await picker.getByRole("button", { name: c.confirm, exact: true }).click();
      await expect(picker.getByRole("heading", { name: s.review, exact: true })).toBeVisible();
      await facts();
      await expect(picker.getByRole("button", { name: s.back, exact: true })).toBeVisible();
      assert.equal(posts.length, 0, "review must precede any store confirmation POST");
      await capture(viewport, 4);
      await picker.getByRole("button", { name: s.back, exact: true }).click();
      await expect(picker.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
      await facts();
      await picker.getByRole("button", { name: s.change, exact: true }).click();
      await picker.getByRole("button", { name: s.browse, exact: true }).click();
      await picker.getByTestId("cvs-picker-county").selectOption("臺北市");
      await picker.getByTestId("cvs-picker-district").selectOption("大安區");
      await expect(picker.locator('[data-testid^="cvs-candidate-"]').first()).toBeVisible();
      await picker.getByRole("button", { name: s.back, exact: true }).click();
      await expect(query).toHaveValue("敦化南路");
      await picker.getByRole("button", { name: s.close, exact: true }).click();
      await expect(picker).not.toBeVisible();
      ledger.push({ locale, viewport, control: "signed search / results / chosen / review / back / change / area / close", operation: "real click/fill/selectOption", expected: "signed copy, four canonical facts, no POST before review, legacy tabs absent", actual: "all assertions satisfied", result: "PASS" });
    }
  } finally {
    page.off("request", watch);
    if (originalViewport) await page.setViewportSize(originalViewport);
  }
}

export async function exerciseCvsPaste({ page, locale, openCheckout, ledger }) {
  const fixtures = JSON.parse(await readFile(new URL("../../packages/format/tests/fixtures/cvs-match-real-cases.json", import.meta.url), "utf8"));
  const row = fixtures.cases.find((item) => item.id === "R7");
  await openCheckout();
  await page.getByTestId("cvs-open-picker").click();
  await page.getByTestId("cvs-brand-FAMI").click();
  const query = page.getByTestId("cvs-picker-query");
  await query.fill(row.input);
  const notice = page.getByTestId("cvs-picker-code-miss");
  await expect(notice).toContainText(locale === "en" ? "Code not in store list: 022305" : locale === "zh-CN" ? "该编号未收录在门店清单中：022305" : "此編號不在門市清單：022305");
  const candidate = page.getByTestId("cvs-candidate-024967");
  await expect(candidate).toContainText("泰山新輔醫店");
  await expect(candidate).toContainText("024967");
  await expect(candidate).toContainText("新北市泰山區貴子路６９號Ｂ１");
  await expect(query).toHaveValue(row.input);
  await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
  await candidate.click();
  await expect(page.getByTestId("cvs-picker-confirm")).toBeEnabled();
  await page.getByTestId("cvs-picker-change").click();
  // Query changes must clear the selected replacement, and a code-only miss has no fuzzy candidates.
  await query.fill("F022305");
  await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
  await expect(page.getByTestId("cvs-picker").locator('[data-testid^="cvs-candidate-"]')).toHaveCount(0);
  await expect(query).toHaveValue("F022305");
  ledger.push({ locale, control: "paste / missing code / candidate / change query", operation: "real fill and click", expected: "022305 notice + unselected 024967; query change clears choice; code-only no fallback", actual: "all visible assertions satisfied", result: "PASS" });
  await page.getByTestId("cvs-picker-close").click();
}

// READ/MEASURE only: count visible text fragments and characters, clipping against scroll ancestors.
// Same upper-middle paired median and 12/100/20 thresholds as the supplied density-probe.json.
async function density(picker, states, state) {
  const count = await picker.evaluate((root) => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const texts = [];
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      const text = node.textContent.trim();
      if (!text) continue;
      const parent = node.parentElement;
      if (!parent || parent.closest("[aria-hidden=true]")) continue;
      const range = document.createRange(); range.selectNodeContents(node);
      const rect = range.getBoundingClientRect();
      let left = 0, top = 0, right = innerWidth, bottom = innerHeight, visible = rect.width > 0 && rect.height > 0;
      for (let el = parent; el && visible; el = el.parentElement) {
        const style = getComputedStyle(el), box = el.getBoundingClientRect();
        if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0) visible = false;
        if (/(hidden|auto|scroll|clip)/.test(style.overflowY)) { top = Math.max(top, box.top); bottom = Math.min(bottom, box.bottom); }
        if (/(hidden|auto|scroll|clip)/.test(style.overflowX)) { left = Math.max(left, box.left); right = Math.min(right, box.right); }
      }
      if (visible && rect.bottom > top && rect.top < bottom && rect.right > left && rect.left < right) texts.push(text);
    }
    // Native field values/placeholders are drawn text too; textarea.value is not a DOM text node.
    for (const el of root.querySelectorAll("input:not([type=radio]):not([type=checkbox]):not([type=hidden]), textarea, select")) {
      const rect = el.getBoundingClientRect(), style = getComputedStyle(el);
      if (rect.width <= 0 || rect.height <= 0 || rect.bottom <= 0 || rect.top >= innerHeight || style.visibility === "hidden" || style.display === "none") continue;
      const text = el.tagName === "SELECT" ? el.selectedOptions[0]?.textContent?.trim() : (el.value || el.placeholder).trim();
      if (text) texts.push(text);
    }
    return [texts.length, texts.reduce((n, text) => n + text.length, 0)];
  });
  states.push({ state, blocks: count[0], chars: count[1] });
}

export async function exerciseCvsPicker({ page, locale, mobile, openCheckout, ledger, densities }) {
  const c = localeCopy[locale], posts = [], apiReads = [];
  const request = (r) => {
    const url = new URL(r.url());
    if (url.pathname === "/api/buyer/cvs-stores" && r.method() === "POST") posts.push(r.postDataJSON());
    if (/\/api\/.*(?:cvs|store-search)/.test(url.pathname)) apiReads.push(url.pathname);
  };
  page.on("request", request);
  const record = (control, expected) => ledger.push({ locale, viewport: mobile ? "mobile" : "desktop", control, operation: "real click/fill/selectOption", expected, actual: expected, result: "PASS" });
  const picker = page.getByTestId("cvs-picker");
  const query = page.getByTestId("cvs-picker-query");
  const choose = (code) => page.getByTestId(`cvs-candidate-${code}`);
  const open = async () => {
    await page.getByTestId("cvs-open-picker").click();
    await expect(picker).toBeVisible();
    await expect(picker.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
    await expect(page.getByTestId("cvs-picker-source")).toHaveText(c.source);
    await expect(query).toHaveAccessibleName(c.query);
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
  };
  const reviewChoice = async () => {
    const before = posts.length;
    const code = await page.getByTestId("cvs-picker-chosen-code").innerText();
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveText(c.confirm);
    await page.getByTestId("cvs-picker-confirm").click();
    await expect(picker.getByRole("heading", { name: signedCopy[locale].review, exact: true })).toBeVisible();
    await expect(page.getByTestId("cvs-picker-chosen-code")).toHaveText(code);
    assert.equal(posts.length, before, "entering review never posts a store");
  };
  const measure = async (state) => { if (mobile && locale === "zh-TW") await density(picker, densities, state); };
  try {
    const deliveryKey = await openCheckout();
    await exerciseSignedCvsParity({ page, locale, ledger, densities });
    await page.getByTestId("cvs-recipient-name").fill("王小明"); // existing synthetic recipient, never a real buyer
    await page.getByTestId("cvs-recipient-phone").fill("0912345678");
    // CheckoutFlow's optionKey is market:country:method, while CVS accepts only the service suffix.
    const selectedDelivery = /^[^:]+:[A-Z]{2}:delivery:([a-z0-9_-]+)$/.exec(deliveryKey);
    assert.ok(selectedDelivery, "selected checkout option must contain market, country and a delivery service");
    const service = selectedDelivery[1];
    await open();
    await page.getByTestId("cvs-brand-UNIMART").click();
    await query.fill("");
    await measure("search-empty");
    await query.fill("不存在的火星門市XYZ");
    await expect(page.getByTestId("cvs-picker-empty")).toContainText(c.empty);
    await expect(query).toHaveValue("不存在的火星門市XYZ");
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
    await measure("no-result");
    record("no-result", "query retained, no choice and confirm disabled");

    await query.fill("231196");
    await expect(picker.locator('[data-testid^="cvs-candidate-"]')).toHaveCount(1);
    await expect(choose("231196")).toContainText("久旺");
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
    await measure("exact-unselected");
    assert.equal(posts.length, 0, "typing exact code never posts or chooses");
    const fixture = JSON.parse(await readFile(new URL("../../packages/format/tests/fixtures/cvs-match-real-cases.json", import.meta.url), "utf8"));
    await query.fill(fixture.cases.find((row) => row.id === "R1").input);
    await expect(picker.locator('[data-testid^="cvs-candidate-"]')).toHaveCount(1);
    await expect(choose("231196")).toContainText("久旺");
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
    await choose("231196").click();
    await reviewChoice();
    const resolved = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/cvs-stores" && r.request().method() === "POST");
    const [response] = await Promise.all([resolved, page.getByTestId("cvs-picker-confirm").click()]);
    assert.equal(response.status(), 200);
    const saved = await response.json();
    assert.equal(saved.kind, "cvs_711");
    assert.equal(saved.code, "231196");
    assert.equal(saved.name, "久旺");
    assert.equal(saved.address, "高雄市大樹區九曲里新鎮路1號1樓");
    assert.equal(posts.length, 1, "exactly one POST on explicit confirm");
    assert.equal(posts[0].store_code, "231196");
    assert.equal(posts[0].service_code, service, "kind is resolved from the current server delivery service");
    for (const key of ["tenant_id", "amount", "total_minor"]) assert.equal(Object.hasOwn(posts[0], key), false);
    await expect(page.getByTestId("cvs-store-name")).toContainText("久旺");
    await expect(page.getByTestId("cvs-store-address")).toHaveText(saved.address);
    await confirmCheckoutAddress(page);
    await expect(page.getByTestId("create-order")).toBeEnabled();
    assert.equal(posts.length, 1, "parent ensure reuses the confirmed pickup");
    record("exact code / row / confirm / checkout", "one server-validated kind/code; canonical full name/address in summary");

    await page.reload();
    await openCheckout();
    // openCheckout adds another product: acknowledge its changed server total before continuing the picker journey.
    await expect(page.getByTestId("quote-reconfirm")).toBeVisible();
    await expect(page.getByTestId("create-order")).toBeDisabled();
    await expect(page.getByTestId("checkout-submit")).toBeDisabled();
    await page.getByRole("button", { name: c.confirmTotal, exact: true }).click();
    await expect(page.getByTestId("quote-reconfirm")).toHaveCount(0);
    record("reload / updated total", "both submits blocked until explicit acknowledgment of the changed server total");
    await page.getByTestId("cvs-recipient-name").fill("王小明");
    await page.getByTestId("cvs-recipient-phone").fill("0912345678");
    await open();
    await query.fill("231196");
    await choose("231196").click();
    await reviewChoice();
    const again = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/cvs-stores" && r.request().method() === "POST");
    const [restored] = await Promise.all([again, page.getByTestId("cvs-picker-confirm").click()]);
    assert.equal(restored.status(), 200);
    assert.equal((await restored.json()).pickup_id, saved.pickup_id, "refresh retains canonical persisted pickup identity");
    await expect(page.getByTestId("cvs-store-address")).toHaveText(saved.address);
    record("reload / reselect / confirm", "same persisted pickup identity and canonical facts");

    await page.getByTestId("cvs-change-store").click();
    // Change returns to the picker directly, preserving the originating query.
    await expect(picker).toBeVisible();
    await expect(query).toHaveValue("231196");
    await query.fill("敦化南路");
    await expect(choose("121275")).toBeVisible();
    await expect(choose("127620")).toBeVisible();
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
    await measure("fuzzy-list");
    const beforeSearch = posts.length;
    await choose("121275").click();
    await page.getByTestId("cvs-picker-change").click();
    await choose("127620").click(); // explicitly choose B, never silently retain best-match A
    await expect(page.getByTestId("cvs-picker-confirm")).toBeEnabled();
    assert.equal(posts.length, beforeSearch);
    await reviewChoice();
    const fuzzy = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/cvs-stores" && r.request().method() === "POST");
    const [chosenB] = await Promise.all([fuzzy, page.getByTestId("cvs-picker-confirm").click()]);
    assert.equal((await chosenB.json()).code, "127620");
    await expect(page.getByTestId("cvs-store-name")).toContainText("建安");
    await expect(page.getByTestId("cvs-store-address")).toHaveText("台北市大安區敦化南路一段187巷29號");
    record("fuzzy list / choose A then B / confirm", "B=127620 persisted, not A=121275");

    await page.getByTestId("cvs-change-store").click();
    await page.getByRole("button", { name: signedCopy[locale].browse, exact: true }).click();
    await measure("region-initial");
    await page.getByTestId("cvs-picker-county").selectOption("臺北市");
    await page.getByTestId("cvs-picker-district").selectOption("大安區");
    const source = JSON.parse(await readFile(new URL("../../internal/fulfillment/cvsdata/stores-20261028.json", import.meta.url), "utf8"));
    const visibleRows = picker.locator('[data-testid^="cvs-candidate-"]');
    assert.ok(await visibleRows.count() > 0);
    for (const id of await visibleRows.evaluateAll((nodes) => nodes.map((node) => node.getAttribute("data-testid")))) {
      const row = source.UNIMART.find((store) => store.store_id === id.replace("cvs-candidate-", ""));
      assert.ok(row && /^[台臺]北市大安區/.test(row.address), "county/district never leaks another region or brand");
    }
    const firstPageCount = await visibleRows.count();
    await picker.getByRole("button", { name: locale === "en" ? "Load more" : locale === "zh-CN" ? "加载更多" : "載入更多", exact: true }).click();
    assert.ok(await visibleRows.count() > firstPageCount, "explicit more control exposes additional stores in the same district");
    record("more stores", "additional canonical rows without an API search");
    await measure("region-list");
    await visibleRows.first().click();
    await expect(page.getByTestId("cvs-picker-confirm")).toBeEnabled();
    // Change then Back returns to search without retaining the area choice.
    await page.getByTestId("cvs-picker-change").click();
    await page.getByRole("button", { name: signedCopy[locale].back, exact: true }).click();
    await query.fill("000000");
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
    await expect(query).toHaveValue("000000");
    assert.equal(posts.length, beforeSearch + 1, "area navigation never posts");
    record("brand / county / district / row / switch path", "only selected region, no POST, changed input clears selection");
    await page.getByTestId("cvs-picker-close").click();
    await expect(picker).not.toBeVisible();
    record("close", "unconfirmed choice discarded");
    assert.ok(apiReads.every((route) => route === "/api/buyer/cvs-stores"), "no server-side search requests");
    assert.ok(!/已驗證可收件|已验证可收件|verified for delivery/i.test(await page.getByTestId("address-section").innerText()));

    // Brand changes use the actual delivery/quote controller, preserving only the ephemeral contact draft.
    // FIXTURE ONLY: persist an older home recipient through the real BFF/Go/PG stack.
    // The regression action and confirmation below use real UI clicks, never API writes.
    const origin = new URL(page.url()).origin;
    const sessionResponse = await page.request.get(`${origin}/api/buyer/session`);
    assert.equal(sessionResponse.status(), 200);
    const session = await sessionResponse.json();
    const fixtureHeaders = { "X-Buyer-Context": session.context, Origin: origin };
    const cartResponse = await page.request.get(`${origin}/api/buyer/cart`, { headers: fixtureHeaders });
    assert.equal(cartResponse.status(), 200);
    const cart = await cartResponse.json();
    const headResponse = await page.request.get(`${origin}/api/buyer/destination`, { headers: fixtureHeaders });
    assert.equal(headResponse.status(), 200);
    const { destination: previous } = await headResponse.json();
    const homeResponse = await page.request.put(`${origin}/api/buyer/destination`, {
      headers: { ...fixtureHeaders, "Idempotency-Key": randomUUID() },
      data: {
        expected_version: previous?.version ?? 0, cart_version: cart.version,
        kind: "home", country: "TW", recipient_name: "陳大文", phone: "0923456789",
        home_address: { region: "", city: "臺北市", postal_code: "", line1: "Synthetic Home Address", line2: "" },
      },
    });
    assert.equal(homeResponse.status(), 200, "synthetic older home destination must be persisted before the brand change");
    const oldHome = await homeResponse.json();
    assert.equal(oldHome.kind, "home");
    await page.getByTestId("cvs-recipient-name").fill("王小明");
    await page.getByTestId("cvs-recipient-phone").fill("0912345678");
    // Interception prepares a static-directory outage fixture; retry is still a real user click.
    let directoryAttempts = 0;
    await page.route("**/cvs/okmart-20261028.json", async (route) => {
      directoryAttempts++;
      if (directoryAttempts === 1) await route.fulfill({ status: 503, body: "" });
      else await route.continue();
    });
    await page.getByTestId("cvs-open-picker").click();
    for (const [brand, code] of [["FAMI", "024793"], ["HILIFE", "2656"], ["OKMART", "000005"], ["UNIMART", "231196"]]) {
      // The brand switch recomputes the quote over POST /api/buyer/quotes and then commits the new
      // delivery kind (aria-pressed). A real user waits for that processing to finish before typing
      // the next code; without it the earlier quote commit can still replace this picker instance.
      const quoteWrite = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
      await page.getByTestId(`cvs-brand-${brand}`).click();
      await quoteWrite;
      await expect(page.getByTestId(`cvs-brand-${brand}`)).toHaveAttribute("aria-pressed", "true");
      if (brand === "OKMART") {
        await picker.getByRole("button", { name: locale === "en" ? "Retry" : locale === "zh-CN" ? "重试" : "重試", exact: true }).click();
        await expect.poll(() => directoryAttempts, { message: "directory only retried by an explicit click" }).toBe(2);
      }
      await query.fill(code);
      await expect(choose(code)).toBeVisible();
      await expect(picker.locator('[data-testid^="cvs-candidate-"]')).toHaveCount(1);
      await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
      await expect(page.getByTestId("cvs-recipient-name")).toHaveValue("王小明");
      await expect(page.getByTestId("cvs-recipient-phone")).toHaveValue("0912345678");
      record(`brand ${brand}`, "fresh server quote, exact selected-brand code, retained contact and no preselection");
    }
    await page.unroute("**/cvs/okmart-20261028.json");

    await choose("231196").click();
    await reviewChoice();
    await page.getByTestId("cvs-picker-confirm").click();
    const destinationSaved = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/destination" && r.request().method() === "PUT");
    const [destinationResponse] = await Promise.all([destinationSaved, confirmCheckoutAddress(page)]);
    assert.equal(destinationResponse.status(), 200);
    const resumedDestination = await destinationResponse.json();
    assert.equal(resumedDestination.recipient_name, "王小明", "CVS resume must override the older home recipient after hydration");
    assert.equal(resumedDestination.phone, "0912345678");
    assert.equal(resumedDestination.version, oldHome.version + 1, "resume still loads the current server head for CAS");
    await expect(page.getByTestId("quote-reconfirm")).toHaveCount(0);
    await expect(page.getByTestId("create-order")).toBeEnabled();
    record("older home recipient / brand switch / confirm destination", "current CVS contact retained after hydration and saved against latest home head");
    await page.getByTestId("cvs-change-store").click();

    // Refusal fixtures exercise UI recovery; the existing real Go/PG refusal assertions stay in cvs-buyer.mjs.
    let confirmations = 0;
    await page.route("**/api/buyer/cvs-stores", async (route) => {
      confirmations++;
      await route.fulfill({ status: confirmations === 1 ? 422 : 429, headers: { "Content-Type": "application/json", "Retry-After": "1" }, body: JSON.stringify({ code: confirmations === 1 ? "store_not_found" : "too_many_stores_entered" }) });
    });
    await choose("231196").click();
    await reviewChoice();
    await page.getByTestId("cvs-picker-confirm").click();
    await expect(choose("231196")).toHaveCount(0);
    await expect(query).toHaveValue("231196");
    await expect(page.getByTestId("cvs-picker-confirm")).toHaveCount(0);
    await query.fill("131386");
    await choose("131386").click();
    await reviewChoice();
    await page.getByTestId("cvs-picker-confirm").click();
    await expect(picker.getByRole("alert")).toContainText(locale === "en" ? "Please wait" : locale === "zh-CN" ? "请稍后" : "請稍後");
    await expect(page.getByTestId("cvs-picker-chosen-code")).toHaveText("131386");
    await expect(picker.getByRole("heading", { name: signedCopy[locale].review, exact: true })).toBeVisible();
    await expect(page.getByTestId("cvs-picker-confirm")).toBeEnabled();
    assert.equal(confirmations, 2, "cooldown expiration cannot auto-replay a POST");
    await page.unroute("**/api/buyer/cvs-stores");
    const retry = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/cvs-stores" && r.request().method() === "POST");
    const [confirmedRetry] = await Promise.all([retry, page.getByTestId("cvs-picker-confirm").click()]);
    assert.equal(confirmedRetry.status(), 200);
    await expect(page.getByTestId("cvs-store-code")).toHaveText("131386");
    record("422 / 429 / explicit retry", "invalid row removed with query retained; cooldown keeps choice; no auto replay; explicit retry persists canonical identity");
    await page.reload();
    // READ ONLY: independently read the actual persisted destination after refresh.
    const persistedResponse = await page.request.get(`${origin}/api/buyer/destination`, { headers: fixtureHeaders });
    assert.equal(persistedResponse.status(), 200);
    const { destination: persisted } = await persistedResponse.json();
    assert.equal(persisted.id, resumedDestination.id);
    assert.equal(persisted.recipient_name, "王小明");
    assert.equal(persisted.phone, "0912345678");
    record("refresh / persisted resumed destination", "server retains the explicitly confirmed CVS contact B, not older home contact A");
  } finally { page.off("request", request); }
}

export function assertPickerDensity(states) {
  assert.ok(states.length >= 6, "all registered mobile picker states measured");
  for (const state of states) assert.ok(state.blocks <= 20, `${state.state}: mobile screen exceeds 20 visible text blocks (${state.blocks})`);
  const sorted = [...states].sort((a, b) => a.blocks - b.blocks);
  const median = sorted[Math.floor(sorted.length / 2)];
  assert.ok(median.blocks <= 12 && median.chars <= 100, `mobile paired median ${JSON.stringify(median)}`);
  return { states, median, maxBlocks: Math.max(...states.map((row) => row.blocks)) };
}
