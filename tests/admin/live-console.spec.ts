// Purpose: LC-U1 real-click workspace contract gate in three locales and two viewport sizes.
// Depends on: @playwright/test, shared real-click console controls, node:fs/promises, node:crypto; signed OIDC/Next/PG harness env LC_BROWSER_CONSOLE_*.
// Used by: browser_live_console_test.go and test-local.sh --browser-live-console.
// Invariants: I01/I02/I06/I10/I11/I14/I18; Console upstream/receipts are explicitly MOCK, never SQL/provider acceptance.
import { expect, type Page, type APIRequestContext, type BrowserContext } from "@playwright/test";
import { test } from "./inbox-private-evidence";
import { mkdir, writeFile } from "node:fs/promises";
import { randomBytes } from "node:crypto";
import { workspaceCopy } from "../../apps/admin/src/features/live/workspace-copy";
import { money } from "../../packages/format/src/index";
import { commentCopy } from "../../apps/admin/src/features/live/comment-copy";
import { nativePage } from "./fixtures/native-device";
import { showLivePanel as livePanel, showLiveBuyerDetails as buyerDetails, showLiveTools } from "./fixtures/live-console-controls";
import type { ConsoleSnapshot } from "../../apps/admin/src/features/live/console-model";
import type { StreamComment } from "../../apps/admin/src/features/live/comment-model";
import type { BuyerData } from "../../apps/admin/lib/inbox-types";
import type { Offer } from "../../apps/admin/lib/claims-model";
import type { StudioPage } from "../../apps/admin/lib/studio-model";

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
// The existing comment screenshot below and the dynamic coverage oracle use the same actual mask set.
const commentScreenshotMaskSelectors = ['[data-testid="comment-rows"]', '[data-testid="buyer-panel"]', '[data-testid="comment-reply-text"]'] as const;

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
  await expect(page.getByTestId("live-console")).toHaveAttribute("data-phase", value);
  await showLiveTools(page);
}

test.describe("LC-U2a REAL_PG comment stream",()=>{
  // Comment text/name/PSIDs may not enter automatic failure artifacts, even in synthetic fixtures.
  const reads=new WeakMap<Page,Record<string,unknown>[]>();
  const scopeReads=new WeakMap<Page,Record<string,unknown>[]>();
  test.beforeEach(async({page})=>{
    const rows:Record<string,unknown>[]=[];reads.set(page,rows);
    const scoped:Record<string,unknown>[]=[];scopeReads.set(page,scoped);
    page.on("response",r=>{
      const path=new URL(r.url()).pathname;
      const category=path===`/api/stores/${store}/live-sessions/${comments.session}/claims/blocklist/check`?"restriction":
        path.endsWith("/comments")?"comments":path.endsWith("/inbox/conversations")?"conversations":
        path.endsWith("/inbox/buyer-panel")?"buyer-panel":path.endsWith("/message-templates")?"templates":null;
      if(category)scoped.push({path,category,status:r.status(),method:r.request().method()});
    });
    page.on("response",async r=>{
      if(!new URL(r.url()).pathname.endsWith("/comments"))return;
      let value:any;try{value=await r.json();}catch{value={};}
      rows.push({status:r.status(),count:Array.isArray(value.items)?value.items.length:null,epoch:typeof value.epoch==="number"?value.epoch:null,reset:value.reset===true,
        code:["not_found","no_source","forbidden","stream_unavailable","invalid_request","invalid_cursor"].includes(value.code)?value.code:null});
    });
  });
  test.afterEach(async({page},info)=>{
    const id=info.testId.replace(/[^a-zA-Z0-9_-]/g,"");
    await writeFile(`${evidence}/comment-reads-${id}.json`,JSON.stringify(reads.get(page)??[]));
    await writeFile(`${evidence}/scope-reads-${id}.json`,JSON.stringify(scopeReads.get(page)??[]));
  });
  for(const [li,locale] of locales.entries())for(const [wi,size] of streamSizes.entries()) {
    test(`pagination filters buyer and one-shot replies ${locale}-${size.width}`,async({page,request})=>{
      const c=commentCopy(locale);await page.setViewportSize(size);await login(page);await page.goto(route(locale,comments.session));
      await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(50);
      const before=(await facts(request)).comments;
      await livePanel(page, "comments");await page.getByTestId("comment-older").click();
      await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(60);
      expect((await facts(request)).comments.older_requests).toBe(before.older_requests+1);
      await livePanel(page, "comments");await page.getByTestId("comment-filter-keyword").click();await expect(page.getByTestId("comment-rows").locator("li")).toHaveCount(1);
      await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.claim_ref}`).click();
      await buyerDetails(page);await expect(page.getByTestId("buyer-panel")).toBeVisible();await expect(page.getByTestId("buyer-panel")).toContainText("A1");
      await livePanel(page, "comments");await page.getByTestId("comment-filter-private").click();await expect(page.getByTestId("comment-conversations").locator("li")).toHaveCount(1);
      await livePanel(page, "comments");await page.getByTestId("comment-filter-unreplied").click();await expect(page.getByTestId("comment-conversations").locator("li")).toHaveCount(1);
      await livePanel(page, "comments");await page.getByTestId("comment-filter-all").click();
      await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.reply_refs[li*2+wi]}`).click();
      await expect(page.getByTestId("comment-reply")).toContainText(c.one);await expect(page.getByTestId("comment-reply")).toContainText(c.window);
      await livePanel(page, "buyer");await page.getByTestId("comment-reply-text").fill(`Synthetic thanks ${li}-${wi}`);
      const sent=page.waitForResponse(r=>r.url().endsWith("/private-reply")&&r.request().method()==="POST",{timeout:15000});
      await livePanel(page, "buyer");await page.getByTestId("comment-send").click();expect((await sent).status()).toBe(200);
      await expect(page.getByTestId("comment-rule")).toHaveText(c.used);
      await expect(page.getByTestId("comment-send")).toBeDisabled();
      expect((await facts(request)).comments.private_operations).toBe(before.private_operations+1);
      // Explicit merchant verification unlocks only future sends; queued is not a final delivery fact.
      await livePanel(page, "buyer");if(await page.getByTestId("comment-verified").isVisible()) {
        page.once("dialog",dialog=>dialog.accept());await livePanel(page, "buyer");await page.getByTestId("comment-verified").click();
        expect((await facts(request)).comments.private_operations).toBe(before.private_operations+1);
      }
      await livePanel(page, "buyer");await page.getByTestId("comment-reply").getByRole("button",{name:c.publicReply,exact:true}).click();
      await expect(page.getByTestId("comment-reply")).toContainText(c.publicRule);
      await livePanel(page, "buyer");await page.getByTestId("comment-reply-text").fill("https://checkout.stripe.com/pay/synthetic");
      const denied=page.waitForResponse(r=>r.url().endsWith("/public-reply")&&r.request().method()==="POST",{timeout:15000});
      await livePanel(page, "buyer");await page.getByTestId("comment-send").click();expect((await denied).status()).toBe(422);
      await expect(page.getByTestId("comment-reply-error")).toHaveText(c.public_reply_forbidden_content);
      expect((await facts(request)).comments.private_operations).toBe(before.private_operations+1);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
      // Mask the bounded scrolling region: masking offscreen children projects rectangles outside their clipped ancestor.
      await page.screenshot({path:`${evidence}/comments-${locale}-${size.width}.png`,fullPage:true,animations:"disabled",mask:commentScreenshotMaskSelectors.map(selector=>page.locator(selector))});
    });
  }
  test("native hidden tab stops A2 and clears private selections",async({request})=>{
    const {page,close}=await nativePage(evidence,"console-native-");
    try {
    // READ/MEASURE only: preserve native fetch arguments/results and count starts synchronously
    // inside the document, so an aborted pre-hide read cannot masquerade as a hidden poll.
    await page.addInitScript(()=>{
      const original=window.fetch.bind(window);
      const counts={visible:0,hidden:0,visibility:[] as {state:string;trusted:boolean}[]};
      (window as unknown as {a2Starts:typeof counts}).a2Starts=counts;
      document.addEventListener("visibilitychange",event=>counts.visibility.push({state:document.visibilityState,trusted:event.isTrusted}));
      window.fetch=(...args:Parameters<typeof fetch>)=>{
        const input=args[0],url=new URL(input instanceof Request?input.url:String(input),location.href);
        if(url.pathname.endsWith("/comments"))counts[document.visibilityState==="hidden"?"hidden":"visible"]++;
        return original(...args);
      };
    });
    await login(page);await page.goto(route("en",comments.session));await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.claim_ref}`).click();await buyerDetails(page);await expect(page.getByTestId("buyer-panel")).toBeVisible();
    // A visible DOM may retain the last result while a poll is in flight. Establish actual
    // network readiness before hiding, rather than comparing a server-arrival counter mid-read.
    await page.waitForLoadState("networkidle",{timeout:10000});
    // Match the passing INU05 native fixture: a real app cover remains the active tab.
    const cover=await page.context().newPage();await cover.goto(`${origin}/en/settings`);await cover.bringToFront();
    await expect.poll(()=>page.evaluate(()=>document.visibilityState)).toBe("hidden");
    await expect(page.getByTestId("comment-rows")).toHaveCount(0);await expect(page.getByTestId("buyer-panel")).toHaveCount(0);
    const visibilityStart=await page.evaluate(()=>(window as unknown as {a2Starts:{visibility:unknown[]}}).a2Starts.visibility.length);
    const before=(await facts(request)).comments.requests;
    await cover.waitForTimeout(6500); // two real 3s intervals; absence of requests is the assertion.
    const after=(await facts(request)).comments.requests;
    const starts=await page.evaluate(()=>(window as unknown as {a2Starts:{visible:number;hidden:number;visibility:{state:string;trusted:boolean}[]}}).a2Starts);
    await writeFile(`${evidence}/native-hidden-read-counts.json`,JSON.stringify({class:"REAL_PG",...starts,serverReadsBefore:before,serverReadsAfter:after,documentVisibility:await page.evaluate(()=>document.visibilityState)}));
    expect(await page.evaluate(()=>document.visibilityState)).toBe("hidden");
    expect(starts.visibility.slice(visibilityStart).every(event=>event.state==="hidden"&&event.trusted)).toBe(true);
    expect(after).toBe(before);
    expect(starts.visible).toBeGreaterThan(0);expect(starts.hidden).toBe(0);
    await page.bringToFront();await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();await cover.close();
    } finally { await close(); }
  });
  test("queued public reply stays fenced after worker UNKNOWN selection reload and dedupe expiry",async({page,request})=>{
    await login(page);await page.goto(route("en",comments.session));
    const ref=comments.reply_refs[1];
    await livePanel(page, "comments");await page.getByTestId(`comment-select-${ref}`).click();await page.getByRole("button",{name:"Public reply",exact:true}).click();
    await livePanel(page, "buyer");await page.getByTestId("comment-reply-text").fill("Synthetic queued public case");
    const before=(await facts(request)).comments;
    await fault(request,comments.session,"public_graph_unknown");
    try {
      const ack=page.waitForResponse(r=>r.url().endsWith(`/comments/${ref}/public-reply`)&&r.request().method()==="POST",{timeout:15000});
      await livePanel(page, "buyer");await page.getByTestId("comment-send").click();expect((await (await ack).json()).send_state).toBe("queued");
      await expect.poll(async()=>(await facts(request)).comments.public_unknown,{timeout:15000}).toBe(before.public_unknown+1);
      await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.reply_refs[2]}`).click();
      await page.getByRole("button",{name:"Public reply",exact:true}).click();
      await livePanel(page, "buyer");await page.getByTestId("comment-reply-text").fill("Synthetic other comment");await expect(page.getByTestId("comment-send")).toBeEnabled();
      await livePanel(page, "comments");await page.getByTestId(`comment-select-${ref}`).click();await expect(page.getByTestId("comment-send")).toBeDisabled();
      await page.reload();await livePanel(page, "comments");await page.getByTestId(`comment-select-${ref}`).click();
      await page.waitForTimeout(31000); // REAL backend dedupe window elapsed; safety must not be a 30s timer.
      await expect(page.getByTestId("comment-send")).toBeDisabled();await expect(page.getByTestId("comment-verified")).toBeVisible();
      expect((await facts(request)).comments.public_requests).toBe(before.public_requests+1);
      page.once("dialog",d=>d.accept());await livePanel(page, "buyer");await page.getByTestId("comment-verified").click();
      await page.getByRole("button",{name:"Public reply",exact:true}).click();
      await livePanel(page, "buyer");await page.getByTestId("comment-reply-text").fill("Synthetic verified draft");await expect(page.getByTestId("comment-send")).toBeEnabled();
      expect((await facts(request)).comments.public_requests).toBe(before.public_requests+1);
    } finally { await fault(request,comments.session,"public_graph_restore"); }
  });
  test("unknown public ACK stays fenced across selection and reload",async({page,request})=>{
    await login(page);await page.goto(route("en",comments.session));await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.latest_ref}`).click();await page.getByRole("button",{name:"Public reply",exact:true}).click();
    await livePanel(page, "buyer");await page.getByTestId("comment-reply-text").fill("Synthetic public ACK case");
    // FAULT INJECTION: the command reaches real Go/PG; only its acknowledgement is dropped.
    await page.route(`**/comments/${comments.latest_ref}/public-reply`,async r=>{const real=await r.fetch();expect(real.status()).toBe(200);await r.abort("connectionreset");},{times:1});
    const before=(await facts(request)).comments.public_requests;
    await livePanel(page, "buyer");await page.getByTestId("comment-send").click();await expect(page.getByTestId("comment-unknown")).toBeVisible();
    await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.reply_refs[0]}`).click();await expect(page.getByTestId("comment-send")).toBeDisabled();
    await page.reload();await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.latest_ref}`).click();await expect(page.getByTestId("comment-send")).toBeDisabled();
    expect((await facts(request)).comments.public_requests).toBe(before+1);
    const flags=await page.evaluate(()=>Object.entries(sessionStorage).filter(([key])=>key.startsWith("live-comment-unresolved:")));
    expect(flags).toHaveLength(1);expect(flags[0][1]).toBe("1");expect(flags[0][0]).not.toContain(comments.latest_ref);
  });
  for(const view of ["all","private"] as const)test(`LCU2_404 real store grant revoked clears ${view} view and selected buyer`,async({page,request})=>{
    await login(page);await page.goto(route("en",comments.session));
    await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    if(view==="private") {
      await livePanel(page, "comments");await page.getByTestId("comment-filter-private").click();
      await page.getByTestId("comment-conversations").getByRole("button").first().click();
    } else { await livePanel(page, "comments");await page.getByTestId(`comment-select-${comments.claim_ref}`).click(); }
    await buyerDetails(page);await expect(page.getByTestId("buyer-panel")).toBeVisible();
    const beforeSession=(await authorityCookies(page.context())).find(c=>c.name==="__Host-commerce_session")?.value;
    let reads=0;
    page.on("request",r=>{const path=new URL(r.url()).pathname;if(path.endsWith("/comments")||path.endsWith("/inbox/conversations"))reads++;});
    try {
      const denied=page.waitForResponse(r=>{
        const path=new URL(r.url()).pathname;
        // Any scoped read may observe the loss first, including the selected buyer's exact GET restriction check;
        // the assertions below still require every private view cleared, the login kept and zero reads afterwards.
        return r.status()===404&&(path.endsWith("/comments")||path.endsWith("/inbox/conversations")||path.endsWith("/inbox/buyer-panel")||path.endsWith("/message-templates")||
          (r.request().method()==="GET"&&path===`/api/stores/${store}/live-sessions/${comments.session}/claims/blocklist/check`));
      },{timeout:15000});
      // FIXTURE/SETUP: only this synthetic store/principal grant is deleted in PG; no response interception.
      await fault(request,comments.session,"grant_revoke");await denied;
      await expect(page.getByTestId("comment-refresh")).toBeDisabled();
      await expect(page.getByTestId("comment-rows")).toHaveCount(0);
      await expect(page.getByTestId("comment-conversations")).toHaveCount(0);
      await expect(page.getByTestId("buyer-panel")).toHaveCount(0);
      await expect(page.locator("[data-private=comment-author],[data-private=comment-text]")).toHaveCount(0);
      const afterSession=(await authorityCookies(page.context())).find(c=>c.name==="__Host-commerce_session")?.value;
      expect(!!beforeSession&&afterSession===beforeSession,"scope loss must be exercised with a still-valid login").toBe(true);
      await fault(request,comments.session,"grant_restore");
      const stopped=reads;
      await page.waitForTimeout(6500); // Two healthy A2 intervals: assert absence, not a readiness sleep.
      expect(reads).toBe(stopped);await expect(page.getByTestId("comment-refresh")).toBeDisabled();
      await writeFile(`${evidence}/grant-revocation-${view}.json`,JSON.stringify({class:"REAL_PG",view,status:404,privateRows:0,buyerPanel:0,sessionRetained:true,postLossReads:reads-stopped}));
      await page.reload();await expect(page.getByTestId(`comment-row-${comments.latest_ref}`)).toBeVisible();
    } finally { await fault(request,comments.session,"grant_restore"); }
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
    // CONSOLE-FIX: choosing the mobile task is allowed; opening Advanced must never be
    // required to discover primary seller actions. Keep the existing helper calls below.
    await livePanel(page, "live");
    await expect(page.getByTestId("live-primary-action")).toBeVisible();
    await expect(page.getByTestId("live-console").getByRole("switch")).toBeVisible();
    await expect(page.getByTestId("live-console").locator('input[data-testid^="live-stock-"]')).toBeVisible();
    await phase(page, "draft");
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    const stats = page.getByTestId("live-console").locator(".live-status-bar, .live-revenue-stats");
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
    await showLiveTools(page); // refresh remounts the signed disclosures closed; reopen with real clicks
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(true);

    await showLiveTools(page);
    await page.getByTestId(`live-stock-${offer}`).fill("17");
    await page.getByTestId(`live-stock-save-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Stock).toBe(17);
    const stockReceipt = (await facts(request)).receipts.at(-1)!;
    expect(stockReceipt.input).toMatchObject({ delta: 5, expected_version: 1, reason: "live_console_edit" });
    await page.reload();
    await phase(page, "live");
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
    await phase(page, "live");
    await expect(stockInput).toHaveValue("23");
    const beforeCancel = (await facts(request)).receipts.length;
    await stockInput.fill("24");
    await page.getByTestId(`live-offer-${offer}`).getByRole("button", { name: workspaceCopy[locale].cancel, exact: true }).click();
    await expect(stockInput).toHaveValue("23");
    expect((await facts(request)).receipts).toHaveLength(beforeCancel);
    await showLiveTools(page);
    await page.getByTestId(`live-recommend-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Recommended).toEqual({ offer_id: offer, at: "2030-01-01T00:00:00Z" });
    expect((await facts(request)).receipts.at(-1)!.input.post_comment).toBe(false);

    // Server-side CAS bump, then a real click must reconcile before a fresh command.
    await fault(request, scene, "conflict");
    const conflictCount = (await facts(request)).receipts.length;
    await showLiveTools(page);
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).receipts.length).toBe(conflictCount + 1);
    expect((await facts(request)).receipts.at(-1)!.status).toBe(409);
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    await page.getByTestId("live-console-refresh").click();
    await showLiveTools(page);
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(false);
    await page.getByTestId("live-console-refresh").click();
    // The full reload guarantees a fresh subtree with native disclosure defaults.
    // Its saved stock and offer state must remain visible before the disclosure helper.
    await page.reload();
    await livePanel(page, "live");
    await expect(page.getByTestId(`live-stock-${offer}`)).toBeVisible();
    await expect(page.getByTestId(`live-stock-${offer}`)).toHaveValue("23");
    await expect(page.getByTestId(`live-offer-toggle-${offer}`)).toBeVisible();
    await expect(page.getByTestId(`live-offer-toggle-${offer}`)).not.toBeChecked();
    await expect(page.getByTestId("live-primary-action")).toBeVisible();
    await showLiveTools(page);

    // I06: lose ACK after a committed MOCK effect. Polling is read-only; explicit retry must reuse the key.
    await fault(request, scene, "unknown");
    const beforeUnknown = (await facts(request)).receipts.length;
    await showLiveTools(page);
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
    await showLiveTools(page);
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
    await phase(page, "draft");
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
      await phase(page, "draft");
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
      await phase(page, "draft");
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

// PARITY-LIVE: v3 is the current DELIVERY section; its 37 predecessor roots are explicitly superseded.
// All responses below are MOCK fixtures for EXISTING DTOs. Authentication stays real. No provider send escapes.
// Copy assertions deliberately do not import production translations: a wrong translation must fail this gate.
const parityCopy = {
  "zh-TW": { title: "直播控台", session: "秋日包款直播", stream: "留言與私訊", live: "當場直播", products: "本場商品", product: "焦糖托特包", buyer: "買家", private: "私訊", public: "公開回覆", send: "送出私訊", publicSend: "送出公開回覆", reply: "回覆內容", all: "全部", keyword: "關鍵字", unreplied: "待回覆", liveTab: "直播", commentsTab: "留言", buyerTab: "買家", checkout: "結帳連結", stock: "庫存說明", pause: "暫停接單", resume: "繼續接單", off: "下架商品", on: "重新上架", pending: "確認送出狀態中（不會重複傳）", sent: "私訊已送出", blocked: "這則留言已私訊過一次，買家回覆後 24 小時內可再聊。", wait: "等待買家回覆", waitPending: "正在確認，請稍候", sold: "這款已賣完，我再幫你確認補貨時間。", question: "請問米白色還有嗎？", registered: "已登記（還沒付款）", restricted: "受限", expired: "窗口結束" },
  "zh-CN": { title: "直播控制台", session: "秋季包款直播", stream: "评论与私信", live: "本场直播", products: "本场商品", product: "焦糖托特包", buyer: "买家", private: "私信", public: "公开回复", send: "发送私信", publicSend: "发送回复", reply: "回复内容", all: "全部", keyword: "关键词", unreplied: "待回复", liveTab: "直播", commentsTab: "评论", buyerTab: "买家", checkout: "结账链接", stock: "库存说明", pause: "暂停接单", resume: "继续接单", off: "下架商品", on: "重新上架", pending: "正在确认发送状态（不会重复发送）", sent: "私信已发送", blocked: "这条评论已私信过一次，买家回复后 24 小时内可以继续沟通。", wait: "等待买家回复", waitPending: "正在确认，请稍候", sold: "这款已售罄，我再帮你确认补货时间。", question: "请问米白色还有吗？", registered: "已登记（尚未付款）", restricted: "受限", expired: "回复窗口已结束" },
  en: { title: "Live console", session: "Autumn bags live", stream: "Comments and messages", live: "Current broadcast", products: "Session products", product: "Caramel tote bag", buyer: "Buyer", private: "Private message", public: "Public reply", send: "Send private message", publicSend: "Send reply", reply: "Reply text", all: "All", keyword: "Keywords", unreplied: "Awaiting reply", liveTab: "Live", commentsTab: "Comments", buyerTab: "Buyer", checkout: "Checkout link", stock: "Stock details", pause: "Pause orders", resume: "Resume orders", off: "Unlist product", on: "List product", pending: "Confirming delivery (will not send again)", sent: "Private message sent", blocked: "This comment already received a private reply. Continue within 24 hours after the buyer replies.", wait: "Waiting for the buyer to reply", waitPending: "Confirming, please wait", sold: "This item is sold out. I will check when it will be back in stock.", question: "Is the ivory colour still available?", registered: "Registered (not paid yet)", restricted: "Restricted", expired: "Reply window ended" },
} as const;
type ParityLocale = keyof typeof parityCopy;
// Exact current v3 keys/IDs from ux-live/output/ux-live/v3/frames.json, not the historical audit.
const parityFrames: readonly (readonly [string, string])[] = [
  ["1429:39502", "D|selected|buyer|private|A|全部"],
  ["1429:39607", "M|selected|buyer|private|A|全部"],
  ["1429:39683", "D|normal|buyer|private|A|全部"],
  ["1429:39782", "M|normal|live|private|A|全部"],
  ["1429:39834", "M|normal|comments|private|A|全部"],
  ["1429:39911", "M|normal|buyer|private|A|全部"],
  ["1429:39941", "M|selected|live|private|A|全部"],
  ["1429:39993", "M|selected|comments|private|A|全部"],
  ["1429:40070", "D|sent|buyer|private|A|全部"],
  ["1429:40177", "M|sent|live|private|A|全部"],
  ["1429:40230", "M|sent|comments|private|A|全部"],
  ["1429:40308", "M|sent|buyer|private|A|全部"],
  ["1429:40347", "D|blocked|buyer|private|A|全部"],
  ["1429:40452", "M|blocked|live|private|A|全部"],
  ["1429:41159", "M|blocked|comments|private|A|全部"],
  ["1429:41236", "M|blocked|buyer|private|A|全部"],
  ["1429:41273", "D|pending|buyer|private|A|全部"],
  ["1429:41380", "M|pending|live|private|A|全部"],
  ["1429:41433", "M|pending|comments|private|A|全部"],
  ["1429:41511", "M|pending|buyer|private|A|全部"],
  ["1429:41550", "D|sold|buyer|private|A|全部"],
  ["1429:41653", "M|sold|live|private|A|全部"],
  ["1429:41703", "M|sold|comments|private|A|全部"],
  ["1429:41780", "M|sold|buyer|private|A|全部"],
  ["1429:41817", "D|paused|buyer|private|A|全部"],
  ["1429:41922", "M|paused|live|private|A|全部"],
  ["1429:41974", "M|paused|buyer|private|A|全部"],
  ["1429:42011", "D|off|buyer|private|A|全部"],
  ["1429:42116", "M|off|live|private|A|全部"],
  ["1429:42168", "M|off|buyer|private|A|全部"],
  ["1429:42205", "D|publicSent|buyer|private|A|全部"],
  ["1429:42310", "M|publicSent|live|private|A|全部"],
  ["1429:42362", "M|publicSent|buyer|private|A|全部"],
  ["1429:42399", "D|selected|buyer|private|B|全部"],
  ["1429:42507", "D|selected|buyer|public|A|全部"],
  ["1429:42612", "D|selected|buyer|private|A|關鍵字"],
  ["1429:42690", "D|selected|buyer|private|A|私訊"],
  ["1429:42768", "D|selected|buyer|private|A|待回覆"],
  ["1429:42846", "M|selected|buyer|private|B|全部"],
  ["1429:42886", "M|selected|buyer|public|A|全部"],
  ["1429:42923", "M|selected|comments|private|A|關鍵字"],
  ["1429:42973", "M|selected|comments|private|A|私訊"],
  ["1429:43023", "M|selected|comments|private|A|待回覆"],
  ["1429:43073", "M|selected|live|private|B|全部"],
  ["1429:43125", "M|selected|comments|private|B|全部"],
  ["1429:43202", "D|selected|buyer|public|B|全部"],
  ["1429:43310", "M|selected|live|public|B|全部"],
  ["1429:43362", "M|selected|comments|public|B|全部"],
  ["1429:43439", "M|selected|buyer|public|B|全部"],
  ["1429:43479", "D|sent|buyer|private|B|全部"],
  ["1429:43589", "M|sent|live|private|B|全部"],
  ["1429:43642", "M|sent|comments|private|B|全部"],
  ["1429:43720", "M|sent|buyer|private|B|全部"],
  ["1429:43762", "M|selected|live|public|A|全部"],
  ["1429:43814", "M|selected|comments|public|A|全部"],
  ["1429:43905", "M|paused|comments|private|A|全部"],
  ["1429:43982", "M|off|comments|private|A|全部"],
  ["1429:44059", "M|publicSent|comments|private|A|全部"],
  ["1429:51651", "D|publicSent|buyer|public|B|全部"],
  ["1429:51759", "M|publicSent|buyer|public|B|全部"],
  ["1429:54848", "M|publicSent|live|public|B|全部"],
  ["1429:54900", "M|publicSent|comments|public|B|全部"],
  ["1429:45746", "D|normal|buyer|private|A|關鍵字"],
  ["1429:45845", "D|normal|buyer|private|A|私訊"],
  ["1429:45944", "D|normal|buyer|private|A|待回覆"],
  ["1429:46043", "M|normal|comments|private|A|關鍵字"],
  ["1429:46120", "M|normal|comments|private|A|私訊"],
  ["1429:46197", "M|normal|comments|private|A|待回覆"],
  ["1429:46274", "D|sent|buyer|private|A|關鍵字"],
  ["1429:46381", "D|sent|buyer|private|A|私訊"],
  ["1429:46488", "D|sent|buyer|private|A|待回覆"],
  ["1429:46595", "M|sent|comments|private|A|關鍵字"],
  ["1429:46673", "M|sent|comments|private|A|私訊"],
  ["1429:46751", "M|sent|comments|private|A|待回覆"],
  ["1429:46829", "D|blocked|buyer|private|A|關鍵字"],
  ["1429:46934", "D|blocked|buyer|private|A|私訊"],
  ["1429:47039", "D|blocked|buyer|private|A|待回覆"],
  ["1429:47144", "M|blocked|comments|private|A|關鍵字"],
  ["1429:47221", "M|blocked|comments|private|A|私訊"],
  ["1429:47298", "M|blocked|comments|private|A|待回覆"],
  ["1429:47375", "D|pending|buyer|private|A|關鍵字"],
  ["1429:47482", "D|pending|buyer|private|A|私訊"],
  ["1429:47589", "D|pending|buyer|private|A|待回覆"],
  ["1429:47696", "M|pending|comments|private|A|關鍵字"],
  ["1429:47774", "M|pending|comments|private|A|私訊"],
  ["1429:47852", "M|pending|comments|private|A|待回覆"],
  ["1429:47930", "D|sold|buyer|private|A|關鍵字"],
  ["1429:48033", "D|sold|buyer|private|A|私訊"],
  ["1429:48136", "D|sold|buyer|private|A|待回覆"],
  ["1429:48239", "D|sold|buyer|public|A|全部"],
  ["1429:48342", "M|sold|comments|private|A|關鍵字"],
  ["1429:48419", "M|sold|comments|private|A|私訊"],
  ["1429:48496", "M|sold|comments|private|A|待回覆"],
  ["1429:48573", "M|sold|buyer|public|A|全部"],
  ["1429:48610", "D|paused|buyer|private|A|關鍵字"],
  ["1429:48715", "D|paused|buyer|private|A|私訊"],
  ["1429:48820", "D|paused|buyer|private|A|待回覆"],
  ["1429:48925", "D|paused|buyer|public|A|全部"],
  ["1429:49030", "M|paused|buyer|public|A|全部"],
  ["1429:49067", "D|off|buyer|private|A|關鍵字"],
  ["1429:49172", "D|off|buyer|private|A|私訊"],
  ["1429:49277", "D|off|buyer|private|A|待回覆"],
  ["1429:49382", "D|off|buyer|public|A|全部"],
  ["1429:49487", "M|off|buyer|public|A|全部"],
  ["1429:49524", "D|publicSent|buyer|private|A|關鍵字"],
  ["1429:49629", "D|publicSent|buyer|private|A|私訊"],
  ["1429:49734", "D|publicSent|buyer|private|A|待回覆"],
  ["1429:49839", "D|selected|buyer|private|B|關鍵字"],
  ["1429:49947", "D|selected|buyer|private|B|私訊"],
  ["1429:50055", "D|selected|buyer|private|B|待回覆"],
  ["1429:50163", "M|selected|comments|private|B|關鍵字"],
  ["1429:50240", "M|selected|comments|private|B|私訊"],
  ["1429:50317", "M|selected|comments|private|B|待回覆"],
  ["1429:50394", "D|sent|buyer|private|B|關鍵字"],
  ["1429:50504", "D|sent|buyer|private|B|私訊"],
  ["1429:50614", "D|sent|buyer|private|B|待回覆"],
  ["1429:50724", "M|sent|comments|private|B|關鍵字"],
  ["1429:50802", "M|sent|comments|private|B|私訊"],
  ["1429:50880", "M|sent|comments|private|B|待回覆"],
  ["1429:50958", "M|paused|comments|private|A|關鍵字"],
  ["1429:51035", "M|paused|comments|private|A|私訊"],
  ["1429:51112", "M|paused|comments|private|A|待回覆"],
  ["1429:51189", "M|off|comments|private|A|關鍵字"],
  ["1429:51266", "M|off|comments|private|A|私訊"],
  ["1429:51343", "M|off|comments|private|A|待回覆"],
  ["1429:51420", "M|publicSent|comments|private|A|關鍵字"],
  ["1429:51497", "M|publicSent|comments|private|A|私訊"],
  ["1429:51574", "M|publicSent|comments|private|A|待回覆"],
  ["1429:51802", "D|sent|buyer|private|A|全部|paused"],
  ["1429:51909", "D|sent|buyer|private|A|全部|off"],
  ["1429:52016", "M|sent|live|private|A|全部|paused"],
  ["1429:52069", "M|sent|live|private|A|全部|off"],
  ["1429:52122", "D|blocked|buyer|private|A|全部|paused"],
  ["1429:52227", "D|blocked|buyer|private|A|全部|off"],
  ["1429:52332", "M|blocked|live|private|A|全部|paused"],
  ["1429:52384", "M|blocked|live|private|A|全部|off"],
  ["1429:52436", "D|pending|buyer|private|A|全部|paused"],
  ["1429:52543", "D|pending|buyer|private|A|全部|off"],
  ["1429:52650", "M|pending|live|private|A|全部|paused"],
  ["1429:52703", "M|pending|live|private|A|全部|off"],
  ["1429:52756", "D|sent|buyer|private|B|全部|paused"],
  ["1429:52866", "D|sent|buyer|private|B|全部|off"],
  ["1429:52976", "M|sent|live|private|B|全部|paused"],
  ["1429:53029", "M|sent|live|private|B|全部|off"],
  ["1429:60322", "D|sent|buyer|private|A|全部|sold"],
  ["1429:60430", "M|sent|live|private|A|全部|sold"],
  ["1429:60484", "M|sent|comments|private|A|全部|sold"],
  ["1429:60563", "M|sent|buyer|private|A|全部|sold"],
  ["1429:60603", "D|publicSent|buyer|private|A|全部|sold"],
  ["1429:60709", "M|publicSent|buyer|private|A|全部|sold"],
  ["1429:60747", "M|publicSent|live|private|A|全部|sold"],
  ["1429:60800", "M|publicSent|comments|private|A|全部|sold"],
];
const parityIDs = {
  offer: "11111111-1111-4111-8111-111111111111",
  sku: "22222222-2222-4222-8222-222222222222",
  warehouse: "33333333-3333-4333-8333-333333333333",
  operation: "44444444-4444-4444-8444-444444444444",
  outbound: "55555555-5555-4555-8555-555555555555",
};

/** MOCK only: fixtures model upstream observations; clicks never change DOM or bypass the real route. */
async function parityFixture(page: Page, locale: ParityLocale, state = "selected", buyer = "A", context = "", dispatchFromClick = false, loseWindowAck = false) {
  const c = parityCopy[locale], at = "2026-10-30T00:00:00Z";
  let receipt = dispatchFromClick ? "" : state === "pending" ? "UNKNOWN" : state === "sent" ? "SUCCEEDED" : state === "blocked" ? "BLOCKED_POLICY" : "";
  let publicReplies = !dispatchFromClick && state === "publicSent" ? 1 : 0;
  let replyPosts = 0, paused = state === "paused" || context === "paused", off = state === "off" || context === "off";
  const sold = state === "sold" || context === "sold";
  const windowRequests: Readonly<{ method: string; path: string; body: string; key: string }>[] = [];
  const windowReceipts = new Map<string, ReturnType<typeof window>>();
  let windowEffects = 0, consoleReads = 0, releaseWindowReceipt!: () => void;
  const windowAck = loseWindowAck ? new Promise<void>(resolve => { releaseWindowReceipt = resolve; }) : Promise.resolve();
  const window = () => ({ session_id: comments.session, state: paused ? "CLOSED" : "OPEN", generation: 1, version: 1 + windowEffects, match_mode: "KEYWORD_QTY_ONLY", opened_at: at, closed_at: paused ? at : null });
  const offer = (): Offer => ({ offer_id: parityIDs.offer, session_id: comments.session, keyword: "101", sku_id: parityIDs.sku, sku_code: "SKU-1042", product_name: c.product, max_quantity_per_claim: 5, active: !off, version: 1, activated_at: at, updated_at: at, sku_price_minor: 10000, currency: "TWD", live_price_minor: null });
  const snapshot = (): ConsoleSnapshot => ({
    session: { id: comments.session, title: c.session, lifecycle: "live", version: 1, started_at: at, ended_at: null },
    window: { state: paused ? "CLOSED" : "OPEN", generation: 1, opened_at: at, match_mode: "KEYWORD_QTY_ONLY" },
    stats: { comments: { total: 128, source: "graph_summary" }, keyword_comments: 36, buyers: 2, orders: { count: 12, amount_minor: 120000 }, paid: { count: 8, amount_minor: 80000 }, currency: "TWD", as_of: at },
    offers: [{ offer_id: parityIDs.offer, keyword: "101", sku_id: parityIDs.sku, product_name: c.product, variant_label: "SKU-1042", active: !off, version: 1, live_price_minor: null, sku_price_minor: 10000, stock: { tracked: true, sellable: sold ? 0 : 18, reserved: 2, warehouse_id: parityIDs.warehouse, balance_version: 1 }, claimed: { buyers: 1, quantity: 2 }, ordered_qty: 20, paid_qty: 20, paid_amount_minor: 200000, sold_out: sold, low_stock: false }],
    capabilities: { facebook: Object.fromEntries(["read_comment", "private_reply", "dm_session", "reply_public"].map(name => [name, { state: state === "restricted" && name === "private_reply" ? "missing_permission" : "ok", reason: "", evidence: "MOCK", checked_at: at }])) },
    stream: { state: "live", poll_interval_ms: 5000, last_ok_at: at, lag_ms: 0, source_platform: "facebook", video_embeddable: false }, recommended: null,
  });
  const rows = (): StreamComment[] => ["A", "B"].map((target, index) => ({
    ref: index ? comments.reply_refs[0]! : comments.claim_ref, seq: index + 1, parent_ref: null, created_at: at,
    author_name: `${c.buyer} ${target}`, text: index ? c.question : "101+2", is_page: false, has_attachment: false,
    marks: {
      intake: null,
      claim: index ? null : { status: "ACCEPTED", reason: null, offer_id: parityIDs.offer, keyword: "101", quantity: 2, bundle_id: comments.bundle },
      private_reply: target === buyer && receipt ? { kind: "manual", state: receipt, blocked_reason: state === "blocked" ? "window_closed" : null } : null,
      printed: null, public_replies: target === buyer ? publicReplies : 0,
      private_reply_available: !(target === buyer && (receipt || state === "expired")),
      ...(target === buyer && state === "expired" ? { private_reply_unavailable_reason: "expired_7d" } : {}),
    },
  }));
  // SETUP: intercept only synthetic, server-shaped console responses on this authenticated MOCK route.
  // Existing REAL_PG safety tests above remain intact and exercise the unmocked implementation.
  await page.route(`**/api/stores/${store}/**`, async intercepted => {
    const request = intercepted.request(), path = new URL(request.url()).pathname;
    const base = `/api/stores/${store}/live-sessions/${comments.session}`;
    // The real BFF always answers private reads with `cache-control: private, no-store`; studio-client.json() rejects anything else.
    const json = (value: unknown) => intercepted.fulfill({ status: 200, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify(value) });
    // SETUP: this parity-only scene set matches the detail below; legacy LC-U1 scenes stay in their own fixture.
    if (path === `/api/stores/${store}/live-sessions` && request.method() === "GET") return json({
      items: [{ session_id: comments.session, program_id: parityIDs.sku, title: c.session, scheduled_at: null,
        aspect_ratio: "9:16", state: "LIVE", version: 1, created_at: at, updated_at: at }],
      next_cursor: "",
    } satisfies StudioPage);
    if (path === `${base}/console`) { consoleReads++; return json(snapshot()); }
    if (path === `${base}/claims` && request.method() === "GET") return json({ window: window(), offers: [offer()], stats: { generation: 1, accepted: 1, rejected: Object.fromEntries(["NO_MATCH", "UNKNOWN_KEYWORD", "OFFER_INACTIVE", "INVALID_QUANTITY", "QUANTITY_REQUIRED", "QUANTITY_OVER_MAX", "BUNDLE_LIMIT"].map(key => [key, 0])) } });
    if (path === `${base}/comments` && request.method() === "GET") return json({ epoch: 1, reset: false, scan_exhausted: false, items: rows(), next: { epoch: 1, seq: 2 }, older_cursor: null, stream: snapshot().stream });
    if (path === `/api/stores/${store}/inbox/buyer-panel`) {
      const data: BuyerData = { display_name: `${c.buyer} A`, platform: "facebook", purchase_ordinal: 0, claims: [{ session_id: comments.session, offer_id: parityIDs.offer, keyword: "101", quantity: 2 }], claim_total_minor: 20000, orders: [], linked_customer_id: null, window_open_until: null, link_pending_manual: false };
      return json(data);
    }
    if (path === `/api/stores/${store}/inbox/conversations`) return json({ items: [{ bundle_id: comments.bundle, session_id: comments.session, platform: "facebook", display_name: `${c.buyer} A`, last_at: at, unread: true, unreplied: true, mode: null, assignee: null, window_open_until: null, linked_customer_id: null }], next_cursor: "", unread_total: 1 });
    if (path === `${base}/claims/blocklist/check`) return json({ restricted: false });
    if (/\/comments\/[^/]+\/(private|public)-reply$/.test(path) && request.method() === "POST") {
      replyPosts++;
      const sendState = dispatchFromClick ? state === "pending" ? "unknown" : "sent" : "queued";
      if (path.endsWith("/private-reply")) receipt = sendState === "sent" ? "SUCCEEDED" : sendState === "unknown" ? "UNKNOWN" : "READY";
      else publicReplies++;
      return json({ send_state: sendState, operation_id: parityIDs.operation, outbound_id: parityIDs.outbound });
    }
    if (path === `${base}/claims/window` && request.method() === "POST") {
      windowRequests.push(Object.freeze({ method: request.method(), path, body: request.postData() ?? "", key: request.headers()["idempotency-key"] ?? "" }));
      if (loseWindowAck) {
        // MOCK lost ACK: a synthetic receipt commits before transport fails. Only this opt-in case waits on replay.
        const captured = windowRequests.at(-1)!;
        if (!windowReceipts.has(captured.key)) {
          paused = request.postDataJSON().state === "CLOSED"; windowEffects++;
          windowReceipts.set(captured.key, window());
        }
        if (windowRequests.length === 1) return intercepted.abort("failed");
        await windowAck;
        return json(windowReceipts.get(captured.key));
      }
      paused = request.postDataJSON().state === "CLOSED";
      return json(window());
    }
    if (path === `${base}/claims/offers/${parityIDs.offer}` && request.method() === "PATCH") {
      off = !request.postDataJSON().active;
      return json(offer());
    }
    return intercepted.continue();
  });
  return { posts: () => replyPosts, windowRequests, windowEffects: () => windowEffects, consoleReads: () => consoleReads, releaseWindowReceipt: () => releaseWindowReceipt?.() };
}

async function paritySelect(page: Page, locale: ParityLocale, buyer = "A") {
  await page.getByRole("button", { name: `${parityCopy[locale].buyer} ${buyer}`, exact: true }).click();
}
async function parityTab(page: Page, locale: ParityLocale, tab: string) {
  const c = parityCopy[locale];
  await page.getByRole("tab", { name: tab === "live" ? c.liveTab : tab === "comments" ? c.commentsTab : c.buyerTab, exact: true }).click();
}
async function parityShot(page: Page, id: string) {
  await mkdir(`${evidence}/parity`, { recursive: true });
  await page.screenshot({ path: `${evidence}/parity/${id.replace(":", "-")}.png`, animations: "disabled" });
}

/** READ ONLY: report counts for synthetic private text nodes, never return their text or capture an unmasked image. */
async function parityPrivateTextCounts(page: Page) {
  return page.evaluate(({ regions, tokens, masks }) => {
    const covered = Array.from(document.querySelectorAll(masks.join(",")));
    return regions.map(selector => {
      const root = document.querySelector(selector), counts = { names: 0, bodies: 0, visible: 0, unmarked: 0, unmasked: 0 };
      if (!root) return counts;
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        const name = node.textContent?.includes(tokens[0]!) === true, body = node.textContent?.includes(tokens[1]!) === true;
        if (!name && !body) continue;
        const range = document.createRange(); range.selectNodeContents(node);
        let visible = Array.from(range.getClientRects()).some(rect => rect.width > 0 && rect.height > 0);
        for (let element = node.parentElement; element; element = element.parentElement) {
          const style = getComputedStyle(element);
          if (style.display === "none" || style.visibility === "hidden" || style.opacity === "0") visible = false;
        }
        if (!visible) continue;
        if (name) counts.names++; if (body) counts.bodies++;
        counts.visible++;
        if (!node.parentElement?.closest("[data-private]")) counts.unmarked++;
        if (!covered.some(mask => mask.contains(node))) counts.unmasked++;
      }
      return counts;
    });
  }, { regions: [".live-buyer-context", '[data-testid="live-reply-summary"]', ".comment-reply-target"], tokens: [`${parityCopy.en.buyer} B`, parityCopy.en.question], masks: [...commentScreenshotMaskSelectors] });
}

// The legacy test block is intentionally untouched. Each current signed state is selected by an actual click.
// Exact IDs let the independent reviewer compare the real route's artifact to the v3 DELIVERY export.
test.describe("PARITY-LIVE current Figma route parity (MOCK)", () => {
  // Test-level sharding is confined to this isolated block; global workers and legacy ordering stay unchanged.
  test.describe.configure({ mode: "parallel" });
  test.afterEach(async ({}, info) => {
    await info.attach("click-ledger-outcome", { body: JSON.stringify({ page: "/[locale]/studio/console", test: info.title, expected: "signed copy, controls, buyer context and reply fencing", actual: info.status, result: info.status === "passed" ? "PASS in MOCK browser only" : "FAIL or NOT_RUN; inspect assertion failure", persistence: "explicit reload checks only; no buyer text stored" }), contentType: "application/json" });
  });
  test("selected buyer context summary and reply target private text has real screenshot coverage", async ({ page }) => {
    await parityFixture(page, "en"); await page.setViewportSize({ width: 1440, height: 1100 });
    await login(page); await page.goto(route("en", comments.session)); await paritySelect(page, "en", "B");
    for (const selector of [".live-buyer-context", '[data-testid="live-reply-summary"]', ".comment-reply-target"]) await expect(page.locator(selector)).toBeVisible();
    const regions = await parityPrivateTextCounts(page);
    expect(regions).toHaveLength(3);
    for (const [index, counts] of regions.entries()) {
      expect(counts.names, `region ${index} really contains the selected synthetic author`).toBeGreaterThan(0);
      if (index < 2) expect(counts.bodies, `region ${index} really contains the selected synthetic body`).toBeGreaterThan(0);
      expect(counts.visible, `region ${index} must render private text before checking protection`).toBeGreaterThan(0);
    }
    // Both are required: adding a marker alone cannot falsely certify an actual screenshot mask.
    expect(regions.map(counts => counts.unmasked), "actual screenshot masks must cover every visible private text node").toEqual([0, 0, 0]);
    expect(regions.map(counts => counts.unmarked), "private text nodes must have a private ancestor").toEqual([0, 0, 0]);
  });

  test("scoped 403 clears new buyer context summary and reply target text from the DOM", async ({ page }) => {
    await parityFixture(page, "en"); await page.setViewportSize({ width: 1440, height: 1100 });
    await login(page); await page.goto(route("en", comments.session)); await paritySelect(page, "en", "B");
    for (const selector of [".live-buyer-context", '[data-testid="live-reply-summary"]', ".comment-reply-target"]) await expect(page.locator(selector)).toBeVisible();
    const before = await parityPrivateTextCounts(page);
    for (const [index, counts] of before.entries()) {
      expect(counts.names, `region ${index} initially has selected author`).toBeGreaterThan(0);
      if (index < 2) expect(counts.bodies, `region ${index} initially has selected body`).toBeGreaterThan(0);
      expect(counts.visible, `region ${index} initially renders private text`).toBeGreaterThan(0);
    }
    const path = `/api/stores/${store}/live-sessions/${comments.session}/comments`;
    // MOCK boundary fault: the existing client receives its real scoped-403 response shape after selection.
    await page.route(`**${path}**`, intercepted => new URL(intercepted.request().url()).pathname === path && intercepted.request().method() === "GET"
      ? intercepted.fulfill({ status: 403, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify({ code: "forbidden" }) })
      : intercepted.fallback());
    const denied = page.waitForResponse(response => new URL(response.url()).pathname === path && response.status() === 403, { timeout: 15000 });
    await page.getByTestId("comment-refresh").click(); await denied;
    await expect(page.getByTestId("comment-refresh")).toBeDisabled();
    await expect.poll(async () => (await parityPrivateTextCounts(page)).map(counts => counts.names + counts.bodies)).toEqual([0, 0, 0]);
    // Read the whole document too: hidden/unmarked copies must disappear, not merely gain a mask.
    const remaining = () => page.evaluate(tokens => {
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT); let count = 0;
      for (let node = walker.nextNode(); node; node = walker.nextNode()) if (tokens.some(token => node.textContent?.includes(token))) count++;
      return count;
    }, [`${parityCopy.en.buyer} B`, parityCopy.en.question]);
    await expect.poll(remaining).toBe(0);
    await expect(page.getByTestId("comment-rows")).toHaveCount(0);
    await expect(page.getByTestId("live-reply-summary")).toHaveCount(0);
    await expect(page.locator(".comment-reply-target")).toHaveCount(0);
  });

  test("claim-window lost ACK reload requires explicit replay of the original command", async ({ page }) => {
    const locale = "en", c = parityCopy[locale], copy = workspaceCopy[locale];
    const mock = await parityFixture(page, locale, "selected", "A", "", false, true);
    const receiptKeys = [`live-workspace-command:${store}:${comments.session}`, `live-console-window:${store}:${comments.session}`];
    // READ ONLY: inspect command fences, never auth cookies or buyer data, without altering browser state.
    const journal = () => page.evaluate(keys => keys.map(key => sessionStorage.getItem(key)), receiptKeys);
    try {
      await login(page); await page.goto(route(locale, comments.session));
      await expect(page.getByRole("heading", { name: c.session, exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: c.pause, exact: true })).toBeEnabled();
      await expect(page.getByTestId("live-primary-action")).toBeVisible();
      await expect(page.getByTestId("live-primary-action")).toBeEnabled();
      await page.getByRole("button", { name: c.pause, exact: true }).click();
      await expect.poll(() => mock.windowRequests.length).toBe(1);
      const first = mock.windowRequests[0]!;
      expect(first.method).toBe("POST"); expect(first.path).toBe(`/api/stores/${store}/live-sessions/${comments.session}/claims/window`);
      expect(first.key).not.toBe(""); expect(JSON.parse(first.body)).toEqual({ expected_version: 1, state: "CLOSED", match_mode: "KEYWORD_QTY_ONLY" });
      await expect(page.getByTestId("live-console")).toContainText(copy.uncertain);
      await expect(page.getByRole("button", { name: c.resume, exact: true })).toBeDisabled();
      await expect(page.getByTestId("live-primary-action")).toBeDisabled();
      const pendingJournal = await journal(); expect(pendingJournal.some(Boolean)).toBe(true);
      // Observe another real A1 read: polling must not turn UNKNOWN into an automatic second POST or clear its fence.
      const before = mock.consoleReads(); await expect.poll(mock.consoleReads, { timeout: 8000 }).toBeGreaterThan(before);
      expect(mock.windowRequests).toHaveLength(1); expect(await journal()).toEqual(pendingJournal);
      page.on("dialog", dialog => dialog.accept());
      await page.reload();
      await expect(page.getByRole("button", { name: c.resume, exact: true })).toBeDisabled();
      await expect(page.getByTestId("live-primary-action")).toBeDisabled();
      await expect(page.getByTestId("live-console")).toContainText(copy.uncertain);
      const reloadedReads = mock.consoleReads(); await expect.poll(mock.consoleReads, { timeout: 8000 }).toBeGreaterThan(reloadedReads);
      expect(mock.windowRequests).toHaveLength(1); expect(await journal()).toEqual(pendingJournal);
      // Baseline 7be can fail first at the shared-write fence above; it also lacks this original-command recovery control.
      await expect(page.getByTestId("live-command-retry")).toBeVisible();
      await page.getByTestId("live-command-retry").click();
      await expect.poll(() => mock.windowRequests.length).toBe(2);
      expect(mock.windowRequests[1]).toEqual(first); expect(mock.windowEffects()).toBe(1);
      expect(await journal()).toEqual(pendingJournal); // Dispatch alone cannot clear an unconfirmed receipt.
      mock.releaseWindowReceipt();
      await expect(page.getByRole("button", { name: c.resume, exact: true })).toBeEnabled();
      await expect.poll(journal).toEqual([null, null]); expect(mock.windowRequests).toHaveLength(2); expect(mock.windowEffects()).toBe(1);
    } finally { mock.releaseWindowReceipt(); }
  });

  test("legacy claim-window bool stays UNKNOWN without deletion or a fresh-key retry", async ({ page }) => {
    const locale = "en", c = parityCopy[locale], mock = await parityFixture(page, locale);
    await login(page); await page.goto(route(locale, comments.session));
    await expect(page.getByRole("button", { name: c.pause, exact: true })).toBeEnabled();
    const legacyKey = `live-console-window:${store}:${comments.session}`;
    // FIXTURE SETUP: simulate an already persisted pre-journal marker; no DOM/auth mutation or storage clearing.
    await page.evaluate(key => sessionStorage.setItem(key, "1"), legacyKey);
    await page.reload();
    await expect(page.getByRole("button", { name: c.pause, exact: true })).toBeDisabled();
    await expect(page.getByTestId("live-console")).toContainText(workspaceCopy[locale].uncertain);
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    await page.getByTestId("live-console-refresh").click();
    const before = mock.consoleReads(); await expect.poll(mock.consoleReads, { timeout: 8000 }).toBeGreaterThan(before);
    expect(mock.windowRequests).toHaveLength(0);
    expect(await page.evaluate(key => sessionStorage.getItem(key), legacyKey)).toBe("1");
    await expect(page.getByRole("button", { name: c.pause, exact: true })).toBeDisabled();
  });

  for (const [id, key] of parityFrames) test(`frame ${id} ${key}`, async ({ page }, info) => {
    const [device, state, tab, channel, buyer, filter, context] = key.split("|");
    const locale = "zh-TW", c = parityCopy[locale];
    const dispatchFromClick = ["sent", "pending", "publicSent"].includes(state!);
    const mock = await parityFixture(page, locale, state, buyer, context, dispatchFromClick);
    await page.setViewportSize({ width: device === "D" ? 1440 : 390, height: device === "D" ? 900 : 844 });
    await login(page); await page.goto(route(locale, comments.session));
    // RED must fail here on the signed content, not during compilation, login or setup.
    if (device === "M") await parityTab(page, locale, "comments");
    await expect(page.getByRole("heading", { name: c.stream, exact: true })).toBeVisible();
    await expect(page.getByTestId("live-workspace")).not.toContainText("畫面顯示時每 5 秒更新");
    await expect(page.getByTestId("live-workspace")).not.toContainText(/\b(?:MOCK|UNKNOWN|queued)\b/);
    await expect(page.getByTestId("live-phase")).toHaveCount(0);
    if (state !== "normal") await paritySelect(page, locale, buyer);
    if (dispatchFromClick) {
      if (state === "publicSent") await page.getByTestId("comment-reply").getByRole("button", { name: c.public, exact: true }).click();
      await expect(page.getByRole("textbox", { name: c.reply, exact: true })).not.toHaveValue("");
      await page.getByRole("button", { name: state === "publicSent" ? c.publicSend : c.send, exact: true }).click();
      await expect.poll(mock.posts).toBe(1);
    }
    if (channel === "public") await page.getByTestId("comment-reply").getByRole("button", { name: c.public, exact: true }).click();
    if (filter !== "全部") {
      const control = page.getByTestId(`comment-filter-${filter === "關鍵字" ? "keyword" : filter === "私訊" ? "private" : "unreplied"}`);
      await expect(control).toHaveRole("button"); await expect(control).toHaveAccessibleName(filter!); await control.click();
    }
    if (device === "M") await parityTab(page, locale, tab!);
    const console = page.getByTestId("live-console");
    if (device === "D" || tab === "live") {
      await expect(console.getByRole("heading", { name: c.product, exact: true })).toBeVisible();
      await expect(console).toContainText("SKU-1042");
      if (state !== "sold" && context !== "sold") await expect(console.getByRole("button", { name: state === "off" || context === "off" ? c.on : c.off, exact: true })).toBeVisible();
    }
    if (state !== "normal") {
      const summary = page.getByTestId("live-reply-summary");
      await expect(summary).toBeVisible();
      await expect(summary).toContainText(`${c.buyer} ${buyer}`);
      await expect(summary).toContainText(buyer === "B" ? c.question : "101+2");
      if (buyer === "A") {
        await expect(summary).toContainText(c.product);
        await expect(summary).toContainText(c.registered);
        await expect(summary).toContainText(/2/);
        await expect(summary).toContainText(/5/);
        await expect(summary).toContainText(state === "sold" || context === "sold" ? /0/ : /18/);
        await expect(summary).toContainText(/20/);
      }
      const send = page.getByTestId("comment-reply").getByRole("button", { name: state === "pending" ? c.waitPending : ["sent", "blocked", "publicSent"].includes(state!) ? c.wait : channel === "public" ? c.publicSend : c.send, exact: true });
      if (["pending", "sent", "blocked", "publicSent"].includes(state)) await expect(send).toBeDisabled();
      if (state === "pending") await expect(console).toContainText(c.pending);
      if (state === "sent") await expect(console).toContainText(c.sent);
      if (state === "blocked") await expect(console).toContainText(c.blocked);
      if (state === "sold" || context === "sold") {
        await expect(page.getByRole("textbox", { name: c.reply, exact: true })).toHaveValue(c.sold);
        await expect(page.getByTestId("comment-reply").getByRole("button", { name: c.checkout, exact: true })).toHaveCount(0);
      }
    }
    // Read-only viewport measurement, never a DOM mutation or replacement for a user interaction.
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
    expect(mock.posts()).toBe(dispatchFromClick ? 1 : 0);
    await parityShot(page, id);
    await info.attach("click-ledger", { body: JSON.stringify({ route: "/zh-TW/studio/console", frame: id, key, actions: [device === "M" ? "click Comments tab" : "open route", ...(state === "normal" ? [] : [`click Buyer ${buyer}`]), ...(channel === "public" ? ["click Public reply"] : []), ...(filter === "全部" ? [] : [`click ${filter}`]), ...(device === "M" ? [`click ${tab} tab`] : [])], result: "visible copy, summary, disabled controls and overflow asserted; screenshot captured", persistence: "read state reconstructed from MOCK upstream; draft persistence not claimed" }), contentType: "application/json" });
  });

  for (const locale of locales) for (const width of [1440, 390]) test(`main screen and controls ${locale} ${width}`, async ({ page }, info) => {
    const c = parityCopy[locale];
    const mock = await parityFixture(page, locale);
    await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
    await login(page); await page.goto(route(locale, comments.session));
    if (width === 390) await parityTab(page, locale, "comments");
    await expect(page.getByRole("heading", { name: c.stream, exact: true })).toBeVisible();
    await paritySelect(page, locale);
    if (width === 390) await parityTab(page, locale, "buyer");
    const reply = page.getByRole("textbox", { name: c.reply, exact: true });
    await reply.fill("Synthetic private draft A");
    await page.getByTestId("comment-reply").getByRole("button", { name: c.public, exact: true }).click();
    await reply.fill("Synthetic public draft A");
    await page.getByTestId("comment-reply").getByRole("button", { name: c.private, exact: true }).click();
    await expect(reply).toHaveValue("Synthetic private draft A");
    if (width === 390) await parityTab(page, locale, "comments");
    await paritySelect(page, locale, "B");
    if (width === 390) await parityTab(page, locale, "buyer");
    await expect(reply).not.toHaveValue("Synthetic private draft A");
    await reply.fill("Synthetic independent draft B");
    if (width === 390) await parityTab(page, locale, "comments");
    await paritySelect(page, locale);
    if (width === 390) await parityTab(page, locale, "buyer");
    await expect(reply).toHaveValue("Synthetic private draft A");
    await page.getByRole("button", { name: c.stock, exact: true }).click();
    await expect(reply).not.toHaveValue("Synthetic private draft A");
    await page.getByRole("button", { name: c.checkout, exact: true }).click();
    await expect(reply).not.toHaveValue("");
    // One real click dispatches to the MOCK network boundary; queued must not be displayed as delivered.
    await page.getByRole("button", { name: c.send, exact: true }).click();
    await expect.poll(mock.posts).toBe(1);
    await expect(page.getByTestId("comment-send")).toBeDisabled();
    await expect(page.getByTestId("live-console")).not.toContainText(c.sent);
    await page.reload();
    if (width === 390) await parityTab(page, locale, "comments");
    await paritySelect(page, locale);
    if (width === 390) await parityTab(page, locale, "buyer");
    await expect(page.getByTestId("comment-send")).toBeDisabled();
    expect(mock.posts()).toBe(1);
    await parityShot(page, `main-${locale}-${width}`);
    await info.attach("click-ledger", { body: "Buyer A → private/public drafts → Buyer B independent draft → restore A → stock quick reply → checkout quick reply → send once → reload → select A → send remains disabled; MOCK queued is not delivered", contentType: "text/plain" });
  });

  for (const state of ["restricted", "expired", "pending"] as const) test(`${state} blocks send across refresh and context changes`, async ({ page }) => {
    const c = parityCopy["zh-TW"], mock = await parityFixture(page, "zh-TW", state);
    await login(page); await page.goto(route("zh-TW", comments.session));
    await expect(page.getByRole("heading", { name: c.stream, exact: true })).toBeVisible();
    await paritySelect(page, "zh-TW");
    const send = page.getByTestId("comment-send");
    await expect(send).toBeDisabled();
    await page.getByRole("button", { name: c.pause, exact: true }).click();
    await expect(page.getByRole("button", { name: c.resume, exact: true })).toBeVisible();
    await expect(send).toBeDisabled();
    await page.getByRole("button", { name: c.off, exact: true }).click();
    await expect(page.getByRole("button", { name: c.on, exact: true })).toBeVisible();
    await expect(send).toBeDisabled();
    await page.reload(); await paritySelect(page, "zh-TW");
    await expect(send).toBeDisabled();
    await expect(page.getByRole("button", { name: c.resume, exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: c.on, exact: true })).toBeVisible();
    await page.getByRole("button", { name: c.resume, exact: true }).click();
    await page.getByRole("button", { name: c.on, exact: true }).click();
    await expect(send).toBeDisabled();
    expect(mock.posts()).toBe(0);
  });
});
