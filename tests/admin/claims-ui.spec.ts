// KC16 browser gate (contracts/live-keyword-claims-v1.md §11.1): Studio › Claims in the
// packaged admin, then the buyer claim link in the storefront production server, through
// real BFFs, Go transports and PostgreSQL. Started only by browser_live_claims_test.go
// (LC_BROWSER_SUITE=live-claims), which owns every process, the signed MOCK IdP, the
// disposable buyer.example TLS edge + CONNECT proxy and the runner-only control listener.
// Evidence: screenshots (links always masked) and result.json (token SHA-256 only).
import { expect, test, type Page } from "@playwright/test";
import { createHash, randomBytes } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { claimsCopy, hostPrompt } from "../../apps/admin/lib/claims-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const buyerOrigin = required("LC_CLAIMS_BUYER_ORIGIN");
const store = required("LC_CLAIMS_STORE");
const session = required("LC_CLAIMS_SESSION");
const scene = required("LC_CLAIMS_SCENE");
const product = required("LC_CLAIMS_PRODUCT");
const skuA = required("LC_CLAIMS_SKU_A"), skuB = required("LC_CLAIMS_SKU_B");
const skuAID = required("LC_CLAIMS_SKU_A_ID"), skuBID = required("LC_CLAIMS_SKU_B_ID");
const control = required("LC_CLAIMS_CONTROL"), controlKey = required("LC_CLAIMS_CONTROL_KEY");

test.use({
  baseURL: origin,
  // Only buyer.example goes through the proxy; the loopback admin and IdP stay direct.
  launchOptions: { proxy: { server: required("LC_CLAIMS_PROXY"), bypass: "127.0.0.1" } },
});
test.setTimeout(300_000);

const sha256 = (value: string) => createHash("sha256").update(value).digest("hex");
async function controlCall(resource: string, init: RequestInit = {}) {
  const response = await fetch(`${control}/${resource}`, { ...init, headers: { "X-Gate-Key": controlKey } });
  expect(response.status).toBe(200);
  return response.json() as Promise<Record<string, number>>;
}
async function storageLacks(page: Page, secrets: string[]) {
  const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
  for (const secret of secrets) expect(stored).not.toContain(secret);
}
async function fitsWidth(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
}
// Buyer pages have no fixed chrome, so they are captured whole; the admin shell has a
// fixed rail and skip link, so admin evidence is the settled viewport (Studio idiom).
async function shot(page: Page, name: string, fullPage = true) {
  await page.screenshot({ path: path.join(evidence, `${name}.png`), fullPage, animations: "disabled" });
}
async function merchantShot(page: Page, name: string) {
  if ((page.viewportSize()?.width ?? 0) <= 680)
    await expect.poll(() => page.locator(".rail").evaluate((rail) => Math.ceil(rail.getBoundingClientRect().right))).toBeLessThanOrEqual(0);
  await shot(page, name, false);
}


// Comment source (claim-source HTTP interface). The Go route is built in parallel, so this
// phase is MOCK: the claim-source BFF path is answered in the browser from the frozen wire
// shapes; every other claims call in this file still runs through the real chain. Remove the
// route once the Go route is merged so the same steps run against Go + PostgreSQL.
type Wire = Record<string, unknown>;
const sourceCodes = ["input_invalid", "input_unresolvable", "binding_missing", "binding_ambiguous", "source_conflict", "version_changed"] as const;
async function claimSourcePhase(merchant: Page, pass: (name: string) => void) {
  let source: Wire | null = null;
  let readStatus = 200;
  let failAfterApply = false;
  const refusals: { status: number; code: string }[] = [];
  const puts: { key: string; body: string }[] = [];
  const applied = new Map<string, Wire>();
  const headers = { "content-type": "application/json", "cache-control": "private, no-store" };
  await merchant.route(/\/api\/stores\/[^/]+\/live-sessions\/[^/]+\/claim-source$/, async (route) => {
    const request = route.request();
    if (request.method() === "GET") {
      if (readStatus !== 200) return route.fulfill({ status: readStatus, headers, body: JSON.stringify({ code: "retry_later" }) });
      return route.fulfill({ status: 200, headers, body: JSON.stringify({ source }) });
    }
    expect(request.method()).toBe("PUT");
    const key = request.headers()["idempotency-key"] ?? "";
    const raw = request.postData() ?? "";
    puts.push({ key, body: raw });
    const refusal = refusals.shift();
    if (refusal) return route.fulfill({ status: refusal.status, headers, body: JSON.stringify({ code: refusal.code, message: "x", request_id: "", retryable: false, details: {} }) });
    if (applied.has(key)) return route.fulfill({ status: 200, headers, body: JSON.stringify(applied.get(key)) });
    const body = JSON.parse(raw) as Wire;
    expect(Object.keys(body).sort()).toEqual(["active", "expected_version", "input", "private_reply", "reply_locale"]);
    if (body.expected_version !== ((source?.version as number | undefined) ?? 0))
      return route.fulfill({ status: 409, headers, body: JSON.stringify({ code: "version_changed" }) });
    const post = /^https:\/\/www\.facebook\.com\/[^/]+\/posts\/([0-9]+)$/.exec(String(body.input));
    source = { id: "33333333-3333-4333-8333-333333333333", platform: "facebook", object: "page", asset_id: "page-asset-1",
      source_object_id: post ? post[1] : String(body.input), private_reply: body.private_reply, reply_locale: body.reply_locale,
      active: body.active, version: ((source?.version as number | undefined) ?? 0) + 1, verified: false,
      intake_count: 4, intake_capped: 1, updated_at: "2026-09-29T08:30:00Z" };
    applied.set(key, source);
    if (failAfterApply) { failAfterApply = false; return route.fulfill({ status: 503, headers, body: JSON.stringify({ code: "retry_later" }) }); }
    return route.fulfill({ status: 200, headers, body: JSON.stringify(source) });
  });

  const section = merchant.getByTestId("claims-source");
  const input = section.getByLabel("Post or media link or ID", { exact: true });
  const save = section.getByRole("button", { name: "Save comment source" });
  const alert = section.getByRole("alert");
  const active = section.locator("#claims-source-active");
  await merchant.getByRole("button", { name: "Refresh facts" }).click();
  await expect(section.getByTestId("claims-source-none")).toBeVisible();
  await expect(input).toHaveAccessibleDescription(claimsCopy.en.sourceInputHint);
  // Empty input is refused in the browser: no request, an alert, and the field flagged.
  await save.click();
  await expect(alert).toHaveText(claimsCopy.en.sourceInputInvalid);
  await expect(input).toHaveAttribute("aria-invalid", "true");
  expect(puts).toHaveLength(0);
  await input.fill("two words");
  await save.click();
  await expect(alert).toHaveText(claimsCopy.en.sourceInputInvalid);
  expect(puts).toHaveLength(0);

  // First bind: byte-exact body, fresh key, re-read shows the server's facts.
  await input.fill("  https://www.facebook.com/somepage/posts/123456789012345 ");
  await section.getByLabel("Reply language", { exact: true }).selectOption("zh-TW");
  await section.getByLabel("Send a private reply with the cart link").check();
  await expect(input).not.toHaveAttribute("aria-invalid", "true");
  await save.click();
  await expect(section.getByTestId("claims-source-object")).toHaveText("123456789012345");
  expect(puts).toHaveLength(1);
  expect(puts[0].key).toMatch(/^[0-9a-f-]{36}$/);
  expect(puts[0].body).toBe(JSON.stringify({ input: "https://www.facebook.com/somepage/posts/123456789012345", private_reply: true,
    reply_locale: "zh-TW", active: true, expected_version: 0 }));
  await expect(section.getByTestId("claims-source-verified")).toHaveText("Unverified");
  await expect(section.getByTestId("claims-source-count")).toHaveText("4");
  await expect(section.getByTestId("claims-source-capped")).toHaveText("1");
  await expect(section.getByTestId("claims-source-status")).toContainText("Facebook post or live video");
  await expect(input).toHaveValue("123456789012345");
  await expect(save).toBeDisabled();
  await merchantShot(merchant, "merchant-claims-source-bound-en");
  pass("comment source: empty/space input refused in the browser; first bind sends the exact five-key body, then re-reads and shows platform, unverified label and intake counts");

  // Rebind with the version CAS: deactivate (a new key), then every refusal in every locale.
  await active.uncheck();
  await save.click();
  await expect(section.getByTestId("claims-source-status")).toContainText("Paused");
  expect(puts).toHaveLength(2);
  expect(JSON.parse(puts[1].body)).toMatchObject({ active: false, expected_version: 1, input: "123456789012345" });
  expect(puts[1].key).not.toBe(puts[0].key);
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    if (locale !== "en") await merchant.goto(`${origin}/${locale}/studio/claims?store=${store}&scene=${session}`);
    const words = claimsCopy[locale];
    const box = merchant.getByTestId("claims-source");
    await expect(box.getByTestId("claims-source-status")).toBeVisible();
    await box.locator("#claims-source-active").check();
    for (const [index, code] of sourceCodes.entries()) {
      // Statuses differ on purpose: the wording keys on the backend code, not the HTTP status.
      refusals.push({ status: index % 2 ? 409 : 422, code });
      await box.locator(".claims-source-form button[type=submit]").click();
      await expect(box.getByRole("alert")).toHaveText(words.sourceErrors[code]);
    }
    refusals.push({ status: 403, code: "forbidden" });
    await box.locator(".claims-source-form button[type=submit]").click();
    await expect(box.getByRole("alert")).toHaveText(words.sourceForbidden);
    refusals.push({ status: 409, code: "no_such_code" });
    await box.locator(".claims-source-form button[type=submit]").click();
    await expect(box.getByRole("alert")).toHaveText(words.conflict.source);
    if (locale === "zh-TW") await merchantShot(merchant, "merchant-claims-source-refused-zh-TW");
  }
  pass("comment source: all six refusal codes plus forbidden and an unknown 409 are worded in en, zh-CN and zh-TW");

  // Unknown result: the write committed but the answer was lost. Fields lock, retry reuses
  // the identical key and body, and the server-side replay leaves exactly one new version.
  await merchant.goto(`${origin}/en/studio/claims?store=${store}&scene=${session}`);
  await expect(section.getByTestId("claims-source-status")).toBeVisible();
  const before = puts.length, version = source!.version as number;
  await active.check();
  failAfterApply = true;
  await save.click();
  await expect(alert).toContainText(claimsCopy.en.uncertain);
  await expect(input).toBeDisabled();
  await section.getByRole("button", { name: "Retry same request" }).click();
  await expect(section.getByTestId("claims-source-status")).toContainText("On");
  expect(puts).toHaveLength(before + 2);
  expect(puts[before + 1]).toEqual(puts[before]);
  expect(source!.version).toBe(version + 1);
  pass("comment source: unknown result locks the form; retry reuses the same Idempotency-Key and bytes and the replay applies once");

  // A failing source read must not take the rest of the claims panel down.
  readStatus = 503;
  await merchant.getByRole("button", { name: "Refresh facts" }).click();
  await expect(section.getByText(claimsCopy.en.sourceUnavailable)).toBeVisible();
  await expect(section.getByLabel("Post or media link or ID", { exact: true })).toHaveCount(0);
  await expect(merchant.getByRole("heading", { level: 2, name: "Offers" })).toBeVisible();
  readStatus = 200;
  await merchant.getByRole("button", { name: "Refresh facts" }).click();
  await expect(section.getByTestId("claims-source-status")).toBeVisible();
  pass("comment source: an unavailable source read is isolated to its own section");
  await merchant.unroute(/\/api\/stores\/[^/]+\/live-sessions\/[^/]+\/claim-source$/);
}

test("KC16 Studio › Claims → one-time link → buyer cart, three locales, MOCK ingress", async ({ browser }) => {
  let cases = 0;
  const pass = (name: string) => { cases++; console.log(`PASS ${name}`); };
  const facts = await controlCall("facts");

  // Merchant: signed MOCK IdP → Studio scene → Keyword claims.
  const merchantContext = await browser.newContext({ viewport: { width: 1586, height: 992 } });
  await merchantContext.grantPermissions(["clipboard-read", "clipboard-write"], { origin });
  const merchant = await merchantContext.newPage();
  const pageErrors: string[] = [];
  merchant.on("pageerror", (error) => pageErrors.push(error.message));
  await merchant.goto(`${origin}/en/`);
  await merchant.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(merchant.getByRole("button", { name: "Live workspace" })).toBeVisible();
  await merchant.goto(`${origin}/en/studio?store=${store}&scene=${session}`);
  await expect(merchant.getByTestId("merchant-studio")).toBeVisible();
  await merchant.getByTestId("studio-open-claims").click();
  await expect(merchant.getByTestId("merchant-claims")).toBeVisible();
  await expect(merchant.getByRole("heading", { level: 1, name: "Keyword claims" })).toBeVisible();
  await expect(merchant.getByText(`Scene: ${scene}`)).toBeVisible();
  await expect(merchant.getByText("MOCK capture — comments are not read automatically yet", { exact: true })).toBeVisible();
  await expect(merchant.getByTestId("claims-window-state")).toHaveText("Closed");
  pass("signed MOCK IdP merchant opens Studio › Claims for the scene with the MOCK label");

  await merchant.getByRole("button", { name: "Open claim window" }).click();
  await expect(merchant.getByTestId("claims-window-state")).toHaveText("Open");
  await expect(merchant.getByLabel("Quantity rule", { exact: true })).toBeDisabled();
  const offerForm = merchant.locator(".claims-offer-form");
  for (const [keyword, sku, max, canonical] of [["ａ１", skuA, "5", "A1"], ["b2", skuB, "3", "B2"]]) {
    await offerForm.getByLabel("Keyword", { exact: true }).fill(keyword);
    await offerForm.getByLabel("Product", { exact: true }).selectOption({ label: product });
    await expect(offerForm.getByLabel("SKU", { exact: true })).toBeEnabled();
    await offerForm.getByLabel("SKU", { exact: true }).selectOption({ label: sku });
    await offerForm.getByLabel("Max per claim", { exact: true }).fill(max);
    await offerForm.getByRole("button", { name: "Add offer" }).click();
    await expect(merchant.getByTestId(`offer-${canonical}`)).toContainText(sku);
  }
  await expect(merchant.getByTestId("host-prompt")).toHaveText(hostPrompt("en", "EXACT", "A1"));
  await merchant.getByLabel("Prompt language", { exact: true }).selectOption("zh-TW");
  await expect(merchant.getByTestId("host-prompt")).toHaveText(hostPrompt("zh-TW", "EXACT", "A1"));
  await merchant.getByLabel("Prompt language", { exact: true }).selectOption("zh-CN");
  await expect(merchant.getByTestId("host-prompt")).toHaveText(hostPrompt("zh-CN", "EXACT", "A1"));
  pass("window opened; A1/B2 offers bound to real SKUs (full-width keyword canonicalized); frozen host prompts");

  const manual = merchant.locator(".claims-manual-form");
  const outcome = merchant.getByTestId("claims-manual-result");
  await manual.getByLabel("Buyer label", { exact: true }).fill("@Amy ");
  await manual.getByLabel("Comment text", { exact: true }).fill("A1");
  await manual.getByRole("button", { name: "Record comment" }).click();
  await expect(outcome).toHaveText("Recorded: A1 × 1");
  await expect(manual.getByLabel("Buyer", { exact: true })).not.toHaveValue("");
  for (const [text, expected] of [["A1+2", "Recorded: A1 × 2 (was 1)"], ["B2", "Recorded: B2 × 1"],
    ["A1 是不是红色", "Not recorded: Not understood"]]) {
    await manual.getByLabel("Comment text", { exact: true }).fill(text);
    await manual.getByRole("button", { name: "Record comment" }).click();
    await expect(outcome).toHaveText(expected);
  }
  await expect(merchant.getByText("1 not understood — pin the host prompt")).toBeVisible();
  await expect(merchant.getByTestId("claims-accepted")).toHaveText("3");
  await expect(merchant.getByTestId("claims-rejected-NO_MATCH")).toHaveText("1");
  const bundle = merchant.getByTestId("bundle-amy");
  await expect(bundle).toContainText("A1 × 2");
  await expect(bundle).toContainText("B2 × 1");
  await expect(bundle).toContainText("Not opened yet");
  pass("manual A1, A1+2 (sets, not adds), B2 and invalid text recorded with per-reason counters");

  async function issueLink(button: string) {
    await bundle.getByRole("button", { name: button, exact: true }).click();
    const dialog = merchant.getByRole("dialog");
    const value = dialog.getByTestId("claims-link-value");
    await expect(value).toHaveText(/^https:\/\/buyer\.example\/en\/claim#t=•+$/);
    await merchantShot(merchant, `merchant-link-dialog-masked-${button.replaceAll(" ", "-").toLowerCase()}`);
    await dialog.getByRole("button", { name: "Reveal link" }).click();
    const url = (await value.textContent()) ?? "";
    expect(url).toMatch(/^https:\/\/buyer\.example\/en\/claim#t=[A-Za-z0-9_-]{43}$/);
    await dialog.getByRole("button", { name: "Copy link" }).click();
    expect(await merchant.evaluate(() => navigator.clipboard.readText())).toBe(url);
    await dialog.getByRole("button", { name: "Copy message" }).click();
    expect(await merchant.evaluate(() => navigator.clipboard.readText())).toContain(url);
    await dialog.getByRole("button", { name: "Close and discard" }).click();
    await expect(merchant.getByRole("dialog")).toHaveCount(0);
    expect(await merchant.content()).not.toContain(url.split("#t=")[1]);
    return url;
  }
  const firstURL = await issueLink("Create link");
  const firstToken = firstURL.split("#t=")[1];
  await storageLacks(merchant, [firstToken]);
  await expect(bundle).toContainText(/Active until/);
  await merchant.evaluate(() => window.scrollTo(0, 0));
  await merchantShot(merchant, "merchant-claims-desktop");
  pass("one-time link masked by default, copied, discarded on close and never stored");

  // Buyer: open the link in three locales (preview only), through the TLS edge.
  const buyerContext = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 } });
  const buyer = await buyerContext.newPage();
  const buyerRequests: string[] = [];
  buyer.on("request", (request) => buyerRequests.push(`${request.url()} ${request.headers()["referer"] ?? ""}`));
  buyer.on("pageerror", (error) => pageErrors.push(error.message));
  const copies = {
    en: { title: "Your claimed items", quantity: "Quantity", price: "Current price, final at checkout", add: "Add to cart" },
    "zh-CN": { title: "你登记的商品", quantity: "数量", price: "当前价格，以结账时为准", add: "加入购物车" },
    "zh-TW": { title: "你登記的商品", quantity: "數量", price: "目前價格，以結帳時為準", add: "加入購物車" },
  } as const;
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    const response = await buyer.goto(firstURL.replace("/en/claim", `/${locale}/claim`));
    expect(response?.status()).toBe(200);
    expect(response?.headers()["referrer-policy"]).toBe("no-referrer");
    await expect(buyer.getByRole("heading", { level: 1, name: copies[locale].title })).toBeVisible();
    await expect(buyer.getByTestId("claim-line-A1")).toContainText(`${copies[locale].quantity} 2`);
    await expect(buyer.getByTestId("claim-line-B2")).toContainText(`${copies[locale].quantity} 1`);
    await expect(buyer.getByTestId("claim-line-A1")).toContainText(skuA);
    await expect(buyer.getByText(copies[locale].price)).toBeVisible();
    expect(new URL(buyer.url()).hash).toBe("");
    await storageLacks(buyer, [firstToken]);
    await fitsWidth(buyer);
    await shot(buyer, `buyer-claim-${locale}-390`);
  }
  pass("buyer preview in en, zh-CN and zh-TW: fragment dropped, no-referrer, 390px without overflow");

  // 409: the claim changes after the buyer previewed it.
  await manual.getByLabel("Comment text", { exact: true }).fill("A1+3");
  await manual.getByRole("button", { name: "Record comment" }).click();
  await expect(outcome).toHaveText("Recorded: A1 × 3 (was 2)");
  await buyer.getByRole("button", { name: copies["zh-TW"].add }).click();
  const conflict = buyer.getByTestId("claim-conflict");
  await expect(conflict).toContainText("你的購物車中有商品已無法購買，或登記內容已變更。請先檢查購物車，然後再試一次。");
  await expect(conflict.getByRole("link", { name: "檢查購物車" })).toHaveAttribute("href", "#claim-cart");
  await shot(buyer, "buyer-claim-409-zh-TW-390");
  await conflict.getByRole("button", { name: "重新讀取登記" }).click();
  await expect(buyer.getByTestId("claim-line-A1")).toContainText("數量 3");
  await buyer.getByRole("button", { name: copies["zh-TW"].add }).click();
  await expect(buyer.getByTestId("claim-added")).toHaveText("已加入購物車。");
  await expect(buyer.getByTestId(`claim-cart-${skuAID}`)).toContainText("× 3");
  await expect(buyer.getByTestId(`claim-cart-${skuBID}`)).toContainText("× 1");
  const cart = await buyer.evaluate(async () => {
    const state = await (await fetch("/api/buyer/session")).json();
    return (await fetch("/api/buyer/cart", { headers: { "X-Buyer-Context": state.context } })).json();
  });
  expect(cart.items).toEqual([{ sku_id: skuAID, quantity: 3 }, { sku_id: skuBID, quantity: 1 }].sort((a, b) => a.sku_id.localeCompare(b.sku_id)));
  await shot(buyer, "buyer-claim-added-zh-TW-390");
  pass("stale claim → 409 copy with cart link, explicit reload, then cart holds A1×3 and B2×1");

  await buyer.goto(firstURL);
  await expect(buyer.getByTestId("claim-line-A1")).toContainText("Already in your cart");
  await expect(buyer.getByTestId("claim-add")).toBeDisabled();
  await merchant.getByRole("button", { name: "Refresh facts" }).click();
  await expect(bundle).toContainText("Opened by a buyer");
  await expect(bundle).toContainText("A1 × 3");
  await expect(bundle).toContainText("In cart");
  pass("bound buyer re-opens the link with nothing pending; merchant sees binding and applied lines");

  // 404: unknown token, no token, and the old token after a rotation.
  await buyer.goto(`${buyerOrigin}/en/claim#t=${randomBytes(32).toString("base64url")}`);
  await expect(buyer.getByTestId("claim-not-found")).toHaveText("This link expired or was replaced — ask the seller for a new link");
  await buyer.goto(`${buyerOrigin}/zh-CN/claim`);
  await expect(buyer.getByTestId("claim-not-found")).toHaveText("此链接已过期或已被替换——请向卖家索取新链接");
  const secondURL = await issueLink("Replace link");
  const secondToken = secondURL.split("#t=")[1];
  expect(secondToken).not.toBe(firstToken);
  await buyer.goto(firstURL.replace("/en/claim", "/zh-TW/claim"));
  await expect(buyer.getByTestId("claim-not-found")).toHaveText("此連結已過期或已被替換——請向賣家索取新連結");
  await shot(buyer, "buyer-claim-404-zh-TW-390");
  await buyer.goto(secondURL);
  await expect(buyer.getByTestId("claim-line-B2")).toContainText("Already in your cart");
  pass("unknown, missing and rotated-out tokens show the 404 copy in each locale; the new link still serves the bound buyer");

  // The token only ever travelled as the claim header on B1/B2.
  for (const token of [firstToken, secondToken]) {
    const audit = await controlCall("observe-token", { method: "POST", body: token });
    expect(audit).toEqual({ url_hits: 0, misplaced_headers: 0, claim_calls: expect.any(Number) });
    expect(audit.claim_calls).toBeGreaterThan(0);
    for (const seen of buyerRequests) expect(seen.split("#")[0]).not.toContain(token);
    for (const log of ["admin-next.log", "storefront-next.log"])
      expect(await readFile(path.join(evidence, log), "utf8")).not.toContain(token);
  }
  expect(await controlCall("facts")).toEqual(facts);
  pass("tokens never in a URL, Referer, log, storage or other route; no inventory, quote or order effect");

  await claimSourcePhase(merchant, pass);

  await merchant.setViewportSize({ width: 390, height: 844 });
  await fitsWidth(merchant);
  await merchant.evaluate(() => window.scrollTo(0, 0));
  await merchantShot(merchant, "merchant-claims-390");
  expect(pageErrors).toEqual([]);
  pass("merchant claims page fits a 390px viewport without page errors");

  await writeFile(path.join(evidence, "result.json"), JSON.stringify({
    cases, locales: ["en", "zh-CN", "zh-TW"], link_sha256: sha256(secondToken), generation: 2,
    boundary: "production Next builds; signed MOCK IdP; MOCK manual ingress; synthetic buyer.example TLS edge; no provider or deployment acceptance",
  }, null, 2), { mode: 0o600 });
});
