// Purpose: LC-U1 real-click workspace contract gate in three locales and two viewport sizes.
// Depends on: @playwright/test, node:fs/promises, node:crypto; signed OIDC/Next/PG harness env LC_BROWSER_CONSOLE_*.
// Used by: browser_live_console_test.go and test-local.sh --browser-live-console.
// Invariants: I01/I02/I06/I10/I11/I14/I18; Console upstream/receipts are explicitly MOCK, never SQL/provider acceptance.
import { expect, type Page, type APIRequestContext, type BrowserContext } from "@playwright/test";
import { test } from "./inbox-private-evidence";
import { writeFile } from "node:fs/promises";
import { randomBytes } from "node:crypto";
import { workspaceCopy } from "../../apps/admin/src/features/live/workspace-copy";
import { money } from "../../packages/format/src/index";
import { commentCopy } from "../../apps/admin/src/features/live/comment-copy";
import { nativePage } from "./fixtures/native-device";

const required = (name: string): string => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const api = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_CONSOLE_STORE");
const otherStore = required("LC_BROWSER_CONSOLE_OTHER_STORE");
const otherScene = required("LC_BROWSER_CONSOLE_OTHER_SCENE");
const lateScene = required("LC_BROWSER_CONSOLE_LATE_SCENE");
const lateDestination = required("LC_BROWSER_CONSOLE_LATE_DEST");
const control = required("LC_BROWSER_CONSOLE_CONTROL");
const narrowToken = required("LC_BROWSER_CONSOLE_NARROW_TOKEN");
const readToken = required("LC_BROWSER_CONSOLE_READ_TOKEN");
const scenes: string[] = JSON.parse(required("LC_BROWSER_CONSOLE_SCENES"));
const comments: {session:string;bundle:string;claim_ref:string;latest_ref:string;old_ref:string;reset_ref:string;reply_refs:string[]}=JSON.parse(required("LC_BROWSER_CONSOLE_COMMENTS"));
if (scenes.length !== 6 || new Set(scenes).size !== 6) throw new Error("six independent Console scenes required");
const locales = ["en", "zh-TW", "zh-CN"] as const;
const sizes = [{ width: 1586, height: 992 }, { width: 390, height: 844 }];
const streamSizes = [{width:1440,height:992},{width:390,height:844}];
type Receipt = { scene: string; action: string; key_hash: string; body_hash: string; status: number; effect: boolean; input: Record<string, unknown> };
type Scene = { ID: string; Title: string; Phase: string; Offer: string; SKU: string; Warehouse: string; Active: boolean; Stock: number; StockVersion: number; OfferVersion: number; Reads: number[] | null; Recommended: unknown; Copied: boolean };
type Facts = { class: "MOCK"; scenes: Record<string, Scene>; receipts: Receipt[]; bad_authority: number;comments:{requests:number;older_requests:number;private_requests:number;public_requests:number;private_operations:number;graph_sends:number} };

// Trace records credential headers; page-only screenshots and redacted receipts are the evidence.
test.use({ baseURL: origin, trace: "off", screenshot: "off", video:"off" });
test.setTimeout(110_000);

async function facts(request: APIRequestContext): Promise<Facts> {
  const response = await request.get(`${api}/__test/live-console/facts`, { headers: { "X-Console-Control": control } });
  expect(response.status()).toBe(200);
  const value = await response.json() as Facts;
  expect(value.class).toBe("MOCK");
  expect(Array.isArray(value.receipts)).toBe(true);
  expect(value.bad_authority).toBe(0);
  return value;
}
async function fault(request: APIRequestContext, scene: string, mode: string) {
  // G-UI8 audit [FIXTURE/SETUP]: loopback-only fault control never patches product DOM/API responses in the browser.
  const response = await request.post(`${api}/__test/live-console/fault`, { headers: { "X-Console-Control": control }, data: { scene, mode } });
  expect(response.status()).toBe(200);
}
async function refreshList(page: Page, status: number) {
  // Match the list's exact GET path, not A1 or an implicit background poll.
  await Promise.all([
    page.waitForResponse((response) => new URL(response.url()).pathname === `/api/stores/${store}/live-sessions` &&
      response.request().method() === "GET" && response.status() === status, { timeout: 15_000 }),
    page.getByTestId("live-console-refresh").click(),
  ]);
}
async function login(page: Page) {
  await page.goto(`${origin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("shell-store-selector")).toBeAttached();
}
async function authorityCookies(context: BrowserContext) {
  // Cookie inspection filters by scheme too. Query the same host's Secure
  // scope; the actual BFF request still uses the unchanged loopback origin.
  return context.cookies(origin.replace(/^http:/, "https:"));
}
async function authorityCookie(context: BrowserContext): Promise<string> {
  // SECURITY/NEGATIVE: Node's APIRequestContext omits Secure cookies on this HTTP
  // loopback fixture, unlike Chromium. Pin the existing signed browser authority
  // so the negative probe reaches CSRF/permission checks, not the 401 login gate.
  const cookies = await authorityCookies(context);
  expect(cookies.some((cookie) => cookie.name === "__Host-commerce_session")).toBe(true);
  return cookies.filter((cookie) => ["__Host-commerce_session", "__Host-commerce_csrf"].includes(cookie.name))
    .map((cookie) => `${cookie.name}=${cookie.value}`).join("; ");
}
const route = (locale: string, scene: string, selectedStore = store) => `${origin}/${locale}/studio/console?store=${selectedStore}&scene=${scene}`;
async function phase(page: Page, value: string) {
  await expect(page.getByTestId("live-phase")).toHaveAttribute("data-phase", value);
}

test.describe("LC-U2a REAL_PG comment stream",()=>{
  // Comment text/name/PSIDs may not enter automatic failure artifacts, even in synthetic fixtures.
  const reads=new WeakMap<Page,Record<string,unknown>[]>();
  test.beforeEach(async({page})=>{
    const rows:Record<string,unknown>[]=[];reads.set(page,rows);
    page.on("response",async r=>{
      if(!new URL(r.url()).pathname.endsWith("/comments"))return;
      let value:any;try{value=await r.json();}catch{value={};}
      rows.push({status:r.status(),count:Array.isArray(value.items)?value.items.length:null,epoch:typeof value.epoch==="number"?value.epoch:null,reset:value.reset===true,
        code:["not_found","no_source","forbidden","stream_unavailable","invalid_request","invalid_cursor"].includes(value.code)?value.code:null});
    });
  });
  test.afterEach(async({page},info)=>{await writeFile(`${evidence}/comment-reads-${info.testId.replace(/[^a-zA-Z0-9_-]/g,"")}.json`,JSON.stringify(reads.get(page)??[]));});
  for(const [li,locale] of locales.entries())for(const [wi,size] of streamSizes.entries()) {
    test(`pagination filters buyer and one-shot replies ${locale}-${size.width}`,async({page,request})=>{
      const c=commentCopy(locale);await page.setViewportSize(size);await login(page);await page.goto(route(locale,comments.session));
      await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(50);
      const before=(await facts(request)).comments;
      await page.getByTestId("comment-older").click();
      await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(60);
      expect((await facts(request)).comments.older_requests).toBe(before.older_requests+1);
      await page.getByTestId("comment-filter-keyword").click();await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(1);
      await page.getByTestId(`comment-select-${comments.claim_ref}`).click();
      await expect(page.getByTestId("buyer-panel")).toBeVisible();await expect(page.getByTestId("buyer-panel")).toContainText("A1");
      await page.getByTestId("comment-filter-private").click();await expect(page.getByTestId("comment-conversations").locator("li")).toHaveCount(1);
      await page.getByTestId("comment-filter-unreplied").click();await expect(page.getByTestId("comment-conversations").locator("li")).toHaveCount(1);
      await page.getByTestId("comment-filter-all").click();
      await page.getByTestId(`comment-select-${comments.reply_refs[li*2+wi]}`).click();
      await expect(page.getByTestId("comment-reply")).toContainText(c.one);await expect(page.getByTestId("comment-reply")).toContainText(c.window);
      await page.getByTestId("comment-reply-text").fill(`Synthetic thanks ${li}-${wi}`);
      const sent=page.waitForResponse(r=>r.url().endsWith("/private-reply")&&r.request().method()==="POST",{timeout:15000});
      await page.getByTestId("comment-send").click();expect((await sent).status()).toBe(200);
      await expect(page.getByTestId("comment-rule")).toHaveText(c.used);
      await expect(page.getByTestId("comment-send")).toBeDisabled();
      expect((await facts(request)).comments.private_operations).toBe(before.private_operations+1);
      // Explicit merchant verification unlocks only future sends; queued is not a final delivery fact.
      if(await page.getByTestId("comment-verified").isVisible()) {
        page.once("dialog",dialog=>dialog.accept());await page.getByTestId("comment-verified").click();
        expect((await facts(request)).comments.private_operations).toBe(before.private_operations+1);
      }
      await page.getByTestId("comment-reply").getByRole("button",{name:c.publicReply,exact:true}).click();
      await expect(page.getByTestId("comment-reply")).toContainText(c.publicRule);
      await page.getByTestId("comment-reply-text").fill("https://checkout.stripe.com/pay/synthetic");
      const denied=page.waitForResponse(r=>r.url().endsWith("/public-reply")&&r.request().method()==="POST",{timeout:15000});
      await page.getByTestId("comment-send").click();expect((await denied).status()).toBe(422);
      await expect(page.getByTestId("comment-reply-error")).toHaveText(c.public_reply_forbidden_content);
      expect((await facts(request)).comments.private_operations).toBe(before.private_operations+1);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
      await page.screenshot({path:`${evidence}/comments-${locale}-${size.width}.png`,fullPage:true,animations:"disabled",mask:[page.locator('[data-private]'),page.getByTestId("buyer-panel"),page.getByTestId("comment-reply-text")]});
    });
  }
  test("native hidden tab stops A2 and clears private selections",async({request})=>{
    const {page,close}=await nativePage(evidence,"console-native-");
    try {
    await login(page);await page.goto(route("en",comments.session));await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    await page.getByTestId(`comment-select-${comments.claim_ref}`).click();await expect(page.getByTestId("buyer-panel")).toBeVisible();
    const cover=await page.context().newPage();await cover.goto("about:blank");await cover.bringToFront();
    await expect.poll(()=>page.evaluate(()=>document.visibilityState)).toBe("hidden");
    await expect(page.getByTestId("comment-rows")).toHaveCount(0);await expect(page.getByTestId("buyer-panel")).toHaveCount(0);
    const before=(await facts(request)).comments.requests;
    await cover.waitForTimeout(6500); // two real 3s intervals; absence of requests is the assertion.
    expect((await facts(request)).comments.requests).toBe(before);
    await page.bringToFront();await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();await cover.close();
    } finally { await close(); }
  });
  test("unknown public ACK stays fenced across selection and reload",async({page,request})=>{
    await login(page);await page.goto(route("en",comments.session));await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    await page.getByTestId(`comment-select-${comments.latest_ref}`).click();await page.getByRole("button",{name:"Public reply",exact:true}).click();
    await page.getByTestId("comment-reply-text").fill("Synthetic public ACK case");
    // FAULT INJECTION: the command reaches real Go/PG; only its acknowledgement is dropped.
    await page.route(`**/comments/${comments.latest_ref}/public-reply`,async r=>{const real=await r.fetch();expect(real.status()).toBe(200);await r.abort("connectionreset");},{times:1});
    const before=(await facts(request)).comments.public_requests;
    await page.getByTestId("comment-send").click();await expect(page.getByTestId("comment-unknown")).toBeVisible();
    await page.getByTestId(`comment-select-${comments.reply_refs[0]}`).click();await expect(page.getByTestId("comment-send")).toBeDisabled();
    await page.reload();await page.getByTestId(`comment-select-${comments.latest_ref}`).click();await expect(page.getByTestId("comment-send")).toBeDisabled();
    expect((await facts(request)).comments.public_requests).toBe(before+1);
    const flags=await page.evaluate(()=>Object.entries(sessionStorage).filter(([key])=>key.startsWith("live-comment-unresolved:")));
    expect(flags).toHaveLength(1);expect(flags[0][1]).toBe("1");expect(flags[0][0]).not.toContain(comments.latest_ref);
  });
  test("LCU2_RESET epoch replacement clears every old comment before reread",async({page,request})=>{
    await login(page);await page.goto(route("en",comments.session));await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    await fault(request,comments.session,"comments_reset");
    await expect(page.getByTestId(`comment-row-${comments.reset_ref}`)).toBeVisible();
    await expect(page.getByTestId(`comment-row-${comments.latest_ref}`),"LCU2_RESET old epoch retained").toHaveCount(0);
    await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(1);
  });
});
async function screenshot(page: Page, name: string) {
  // G-UI8 audit [READ/MEASURE]: layout and storage reads do not change product state.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: `${evidence}/${name}.png`, fullPage: true, animations: "disabled" });
}

for (const [localeIndex, locale] of locales.entries()) for (const [sizeIndex, size] of sizes.entries()) {
  const index = localeIndex * 2 + sizeIndex;
  const scene = scenes[index]!;
  const name = `${locale}-${size.width}`;
  test(`LC-U1 MOCK contract real clicks ${name}`, async ({ page, request, context }) => {
    await page.setViewportSize(size);
    await login(page);
    await page.goto(route(locale, scene));
    await expect(page.getByTestId("live-workspace")).toBeVisible();
    await expect(page.getByTestId("live-console")).toBeVisible();
    await phase(page, "draft");
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    const stats = page.getByTestId("live-console").locator(".live-status-bar");
    await expect(stats.locator("div").filter({ has: page.getByText(workspaceCopy[locale].amount, { exact: true }) }).locator("dd")).toHaveText(money(locale, "TWD", 60000));
    await expect(stats.locator("div").filter({ has: page.getByText(workspaceCopy[locale].paid, { exact: true }) }).locator("dd")).toHaveText(money(locale, "TWD", 20000));
    await expect(stats.locator("div").filter({ has: page.getByText(workspaceCopy[locale].comments, { exact: true }) }).locator("dd")).toHaveText("—");
    const initial = (await facts(request)).scenes[scene]!;
    const offer = initial.Offer;
    await expect(page.getByTestId(`live-offer-${offer}`)).toContainText("MOCK Console Tea");
    await screenshot(page, `${name}-draft`);
    await expect(page.locator("iframe")).toHaveCount(0);
    // Actual BFF CSRF/Origin refusal must happen before the MOCK write transport receives a command.
    const beforeRefusal = (await facts(request)).receipts.length;
    const refused = await page.request.post(`${origin}/api/stores/${store}/live-sessions/${scene}/lifecycle`, {
      headers: { Cookie: await authorityCookie(page.context()), Origin: origin, "Idempotency-Key": `lc-u1-csrf-${name}` },
      data: { action: "start", expected_version: 1 },
    });
    expect(refused.status()).toBe(403);
    expect((await facts(request)).receipts).toHaveLength(beforeRefusal);

    // Real elapsed cadence, without fake clock/DOM mutation: two successive 5 s reads.
    const baseline = initial.Reads?.length ?? 0;
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Reads?.length ?? 0, { timeout: 15_000 }).toBeGreaterThanOrEqual(baseline + 2);
    const timed = (await facts(request)).scenes[scene]!.Reads!;
    const cadence = timed.slice(baseline - 1);
    for (let i = 1; i < cadence.length; i++) {
      expect(cadence[i]! - cadence[i - 1]!).toBeGreaterThanOrEqual(3500);
      expect(cadence[i]! - cadence[i - 1]!).toBeLessThanOrEqual(8000);
    }
    if (locale === "en" && size.width === 1586) {
      // Preview refresh/focus reads happen only after measuring the untouched polling window above.
      await fault(request, scene, "facebook_source");
      await page.getByTestId("live-console-refresh").click();
      const url = "https://www.facebook.com/123/posts/456";
      // MOCK external dependency only: actual link click opens the real URL shape without a live Facebook request.
      await page.context().route(url, (route) => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>MOCK Facebook post</title><h1>MOCK Facebook post</h1>" }));
      const link = page.getByRole("link", { name: workspaceCopy.en.openFacebook, exact: true });
      await expect(link).toHaveAttribute("href", url);
      await expect(link).toHaveAttribute("rel", "noopener noreferrer");
      const popupReady = page.waitForEvent("popup", { timeout: 15_000 });
      await link.click();
      const popup = await popupReady;
      await expect(popup).toHaveURL(url);
      await popup.close();
      await page.bringToFront();
      await phase(page, "draft");
      await expect(page.locator("iframe")).toHaveCount(0);
    }
    await page.getByTestId("live-primary-action").click();
    await phase(page, "live");
    expect((await facts(request)).scenes[scene]!.Phase).toBe("live");
    await screenshot(page, `${name}-live`);

    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(false);
    expect((await facts(request)).receipts.at(-1)!.input.max_quantity_per_claim).toBe(10);
    await page.getByTestId("live-console-refresh").click();
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(true);

    await page.getByTestId(`live-stock-${offer}`).fill("17");
    await page.getByTestId(`live-stock-save-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Stock).toBe(17);
    const stockReceipt = (await facts(request)).receipts.at(-1)!;
    expect(stockReceipt.input).toMatchObject({ delta: 5, expected_version: 1, reason: "live_console_edit" });
    await page.reload();
    await expect(page.getByTestId(`live-stock-${offer}`)).toHaveValue("17");

    // FIXTURE/STIMULUS: another operator changes this MOCK inventory through the
    // real authenticated BFF. It is not a replacement for the editing merchant's
    // clicks below, and does not patch the DOM or intercept the A1 response.
    const externalStock = async (delta: number, version: number) => {
      const csrf = (await authorityCookies(context)).find((cookie) => cookie.name === "__Host-commerce_csrf")?.value;
      expect(!!csrf).toBe(true);
      const currentOrigin = new URL(page.url()).origin;
      const response = await context.request.post(`${currentOrigin}/api/stores/${store}/inventory/adjustments`, {
        headers: { Cookie: await authorityCookie(context), Origin: currentOrigin, "X-CSRF-Token": csrf!, "Idempotency-Key": randomBytes(16).toString("hex") },
        data: { warehouse_id: initial.Warehouse, sku_id: initial.SKU, delta, expected_version: version, reason: "live_console_edit" },
      });
      expect(response.status()).toBe(200);
    };
    // Idle rows follow polled stock without any refresh button.
    await externalStock(1, 2);
    const stockInput = page.getByTestId(`live-stock-${offer}`);
    await expect(stockInput).toHaveValue("18");
    await stockInput.fill("23");
    const changedPoll = page.waitForResponse(async (response) =>
      response.request().method() === "GET" && new URL(response.url()).pathname.endsWith(`/${scene}/console`) &&
      (await response.json()).offers.some((item: { offer_id: string; stock: { balance_version: number } }) => item.offer_id === offer && item.stock.balance_version === 4),
    { timeout: 15_000 });
    await externalStock(2, 3);
    await changedPoll;
    await expect(stockInput).toHaveValue("23");
    await expect(stockInput).toBeFocused();
    const changed = page.getByTestId(`live-stock-changed-${offer}`);
    await expect(changed).toBeVisible();
    await expect(changed).toContainText("20");
    await expect(page.getByTestId(`live-stock-save-${offer}`)).toBeDisabled();
    await screenshot(page, `${name}-stock-edit-poll`);
    const beforeConfirm = (await facts(request)).receipts.length;
    await page.getByTestId(`live-stock-confirm-${offer}`).click();
    expect((await facts(request)).receipts).toHaveLength(beforeConfirm);
    await expect(stockInput).toHaveValue("23");
    await page.getByTestId(`live-stock-save-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Stock).toBe(23);
    expect((await facts(request)).receipts.at(-1)!.input).toMatchObject({ delta: 3, expected_version: 4, reason: "live_console_edit" });
    await page.reload();
    await expect(stockInput).toHaveValue("23");
    const beforeCancel = (await facts(request)).receipts.length;
    await stockInput.fill("24");
    await page.getByTestId(`live-offer-${offer}`).getByRole("button", { name: workspaceCopy[locale].cancel, exact: true }).click();
    await expect(stockInput).toHaveValue("23");
    expect((await facts(request)).receipts).toHaveLength(beforeCancel);
    await page.getByTestId(`live-recommend-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Recommended).toEqual({ offer_id: offer, at: "2030-01-01T00:00:00Z" });
    expect((await facts(request)).receipts.at(-1)!.input.post_comment).toBe(false);

    // Server-side CAS bump, then a real click must reconcile before a fresh command.
    await fault(request, scene, "conflict");
    const conflictCount = (await facts(request)).receipts.length;
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).receipts.length).toBe(conflictCount + 1);
    expect((await facts(request)).receipts.at(-1)!.status).toBe(409);
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    await page.getByTestId("live-console-refresh").click();
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(false);
    await page.getByTestId("live-console-refresh").click();

    // I06: lose ACK after a committed MOCK effect. Polling is read-only; explicit retry must reuse the key.
    await fault(request, scene, "unknown");
    const beforeUnknown = (await facts(request)).receipts.length;
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const unknown = (await facts(request)).receipts.at(-1)!;
    expect(unknown.status).toBe(503);
    expect(unknown.effect).toBe(true);
    await page.getByTestId("live-console-refresh").click();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    await fault(request, scene, "list_unavailable");
    await refreshList(page, 503);
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    expect((await facts(request)).receipts).toHaveLength(beforeUnknown + 1);
    await fault(request, scene, "list_restore");
    await refreshList(page, 200);
    await expect(page.locator(`#live-session-picker option[value="${scene}"]`)).toHaveText((await facts(request)).scenes[scene]!.Title);
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const reads = (await facts(request)).scenes[scene]!.Reads?.length ?? 0;
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Reads?.length ?? 0, { timeout: 8_000 }).toBeGreaterThan(reads);
    expect((await facts(request)).receipts).toHaveLength(beforeUnknown + 1);
    await page.getByTestId("live-command-retry").click();
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    // The retry control hides at request start, not at acknowledgement. Wait
    // for the same-key transport receipt before inspecting exact replay/effect counts.
    await expect.poll(async () => (await facts(request)).receipts.filter((receipt) => receipt.key_hash === unknown.key_hash)).toHaveLength(2);
    const retries = (await facts(request)).receipts.filter((receipt) => receipt.key_hash === unknown.key_hash);
    expect(retries).toHaveLength(2);
    expect(retries.map((receipt) => receipt.body_hash)).toEqual([unknown.body_hash, unknown.body_hash]);
    expect(retries.filter((receipt) => receipt.effect)).toHaveLength(1);

    page.once("dialog", (dialog) => dialog.accept());
    await page.getByTestId("live-primary-action").click();
    await phase(page, "ended");
    await screenshot(page, `${name}-ended`);
    await page.getByTestId("live-session-results").click();
    await expect(page.getByTestId("merchant-studio")).toBeVisible();
    await expect(page.getByTestId(`live-session-results-${scene}`)).toContainText(workspaceCopy[locale].paidOrders);
    // Cover the separate A5 list-copy surface once; every locale/size still exercises console copy below.
    if (index === 0) {
      await page.getByTestId("live-copy-session").click();
      await page.getByLabel(workspaceCopy[locale].name, { exact: true }).fill("LC-U1 copied from session list");
      await page.getByRole("button", { name: workspaceCopy[locale].confirmCopy, exact: true }).click();
      await phase(page, "draft");
      const listCopy = new URL(page.url()).searchParams.get("scene");
      expect(listCopy).not.toBe(scene);
      expect((await facts(request)).scenes[listCopy!]!.Title).toBe("LC-U1 copied from session list");
      await screenshot(page, "en-1586-session-list-copy");
    }
    await page.goto(route(locale, scene));
    await phase(page, "ended");
    await page.getByTestId("live-primary-action").click();
    await page.getByTestId("live-copy-title").fill(`LC-U1 copied ${name}`);
    await expect(page.getByTestId("live-primary-action")).toHaveCount(0);
    await expect(page.getByTestId("live-console").locator("button.primary")).toHaveCount(1);
    await page.getByTestId("live-copy-confirm").click();
    await expect(page).not.toHaveURL(route(locale, scene));
    const copied = new URL(page.url()).searchParams.get("scene");
    expect(copied).toBeTruthy();
    expect(copied).not.toBe(scene);
    await phase(page, "draft");
    expect((await facts(request)).scenes[copied!]!.Title).toBe(`LC-U1 copied ${name}`);
    await page.reload();
    await phase(page, "draft");

    // A second lost ACK remains fenced after reload: reads cannot settle an ambiguous command.
    const copiedOffer = (await facts(request)).scenes[copied!]!.Offer;
    await fault(request, copied!, "unknown");
    await page.getByTestId(`live-recommend-${copiedOffer}`).click();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const reloadUnknown = (await facts(request)).receipts.at(-1)!;
    expect(reloadUnknown.scene).toBe(copied);
    expect(reloadUnknown.status).toBe(503);
    await page.reload();
    await expect(page.getByTestId("live-primary-action")).toBeDisabled();
    await expect(page.getByTestId(`live-offer-toggle-${copiedOffer}`)).toBeDisabled();
    await expect(page.getByTestId(`live-stock-save-${copiedOffer}`)).toBeDisabled();
    await expect(page.getByTestId(`live-recommend-${copiedOffer}`)).toBeDisabled();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const reloadReads = (await facts(request)).scenes[copied!]!.Reads?.length ?? 0;
    await expect.poll(async () => (await facts(request)).scenes[copied!]!.Reads?.length ?? 0, { timeout: 8_000 }).toBeGreaterThan(reloadReads);
    expect((await facts(request)).receipts.filter((receipt) => receipt.key_hash === reloadUnknown.key_hash)).toHaveLength(1);
    await screenshot(page, `${name}-unknown-reload`);
    // An explicit replay after reload must reuse the persisted original request, never make a new operation.
    await page.getByTestId("live-command-retry").click();
    await expect.poll(async () => (await facts(request)).receipts.filter((receipt) => receipt.key_hash === reloadUnknown.key_hash).length).toBe(2);
    const replayed = (await facts(request)).receipts.filter((receipt) => receipt.key_hash === reloadUnknown.key_hash);
    expect(replayed[1].body_hash).toBe(replayed[0].body_hash);
    expect(replayed.filter((receipt) => receipt.effect)).toHaveLength(1);
    await expect(page.getByTestId("live-primary-action")).toBeEnabled();

    // The existing shell store control navigates through the real context; old Console data must disappear.
    await page.getByTestId("shell-store-selector").selectOption(otherStore);
    await expect(page.getByText(`LC-U1 copied ${name}`, { exact: true })).toHaveCount(0);
    await page.goto(route(locale, otherScene, otherStore));
    await expect(page.getByTestId("live-console")).toBeVisible();
    await expect(page.getByText(`LC-U1 copied ${name}`, { exact: true })).toHaveCount(0);
    await page.getByTestId("workspace-sign-out").locator("xpath=ancestor::details/summary").click();
    await page.getByTestId("workspace-sign-out").click();
    await expect(page.getByTestId("live-console")).toHaveCount(0);
    const cookies = await page.context().cookies(origin);
    expect(cookies.filter((cookie) => cookie.name === "__Host-commerce_session" || cookie.name === "__Host-commerce_csrf").map((cookie) => cookie.name)).toHaveLength(0);
    const end = await facts(request);
    await writeFile(`${evidence}/${name}-receipts.json`, JSON.stringify({ class: "MOCK", scene, copied, receipts: end.receipts.filter((receipt) => receipt.scene === scene), cadence }, null, 2));
  });
}

test("LC-U1 unknown receipt stays fenced after real logout and reauthentication", async ({ page, request }) => {
  await login(page);
  await page.goto(route("en", lateDestination));
  await phase(page, "draft");
  const offer = (await facts(request)).scenes[lateDestination]!.Offer;
  await fault(request, lateDestination, "unknown");
  await page.getByTestId(`live-recommend-${offer}`).click();
  await expect(page.getByTestId("live-command-retry")).toBeVisible();
  const receipt = (await facts(request)).receipts.at(-1)!;
  const before = (await authorityCookies(page.context())).find((c) => c.name === "__Host-commerce_csrf")?.value;
  await page.getByTestId("workspace-sign-out").locator("xpath=ancestor::details/summary").click();
  await page.getByTestId("workspace-sign-out").click();
  await expect(page.getByTestId("live-console")).toHaveCount(0);
  // Clearing the private view precedes the hard logout redirect. Do not race
  // that navigation with the next login's goto (net::ERR_ABORTED on Linux CI).
  await page.waitForURL((url) => url.origin === new URL(origin).origin && url.pathname === "/en", { waitUntil: "domcontentloaded", timeout: 15_000 });
  await expect(page.getByRole("button", { name: "Sign in with identity service" })).toBeVisible();
  await login(page);
  const after = (await authorityCookies(page.context())).find((c) => c.name === "__Host-commerce_csrf")?.value;
  expect(Boolean(before && after && before !== after)).toBe(true); // Never put cookie values in assertion diagnostics.
  await page.goto(route("en", lateDestination));
  await phase(page, "draft");
  await expect(page.getByTestId("live-primary-action")).toBeDisabled();
  await expect(page.getByTestId(`live-recommend-${offer}`)).toBeDisabled();
  await expect(page.getByTestId("live-command-retry")).toBeVisible();
  expect((await facts(request)).receipts.filter((r) => r.key_hash === receipt.key_hash)).toHaveLength(1);
  await screenshot(page, "en-reauth-receipt-fence");
});

test("LC-U1 delayed copy cannot navigate back after an actual SPA scene switch", async ({ page, request }) => {
  await page.setViewportSize(sizes[1]!);
  await login(page);
  await page.goto(route("en", lateScene));
  await phase(page, "draft");
  await page.getByTestId("live-primary-action").click();
  await phase(page, "live");
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByTestId("live-primary-action").click();
  await phase(page, "ended");
  await page.getByTestId("live-primary-action").click();
  await page.getByTestId("live-copy-title").fill("Delayed copy scope test");
  await fault(request, lateScene, "delay_copy");
  const response = page.waitForResponse((r) => r.url().endsWith(`/${lateScene}/copy`) && r.request().method() === "POST", { timeout: 15_000 });
  await page.getByTestId("live-copy-confirm").click();
  page.once("dialog", (dialog) => dialog.accept());
  await page.locator("#live-session-picker").selectOption(lateDestination);
  await response;
  await expect.poll(() => new URL(page.url()).searchParams.get("store")).toBe(store);
  await expect.poll(() => new URL(page.url()).searchParams.get("scene")).toBe(lateDestination);
  await expect(page.getByText("Delayed copy scope test", { exact: true })).toHaveCount(0);
  expect((await facts(request)).receipts.filter((r) => r.scene === lateScene && r.action === "copy" && r.effect)).toHaveLength(1);
  await screenshot(page, "en-390-delayed-copy-other-scene");
});

for (const [index, locale] of locales.entries()) {
  test(`LC-U1 ${locale}: bounded live_adjust works, no-stock permission stays disabled, missing A1 is graceful`, async ({ browser, request }) => {
    const context = await browser.newContext({ viewport: sizes[1], ignoreHTTPSErrors: true });
    try {
      const url = origin.replace(/^http:/, "https:");
      const csrf = randomBytes(32).toString("base64url");
      await context.addCookies([
        { name: "__Host-commerce_session", value: narrowToken, url, secure: true, httpOnly: true, sameSite: "Lax" },
        { name: "__Host-commerce_csrf", value: csrf, url, secure: true, httpOnly: false, sameSite: "Lax" },
      ]);
      const page = await context.newPage();
      const scene = lateDestination;
      await page.goto(route(locale, scene));
      await expect(page.getByTestId("live-console")).toBeVisible();
      await phase(page, "draft");
      const body = (await facts(request)).scenes[scene]!;
      await expect(page.getByTestId("live-primary-action")).toBeDisabled();
      await expect(page.getByTestId(`live-offer-toggle-${body.Offer}`)).toBeDisabled();
      await expect(page.getByTestId(`live-recommend-${body.Offer}`)).toBeDisabled();
      await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeEnabled();
      await expect(page.getByTestId("live-console").locator(".live-status-bar > div").first().locator("dd")).toHaveText("—");
      const before = (await facts(request)).receipts.length;
      await page.getByTestId(`live-stock-${body.Offer}`).fill(String(body.Stock + 1001));
      await page.getByTestId(`live-stock-save-${body.Offer}`).click();
      await expect(page.getByTestId(`live-offer-${body.Offer}`).getByRole("alert")).toHaveText(workspaceCopy[locale].stockInvalid);
      expect((await facts(request)).receipts).toHaveLength(before);
      await page.getByTestId(`live-stock-${body.Offer}`).fill(String(body.Stock + 2));
      await page.getByTestId(`live-stock-save-${body.Offer}`).click();
      await expect.poll(async () => (await facts(request)).scenes[scene]!.Stock).toBe(body.Stock + 2);
      await page.reload();
      await expect(page.getByTestId(`live-stock-${body.Offer}`)).toHaveValue(String(body.Stock + 2));
      const receipt = (await facts(request)).receipts.at(-1)!;
      expect(receipt).toMatchObject({ action: "adjustments", status: 200, effect: true });
      expect(receipt.input).toMatchObject({ delta: 2, expected_version: body.StockVersion, reason: "live_console_edit" });
      await fault(request, scene, "below_reserved");
      await page.getByTestId(`live-stock-${body.Offer}`).fill(String(body.Stock + 1));
      await page.getByTestId(`live-stock-save-${body.Offer}`).click();
      await expect(page.getByTestId("live-console").getByRole("alert")).toContainText(workspaceCopy[locale].refusals.below_reserved);
      expect((await facts(request)).scenes[scene]!.Stock).toBe(body.Stock + 2);
      await screenshot(page, `${locale}-narrow-permission`);
      // Setup a distinct signed principal with no inventory grant; assertions use the real page.
      await context.addCookies([{ name: "__Host-commerce_session", value: readToken, url, secure: true, httpOnly: true, sameSite: "Lax" }]);
      await page.reload();
      await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeDisabled();
      await expect(page.getByText(workspaceCopy[locale].stockPermission, { exact: true })).toBeVisible();
      await expect(page.getByTestId(`live-stock-save-${body.Offer}`)).toBeDisabled();
      const after = (await facts(request)).receipts.length;
      await page.getByTestId("live-console-refresh").click();
      await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeDisabled();
      expect((await facts(request)).receipts).toHaveLength(after);
      // Authority negative only: direct BFF request supplements the real disabled-control clicks above.
      const forbidden = await context.request.post(`${origin}/api/stores/${store}/inventory/adjustments`, {
        headers: { Cookie: await authorityCookie(context), origin, "x-csrf-token": csrf, "idempotency-key": `no-stock-${locale}` },
        data: { warehouse_id: body.Warehouse, sku_id: body.SKU, delta: 1, expected_version: body.StockVersion + 1, reason: "live_console_edit" },
      });
      expect(forbidden.status()).toBe(403);
      expect((await forbidden.json()).code).toBe("forbidden");
      expect((await facts(request)).receipts).toHaveLength(after);
      expect((await facts(request)).scenes[scene]!.Stock).toBe(body.Stock + 2);
      await screenshot(page, `${locale}-no-stock-permission`);
      await fault(request, scene, "missing");
      try {
        await page.reload();
        await expect(page.getByTestId("live-console-unavailable")).toBeVisible();
        await expect(page.getByTestId(`live-offer-${body.Offer}`)).toHaveCount(0);
        await screenshot(page, `${locale}-missing-A1`);
      } finally { await fault(request, scene, "restore"); }
    } finally { await context.close(); }
  });
}
