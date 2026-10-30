// Purpose: signed settings-v2 real-click acceptance within the existing single-session settings harness.
// Depends on: Playwright, settings-real.spec.ts's signed MOCK IdP / isolated PG workspace and profile API.
// Used by: settings-real.spec.ts; --browser-identity CI (never a second session or local server).
// Invariants: clicks/fills drive writes; refresh checks persisted receipts; faults are explicitly MOCK;
// no provider calls, fixture tenant authority, forced clicks, DOM writes or direct API mutations.
import { expect, type Page } from "@playwright/test";

const copy = {
  "zh-TW": { profile: "店舖資料 · 未填寫", delivery: "配送方式 · 已開啟 0 種", payment: "收款 · Stripe（平台代收）", edit: "編輯設定" },
  "zh-CN": { profile: "店铺资料 · 未填写", delivery: "配送方式 · 已启用 0 种", payment: "收款 · Stripe（平台代收）", edit: "编辑设置" },
  en: { profile: "Store details · Incomplete", delivery: "Delivery methods · 0 enabled", payment: "Payments · Stripe (collected by platform)", edit: "Edit settings" },
};

// This is a helper, deliberately called by the existing test rather than registering
// another test: the Go fixture independently requires exactly one session/store.
export async function settingsV2Acceptance(page: Page, store: string) {
  const path = `/en/settings?store=${store}`;
  await page.setViewportSize({ width: 390, height: 844 });
  for (const [locale, c] of Object.entries(copy)) {
    await page.goto(`/${locale}/settings?store=${store}`);
    const overview = page.getByTestId("settings-v2-overview");
    await expect(overview, `${locale}: signed three-row overview`).toBeVisible();
    await expect(overview.getByRole("button", { name: c.profile, exact: true })).toBeVisible();
    await expect(overview.getByRole("button", { name: c.delivery, exact: true })).toBeVisible();
    await expect(overview.getByRole("button", { name: c.payment, exact: true })).toBeVisible();
    if (locale !== "en") {
      // READ/MEASURE: analogous to density-probe's visible text-node/character
      // count, scoped to the product state, excluding global navigation chrome.
      const density = await overview.evaluate(root => {
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        const texts: string[] = [];
        while (walker.nextNode()) {
          const node = walker.currentNode;
          if (node.parentElement?.checkVisibility() && node.textContent?.trim()) texts.push(node.textContent.trim());
        }
        return { blocks: texts.length, chars: texts.reduce((n, text) => n + text.length, 0) };
      });
      expect(density.blocks).toBeLessThanOrEqual(12);
      expect(density.chars).toBeLessThanOrEqual(100);
    }
    await overview.getByRole("button", { name: c.edit, exact: true }).click();
    await expect(page.getByTestId("settings-v2-editor")).toBeVisible();
    await page.goto(path);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  // READ/MEASURE only: test product overflow; never changes DOM or performs a write.
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const overview = page.getByTestId("settings-v2-overview");
  // Each signed navigation row must actually open the editor, including Payments.
  for (const name of [copy.en.profile, copy.en.delivery, copy.en.payment]) {
    await overview.getByRole("button", { name, exact: true }).click();
    await expect(page.getByTestId("settings-v2-editor")).toBeVisible();
    await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();
  }
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await page.getByLabel("Store name", { exact: true }).fill("Settings acceptance store");
  await page.getByLabel("Email", { exact: true }).fill("settings@example.test");
  await page.getByLabel("Phone (optional)", { exact: true }).fill("000-000-0000");
  for (const label of ["Home delivery", "7-ELEVEN", "FamilyMart", "Hi-Life", "OK"]) {
    // Fresh stores have no existing market/service/policy: the signed contract
    // forbids fabricated defaults. Disabled switches must explain the prerequisite.
    await expect(page.getByRole("checkbox", { name: label, exact: true })).toBeDisabled();
  }
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  const review = page.getByTestId("settings-v2-review");
  for (const fact of ["Settings acceptance store", "settings@example.test", "000-000-0000", "Stripe", "No cash on delivery", "carrier", "tracking", "label"]) {
    await expect(review).toContainText(fact);
  }
  for (const label of ["Home delivery", "7-ELEVEN", "FamilyMart", "Hi-Life", "OK"]) await expect(review).toContainText(label);
  await page.getByRole("button", { name: "Back to edit", exact: true }).click();
  await expect(page.getByLabel("Email", { exact: true })).toHaveValue("settings@example.test");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  const saved = page.waitForResponse(r => new URL(r.url()).pathname === `/api/stores/${store}/profile` && r.request().method() === "PUT");
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  const receipt = await saved;
  expect(receipt.status()).toBe(200);
  expect(await receipt.json()).toMatchObject({ name: "Settings acceptance store", contact_email: "settings@example.test", contact_phone: "000-000-0000" });
  await expect(page.getByRole("heading", { name: "Settings saved", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();
  await page.reload();
  await expect(overview.getByRole("button", { name: "Store details · Complete", exact: true })).toBeVisible();
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await expect(page.getByLabel("Store name", { exact: true })).toHaveValue("Settings acceptance store");
  await expect(page.getByLabel("Email", { exact: true })).toHaveValue("settings@example.test");
  await expect(page.getByLabel("Phone (optional)", { exact: true })).toHaveValue("000-000-0000");

  // MOCK definite rejection: the saved server profile must remain unchanged,
  // while the attempted edit survives recovery. No automatic write retry.
  let writes = 0;
  const profileURL = `**/api/stores/${store}/profile`;
  await page.route(profileURL, async route => {
    if (route.request().method() !== "PUT") return route.continue();
    writes++;
    await route.fulfill({ status: 422, contentType: "application/json", body: JSON.stringify({ code: "invalid_input", request_id: "settings-v2-mock-reject", retryable: false, details: {} }) });
  });
  await page.getByLabel("Phone (optional)", { exact: true }).fill("000-000-1111");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Settings not saved", exact: true })).toBeVisible();
  await expect(page.getByText("Saved settings are unchanged; your draft is retained.", { exact: true })).toBeVisible();
  expect(writes).toBe(1);
  await page.getByRole("button", { name: "Back to edit", exact: true }).click();
  await expect(page.getByLabel("Phone (optional)", { exact: true })).toHaveValue("000-000-1111");
  await page.unroute(profileURL);
  let uncertainCommand: { key: string; body: string } | null = null;
  await page.route(profileURL, async route => {
    if (route.request().method() !== "PUT") return route.continue();
    writes++;
    uncertainCommand = { key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" };
    await route.abort("failed"); // MOCK uncertain transport, never a blind fresh-key retry.
  });
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  await page.unroute(profileURL);
  await page.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(overview).toBeVisible();
  expect(writes).toBe(2);
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await expect(page.getByLabel("Phone (optional)", { exact: true })).toHaveValue("000-000-0000");
  // Reload above is read-only even when an aborted request has no receipt. An
  // explicit save of the original attempted values must reuse its exact key/body;
  // reading an unchanged profile is not proof the earlier command cannot commit.
  const recovery: Array<{ key: string; body: string }> = [];
  await page.route(profileURL, async route => {
    if (route.request().method() === "PUT") recovery.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    await route.continue();
  });
  await page.getByLabel("Phone (optional)", { exact: true }).fill("000-000-1111");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Settings saved", exact: true })).toBeVisible();
  expect(recovery).toEqual([uncertainCommand]);
  await page.unroute(profileURL);
  await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();

  // All three signed quick-start steps, back controls and explicit dismissal.
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  await expect(page.getByText("1 / 3 · Store name", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await expect(page.getByText("2 / 3 · Delivery", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Back: store name", exact: true }).click();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByText("3 / 3 · First product", { exact: true })).toBeVisible();
  await page.getByLabel("Product name", { exact: true }).fill("Acceptance draft bag");
  await page.getByLabel("Price (TWD)", { exact: true }).fill("1280");
  await page.getByLabel("Stock (units)", { exact: true }).fill("10");
  await expect(page.getByText("Product is saved as a draft, not published.", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Back: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByLabel("Product name", { exact: true })).toHaveValue("Acceptance draft bag");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  for (const fact of ["Acceptance draft bag", "1280", "10", "TWD"]) await expect(review).toContainText(fact);
  const inventoryURL = `**/api/stores/${store}/inventory/adjustments`;
  const inventoryCommands: Array<{ key: string; body: string }> = [];
  await page.route(inventoryURL, async route => {
    expect(route.request().method()).toBe("POST");
    inventoryCommands.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    await route.abort("failed"); // MOCK transport loss BEFORE commit; product/SKU creation still uses REAL_PG.
  });
  // The preceding SKU command can commit before its response is lost. Its
  // unexecuted initial stock must survive recovery, not become the read balance.
  const skuURL = `**/api/stores/${store}/skus`;
  const skuCommands: Array<{ key: string; body: string }> = [];
  await page.route(skuURL, async route => {
    expect(route.request().method()).toBe("POST");
    skuCommands.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    // Fault injection only: forward this real UI command to PG, then lose its
    // response. This never creates a separate API mutation or synthetic receipt.
    const committed = await route.fetch();
    expect(committed.ok()).toBe(true);
    expect(await committed.json()).toMatchObject(JSON.parse(skuCommands[0].body));
    await route.abort("failed");
  });
  // RVFIX post-create boundary: the real product receipt succeeds, but its
  // first detail read fails before a SKU or initial-stock command is sent.
  const createdDetailURL = `**/api/stores/${store}/products/*`;
  let createdDetailReads = 0;
  await page.route(createdDetailURL, async route => {
    if (route.request().method() === "GET" && ++createdDetailReads === 1) {
      await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "unavailable", request_id: "settings-v2-post-create-read", retryable: false, details: {} }) });
    } else await route.continue();
  });
  const postCreateCommands: string[] = [];
  page.on("request", request => {
    if (["POST", "PUT"].includes(request.method()) && new URL(request.url()).pathname.startsWith(`/api/stores/${store}/`)) postCreateCommands.push(new URL(request.url()).pathname);
  });
  const productSaved = page.waitForResponse(r => r.request().method() === "POST" && /\/api\/stores\/[^/]+\/products$/.test(new URL(r.url()).pathname));
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  expect((await productSaved).ok()).toBe(true);
  const createdProduct = await (await productSaved).json();
  expect(createdProduct).toMatchObject({ name: "Acceptance draft bag", status: "draft" });
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(createdDetailReads, "the failure follows the acknowledged product, before SKU dispatch").toBe(1);
  expect(skuCommands).toHaveLength(0);
  expect(inventoryCommands).toHaveLength(0);
  expect(postCreateCommands).toEqual([`/api/stores/${store}/products`]);
  await page.unroute(createdDetailURL);
  // Another operator uses Catalog's real Copy control while the acknowledged
  // product has no SKU. Discovery now leads with a different, newer draft.
  const createObserver = await page.context().newPage();
  await createObserver.goto(`/en/products?store=${store}`);
  const createCopied = createObserver.waitForResponse(r => r.request().method() === "POST" &&
    new URL(r.url()).pathname === `/api/stores/${store}/products/${createdProduct.id}/copy`);
  await createObserver.getByTestId(`product-row-${createdProduct.id}`).getByRole("checkbox").click();
  await createObserver.getByTestId("products-advanced").click();
  await createObserver.getByTestId("products-copy-selected").click();
  const createCopyResponse = await createCopied;
  expect(createCopyResponse.ok()).toBe(true);
  const competingProduct = (await createCopyResponse.json()).id as string;
  expect(competingProduct).not.toBe(createdProduct.id);
  const readCompeting = () => createObserver.waitForResponse(r => r.request().method() === "GET" &&
    new URL(r.url()).pathname === `/api/stores/${store}/products/${competingProduct}`);
  const competingBefore = readCompeting();
  await createObserver.goto(`/en/products/${competingProduct}?store=${store}`);
  // Native, tab-persisted editor choice: Catalog now opens its simplified view.
  await createObserver.getByTestId("catalog-advanced-toggle").click();
  await expect(createObserver.getByTestId("product-form")).toBeVisible();
  const competingBaseline = await (await competingBefore).json();
  expect(competingBaseline.skus).toEqual([]);
  const createDiscovery = createObserver.waitForResponse(r => {
    const url = new URL(r.url());
    return r.request().method() === "GET" && url.pathname === `/api/stores/${store}/catalog-products` &&
      url.searchParams.get("status") === "draft" && url.searchParams.get("limit") === "1";
  });
  await createObserver.goto(path);
  expect((await (await createDiscovery).json()).items[0].id).toBe(competingProduct);
  await createObserver.goto(`/en/products/${competingProduct}?store=${store}`);
  await page.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(overview).toBeVisible();
  await page.reload();
  await expect(overview).toBeVisible();
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByLabel("Product name", { exact: true }), "post-create read recovery must retain acknowledged A despite newer draft discovery").toHaveValue("Acceptance draft bag");
  await expect(page.getByLabel("Price (TWD)", { exact: true })).toHaveValue("1280");
  await expect(page.getByLabel("Stock (units)", { exact: true })).toHaveValue("10");
  expect(postCreateCommands, "Reload, refresh and setup navigation must remain read-only").toEqual([`/api/stores/${store}/products`]);
  expect(skuCommands).toHaveLength(0);
  expect(inventoryCommands).toHaveLength(0);
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  for (const fact of ["Acceptance draft bag", "1280", "10", "TWD"]) await expect(review).toContainText(fact);
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(skuCommands).toHaveLength(1);
  const originalSku = skuCommands[0];
  expect(JSON.parse(originalSku.body).product_id, "explicit Save resumes the acknowledged product without creating another").toBe(createdProduct.id);
  expect(postCreateCommands.filter(p => p === `/api/stores/${store}/products`)).toHaveLength(1);
  expect(originalSku.key).toBeTruthy();
  expect(inventoryCommands, "initial stock has not been submitted after the lost SKU response").toHaveLength(0);
  // RVFIX #303 multi-product recovery: another operator's normal Catalog Copy
  // creates the newer draft B while A's exact SKU command remains unresolved.
  // All writes use native controls; this tab shares only the signed session.
  const productA = JSON.parse(originalSku.body).product_id as string;
  const catalog = await page.context().newPage();
  await catalog.goto(`/en/products?store=${store}`);
  const copied = catalog.waitForResponse(r => r.request().method() === "POST" &&
    new URL(r.url()).pathname === `/api/stores/${store}/products/${productA}/copy`);
  await catalog.getByTestId(`product-row-${productA}`).getByRole("checkbox").click();
  await catalog.getByTestId("products-advanced").click();
  await catalog.getByTestId("products-copy-selected").click();
  const copiedResponse = await copied;
  expect(copiedResponse.ok(), "Catalog creates a real newer draft B").toBe(true);
  const productB = (await copiedResponse.json()).id as string;
  expect(productB).toMatch(/^[0-9a-f-]{36}$/);
  expect(productB).not.toBe(productA);
  const readB = () => catalog.waitForResponse(r => r.request().method() === "GET" &&
    new URL(r.url()).pathname === `/api/stores/${store}/products/${productB}`);
  const beforeB = readB();
  await catalog.goto(`/en/products/${productB}?store=${store}`);
  await catalog.getByTestId("catalog-advanced-toggle").click();
  await expect(catalog.getByTestId("product-form")).toBeVisible();
  // READ ONLY: baseline B's actual SKU/inventory facts, not synthetic fixtures.
  const baselineB = await (await beforeB).json();
  expect(baselineB).toMatchObject({ id: productB, status: "draft" });
  await expect(catalog.getByTestId("product-name")).toHaveValue(baselineB.name);
  // READ ONLY: observe settings' own signed discovery request, as with readB.
  const discovery = catalog.waitForResponse(r => {
    const url = new URL(r.url());
    return r.request().method() === "GET" && url.pathname === `/api/stores/${store}/catalog-products` &&
      url.searchParams.get("status") === "draft" && url.searchParams.get("limit") === "1";
  });
  await catalog.goto(`/en/settings?store=${store}`);
  const latestDraft = await discovery;
  expect(latestDraft.ok()).toBe(true);
  expect((await latestDraft.json()).items[0].id, "B really precedes A in catalog discovery").toBe(productB);
  // Return the observer tab to B after reading Settings discovery, so its
  // later native refresh checks B's persisted editor rather than Settings.
  await catalog.goto(`/en/products/${productB}?store=${store}`);
  await page.unroute(skuURL);
  await page.route(skuURL, async route => {
    skuCommands.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    await route.continue();
  });
  await page.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(overview, "pending SKU recovery must use intent-bound A when the newest draft is B").toBeVisible();
  await expect(overview).toBeVisible();
  expect(skuCommands, "Reload must not replay the SKU command").toEqual([originalSku]);
  expect(inventoryCommands, "Reload must not initialize stock").toHaveLength(0);
  await page.reload();
  await expect(overview).toBeVisible();
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByLabel("Product name", { exact: true })).toHaveValue("Acceptance draft bag");
  await expect(page.getByLabel("Price (TWD)", { exact: true })).toHaveValue("1280");
  await expect(page.getByLabel("Stock (units)", { exact: true }), "a committed SKU with a lost response must retain the unexecuted initial stock across Reload and refresh").toHaveValue("10");
  expect(skuCommands, "refresh and reopening setup must remain read-only").toEqual([originalSku]);
  expect(inventoryCommands).toHaveLength(0);
  // READ ONLY: locate this exact pending command's nonsecret draft, without
  // assuming a session hash or borrowing another store/session's cache.
  const recoveryCache = await page.evaluate(commandKey => {
    const entry = Object.entries(sessionStorage).find(([key, value]) => key.endsWith(":command") && JSON.parse(value).key === commandKey);
    if (!entry) return null;
    const key = entry[0].slice(0, -":command".length), value = sessionStorage.getItem(key);
    return value === null ? null : { key, value, command: entry[1] };
  }, originalSku.key);
  if (!recoveryCache) throw new Error("pending SKU must retain its original scoped draft");
  const originalDraft = JSON.parse(recoveryCache.value);
  const invalidDrafts: Array<[string, string | null]> = [
    ["missing", null],
    ["wrong profile version", JSON.stringify({ ...originalDraft, version: originalDraft.version + 1 })],
    ...[
      { productName: "Different product" }, { price: "1" }, { stock: "" }, { stock: "-1" }, { stock: "1.5" },
    ].map(patch => [JSON.stringify(patch), JSON.stringify({ ...originalDraft, draft: { ...originalDraft.draft, ...patch } })] as [string, string]),
  ];
  for (const [scenario, invalid] of invalidDrafts) {
    // FIXTURE PREPARATION ONLY: model missing/corrupt recovery storage. This
    // never changes DOM or sends commands; all recovery is real Reload clicks.
    await page.evaluate(({ key, invalid }) => {
      if (invalid === null) sessionStorage.removeItem(key);
      else sessionStorage.setItem(key, invalid);
    }, { key: recoveryCache.key, invalid });
    await page.reload();
    await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true }), scenario).toBeVisible();
    await page.getByRole("button", { name: "Reload", exact: true }).click();
    // The heading persists while load() is pending. Wait for its controls to
    // return before checking storage or replacing this recovery fixture.
    await expect(page.getByRole("button", { name: "Reload", exact: true }), scenario).toBeEnabled();
    await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true }), scenario).toBeVisible();
    await expect(overview).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Save settings", exact: true })).toHaveCount(0);
    // READ ONLY: a failed recovery must leave both the draft and command intact.
    expect(await page.evaluate(key => sessionStorage.getItem(key), recoveryCache.key), scenario).toBe(invalid);
    expect(await page.evaluate(key => sessionStorage.getItem(`${key}:command`), recoveryCache.key), scenario).toBe(recoveryCache.command);
    expect(skuCommands, scenario).toEqual([originalSku]);
    expect(inventoryCommands, scenario).toHaveLength(0);
  }
  // FIXTURE RESTORATION ONLY: put back the original draft, then use real UI
  // navigation to resume the unchanged exact-replay and persisted-stock checks.
  await page.evaluate(({ key, value }) => sessionStorage.setItem(key, value), recoveryCache);
  await page.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(overview).toBeVisible();
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByLabel("Stock (units)", { exact: true })).toHaveValue("10");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await expect(review).toContainText("Stock (units): 10");
  // A normal SKU acknowledgement clears its command journal. Fail only the
  // immediately following product read, before any initial stock command exists.
  // MOCK read fault only; the replayed SKU still receives its actual PG receipt.
  const productURL = `**/api/stores/${store}/products/${JSON.parse(originalSku.body).product_id}`;
  let productReads = 0;
  await page.route(productURL, async route => {
    expect(route.request().method()).toBe("GET");
    if (++productReads === 1) {
      await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "unavailable", request_id: "settings-v2-post-sku-read", retryable: false, details: {} }) });
    } else await route.continue();
  });
  const skuAcknowledged = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === `/api/stores/${store}/skus`);
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(skuCommands, "explicit Save must replay the identical SKU key and body exactly once").toEqual([originalSku, originalSku]);
  const acknowledged = await skuAcknowledged;
  expect(acknowledged.status(), "the SKU write succeeded; only its follow-up read failed").toBe(200);
  expect(await acknowledged.json()).toMatchObject(JSON.parse(originalSku.body));
  expect(productReads, "only the immediate post-acknowledgement product read has failed").toBe(1);
  expect(inventoryCommands, "the failed read occurs before any initial adjustment").toHaveLength(0);
  // READ ONLY: distinguish this failure from a still-pending SKU command.
  expect(await page.evaluate(key => sessionStorage.getItem(`${key}:command`), recoveryCache.key)).toBeNull();
  // READ ONLY: the next unsent operation must survive independently of the
  // acknowledged SKU journal, and remain bound to this product/warehouse.
  const stockIntentKey = `${recoveryCache.key}:initial-stock`;
  const stockIntent = await page.evaluate(key => sessionStorage.getItem(key), stockIntentKey);
  expect(stockIntent).not.toBeNull();
  if (!stockIntent) throw new Error("acknowledged SKU must retain its unsent stock intent");
  const originalIntent = JSON.parse(stockIntent);
  expect(originalIntent.product_id, "stock intent remains bound to A after a newer draft B is created").toBe(productA);
  for (const patch of [
    { product_id: productB }, { product_id: "../profile" },
    { product_id: "00000000-0000-4000-8000-000000000001" }, { code: "different-sku" },
    { warehouse_id: "00000000-0000-4000-8000-000000000002" }, { currency: "USD" },
    { price_minor: 1 }, { quantity: -1 }, { quantity: 1.5 }, { quantity: 11 },
  ]) {
    const invalid = JSON.stringify({ ...originalIntent, ...patch });
    // FIXTURE PREPARATION ONLY: corrupt local recovery data, never DOM or a
    // backend mutation. Refresh and Reload remain actual user interactions.
    await page.evaluate(({ key, value }) => sessionStorage.setItem(key, value), { key: stockIntentKey, value: invalid });
    await page.reload();
    await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Reload", exact: true }).click();
    await expect(page.getByRole("button", { name: "Reload", exact: true })).toBeEnabled();
    await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
    await expect(overview).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Save settings", exact: true })).toHaveCount(0);
    // READ ONLY: failed recovery neither rewrites its evidence nor sends a command.
    expect(await page.evaluate(key => sessionStorage.getItem(key), stockIntentKey)).toBe(invalid);
    expect(await page.evaluate(key => sessionStorage.getItem(`${key}:command`), recoveryCache.key)).toBeNull();
    expect(skuCommands).toEqual([originalSku, originalSku]);
    expect(inventoryCommands).toHaveLength(0);
  }
  // FIXTURE RESTORATION ONLY: resume the original positive recovery journey.
  await page.evaluate(({ key, value }) => sessionStorage.setItem(key, value), { key: stockIntentKey, value: stockIntent });
  await page.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(overview, "acknowledged SKU recovery must use stock-intent A while newer B remains a draft").toBeVisible();
  await expect(overview).toBeVisible();
  expect(skuCommands, "Reload after an acknowledged SKU must only read").toEqual([originalSku, originalSku]);
  expect(inventoryCommands).toHaveLength(0);
  // RVFIX #303: ordinary settings saves must keep the recovery draft paired
  // with the unsent stock intent, even when the profile version advances.
  // Both branches use normal controls; no storage restoration hides draft loss.
  for (const editProfile of [false, true]) {
    await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
    if (editProfile) await page.getByLabel("Phone (optional)", { exact: true }).fill("000-000-3333");
    await page.getByRole("button", { name: "Review settings", exact: true }).click();
    await expect(review).toContainText("Stock (units): 10");
    await page.getByRole("button", { name: "Save settings", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Settings saved", exact: true })).toBeVisible();
    await page.reload();
    await expect(overview, `ordinary save (profile edit: ${editProfile}) must retain usable stock recovery after refresh`).toBeVisible();
    // READ ONLY: the same intent and its quantity-bearing draft survive; a
    // profile save may update the draft version but must not initialize stock.
    expect(await page.evaluate(key => sessionStorage.getItem(key), stockIntentKey)).toBe(stockIntent);
    const retained = await page.evaluate(key => sessionStorage.getItem(key), recoveryCache.key);
    expect(retained).not.toBeNull();
    expect(JSON.parse(retained!).draft).toMatchObject({ productName: "Acceptance draft bag", price: "1280", stock: "10" });
    expect(skuCommands, "ordinary settings save cannot repeat the acknowledged SKU").toEqual([originalSku, originalSku]);
    expect(inventoryCommands, "ordinary settings save cannot send the retained stock").toHaveLength(0);
    if (editProfile) {
      await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
      await expect(page.getByLabel("Phone (optional)", { exact: true })).toHaveValue("000-000-3333");
      await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();
    }
  }
  await page.reload();
  await expect(overview).toBeVisible();
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByLabel("Product name", { exact: true })).toHaveValue("Acceptance draft bag");
  await expect(page.getByLabel("Price (TWD)", { exact: true })).toHaveValue("1280");
  await expect(page.getByLabel("Stock (units)", { exact: true }), "an acknowledged SKU followed by a failed product read must retain the unsent initial stock across Reload and refresh").toHaveValue("10");
  expect(skuCommands, "refresh and setup navigation must not repeat an acknowledged SKU").toEqual([originalSku, originalSku]);
  expect(inventoryCommands, "reads must not initialize the retained stock").toHaveLength(0);
  await page.unroute(productURL);
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await expect(review).toContainText("Stock (units): 10");
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(skuCommands, "explicit stock recovery must not resend the acknowledged SKU").toEqual([originalSku, originalSku]);
  await page.unroute(skuURL);
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(inventoryCommands).toHaveLength(1);
  const originalInventory = inventoryCommands[0];
  expect(originalInventory.key).toBeTruthy();
  expect(JSON.parse(originalInventory.body)).toMatchObject({ delta: 10, expected_version: 0, reason: "quickstart_initial_stock" });
  await page.unroute(inventoryURL);
  await page.route(inventoryURL, async route => {
    inventoryCommands.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    await route.continue();
  });
  await page.getByRole("button", { name: "Reload", exact: true }).click();
  await expect(overview).toBeVisible();
  expect(inventoryCommands, "Reload must only read; replay requires an explicit Save").toEqual([originalInventory]);
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  await page.getByRole("button", { name: "Next: delivery", exact: true }).click();
  await page.getByRole("button", { name: "Next: product", exact: true }).click();
  await expect(page.getByLabel("Product name", { exact: true })).toHaveValue("Acceptance draft bag");
  await expect(page.getByLabel("Price (TWD)", { exact: true })).toHaveValue("1280");
  await expect(page.getByLabel("Stock (units)", { exact: true }), "Reload must retain the original pending stock, not replace it with the current zero balance").toHaveValue("10");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await expect(review).toContainText("Stock (units): 10");
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Settings saved", exact: true })).toBeVisible();
  expect(inventoryCommands, "exactly one explicit replay of the original key and body").toEqual([originalInventory, originalInventory]);
  expect(JSON.parse(originalInventory.body).sku_id, "the original stock command targets A's acknowledged SKU").toBe((await acknowledged.json()).id);
  // Native refresh proves B's editor and persisted SKU/inventory facts did not
  // change during either recovery; only A received the original command replay.
  const afterB = readB();
  await catalog.reload();
  await expect(catalog.getByTestId("product-form")).toBeVisible();
  await expect(catalog.getByTestId("product-name")).toHaveValue(baselineB.name);
  const recoveredB = await (await afterB).json();
  expect(recoveredB.version).toBe(baselineB.version);
  expect(recoveredB.skus, "A's recovery must not mutate B's SKU or inventory").toEqual(baselineB.skus);
  // The competitor created at the earlier product-ACK boundary must also
  // remain unchanged after A's explicit SKU and stock recovery completes.
  const competingAfter = readCompeting();
  await createObserver.reload();
  await expect(createObserver.getByTestId("product-form")).toBeVisible();
  await expect(createObserver.getByTestId("product-name")).toHaveValue(competingBaseline.name);
  const competingRecovered = await (await competingAfter).json();
  expect(competingRecovered.version).toBe(competingBaseline.version);
  expect(competingRecovered.skus).toEqual(competingBaseline.skus);
  await createObserver.goto(`/en/products?store=${store}`);
  await createObserver.getByTestId(`product-row-${competingProduct}`).getByRole("checkbox").click();
  await createObserver.getByTestId("product-batch").getByRole("button", { name: "Archive products", exact: true }).click();
  const competingArchived = createObserver.waitForResponse(r => r.request().method() === "POST" &&
    new URL(r.url()).pathname === `/api/stores/${store}/products/bulk-status`);
  await createObserver.getByRole("button", { name: "Confirm archive", exact: true }).click();
  expect((await competingArchived).ok()).toBe(true);
  await createObserver.reload();
  await expect(createObserver.getByTestId(`product-row-${competingProduct}`)).toContainText("Archived");
  await createObserver.close();
  // Retire only this CI-owned competing draft through the real Catalog controls
  // after both recovery scenarios, preserving the original single-draft checks.
  await catalog.goto(`/en/products?store=${store}`);
  await catalog.getByTestId(`product-row-${productB}`).getByRole("checkbox").click();
  await catalog.getByTestId("product-batch").getByRole("button", { name: "Archive products", exact: true }).click();
  const archivedB = catalog.waitForResponse(r => r.request().method() === "POST" &&
    new URL(r.url()).pathname === `/api/stores/${store}/products/bulk-status`);
  await catalog.getByRole("button", { name: "Confirm archive", exact: true }).click();
  expect((await archivedB).ok()).toBe(true);
  await catalog.reload();
  await expect(catalog.getByTestId(`product-row-${productB}`)).toContainText("Archived");
  await catalog.close();
  // READ ONLY: only the accepted initial adjustment retires its scoped intent.
  expect(await page.evaluate(key => sessionStorage.getItem(key), stockIntentKey)).toBeNull();
  await page.unroute(inventoryURL);
  await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();
  await page.reload();
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  for (const fact of ["Acceptance draft bag", "1280", "10", "TWD"]) await expect(review).toContainText(fact);
  await page.getByRole("button", { name: "Back to edit", exact: true }).click();
  await page.getByRole("button", { name: "Back to overview, keep draft", exact: true }).click();
  await page.getByRole("button", { name: "Start quick setup", exact: true }).click();
  // Skip is an explicit dismissal; it must persist in the existing server marker.
  // MOCK only the lost response AFTER the real authenticated PG command commits.
  const quickstartURL = `**/api/stores/${store}/quickstart`;
  const markerCommands: Array<{ key: string; body: string }> = [];
  let skippedStatus: number | undefined;
  await page.route(quickstartURL, async route => {
    if (route.request().method() !== "POST") return route.continue();
    markerCommands.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    const committed = await route.fetch(); // Fault injection: forward the UI command, never initiate a separate API mutation.
    skippedStatus = committed.status();
    expect(await committed.json()).toMatchObject({ completed: true });
    await route.abort("failed");
  });
  await page.getByRole("button", { name: "Skip", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(skippedStatus).toBe(200);
  expect(markerCommands).toHaveLength(1);
  expect(markerCommands[0].key).toBeTruthy();
  expect(markerCommands[0].body).toBe("{}");
  await page.unroute(quickstartURL);
  await page.reload();
  await expect(overview).toBeVisible();
  await expect(page.getByTestId("settings-v2-quickstart-prompt")).toHaveCount(0);
  // No storage clearing or hidden setup control: ordinary profile edits must
  // work after the completed marker is read, including across another refresh.
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await page.getByLabel("Phone (optional)", { exact: true }).fill("000-000-2222");
  await page.getByRole("button", { name: "Review settings", exact: true }).click();
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Settings saved", exact: true }), "a committed marker with a lost response must not block later profile saves").toBeVisible();
  await page.reload();
  await expect(page.getByTestId("settings-v2-quickstart-prompt")).toHaveCount(0);
  await overview.getByRole("button", { name: "Edit settings", exact: true }).click();
  await expect(page.getByLabel("Phone (optional)", { exact: true })).toHaveValue("000-000-2222");
  await deliverySwitches(page, store, path);
  await page.setViewportSize({ width: 1586, height: 992 });
}

async function deliverySwitches(page: Page, store: string, path: string) {
  // MOCK delivery receipts only: UI safety/field-preservation coverage, not PG
  // shipping acceptance. Deliberately different codes/kinds catch guessed paths.
  const market = "80000000-0000-4000-8000-000000000011";
  const methods = [
    ["home", "manual_home", "Home delivery"], ["cvs_711", "manual_seven", "7-ELEVEN"],
    ["cvs_familymart", "manual_family", "FamilyMart"], ["cvs_hilife", "manual_hilife", "Hi-Life"],
    ["cvs_okmart", "manual_ok", "OK"],
  ];
  const services = methods.map(([kind, code], index) => ({
    market_id: market, country: "TW", code, delivery_kind: kind, mode: "MANUAL",
    version: 7, policy_version: 3, policy_method: `delivery:${code}`, currency: "TWD",
    name_hans: `配送${index}`, name_hant: `配送${index}`, name_en: `Delivery ${index}`,
    enabled: false, visible: true, sort_order: index + 2,
  }));
  const writes: string[] = [];
  const pattern = `**/api/stores/${store}/markets**`;
  await page.route(pattern, async route => {
    const request = route.request(), url = new URL(request.url());
    const suffix = url.pathname.split(`/api/stores/${store}/`)[1];
    const reply = (body: unknown) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "GET" && suffix === "markets") return reply({ items: [{ id: market, code: "tw", name: "Taiwan", currency: "TWD", version: 1, active: true }], next_cursor: "" });
    if (request.method() === "GET" && suffix === `markets/${market}/countries/TW/delivery-services`) return reply({ items: services, next_cursor: "" });
    const service = services.find(s => suffix === `markets/${market}/countries/TW/delivery-services/${s.code}` || suffix === `markets/${market}/countries/TW/delivery-services/${s.code}/policy`);
    if (!service) return route.continue();
    if (request.method() === "GET" && suffix.endsWith("/policy")) return reply({ market_id: market, country: "TW", method: service.policy_method, currency: "TWD", version: 3, shipping_mode: "country_flat", shipping_minor: 80, tax_mode: "none", tax_basis: "goods", tax_rate_bps: 0, quote_ttl_seconds: 600, enabled: true });
    if (request.method() === "GET") return reply(service);
    expect(request.method(), "only the existing service PUT may mutate delivery").toBe("PUT");
    const input = request.postDataJSON();
    for (const key of ["market_id", "country", "code", "policy_version", "name_hans", "name_hant", "name_en", "delivery_kind", "mode", "visible", "sort_order"] as const) expect(input[key], `${service.code}: preserve ${key}`).toEqual(service[key]);
    expect(input.expected_version).toBe(service.version);
    expect(input.enabled).toBe(true);
    expect(request.headers()["idempotency-key"]).toBeTruthy();
    writes.push(service.code);
    service.enabled = true;
    service.version++;
    return reply(service);
  });
  try {
    await page.goto(path);
    await page.getByTestId("settings-v2-overview").getByRole("button", { name: "Edit settings", exact: true }).click();
    for (const [, , label] of methods) {
      const control = page.getByRole("checkbox", { name: label, exact: true });
      await expect(control).toBeEnabled();
      await control.check();
      await expect(control).toBeChecked();
    }
    await page.getByRole("button", { name: "Review settings", exact: true }).click();
    for (const [, , label] of methods) await expect(page.getByTestId("settings-v2-review")).toContainText(`${label}: on`);
    await page.getByRole("button", { name: "Save settings", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Settings saved", exact: true })).toBeVisible();
    expect(writes.sort()).toEqual(services.map(s => s.code).sort());
    await page.reload();
    await expect(page.getByRole("button", { name: "Delivery methods · 5 enabled", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Edit settings", exact: true }).click();
    for (const [, , label] of methods) await expect(page.getByRole("checkbox", { name: label, exact: true })).toBeChecked();
  } finally {
    await page.unroute(pattern);
  }
}
