// Purpose: real-click browser gate of the platform card-payments page (enable dialog, disable, every state badge, BLOCKED and platform-CLOSED rules, 409 reload) against real PG, Go and the production admin Next build.
// Depends on: @playwright/test, node:crypto, node:fs/promises, node:path, ../../apps/admin/lib/card-payments-copy, ../../apps/admin/src/shell-copy, ../../packages/format/src/index; harness env: LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_EVIDENCE, LC_BROWSER_STORE, LC_BROWSER_CONTROL, LC_BROWSER_CONTROL_KEY
// Used by: apps/admin/src/features/settings/routes.ts (card-payments), scripts/dev/test-local.sh (--browser-card-payments), tests/foundation/browser_card_payments_test.go
// W4-U1 (contracts/stripe-platform-account-v1.md §0.3 AD-PF2, §2, §3.3, §5; docs/delivery/units/w4-u1-payment-activation-ui.md "Integrator 裁决").
// BFF routes exercised through the UI: GET|PUT /api/stores/{store}/payments/card -> Go /v1/admin/stores/{store}/payments/card (keyless CAS PUT).
// Started only by tests/foundation/browser_card_payments_test.go (build tag browser), which owns the isolated PG, the real payment worker, the
// platform Stripe state (designate / open / allowlist / block through the operator definers), the Go API, the production admin Next, the signed
// mock IdP and the runner-only control listener. Evidence class: BROWSER, MOCK Stripe (the independent fake; no Stripe, no key, test mode only).
// Every assertion restates the integrator ruling or the contract, not the implementation.
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { cardPaymentsCopy } from "../../apps/admin/lib/card-payments-copy";
import { shellCopy } from "../../apps/admin/src/shell-copy";
import { money } from "../../packages/format/src/index";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const control = required("LC_BROWSER_CONTROL");
const controlKey = required("LC_BROWSER_CONTROL_KEY");

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.describe.configure({ mode: "serial" });

const en = cardPaymentsCopy.en;
// the contract §5 sentence, verbatim (zh-TW is the owner-supplied source text) and the two ruling-fixed strings
const TERMS_ZH_TW = "款項由平台代收，按結算週期以銀行轉帳撥付；退款與爭議款會從你的結算中扣除";
const NOT_OPEN_ZH_TW = "信用卡收款尚未開放";
const BLOCKED_ZH_TW = "已被平台暫停，請聯絡客服";
const leak = /(?:acct_|sk_(?:live|test)_|rk_(?:live|test)_|pk_(?:live|test)_|whsec_)[A-Za-z0-9]/;

type Summary = {
  platform_state: string; store_state: string; allowed: boolean; terms_version: string | null; accepted_terms_version: string | null;
  display_name: string | null; descriptor_preview: string | null; currency: string | null; min_minor: number | null; max_minor: number | null; version: number;
};

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await page.getByTestId("nav-orders").waitFor({ state: "attached" });
  const menu = page.locator('button[aria-controls="workspace-navigation"]');
  const drawer = await menu.isVisible();
  if (drawer) await menu.click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  if (drawer) await page.keyboard.press("Escape");
}
// A runner-only state change made through the real operator definers (never the UI under test).
async function ctl(resource: string) {
  const response = await fetch(`${control}/${resource}`, { method: "POST", headers: { "X-Gate-Key": controlKey } });
  expect(response.status, `control ${resource}`).toBe(204);
}
// The server's own answer, read through the same BFF the page uses. An in-page fetch: the Secure __Host- session cookie is sent by the browser,
// not by the APIRequestContext jar (page.request) over http, which answers 401 (as customers-billing.spec.ts / orders-ui.spec.ts do).
async function summaryOf(page: Page): Promise<Summary> {
  // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change)
  const response = await page.evaluate(async (target) => {
    const r = await fetch(target, { credentials: "same-origin", cache: "no-store" });
    return { status: r.status, body: await r.json().catch(() => null) };
  }, `/api/stores/${store}/payments/card`);
  expect(response.status).toBe(200);
  return response.body as Summary;
}
const cardUrl = (locale: string) => `/${locale}/settings/payments/card?store=${store}`;

// Every API body and request body the page saw: none may carry an account id, key or approval id (brief: DOM, network, storage).
function watch(page: Page) {
  const seen: string[] = [];
  page.on("response", async (response) => {
    if (!response.url().includes("/api/")) return;
    try {
      seen.push(response.url() + " " + (await response.text()));
    } catch {
      /* redirect or aborted body */
    }
  });
  page.on("request", (request) => {
    if (request.url().includes("/api/")) seen.push(request.url() + " " + (request.postData() ?? "") + " " + JSON.stringify(request.headers()));
  });
  return {
    async assertClean() {
      for (const text of seen) expect(text).not.toMatch(leak);
      expect(await page.content()).not.toMatch(leak);
      // G-UI8 audit [READ/MEASURE]: scans client storage for credentials (read only)
      const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
      expect(stored).not.toMatch(leak);
      expect(seen.length).toBeGreaterThan(0);
    },
  };
}
const manifestPath = path.join(evidence, "screenshots.json");
async function fitsWidth(page: Page) {
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  await fitsWidth(page);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}
// One PUT of the page, with the exact request the browser made (headers + body) and the server's status.
async function clickAndCapturePut(page: Page, testid: string) {
  const pending = page.waitForResponse((r) => r.url().includes("/payments/card") && r.request().method() === "PUT");
  await page.getByTestId(testid).click();
  const response = await pending;
  const request = response.request();
  return { status: response.status(), headers: request.headers(), body: request.postDataJSON() as Record<string, unknown> };
}
const badge = (page: Page) => page.getByTestId("card-state-badge");

test("CPU1 enabled store: badges, limits from the server, no credential input, entry link from Settings", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  // entry only: the Settings wizard's payment step links to the page (the page itself is a nav:false route)
  await page.goto(`/en/settings?store=${store}`);
  const link = page.getByTestId("settings-card-payments-link");
  await expect(link).toBeVisible();
  await link.click();
  await expect(page).toHaveURL(new RegExp(`/en/settings/payments/card\\?store=${store}`));
  await expect(page.getByTestId("card-payments-page")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(shellCopy.en.cardPayments);

  const summary = await summaryOf(page);
  expect([summary.platform_state, summary.store_state, summary.allowed]).toEqual(["OPEN", "ENABLED", true]);
  await expect(badge(page)).toHaveText(en.stateENABLED);
  await expect(page.getByTestId("platform-state-badge")).toHaveText(en.platformOPEN);
  await expect(page.getByTestId("card-descriptor-preview")).toContainText(summary.descriptor_preview!);
  // limits are the server's (contract: TWD min NT$25); the maximum is never hard-coded in the UI
  await expect(page.getByTestId("card-currency")).toHaveText("TWD");
  await expect(page.getByTestId("card-min")).toHaveText("NT$25");
  await expect(page.getByTestId("card-min")).toHaveText(money("en", summary.currency!, summary.min_minor!));
  await expect(page.getByTestId("card-max")).toHaveText(money("en", summary.currency!, summary.max_minor!));
  // no credential input of any kind, and nothing to enable while it is enabled
  await expect(page.locator('[data-testid="card-payments-page"] input')).toHaveCount(0);
  await expect(page.locator('input[type="password"]')).toHaveCount(0);
  await expect(page.getByTestId("card-not-open")).toHaveCount(0);
  await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
  await expect(page.getByTestId("card-disable-open")).toBeVisible();
  // the settlement statements have no nav entry: billing:manage holders reach them from here (review P2-6)
  await expect(page.getByTestId("card-settlements-link")).toHaveAttribute("href", `/en/settings/settlements?store=${store}`);
  await shot(page, "card-enabled", "en", "desktop");
  await spy.assertClean();
});

test("CPU2 disable: one keyless CAS PUT, the page shows DISABLED and offers enable again", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await page.goto(cardUrl("en"));
  const summary = await summaryOf(page);
  await page.getByTestId("card-disable-open").click();
  await expect(page.getByTestId("card-disable-dialog")).toContainText(en.disableConfirm);
  const put = await clickAndCapturePut(page, "card-disable-confirm");
  expect(put.status).toBe(200);
  expect(put.headers["idempotency-key"], "the card PUT is CAS-guarded, never keyed").toBeUndefined();
  expect(put.body).toEqual({ enabled: false, terms_version: summary.accepted_terms_version, descriptor_suffix: null, expected_version: summary.version });
  await expect(page.getByTestId("card-saved")).toHaveText(en.saved);
  await expect(badge(page)).toHaveText(en.stateDISABLED);
  await expect(page.getByTestId("card-enable-open")).toBeVisible();
  await expect(page.getByTestId("card-disable-open")).toHaveCount(0);
  await spy.assertClean();
});

test("CPU3 enable dialog: verbatim terms, accept box, suffix length and charset refusal, live preview, exact PUT body", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await page.goto(cardUrl("en"));
  const summary = await summaryOf(page);
  expect(summary.store_state).toBe("DISABLED");
  await page.getByTestId("card-enable-open").click();
  const dialog = page.getByTestId("card-enable-dialog");
  await expect(dialog).toBeVisible();
  await expect(page.getByTestId("card-terms")).toHaveText(en.terms);
  await expect(page.getByTestId("card-terms-version")).toContainText(summary.terms_version!);
  await expect(page.getByTestId("card-preview")).toContainText(summary.descriptor_preview!);
  // nothing happens until the terms are accepted
  const confirm = page.getByTestId("card-enable-confirm");
  await expect(confirm).toBeDisabled();
  const input = page.getByTestId("card-suffix-input");
  await input.fill("S"); // charset/min-length rule: 2..10 characters
  await expect(page.getByTestId("card-suffix-invalid")).toHaveText(en.suffixRule);
  await page.getByTestId("card-terms-accept").check();
  await expect(confirm).toBeDisabled();
  await input.fill("ABCDEFGHIJK"); // 11 characters: the length refusal (the server answers descriptor_suffix_too_long)
  await expect(page.getByTestId("card-suffix-too-long")).toHaveText(en.suffixTooLong);
  await expect(page.getByTestId("card-suffix-invalid")).toHaveCount(0);
  await expect(confirm).toBeDisabled();
  await input.fill("SH_OP"); // outside the charset
  await expect(page.getByTestId("card-suffix-invalid")).toBeVisible();
  await expect(confirm).toBeDisabled();
  await input.fill("SHOP");
  await expect(page.getByTestId("card-suffix-invalid")).toHaveCount(0);
  await expect(page.getByTestId("card-suffix-too-long")).toHaveCount(0);
  await expect(page.getByTestId("card-preview")).toHaveText(`${en.previewLabel}: ${summary.descriptor_preview!}* SHOP`);
  await shot(page, "card-enable-dialog", "en", "desktop");
  await expect(confirm).toBeEnabled();
  const put = await clickAndCapturePut(page, "card-enable-confirm");
  expect(put.status).toBe(200);
  expect(put.headers["idempotency-key"]).toBeUndefined();
  expect(put.body).toEqual({ enabled: true, terms_version: summary.terms_version, descriptor_suffix: "SHOP", expected_version: summary.version });
  await expect(page.getByTestId("card-saved")).toHaveText(en.saved);
  await expect(page.getByTestId("card-enable-dialog")).toHaveCount(0);
  await expect(badge(page)).toHaveText(en.stateENABLED);
  await expect(page.getByTestId("card-descriptor-preview")).toContainText(`${summary.descriptor_preview!}* SHOP`);
  await spy.assertClean();
});

test("CPU4 409: a competing change reloads the page with the retry message, and the retry succeeds", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await page.goto(cardUrl("en"));
  await expect(badge(page)).toHaveText(en.stateENABLED);
  await ctl("store/bump"); // another session changes the enrollment (real definer): this page's version is stale
  await page.getByTestId("card-disable-open").click();
  const stale = await clickAndCapturePut(page, "card-disable-confirm");
  expect(stale.status).toBe(409);
  await expect(page.getByTestId("card-conflict")).toHaveText(en.conflict);
  await expect(page.getByTestId("card-disable-dialog")).toHaveCount(0);
  // the reload shows what the other session did, and nothing was disabled by the stale request
  await expect(badge(page)).toHaveText(en.stateENABLED);
  await expect(page.getByTestId("card-descriptor-preview")).toContainText("* BUMPED");
  const fresh = await summaryOf(page);
  await page.getByTestId("card-disable-open").click();
  const retry = await clickAndCapturePut(page, "card-disable-confirm");
  expect(retry.status).toBe(200);
  expect(retry.body.expected_version).toBe(fresh.version);
  await expect(badge(page)).toHaveText(en.stateDISABLED);
  await spy.assertClean();
});

test("CPU5 platform not OPEN: 'not open yet' in zh-TW and en, no toggle at all; open again brings the toggle back", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await ctl("platform/close");
  for (const locale of ["zh-TW", "en"] as const) {
    await page.goto(cardUrl(locale));
    await expect(page.getByTestId("card-not-open")).toBeVisible();
    await expect(page.getByTestId("card-not-open")).toHaveText(locale === "zh-TW" ? NOT_OPEN_ZH_TW : en.notOpen);
    await expect(page.getByTestId("platform-state-badge")).toHaveText(cardPaymentsCopy[locale].platformCLOSED);
    await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
    await expect(page.getByTestId("card-disable-open")).toHaveCount(0); // DISABLED store: nothing to turn off either
    await shot(page, "card-platform-closed", locale, "desktop");
  }
  await ctl("platform/open");
  await page.goto(cardUrl("en"));
  await expect(page.getByTestId("card-not-open")).toHaveCount(0);
  await expect(page.getByTestId("card-enable-open")).toBeVisible();
  await spy.assertClean();
});

test("CPU6 BLOCKED: the suspended note, disable only (never enable), and unblock returns to DISABLED", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await ctl("store/block");
  await page.goto(cardUrl("zh-TW"));
  await expect(page.getByTestId("card-blocked-note")).toHaveText(BLOCKED_ZH_TW);
  await expect(badge(page)).toHaveText(cardPaymentsCopy["zh-TW"].stateBLOCKED);
  await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
  await expect(page.getByTestId("card-disable-open")).toBeVisible();
  await shot(page, "card-blocked", "zh-TW", "desktop");
  await page.goto(cardUrl("en"));
  await expect(page.getByTestId("card-blocked-note")).toHaveText(en.blockedNote);
  await expect(badge(page)).toHaveText(en.stateBLOCKED);
  // disable is never gated (contract §3.3): it succeeds and the store stays BLOCKED, with no enable offered
  await page.getByTestId("card-disable-open").click();
  const put = await clickAndCapturePut(page, "card-disable-confirm");
  expect(put.status).toBe(200);
  await expect(badge(page)).toHaveText(en.stateBLOCKED);
  await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
  await ctl("store/unblock");
  await page.goto(cardUrl("en"));
  await expect(badge(page)).toHaveText(en.stateDISABLED);
  await expect(page.getByTestId("card-blocked-note")).toHaveCount(0);
  await expect(page.getByTestId("card-enable-open")).toBeVisible();
  await spy.assertClean();
});

test("CPU6b not allowlisted reads like not open (same message, no 'platform open' badge, no enable); withdrawing the allowlist of an enrolled store blocks it", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await ctl("store/disallow"); // the real platform-disallow definer: the allowlist is withdrawn AND the enrolled store is blocked (PF11)
  await page.goto(cardUrl("zh-TW"));
  await expect(page.getByTestId("card-blocked-note")).toHaveText(BLOCKED_ZH_TW);
  await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
  await ctl("store/unblock"); // the block is cleared, the allowlist is still withdrawn: DISABLED and not allowlisted
  for (const locale of ["zh-TW", "en"] as const) {
    await page.goto(cardUrl(locale));
    await expect(page.getByTestId("card-not-open")).toHaveText(locale === "zh-TW" ? NOT_OPEN_ZH_TW : en.notOpen);
    await expect(page.getByTestId("platform-state-badge")).toHaveCount(0); // "platform open" would contradict the message
    await expect(page.getByTestId("card-blocked-note")).toHaveCount(0);
    await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
    await expect(page.getByTestId("card-disable-open")).toHaveCount(0);
    // never sent to support for an allowlist spot it cannot get (AD-PF2)
    await expect(page.getByTestId("card-payments-page")).not.toContainText(/allowlist|允許名單|允许名单/);
  }
  await shot(page, "card-not-allowlisted", "en", "desktop");
  await ctl("store/allow");
  await page.goto(cardUrl("en"));
  await expect(page.getByTestId("card-not-open")).toHaveCount(0);
  await expect(page.getByTestId("platform-state-badge")).toHaveText(en.platformOPEN);
  await expect(page.getByTestId("card-enable-open")).toBeVisible();
  await spy.assertClean();
});

test("CPU7 three languages: the terms are quoted verbatim in zh-TW, translated in zh-CN; zh-TW enable ends ENABLED", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await page.goto(cardUrl("zh-CN"));
  await page.getByTestId("card-enable-open").click();
  await expect(page.getByTestId("card-terms")).toHaveText(cardPaymentsCopy["zh-CN"].terms);
  await page.getByTestId("card-enable-cancel").click();
  await expect(page.getByTestId("card-enable-dialog")).toHaveCount(0);
  await page.goto(cardUrl("zh-TW"));
  await page.getByTestId("card-enable-open").click();
  await expect(page.getByTestId("card-terms")).toHaveText(TERMS_ZH_TW);
  // the store kept the suffix of its earlier enrollment: the dialog offers it again (the GET only exposes the final preview)
  await expect(page.getByTestId("card-suffix-input")).toHaveValue("BUMPED");
  await shot(page, "card-enable-dialog", "zh-TW", "desktop");
  await page.getByTestId("card-suffix-input").fill("");
  await page.getByTestId("card-terms-accept").check();
  const put = await clickAndCapturePut(page, "card-enable-confirm");
  expect(put.status).toBe(200);
  expect(put.body.descriptor_suffix).toBeNull();
  await expect(badge(page)).toHaveText(cardPaymentsCopy["zh-TW"].stateENABLED);
  await expect(page.getByTestId("card-saved")).toHaveText(cardPaymentsCopy["zh-TW"].saved);
  await spy.assertClean();
});

test("CPU7b platform not OPEN while the store is ENABLED: 'not open yet' and no enable, but the way out (disable) stays (contract §3.3)", async ({ page }) => {
  const spy = watch(page);
  await signedLogin(page);
  await page.goto(cardUrl("en"));
  await expect(badge(page)).toHaveText(en.stateENABLED);
  await ctl("platform/close");
  await page.goto(cardUrl("zh-TW"));
  await expect(page.getByTestId("card-not-open")).toHaveText(NOT_OPEN_ZH_TW);
  await expect(page.getByTestId("platform-state-badge")).toHaveText(cardPaymentsCopy["zh-TW"].platformCLOSED);
  await expect(page.getByTestId("card-enable-open")).toHaveCount(0);
  await expect(page.getByTestId("card-disable-open")).toBeVisible();
  await ctl("platform/open");
  await page.goto(cardUrl("en"));
  await expect(page.getByTestId("card-not-open")).toHaveCount(0);
  await expect(page.getByTestId("card-disable-open")).toBeVisible();
  await spy.assertClean();
});

test("CPU8 phone width: the page and its dialog fit 390px in en and zh-TW", async ({ page }) => {
  const spy = watch(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await signedLogin(page);
  for (const locale of ["en", "zh-TW"] as const) {
    await page.goto(cardUrl(locale));
    await expect(badge(page)).toHaveText(cardPaymentsCopy[locale].stateENABLED);
    await shot(page, "card-enabled", locale, "mobile");
    await page.getByTestId("card-disable-open").click();
    await expect(page.getByTestId("card-disable-dialog")).toBeVisible();
    await shot(page, "card-disable-dialog", locale, "mobile");
    await page.getByTestId("card-disable-cancel").click();
  }
  await spy.assertClean();
});
