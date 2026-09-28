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
import { hostPrompt } from "../../apps/admin/lib/claims-copy";

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
