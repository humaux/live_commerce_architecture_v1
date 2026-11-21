// Purpose: Verify single-amount edits through the default settings entry, with synthetic legacy-store data.
// Depends on: Playwright and the signed settings-real page. MOCK BFF responses, real keyboard and click actions.
// Used by: settings-v2.spec.ts; exercises a lost service acknowledgement after the pricing policy commits.
import { expect, type Page } from "@playwright/test";

export async function settingsAmountAcceptance(source: Page, store: string) {
  const page = await source.context().newPage();
  const market = { id: "80000000-0000-4000-8000-000000000041", code: "tw", name: "Taiwan", currency: "TWD", version: 1, active: true };
  const path = `markets/${market.id}/countries/TW/delivery-services/pickup`;
  const cvs = { version: 5, enabled_chains: ["cvs_711"], pay_at_pickup_enabled: true, pay_at_pickup_max_twd: 3000, pay_at_pickup_max_open: 16 };
  const policy = { market_id: market.id, country: "TW", method: "delivery:pickup", currency: "TWD", version: 3, shipping_mode: "country_flat",
    shipping_minor: 100000, free_shipping_threshold_minor: 150000, tax_mode: "inclusive", tax_basis: "goods", tax_rate_bps: 0, quote_ttl_seconds: 900, enabled: true };
  const service = { market_id: market.id, country: "TW", code: "pickup", delivery_kind: "cvs_711", mode: "MANUAL", version: 2, policy_version: 3, policy_method: policy.method,
    currency: "TWD", name_en: "Pickup", name_hans: "取货", name_hant: "取貨", enabled: true, visible: true, sort_order: 2 };
  const writes: { path: string; key: string; bytes: string }[] = [];
  const prefix = `/api/stores/${store}/`;
  await page.route(`**${prefix}**`, async route => {
    const req = route.request(), resource = new URL(req.url()).pathname.slice(prefix.length);
    if (req.method() === "GET") {
      const known: Record<string, unknown> = {
        markets: { items: [market], next_cursor: "" }, [path.slice(0, path.lastIndexOf("/"))]: { items: [service], next_cursor: "" },
        [path]: service, [path + "/policy"]: policy, "logistics/cvs-settings": cvs,
        profile: { name: "Legacy", contact_email: "", contact_phone: "", version: 2 },
        quickstart: { completed: true, completed_at: "2026-10-01T00:00:00Z", steps: { store_name: true, shipping: true, first_product: true } },
      };
      return route.fulfill({ status: resource in known ? 200 : 404, json: known[resource] ?? { code: "not_found" } });
    }
    writes.push({ path: resource, key: req.headers()["idempotency-key"], bytes: req.postData() ?? "" });
    expect(req.method()).toBe("PUT");
    const input = req.postDataJSON();
    if (resource === "logistics/cvs-settings") {
      const { version, ...fields } = cvs;
      expect(input).toEqual({ ...fields, expected_version: version, pay_at_pickup_max_twd: 5000 });
      Object.assign(cvs, { version: version + 1, pay_at_pickup_max_twd: 5000 });
      return route.fulfill({ json: cvs });
    }
    if (resource === path + "/policy") {
      const { version, ...fields } = policy;
      expect(input).toEqual({ ...fields, expected_version: version, shipping_minor: 8000, configuration_ref: "固定運費" });
      Object.assign(policy, { version: version + 1, shipping_minor: 8000 });
      return route.fulfill({ json: policy });
    }
    expect(resource).toBe(path);
    if (writes.filter(w => w.path === path).length === 1) {
      const { version, currency: _currency, policy_method: _method, ...fields } = service;
      expect(input).toEqual({ ...fields, expected_version: version, policy_version: 4 });
      Object.assign(service, { version: version + 1, policy_version: 4 });
      return route.abort("failed"); // The original service command committed; its response was lost.
    }
    expect(writes.at(-1)).toEqual(writes.at(-2));
    return route.fulfill({ json: service });
  });
  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/zh-TW/settings");
    const cap = page.getByRole("button", { name: "取貨付款上限", exact: true });
    await cap.press("Enter");
    const limit = page.getByRole("textbox", { name: /單筆取貨付款上限/ });
    await expect(limit).toHaveValue("3000");
    await limit.fill("5000");
    await limit.press("Enter");
    await expect(page.getByRole("status").filter({ hasText: "設定已儲存" })).toBeVisible();
    await page.reload(); await cap.click();
    await expect(limit).toHaveValue("5000");
    await page.getByRole("button", { name: "返回總覽", exact: true }).click();
    const shipping = page.getByRole("button", { name: "固定運費", exact: true });
    await shipping.click();
    const fee = page.getByRole("textbox", { name: "固定運費 (TWD)", exact: true });
    await expect(fee).toHaveValue("1000");
    await fee.fill("80"); await fee.press("Enter");
    await expect(page.getByTestId("settings-wizard").getByRole("alert")).toContainText("部分設定已儲存");
    await page.reload(); await shipping.click();
    await expect(fee).toHaveValue("80");
    expect(writes.map(w => w.path)).toEqual(["logistics/cvs-settings", path + "/policy", path]);
    await page.getByRole("button", { name: /相同|同一/ }).click();
    await expect(page.getByRole("status").filter({ hasText: "設定已儲存" })).toBeVisible();
    await page.reload(); await shipping.click();
    await expect(fee).toHaveValue("80");
    expect(writes.map(w => w.path)).toEqual(["logistics/cvs-settings", path + "/policy", path, path]);
    expect(writes.every(w => !!w.key)).toBe(true);
    expect(service.policy_version).toBe(policy.version);
  } finally { await page.close(); }
}
