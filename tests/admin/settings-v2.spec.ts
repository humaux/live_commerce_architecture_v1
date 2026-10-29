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
  const productSaved = page.waitForResponse(r => r.request().method() === "POST" && /\/api\/stores\/[^/]+\/products$/.test(new URL(r.url()).pathname));
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  expect((await productSaved).ok()).toBe(true);
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(skuCommands).toHaveLength(1);
  const originalSku = skuCommands[0];
  expect(originalSku.key).toBeTruthy();
  expect(inventoryCommands, "initial stock has not been submitted after the lost SKU response").toHaveLength(0);
  await page.unroute(skuURL);
  await page.route(skuURL, async route => {
    skuCommands.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() ?? "" });
    await route.continue();
  });
  await page.getByRole("button", { name: "Reload", exact: true }).click();
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
  await page.getByRole("button", { name: "Save settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Latest status unavailable", exact: true })).toBeVisible();
  expect(skuCommands, "explicit Save must replay the identical SKU key and body exactly once").toEqual([originalSku, originalSku]);
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
