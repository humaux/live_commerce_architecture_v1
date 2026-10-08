// Purpose: independent LC-U3 browser gate against production Next/BFF and real Go/PG, with a named retry calibration.
// Depends on: Go-owned LC_DRAWER_* fixture, signed MOCK OIDC, standalone admin build and installed Playwright.
// Used by: TestBrowserManualOrderLink after unchanged legacy cases; no trace/HAR/body/clipboard logging.
// Invariants: I02/I03/I06/I09/I11; only fixed case names, status/counts and synthetic screenshots enter evidence.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createWriteStream } from "node:fs";
import { writeFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch, ctxOpts } from "../storefront/browser-engine.mjs";

const root=process.cwd(), origin=process.env.COMMERCE_PUBLIC_ORIGIN, api=process.env.COMMERCE_API_ORIGIN;
const evidence=process.env.LC_DRAWER_EVIDENCE, store=process.env.LC_DRAWER_STORE, sku=process.env.LC_DRAWER_SKU;
const skuCode=process.env.LC_DRAWER_SKU_CODE;
const fixtures=JSON.parse(process.env.LC_DRAWER_FIXTURES), home=process.env.LC_DRAWER_HOME, cvs=process.env.LC_DRAWER_CVS;
const calibration=process.env.LC_DRAWER_CALIBRATION||"";
assert(evidence && store && sku && home && fixtures.length===6);
await mkdir(evidence,{recursive:true});
const ledger=[], failed=[], results=[];
let browser, child, cases=0, current="DU3-setup";
const log=createWriteStream(path.join(evidence,"admin.log"),{flags:"wx",mode:0o600});
const record=(control,action,actual)=>ledger.push({case:current,control,action,actual,status:"PASS"});
const wait=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const routePath=`/api/stores/${store}/tools/orders/for-buyer`;
const isSubmit=r=>r.request().method()==="POST" && new URL(r.url()).pathname===routePath;
async function openConversation(page,locale,id) {
 await page.goto(`${origin}/${locale}/messages?store=${store}`);
 await page.getByTestId(`conversation-${id}`).click();
 await expect(page.getByTestId("buyer-panel")).toBeVisible();
 const prefill=page.waitForResponse(r=>r.request().method()==="GET" && new URL(r.url()).pathname===`/api/stores/${store}/tools/inbox/order-prefill` && new URL(r.url()).searchParams.get("conversation_id")===id);
 await page.getByTestId("buyer-create-order").click();
 assert.equal((await prefill).status(),200,"real A15 prefill must succeed");
 const drawer=page.getByTestId("create-order-drawer");
 await expect(drawer).toBeVisible();
 // G-UI8 audit [READ/MEASURE]: inspect native dialog state, without altering product behavior.
 assert.equal(await drawer.evaluate(el=>el.tagName==="DIALOG" && el.open),true,"native modal dialog");
 await expect(drawer.getByTestId("drawer-form")).toBeVisible();
 record("BuyerPanel create order","click","real A15 and native dialog opened");
 return drawer;
}
async function submit(page,drawer) {
 const pending=page.waitForResponse(isSubmit);
 await drawer.getByTestId("drawer-submit").click();
 const response=await pending;
 assert(response.headers()["cache-control"]?.includes("no-store"));
 return response;
}
async function fillEmpty(drawer) {
 await drawer.getByTestId("drawer-name").fill("SYNTHETIC Drawer Buyer");
 await drawer.getByTestId("drawer-phone").fill("0912345678");
 await drawer.getByTestId("drawer-email").fill("drawer@example.invalid");
 await drawer.getByTestId("drawer-option").selectOption(home);
 await drawer.getByTestId("drawer-region").fill("Taipei Region");
 await drawer.getByTestId("drawer-postal").fill("100");
 await drawer.getByTestId("drawer-line2").fill("Synthetic Floor 1");
 await drawer.getByTestId("drawer-buyer-locale").selectOption("en");
 await drawer.getByTestId("drawer-city").fill("Taipei");
 await drawer.getByTestId("drawer-line1").fill("1 Synthetic Drawer Road");
 await drawer.getByTestId("drawer-mode-bank_transfer").check();
}
async function addCatalog(drawer) {
 await drawer.getByTestId("drawer-search").fill("LC-U3 synthetic catalog");
 await drawer.getByTestId("drawer-search-button").click();
 const row=drawer.locator(".mt-results > li").first();
 await row.locator(":scope > div button").click();
 const variant=drawer.locator(".mt-variants li").filter({hasText:skuCode});
 await expect(variant).toHaveCount(1);
 await variant.getByRole("button").click();
 await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toBeVisible();
 record("catalog search and variant","fill/search/expand/add","one real catalog SKU selected");
}
async function privateBoundary(page,link="") {
 // G-UI8 audit [READ/MEASURE]: read private DOM/storage/resource surfaces; never persist their contents.
 const state=await page.evaluate(()=>({dom:document.documentElement.outerHTML,storage:JSON.stringify([Object.entries(localStorage),Object.entries(sessionStorage)]),urls:[location.href,...performance.getEntriesByType("resource").map(e=>e.name)]}));
 for(const sentinel of ["SYNTHETIC Drawer Buyer","drawer@example.invalid","1 Synthetic Drawer Road"]) {
  assert(!state.storage.includes(sentinel));assert(state.urls.every(url=>!url.includes(sentinel)));
 }
 if(link) {
  const token=new URL(link).hash.match(/(?:^#|[&])t=([^&]+)/)?.[1];
  assert(token,"buyer link has fragment credential");
  assert(!state.dom.includes(link) && !state.dom.includes(token),"bearer must never enter DOM");
  assert(!state.storage.includes(link) && !state.storage.includes(token));
  assert(state.urls.every(url=>!url.includes(token)));
 }
}
try {
 const env={...process.env,NODE_ENV:"production",NEXT_TELEMETRY_DISABLED:"1",HOSTNAME:"127.0.0.1",PORT:process.env.LC_DRAWER_PORT};
 for(const name of Object.keys(env)) if(name.startsWith("LC_DRAWER_")) delete env[name];
 child=spawn(process.execPath,[path.join(root,"apps/admin/.next/standalone/apps/admin/server.js")],{cwd:path.join(root,"apps/admin"),env,stdio:["ignore",log,log]});
 let ready=false;
 for(let i=0;i<100;i++) {
  if(child.exitCode!==null) throw new Error("owned admin exited");
  try {if((await fetch(`${origin}/api/stores`)).status===401){ready=true;break;}}catch{}
  await wait(50);
 }
 assert(ready,"owned standalone Next readiness");
 browser=await launch({headless:true});
 let index=0;
 for(const locale of ["en","zh-TW","zh-CN"]) for(const width of [1440,390]) {
  const fixture=fixtures[index++];
  const context=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width,height:900},permissions:["clipboard-read","clipboard-write"]}));
  const page=await context.newPage();
  // No trace, console text collection or browser response serialization: these could retain bearer links.
  await page.goto(`${origin}/en`);
  await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
  current=`DU3-linked-${locale}-${width}`;
  let drawer=await openConversation(page,locale,fixture.Linked);
  await expect(drawer.getByTestId("drawer-linked-customer")).toBeVisible();
  await expect(drawer.getByTestId("drawer-name")).toHaveValue("王小明");
  await expect(drawer.getByTestId("drawer-phone")).toHaveValue("0912-345-678");
  await expect(drawer.getByTestId("drawer-option")).toHaveValue(home);
  await expect(drawer.getByTestId("drawer-city")).toHaveValue("中正區");
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");
  await drawer.getByTestId(`drawer-plus-${sku}`).click();
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("3");
  await drawer.getByTestId(`drawer-minus-${sku}`).click();
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");
  await drawer.getByTestId("drawer-mode-bank_transfer").check();
  await drawer.getByTestId("drawer-send-payment-link").check();
  const linkedReply=await submit(page,drawer);assert.equal(linkedReply.status(),201);
  const linked=await linkedReply.json();
  assert.equal(linked.send.state,"not_sent");assert.equal(linked.send.reason,"window_closed");
  assert.equal(linked.live_price,"applied");assert.equal(linked.commercial_state,"AWAITING_TRANSFER");
  await expect(drawer.getByTestId("drawer-live-result")).toBeVisible();
  await expect(drawer.getByTestId("drawer-send-result")).toBeVisible();
  await drawer.getByTestId("drawer-copy-link").click();
  // Read the native clipboard API after the actual product control; never replace navigator.clipboard.writeText.
  // G-UI8 audit [READ/MEASURE]: read the native clipboard after the actual Copy control.
  const copied=await page.evaluate(()=>navigator.clipboard.readText());
  assert(copied===linked.buyer_link,"native clipboard receives exactly the real Go buyer link");
  await privateBoundary(page,copied);
  await page.screenshot({path:path.join(evidence,`linked-${locale}-${width}.png`),fullPage:true});
  record("linked order/send/copy","click +/- then create and copy","real order; window_closed; native clipboard; bearer absent from DOM/storage/URL");
  results.push({case:current,order_id:linked.order_id});cases++;

  current=`DU3-duplicate-${locale}-${width}`;
  await drawer.getByTestId("drawer-close").click();
  drawer=await openConversation(page,locale,fixture.Linked);
  await drawer.getByTestId("drawer-mode-bank_transfer").check();
  const duplicate=await submit(page,drawer);assert.equal(duplicate.status(),409);
  const refusal=await duplicate.json();assert.equal(refusal.code,"bundle_already_ordered");assert.equal(refusal.details.order_id,linked.order_id);
  const view=drawer.getByTestId("drawer-view-order");await expect(view).toBeVisible();
  const target=new URL(await view.getAttribute("href"),origin);
  assert.equal(target.pathname,`/${locale}/orders`);assert.equal(target.searchParams.get("store"),store);assert.equal(target.searchParams.get("order"),linked.order_id);
  await view.click();await expect(page).toHaveURL(target.href);
  record("duplicate/view order","submit again then click view","real 409 points at first order");cases++;

  current=`DU3-unlinked-${locale}-${width}`;
  drawer=await openConversation(page,locale,fixture.Unlinked);
  for(const id of ["name","phone","email"]) await expect(drawer.getByTestId(`drawer-${id}`)).toHaveValue("");
  await expect(drawer.getByTestId("drawer-linked-customer")).toHaveCount(0);
  await expect(drawer.getByTestId("drawer-send-payment-link")).toBeDisabled();
  await expect(drawer.getByTestId("drawer-live-warning")).toBeVisible();
  await expect(drawer.getByTestId("drawer-live-warning")).toContainText(locale==="en" ? "Live" : "直播");
  await drawer.getByTestId("drawer-option").selectOption(home);
  await expect(drawer.getByTestId("drawer-city")).toHaveValue("");await expect(drawer.getByTestId("drawer-line1")).toHaveValue("");
  await addCatalog(drawer);
  await drawer.getByTestId(`drawer-minus-${sku}`).click();
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveCount(0);
  await addCatalog(drawer);
  await drawer.getByTestId("drawer-option").selectOption(cvs);
  await drawer.getByTestId("drawer-store-code").fill("123456");
  await drawer.getByTestId("drawer-store-name").fill("SYNTHETIC Pickup");
  await drawer.getByTestId("drawer-store-address").fill("1 Synthetic Pickup Road");
  await drawer.getByTestId("drawer-mode-pay_at_pickup").check();
  await fillEmpty(drawer);
  await page.screenshot({path:path.join(evidence,`unlinked-${locale}-${width}.png`),fullPage:true});
  record("unlinked warning/empty fields/zero","edit form","no customer guessing; quantity zero removes; valid catalog line re-added");cases++;

  current=`DU3-single-order-${locale}-${width}`;
  const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
  const attempts=[];let armed=true;
  await page.route(`**${routePath}`,async route=>{
   const req=route.request(), headers={...req.headers()};
   if(!armed && calibration==="new-key-on-retry" && locale==="en" && width===1440) headers["idempotency-key"]=crypto.randomUUID();
   const body=req.postData();attempts.push({key:headers["idempotency-key"],body});
   const real=await route.fetch({headers});
   if(armed){armed=false;assert.equal(real.status(),201,"lost reply first commits through real Go");await real.dispose();await route.abort("failed");}
   else await route.fulfill({response:real});
  });
  await drawer.getByTestId("drawer-submit").click();
  await expect(drawer.getByTestId("drawer-error")).toBeVisible();
  // UNKNOWN remains open: retry the same explicit submit, never close/reopen or silently retry.
  await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
  const replay=await submit(page,drawer);
  await expect(drawer.getByTestId("drawer-result")).toBeVisible();
  await page.unroute(`**${routePath}`);
  const after=await (await fetch(`${api}/__test/drawer-facts`)).json();
  const factsReply=await fetch(`${api}/__test/drawer-request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:attempts.map(a=>a.key)})});
  assert.equal(factsReply.status,200);const exact=await factsReply.json();
  const outcome={case:current,attempts:attempts.length,same_key:attempts.length===2&&attempts[0].key===attempts[1].key,same_body:attempts.length===2&&attempts[0].body===attempts[1].body,pg_delta:after.orders-before.orders,pg_request_orders:exact.orders,pg_request_receipts:exact.receipts,replay_status:replay.status()};
  results.push(outcome);
  await privateBoundary(page);
  if(outcome.attempts!==2 || !outcome.same_key || !outcome.same_body || outcome.pg_delta!==1 || exact.orders!==1 || exact.receipts!==1 || replay.status()!==200) {
   failed.push(current);ledger.push({case:current,control:"explicit retry",action:"click",actual:outcome,status:"FAIL"});
  } else record("UNKNOWN explicit retry","commit/abort/click","same key/body; real replay 200; exact one request order and receipt in PG");
  cases++;
  await context.close();
 }
 // Open directly from the real signed-ingress claimed comment row in the actual console.
 const context=await browser.newContext({viewport:{width:1440,height:900}}),page=await context.newPage();
 await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();await expect(page.getByTestId("dashboard-page")).toBeVisible();
 current="DU3-comment-trigger";
 await page.goto(`${origin}/en/studio/console?store=${process.env.LC_DRAWER_COMMENT_STORE}&scene=${process.env.LC_DRAWER_COMMENT_SCENE}`);
 await page.getByTestId(`comment-create-order-${process.env.LC_DRAWER_COMMENT_REF}`).click();
 await expect(page.getByTestId("create-order-drawer")).toBeVisible();
 await expect(page.getByTestId("drawer-live-warning")).toBeVisible();
 record("claimed comment direct create","click","real CommentStream opens bundle-prefilled drawer without guessed conversation");
 await context.close();
} catch (error) {
 const coordinate=String(error?.stack||"").match(/create-order-drawer-gate\.mjs:(\d+):(\d+)/);
 failed.push(current);ledger.push({case:current,control:"scenario",action:"run",actual:coordinate?`assertion failed at create-order-drawer-gate.mjs:${coordinate[1]}:${coordinate[2]}`:"assertion failed; private diagnostics suppressed",status:"FAIL"});
} finally {
 if(browser) await browser.close();
 if(child && child.exitCode===null) {child.kill("SIGTERM");await Promise.race([new Promise(resolve=>child.once("exit",resolve)),wait(1500)]);if(child.exitCode===null) child.kill("SIGKILL");}
 await writeFile(path.join(evidence,"click-ledger.json"),JSON.stringify(ledger,null,2),{mode:0o600});
 await writeFile(path.join(evidence,"result.json"),JSON.stringify({cases,failed,results},null,2),{mode:0o600});
 log.end();
}
for(const name of failed) console.error(`FAIL ${name}`);
console.log(`drawer cases=${cases} failed=${failed.length}`);
if(failed.length) process.exitCode=1;
