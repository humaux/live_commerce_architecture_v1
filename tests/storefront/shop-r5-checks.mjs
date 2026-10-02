// Additional MOCK assertions invoked by shop-gate.mjs; not a substitute for SFR / real PG isolation gates.
import assert from "node:assert/strict";
import path from "node:path";
import { mkdir, writeFile } from "node:fs/promises";
import { expect } from "@playwright/test";

export async function checkR5({ browser, api, origin, raw, evidence }) {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, ignoreHTTPSErrors: true });
  const p = await context.newPage();
  const browserErrors = [];
  p.on("pageerror", error => browserErrors.push(error.message));
  p.on("console", message => { if (message.type() === "error") browserErrors.push(message.text()); });
  const requestStart = api.state.requests.length;
  try {
    // Consumer receives only published collections; an explicitly empty public collection must also disappear.
    api.state.hiddenCollectionSlug = "knitwear";
    api.state.emptyCollectionSlug = "tableware";
    await p.goto(`${origin}/en/products`);
    await p.waitForLoadState("networkidle");
    const chips = p.getByTestId("collection-chips");
    await expect(chips).toBeVisible();
    await expect(chips.locator('a[href$="/collections/home-fragrance"]')).toHaveCount(1);
    for (const slug of ["knitwear", "tableware"]) await expect(chips.locator(`a[href$="/collections/${slug}"]`)).toHaveCount(0);
    assert(await chips.evaluate(el => el.scrollWidth <= el.clientWidth), "G4 chips have no horizontal scrolling at 390px");
    assert(await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "G4 page width");
    api.state.hiddenCollectionSlug = api.state.emptyCollectionSlug = null;

    // A draft/withdrawn sibling is absent from the public catalog response. No client-side fallback may resurrect it.
    api.state.hideProductSlug = "reed-diffuser-box";
    await p.goto(`${origin}/en/products/cedar-fig-candle`);
    const related = p.getByTestId("related-products");
    await expect(related).toBeVisible();
    assert(await related.getByTestId("product-card").count() > 0);
    for (const slug of ["cedar-fig-candle", "reed-diffuser-box"]) await expect(related.locator(`a[href$="/products/${slug}"]`)).toHaveCount(0);
    const foreign = await raw("/en/products/cedar-fig-candle", { host: "foreign-shop.example" });
    assert(!foreign.body.toString().includes('data-testid="related-products"'), "foreign Host must never receive this shop's related cards");
    api.state.hideProductSlug = null;
    assert(!api.state.requests.slice(requestStart).some(r => r.includes("/session/bootstrap")), "G2 public browsing must not bootstrap a buyer session");

    // Explicit payment modes only. Session creation here is the buyer's add-to-cart action, not assurance.
    await p.getByTestId("add-to-cart").click();
    await expect(p.getByTestId("cart-drawer")).toBeVisible();
    await p.keyboard.press("Escape");
    await p.route("**/api/buyer/checkout-options?*", async route => {
      const upstream = await route.fetch();
      const data = await upstream.json();
      const secondPage = new URL(route.request().url()).searchParams.has("cursor");
      if (secondPage) {
        data.items[0].payment_modes = ["bank_transfer"];
        data.items[0].transfer_window_hours = 48;
      }
      data.next_cursor = secondPage ? "" : "r5-next";
      data.items.push({ ...data.items[0], available: false, reason: "coming_soon", delivery_kind: "cvs_711", name_en: "Unavailable test pickup", service_version: undefined, allocation_version: undefined, payment_modes: undefined, transfer_window_hours: undefined });
      await route.fulfill({ response: upstream, json: data });
    });
    await p.reload();
    const assurance = p.getByTestId("product-assurance");
    await expect(assurance).toContainText("Bank transfer");
    await expect(assurance).not.toContainText("Credit card");
    await expect(assurance).not.toContainText("Unavailable test pickup");
    assert(!/48|hours|days|工作天/.test(await assurance.innerText()), "G2 must not turn reservation windows into delivery promises");
    await p.unroute("**/api/buyer/checkout-options?*");
    await p.route("**/api/buyer/checkout-options?*", route => route.fulfill({ status: 503, json: { code: "unavailable" } }));
    await p.reload();
    await expect(assurance).not.toContainText("Bank transfer");
    await expect(assurance).not.toContainText("Taiwan home delivery");
    await expect(p.getByTestId("buy-now")).toBeEnabled();
    await p.unroute("**/api/buyer/checkout-options?*");

    for (const [locale, title] of [["zh-TW", "防詐騙提醒"], ["zh-CN", "防诈骗提醒"], ["en", "Shop safely"]]) {
      await p.goto(`${origin}/${locale}/legal/anti-fraud`);
      await expect(p.getByRole("heading", { level: 1 })).toHaveText(title);
      await expect(p.locator(".sf-footer__legal").locator(`a[href="/${locale}/legal/anti-fraud"]`)).toHaveCount(1);
    }
    // Isolated synthetic merchant colour matches approved comp 09; existing green merchant fixture remains unchanged.
    api.state.designAccent = "#bb4b0e";
    const out = process.env.LC_R5_SHOTS || evidence;
    await mkdir(out, { recursive: true });
    // A fast scroll can skip the inline actions between observer frames; the phone bar must still follow their actual position.
    await p.goto(`${origin}/en/products/cedar-fig-candle`);
    await p.waitForLoadState("networkidle");
    await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await p.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    const stickyPosition = await p.locator(".sf-buy__actions").evaluate(el => ({ top: el.getBoundingClientRect().top, bottom: el.getBoundingClientRect().bottom, scrollY, height: document.documentElement.scrollHeight, viewport: innerHeight }));
    await writeFile(path.join(evidence, "sticky-position.json"), JSON.stringify(stickyPosition, null, 2));
    await expect(p.getByTestId("sticky-buy")).toBeVisible();
    for (const locale of ["zh-TW", "en"]) for (const [width, height] of [[390, 844], [1586, 992]]) {
      await p.setViewportSize({ width, height });
      for (const [name, suffix] of [["product", "products/cedar-fig-candle"], ["products", "products"]]) {
        await p.goto(`${origin}/${locale}/${suffix}`);
        await p.waitForLoadState("networkidle");
        await p.evaluate(async () => { await Promise.all([...document.images].filter(i => i.getBoundingClientRect().top < innerHeight).map(i => i.decode().catch(() => {}))); window.scrollTo(0, 0); });
        assert(await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `R5 ${name}/${locale}/${width} overflow`);
        await p.screenshot({ path: path.join(out, `${name}-${locale}-${width}x${height}.png`), scale: "css" });
      }
    }
    // Focused below-fold evidence: retain real fixed controls, rather than styling them away for a full-page montage.
    for (const locale of ["zh-TW", "en"]) {
      await p.setViewportSize({ width: 390, height: 844 });
      await p.goto(`${origin}/${locale}/products/cedar-fig-candle`);
      await p.waitForLoadState("networkidle");
      for (const [name, target] of [["assurance", p.getByTestId("product-assurance")], ["related", p.getByTestId("related-products")], ["footer", p.locator(".sf-footer__legal")]]) {
        await target.scrollIntoViewIfNeeded();
        await expect(target).toBeVisible();
        await p.evaluate(async () => {
          await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all([...document.querySelectorAll(".sf-sticky")].flatMap(el => el.getAnimations()).map(animation => animation.finished.catch(() => {})));
        });
        await p.screenshot({ path: path.join(out, `${name}-${locale}-390x844.png`), scale: "css" });
      }
      await p.goto(`${origin}/${locale}/legal/anti-fraud`);
      await p.waitForLoadState("networkidle");
      await p.screenshot({ path: path.join(out, `anti-fraud-${locale}-390x844.png`), scale: "css" });
    }
    await p.setViewportSize({ width: 1586, height: 992 });
    await p.goto(`${origin}/en/products/wool-knit-scarf`);
    await p.getByRole("button", { name: "Enlarge photo" }).click();
    await expect(p.locator(".sf-photo-dialog")).toBeVisible();
    await p.keyboard.press("ArrowRight");
    await expect(p.locator(".sf-photo-dialog__controls")).toContainText("2 /");
    await p.screenshot({ path: path.join(out, "gallery-en-desktop.png"), scale: "css" });
    await p.keyboard.press("Escape");
    await expect(p.getByRole("button", { name: "Enlarge photo" })).toBeFocused();
    console.log("PASS R5 extra MOCK: categories hidden/empty, related excludes current/withdrawn, foreign Host, assurance explicit-only/failure/no-bootstrap, three-language safety page, gallery, 8 exact-size captures");
  } catch (error) {
    await writeFile(path.join(evidence, "r5-failure.html"), await p.content());
    await writeFile(path.join(evidence, "r5-failure-errors.json"), JSON.stringify(browserErrors, null, 2));
    await p.screenshot({ path: path.join(evidence, "r5-failure.png"), scale: "css" });
    throw error;
  } finally {
    api.state.hideProductSlug = api.state.hiddenCollectionSlug = api.state.emptyCollectionSlug = api.state.designAccent = null;
    await context.close();
  }
}
