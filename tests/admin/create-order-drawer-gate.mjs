// Purpose: independent LC-U3 browser gate against production Next/BFF and real Go/PG, with a named retry calibration.
// Depends on: Go-owned LC_DRAWER_* fixture, signed MOCK OIDC, standalone admin build and installed Playwright.
// Used by: TestBrowserManualOrderLink after unchanged legacy cases; no trace/HAR/body/clipboard logging.
// Invariants: I02/I03/I06/I09/I11; only fixed case names, status/counts and synthetic screenshots enter evidence.
import assert from "node:assert/strict";
import { runManualRecovery, runManualCreatedRecovery } from "./create-order-manual-recovery.mjs";
import { runCapabilityRecovery } from "./create-order-capability-recovery.mjs";
import { runPartialRecovery } from "./create-order-partial-recovery.mjs";
import { spawn } from "node:child_process";
import { once } from "node:events";
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
const partialFixture=JSON.parse(process.env.LC_DRAWER_PARTIAL_FIXTURE);
const capabilityFixture=JSON.parse(process.env.LC_DRAWER_CAPABILITY_FIXTURE);
assert(evidence && store && sku && home && fixtures.length===6 && partialFixture.Linked && partialFixture.Bundle);
await mkdir(evidence,{recursive:true});
const ledger=[], failed=[], results=[];
let browser, child, cases=0, current="DU3-setup";
const log=createWriteStream(path.join(evidence,"admin.log"),{flags:"wx",mode:0o600});
// spawn requires an opened file descriptor; passing a pending WriteStream throws before Next starts.
await once(log,"open");
const record=(control,action,actual)=>ledger.push({case:current,control,action,actual,status:"PASS"});
const saveEvidence=async()=>{
 await writeFile(path.join(evidence,"click-ledger.json"),JSON.stringify(ledger,null,2),{mode:0o600});
 await writeFile(path.join(evidence,"result.json"),JSON.stringify({cases,failed,results},null,2),{mode:0o600});
};
function recordFailure(error,control="scenario") {
 // Error messages/stacks may include signed headers or private DOM; retain only test coordinates.
 const coordinate=String(error?.stack||"").match(/\b(create-order-(?:drawer-gate|manual-recovery|partial-recovery|capability-recovery)\.mjs):(\d+):(\d+)/);
 failed.push(current);ledger.push({case:current,control,action:"run",actual:coordinate?`assertion failed at ${coordinate[1]}:${coordinate[2]}:${coordinate[3]}`:"assertion failed; private diagnostics suppressed",status:"FAIL"});
}
async function drainContext(context) {
 for(const page of context.pages()) await page.unrouteAll({behavior:"wait"});
 await context.unrouteAll({behavior:"wait"});
}
async function closeContext(context) {
 try {await drainContext(context);} finally {await context.close();}
}
const wait=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const routePath=`/api/stores/${store}/tools/orders/for-buyer`;
const isSubmit=r=>r.request().method()==="POST" && new URL(r.url()).pathname===routePath;
async function openConversation(page,locale,id,inspectPrefill) {
 await page.goto(`${origin}/${locale}/messages?store=${store}`);
 await page.getByTestId(`conversation-${id}`).click();
 await expect(page.getByTestId("buyer-panel")).toBeVisible();
 const prefill=page.waitForResponse(r=>r.request().method()==="GET" && new URL(r.url()).pathname===`/api/stores/${store}/tools/inbox/order-prefill` && new URL(r.url()).searchParams.get("conversation_id")===id);
 const [response]=await Promise.all([prefill,page.getByTestId("buyer-create-order").click()]);
 assert.equal(response.status(),200,"real A15 prefill must succeed");
 if(inspectPrefill) inspectPrefill(await response.json());
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
 const [response]=await Promise.all([pending,drawer.getByTestId("drawer-submit").click()]);
 assert(response.headers()["cache-control"]?.includes("no-store"));
 return response;
}
async function fillEmpty(drawer,prefix="drawer") {
 await drawer.getByTestId(`${prefix}-name`).fill("SYNTHETIC Drawer Buyer");
 await drawer.getByTestId(`${prefix}-phone`).fill("0912345678");
 await drawer.getByTestId(`${prefix}-email`).fill("drawer@example.invalid");
 await drawer.getByTestId(`${prefix}-option`).selectOption(home);
 await drawer.getByTestId(`${prefix}-region`).fill("Taipei Region");
 await drawer.getByTestId(`${prefix}-postal`).fill("100");
 await drawer.getByTestId(`${prefix}-line2`).fill("Synthetic Floor 1");
 await drawer.getByTestId(`${prefix}-buyer-locale`).selectOption("en");
 await drawer.getByTestId(`${prefix}-city`).fill("Taipei");
 await drawer.getByTestId(`${prefix}-line1`).fill("1 Synthetic Drawer Road");
 await drawer.getByTestId(`${prefix}-mode-bank_transfer`).check();
}
async function addCatalog(drawer,prefix="drawer") {
 await drawer.getByTestId(`${prefix}-search`).fill("LC-U3 synthetic catalog");
 await drawer.getByTestId(`${prefix}-search-button`).click();
 const row=drawer.locator(".mt-results > li").first();
 await row.locator(":scope > div button").click();
 const variant=drawer.locator(".mt-variants li").filter({hasText:skuCode});
 await expect(variant).toHaveCount(1);
 await variant.getByRole("button").click();
 await expect(prefix==="drawer"?drawer.getByTestId(`drawer-quantity-${sku}`):drawer.getByTestId("mo-lines").locator('input[type="number"]')).toBeVisible();
 record("catalog search and variant","fill/search/expand/add","one real catalog SKU selected");
}
async function definitiveRefusal(page,form,{requestPath,submitId,errorId,newAttemptId,editDraft=false,unknownFirst=false}) {
 const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
 let posts=0,recoveryPosts=0;
 const observe=request=>{if(request.method()==="POST"){const pathname=new URL(request.url()).pathname;if(pathname===requestPath) posts++;if(pathname===`${requestPath}/recover`) recoveryPosts++;}};
 page.on("request",observe);
 const attempts=[];
 if(unknownFirst) {
  // This CVS option remains available via pay_at_pickup when bank transfer is disabled.
  // A bank-only home row disappears entirely and returns invalid_request instead of this exact mode refusal.
  await form.getByTestId("drawer-option").selectOption(cvs);
  await form.getByTestId("drawer-store-code").fill("123456");
  await form.getByTestId("drawer-store-name").fill("SYNTHETIC Pickup");
  await form.getByTestId("drawer-store-address").fill("1 Synthetic Pickup Road");
  await form.getByTestId("drawer-mode-bank_transfer").check();
  await expect(form.getByTestId(submitId)).toBeEnabled();
  assert.equal((await fetch(`${api}/__test/drawer-bank-transfer`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({enabled:false})})).status,204);
 }
 // This owned settings fault occurs after valid options/Quote loaded; the product request itself remains byte-identical.
 // UNKNOWN drops only the reply to the merchant's frozen request. The separate 422 case mutates quantity at the network edge.
 await page.route(`**${requestPath}`,async route=>{
  attempts.push({key:route.request().headers()["idempotency-key"],body:route.request().postData()});
  const body=JSON.parse(route.request().postData());
  if(!unknownFirst) body.items[0].quantity=0;
  const real=await route.fetch(unknownFirst?{}:{postData:JSON.stringify(body)});
  assert.equal(real.status(),422,"real BFF/Go must refuse invalid quantity or unavailable bank transfer");
  if(unknownFirst) assert.equal((await real.json()).code,"bank_transfer_unavailable","actual business refusal before receipt lookup");
  if(unknownFirst&&attempts.length===1) { await real.dispose();await route.abort("failed");return; }
  await route.fulfill({response:real});
 });
 if(unknownFirst) {
  await form.getByTestId(submitId).click();
  await expect(form.getByTestId(errorId)).toBeVisible();
  await expect(form.getByTestId("drawer-close")).toBeDisabled();
  await expect(form.getByTestId(newAttemptId)).toHaveCount(0);
  await expect(form.getByTestId(submitId)).toBeEnabled();
 }
 const [refusal]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===requestPath),form.getByTestId(submitId).click()]);
 assert.equal(refusal.status(),422);assert(refusal.headers()["cache-control"]?.includes("no-store"));
 await expect(form.getByTestId(errorId)).toBeVisible();
 if(unknownFirst) {
  await expect(form.getByTestId("drawer-close")).toBeDisabled();await expect(form.getByTestId(newAttemptId)).toHaveCount(0);
  await expect(form.getByTestId("drawer-recover-attempt")).toBeVisible();
  const recoveryPath=`${requestPath}/recover`;
  const [recovery]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===recoveryPath),form.getByTestId("drawer-recover-attempt").click()]);
  assert.equal(recovery.status(),200);assert(recovery.headers()["cache-control"]?.includes("no-store"));assert.deepEqual(await recovery.json(),{state:"not_created"});
  assert.equal(recovery.request().headers()["idempotency-key"],attempts[0].key,"A16R retains original key");assert.equal(recovery.request().postData(),attempts[0].body,"A16R retains original body");
  await expect(form.getByTestId("drawer-recovery-result")).toBeVisible();
  const seals=await fetch(`${api}/__test/drawer-recovery-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({key:attempts[0].key})});
  assert.equal(seals.status,200);assert.deepEqual(await seals.json(),{not_created_seals:1},"exact original-key durable absence seal in PG");
  assert.equal((await fetch(`${api}/__test/drawer-bank-transfer`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({enabled:true})})).status,204);
  record("A16R recovery","UNKNOWN/business 422/recover click","same key/body; real durable not_created seal; recovery result; no new order");
 }
 await expect(form.getByTestId(newAttemptId)).toBeVisible();
 if(editDraft) {
  await form.getByTestId("mo-name").fill("SYNTHETIC Refused Draft");
  await expect(form.getByTestId("mo-name")).toHaveValue("SYNTHETIC Refused Draft");
 }
 await expect(form.getByTestId(submitId)).toBeDisabled();
 const refused=await (await fetch(`${api}/__test/drawer-facts`)).json();
 assert.equal(refused.orders,before.orders,"actual definitive refusal creates zero orders");
 assert.equal(posts,unknownFirst?2:1,"only explicit refused POSTs, including after any draft edit");
 if(unknownFirst) {
  assert.equal(attempts[0].key,attempts[1].key,"UNKNOWN replay retains original key");
  assert.equal(attempts[0].body,attempts[1].body,"UNKNOWN replay retains original body");
  await expect(form.getByTestId("drawer-close")).toBeEnabled();
  assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0,"definitive refusal clears opaque OrderGuard");
  const exact=await (await fetch(`${api}/__test/drawer-request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:attempts.map(a=>a.key)})})).json();
  assert.deepEqual(exact,{orders:0,receipts:0},"refused attempts create no PG order or success receipt");
 }
 await page.unroute(`**${requestPath}`);
 await form.getByTestId(newAttemptId).click();
 await expect(form.getByTestId(submitId)).toBeEnabled();
 const after=await (await fetch(`${api}/__test/drawer-facts`)).json();
 assert.equal(posts,unknownFirst?2:1,"explicit new-attempt action must not POST");
 assert.equal(recoveryPosts,unknownFirst?1:0,"only explicit recovery click may POST A16R; new-attempt is local");
 assert.equal(after.orders,before.orders,"explicit new-attempt action creates zero orders");
 page.off("request",observe);
 record(newAttemptId,unknownFirst?"real UNKNOWN/business 422/recovery then explicit click":"real 422 then explicit click","zero merchant-manual order delta; submit remains disabled after refusal/draft edit; new-attempt enables without network write");
 if(unknownFirst) { record("UNKNOWN then coded refusal","abort real business 422/replay/click recovery/click new attempt","same key/body; zero PG orders/receipts; OrderGuard released; close enabled");cases++; }
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
 // Separate signed context and the ordinary ManualOrder page; faults affect only real request/response network edges.
 {
  const context=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),page=await context.newPage();
  current="DU3-manual-unknown-draft-edit";
  await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
  await page.goto(`${origin}/en/orders/new?store=${store}`);
  const form=page.getByTestId("manual-order-form");await expect(form).toBeVisible();
  await addCatalog(form,"mo");await fillEmpty(form,"mo");
  const manualPath=`/api/stores/${store}/tools/orders/manual`,attempts=[];
  const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
  await page.route(`**${manualPath}`,async route=>{
   const req=route.request();attempts.push({key:req.headers()["idempotency-key"],body:req.postData()});
   const real=await route.fetch();
   if(attempts.length===1){assert.equal(real.status(),201,"manual lost reply must first commit through real Go");await real.dispose();await route.abort("failed");}
   else await route.fulfill({response:real});
  });
  await form.getByTestId("manual-order-submit").click();await expect(form.getByTestId("manual-order-error")).toBeVisible();
  await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
  // Real merchant draft edits must never replace the unresolved attempt's immutable body/key.
  await form.getByTestId("mo-name").fill("SYNTHETIC Edited Draft");
  const quantity=form.getByTestId("mo-lines").locator('input[type="number"]');await quantity.fill("2");
  await expect(form.getByTestId("mo-name")).toHaveValue("SYNTHETIC Edited Draft");await expect(quantity).toHaveValue("2");
  const [replay]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===manualPath),form.getByTestId("manual-order-submit").click()]);
  await expect(page.getByTestId("manual-order-result")).toBeVisible();await page.unroute(`**${manualPath}`);
  const after=await (await fetch(`${api}/__test/drawer-facts`)).json();
  const factsReply=await fetch(`${api}/__test/manual-request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:attempts.map(a=>a.key)})});
  assert.equal(factsReply.status,200);const exact=await factsReply.json();
  const outcome={case:current,attempts:attempts.length,same_key:attempts.length===2&&attempts[0].key===attempts[1].key,same_body:attempts.length===2&&attempts[0].body===attempts[1].body,pg_delta:after.orders-before.orders,pg_request_orders:exact.orders,pg_request_receipts:exact.receipts,replay_status:replay.status()};
  results.push(outcome);cases++;
  await privateBoundary(page);
  if(outcome.attempts!==2||!outcome.same_key||!outcome.same_body||outcome.pg_delta!==1||exact.orders!==1||exact.receipts!==1||replay.status()!==200){
   failed.push(current);ledger.push({case:current,control:"manual UNKNOWN draft edit retry",action:"commit/abort/fill/click",actual:outcome,status:"FAIL"});
  }else record("manual UNKNOWN draft edit retry","commit/abort/fill/click","edited draft; original key/body; real replay 200; exact one order and receipt in PG");
  // Success already resolved the old attempt: use the existing explicit "another order" action to start the refusal subcheck.
  await page.getByTestId("manual-order-another").click();await expect(form).toBeVisible();
  await addCatalog(form,"mo");await fillEmpty(form,"mo");
  await definitiveRefusal(page,form,{requestPath:manualPath,submitId:"manual-order-submit",errorId:"manual-order-error",newAttemptId:"manual-order-new-attempt",editDraft:true});
  await closeContext(context);
 }
 let index=0;
 for(const locale of ["en","zh-TW","zh-CN"]) for(const width of [1440,390]) {
  const fixture=fixtures[index++];
  const context=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width,height:900},permissions:["clipboard-read","clipboard-write"]}));
  const page=await context.newPage();
  // Network-edge A13 projection fault: the first claim is unrelated to the real A15 target.
  // All actual target offers, order writes and restriction replies remain real BFF/Go/PG.
  let expectedSession, injectedFirst=false;
  const restrictionRequests=[];
  const observeRestriction=request=>{if(request.method()==="GET"&&new URL(request.url()).pathname.endsWith("/claims/blocklist/check")) restrictionRequests.push(new URL(request.url()));};
  page.on("request",observeRestriction);
  if(locale==="en"&&width===1440) await page.route(`**/api/stores/${store}/inbox/buyer-panel?**`,async route=>{
   if(new URL(route.request().url()).searchParams.get("conversation_id")!==fixture.Linked) { await route.continue();return; }
   const real=await route.fetch();assert.equal(real.status(),200);
   const value=await real.json();assert.equal(value.claims.length,1,"positive real A13 target fixture");
   expectedSession=value.claims[0].session_id;
   const unrelated={...value.claims[0],offer_id:crypto.randomUUID(),session_id:crypto.randomUUID()};
   value.claims.unshift(unrelated);injectedFirst=true;
   await route.fulfill({response:real,json:value});
  });
  // No trace, console text collection or browser response serialization: these could retain bearer links.
  await page.goto(`${origin}/en`);
  await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
  current=`DU3-linked-${locale}-${width}`;
  let remaining;
  let drawer=await openConversation(page,locale,fixture.Linked,prefill=>{
   remaining=prefill.items.find(item=>item.sku_id===sku)?.live_quantity_remaining;
   assert.equal(remaining,2,"fresh claim A15 remaining quantity is exactly two");
  });
  await expect(drawer.getByTestId("drawer-blocklist-warning")).toBeVisible();
  const restrictedCopy={en:"This buyer is on the restricted list.","zh-TW":"此買家已在限制名單中。","zh-CN":"此买家已在限制名单中。"};
  await expect(drawer.getByTestId("drawer-blocklist-warning")).toContainText(restrictedCopy[locale]);
  if(locale==="en"&&width===1440) {
   assert.equal(injectedFirst,true);
   assert(restrictionRequests.length>0,"actual target restriction request was sent");
   assert(restrictionRequests.every(url=>url.pathname===`/api/stores/${store}/live-sessions/${expectedSession}/claims/blocklist/check`&&url.searchParams.get("bundle_id")===fixture.Bundle),"restriction lookup must match actual A15 offer and bundle, not unrelated first claim");
   record("exact restriction session","real click with unrelated first A13 claim","actual A15 session/bundle GET; restricted warning");
   await page.unroute(`**/api/stores/${store}/inbox/buyer-panel?**`);
  }
  page.off("request",observeRestriction);
  record("blocklist warning","open by real BuyerPanel click","restricted boolean warning visible");
  await expect(drawer.getByTestId("drawer-linked-customer")).toBeVisible();
  await expect(drawer.getByTestId("drawer-name")).toHaveValue("王小明");
  await expect(drawer.getByTestId("drawer-phone")).toHaveValue("0912-345-678");
  await expect(drawer.getByTestId("drawer-option")).toHaveValue(home);
  await expect(drawer.getByTestId("drawer-city")).toHaveValue("中正區");
  // The fixture claim is exactly two units and no live-price use exists before this first submit.
  await expect(drawer.getByTestId("drawer-claim-0")).toContainText(new RegExp(`(?:^|\\D)${remaining}(?:\\D|$)`));
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");
  await drawer.getByTestId(`drawer-plus-${sku}`).click();
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("3");
  await drawer.getByTestId(`drawer-minus-${sku}`).click();
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");
  await drawer.getByTestId("drawer-mode-bank_transfer").check();
  await drawer.getByTestId("drawer-send-payment-link").check();
  // The valid payment choice and Quote finish after A15; restriction must not disable that ready form.
  await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
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
  const unavailableCopy={en:"The restriction status could not be checked.","zh-TW":"無法確認限制名單狀態。","zh-CN":"无法确认限制名单状态。"};
  await expect(drawer.getByTestId("drawer-blocklist-warning")).toBeVisible();
  await expect(drawer.getByTestId("drawer-blocklist-warning")).toContainText(unavailableCopy[locale]);
  record("missing restriction session","open zero-claims conversation","explicit localized could-not-check warning");
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
  if(locale==="en"&&width===1440) await definitiveRefusal(page,drawer,{requestPath:routePath,submitId:"drawer-submit",errorId:"drawer-error",newAttemptId:"drawer-new-attempt"});
  if(locale==="en"&&width===1440) {
   await definitiveRefusal(page,drawer,{requestPath:routePath,submitId:"drawer-submit",errorId:"drawer-error",newAttemptId:"drawer-new-attempt",unknownFirst:true});
   await drawer.getByTestId("drawer-close").click();
   const afterRecovery=await (await fetch(`${api}/__test/drawer-facts`)).json();
   await page.reload();assert.equal((await (await fetch(`${api}/__test/drawer-facts`)).json()).orders,afterRecovery.orders,"reload after recovery/new attempt creates no order");
   drawer=await openConversation(page,locale,fixture.Unlinked);
   await addCatalog(drawer);await fillEmpty(drawer);
   await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
   record("settled guard close/reopen","click close then create","new drawer can submit after refused intent");
  }
  await page.screenshot({path:path.join(evidence,`unlinked-${locale}-${width}.png`),fullPage:true});
  record("unlinked warning/empty fields/zero","edit form","no customer guessing; quantity zero removes; valid catalog line re-added");cases++;

  current=`DU3-single-order-${locale}-${width}`;
  // Keep the original new-key calibration isolated from queued replay assertions:
  // a deliberately new key creates a second link, which must reach the exact-order verdict below.
  const queued=locale==="en"&&width===1440&&calibration!=="new-key-on-retry";
  let committed, initialQueuedFacts, originalLink="", regeneratePosts=0;
  const observeRegenerate=request=>{if(request.method()==="POST"&&new URL(request.url()).pathname.endsWith("/orders/manual/regenerate-link")) regeneratePosts++;};
  if(queued) {
   // Fixture-only window control; acceptance itself remains real BuyerPanel/form clicks.
   assert.equal((await fetch(`${api}/__test/drawer-queued-window`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({open:true})})).status,204);
   await drawer.getByTestId("drawer-close").click();drawer=await openConversation(page,locale,fixture.Unlinked);
   await addCatalog(drawer);await fillEmpty(drawer);await drawer.getByTestId("drawer-send-payment-link").check();
   page.on("request",observeRegenerate);
  }
  const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
  const attempts=[];let armed=true,firstDropError,finishFirstDrop;
  // Await the real201, exact initial PG facts and completed abort; UI expectation timeouts are not upstream request budgets.
  const firstDrop=new Promise(resolve=>{finishFirstDrop=resolve;});
  await page.route(`**${routePath}`,async route=>{
   const first=armed;
   try {
   const req=route.request(), headers={...req.headers()};
   if(!armed && calibration==="new-key-on-retry" && locale==="en" && width===1440) headers["idempotency-key"]=crypto.randomUUID();
   const body=req.postData();attempts.push({key:headers["idempotency-key"],body});
   const real=await route.fetch({headers});
   if(armed){armed=false;assert.equal(real.status(),201,"lost reply first commits through real Go");
    if(queued){
     committed=await real.json();assert.equal(committed.send.state,"queued");assert(committed.send.operation_id);originalLink=committed.buyer_link;assert(typeof originalLink==="string");
     // Initial DM issuance and planning now commit atomically. Establish the sealed-link PG baseline
     // before dropping the reply; replay must preserve every fact, including the one initial issuance audit.
     const facts=await fetch(`${api}/__test/drawer-queued-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({order_id:committed.order_id,operation_id:committed.send.operation_id,link:originalLink})});
     assert.equal(facts.status,200);initialQueuedFacts=await facts.json();
     assert.deepEqual(initialQueuedFacts,{operations:1,secrets:1,original_link:true,valid_links:1,regenerations:1});
    }
    await real.dispose();await route.abort("failed");}
   else await route.fulfill({response:real});
   }catch(error){
    if(!first) throw error;
    // Deliver first-handler failures to the awaiting scenario, never an unhandled rejection with raw headers.
    firstDropError=error;
    try {await route.abort("failed");}catch(abortError){firstDropError=new AggregateError([error,abortError],"first response drop failed");}
   }finally{if(first) finishFirstDrop();}
  });
  await drawer.getByTestId("drawer-submit").click();
  await firstDrop;if(firstDropError) throw firstDropError;
  await expect(drawer.getByTestId("drawer-error")).toBeVisible();
  await expect(drawer.getByTestId("drawer-name")).toBeDisabled();
  await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toBeDisabled();
  record("UNKNOWN draft edit protection","inspect native disabled fields","customer and quantity cannot alter unresolved attempt");
  // UNKNOWN remains open: retry the same explicit submit, never close/reopen or silently retry.
  await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
  const replay=await submit(page,drawer);
  await expect(drawer.getByTestId("drawer-result")).toBeVisible();
  if(queued) {
   const value=await replay.json();assert.equal(value.buyer_link,null);assert.equal(value.link_state,"configured");
   assert.equal(value.order_id,committed.order_id);assert.equal(value.send.state,"queued");assert.equal(value.send.operation_id,committed.send.operation_id);
   await expect(drawer.getByTestId("drawer-send-result")).toContainText("queued");
   const copy=drawer.getByTestId("drawer-copy-link"),copyBlocked=await copy.count()===0 || await copy.isDisabled();
   // Retain the failure and continue the original matrix so PG14/29-case evidence remains comparable.
   if(!copyBlocked){failed.push(current);ledger.push({case:current,control:"queued replay copy safety",action:"real submit retry then inspect control",actual:"FAIL copy enabled after queued replay with null buyer_link",status:"FAIL"});}
   else record("queued replay copy safety","real submit retry then inspect control","queued result; copy absent/disabled; no regeneration");
   await privateBoundary(page,originalLink);
   const facts=await fetch(`${api}/__test/drawer-queued-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({order_id:value.order_id,operation_id:value.send.operation_id,link:originalLink})});
   assert.equal(facts.status,200);const dm=await facts.json();results.push({case:current,queued_observation:dm,initial_queued_observation:initialQueuedFacts});assert.deepEqual(dm,initialQueuedFacts,"UNKNOWN replay must not rotate the queued payment link or duplicate its sealed operation");assert.equal(regeneratePosts,0);
   results.push({case:current,queued_replay:true,copy_blocked:copyBlocked,regenerate_posts:regeneratePosts,...dm});
   // Reload and click the persisted order, using a positive loaded table and detail sentinel.
   await page.goto(`${origin}/${locale}/orders?store=${store}`);await page.reload();
   await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
   await page.getByTestId(`order-expand-${value.order_id}`).click();await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(value.order_id));
   record("queued order persistence","reload orders then click exact order","same persisted order row and visible detail");
   assert.equal((await fetch(`${api}/__test/drawer-queued-window`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({open:false})})).status,204);
   await openConversation(page,locale,fixture.Unlinked);drawer=page.getByTestId("create-order-drawer");
   page.off("request",observeRegenerate);originalLink="";
  }
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

  // Keep this privacy regression in the same matrix entry, with no additional order creation.
  current=`DU3-auth-boundary-${locale}-${width}`;
  await drawer.getByTestId("drawer-close").click();
  drawer=await openConversation(page,locale,fixture.Unlinked);
  await addCatalog(drawer);await fillEmpty(drawer);
  await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
  await expect(drawer.getByTestId("drawer-name")).toHaveValue("SYNTHETIC Drawer Buyer");
  let boundaryPosts=0;
  const boundaryRequest=request=>{if(request.method()==="POST" && new URL(request.url()).pathname===routePath) boundaryPosts++;};
  page.on("request",boundaryRequest);
  const beforeBoundary=await (await fetch(`${api}/__test/drawer-facts`)).json();
  const csrf=(await context.cookies()).filter(cookie=>cookie.name==="__Host-commerce_csrf");
  assert.equal(csrf.length,1,"the actual signed login supplied exactly one CSRF cookie");
  // A real session-boundary fault: a rejected replacement cookie makes the real sessionBoundary throw.
  // No cookie value, private field, assertion dump or response body is written to the ledger.
  await context.addCookies([{...csrf[0],value:"LC-U3-rotated-invalid-CSRF"}]);
  await drawer.getByTestId("drawer-submit").click();
  // G-UI8 audit [READ/MEASURE]: poll actual DOM and input properties without modifying the product.
  await expect.poll(()=>page.evaluate(()=>{
   const surfaces=[document.documentElement.outerHTML,...[...document.querySelectorAll("input,textarea")].map(element=>element.value)];
   return ["SYNTHETIC Drawer Buyer","0912345678","drawer@example.invalid","Taipei Region","1 Synthetic Drawer Road","Synthetic Floor 1"].every(value=>surfaces.every(surface=>!surface.includes(value)));
  }),{message:"auth boundary must remove every draft contact/address from the DOM"}).toBe(true);
  const afterBoundary=await (await fetch(`${api}/__test/drawer-facts`)).json();
  page.off("request",boundaryRequest);
  assert.equal(boundaryPosts,0,"rejected session boundary must never POST A16");
  assert.equal(afterBoundary.orders,beforeBoundary.orders,"session boundary creates no order");
  record("CSRF session boundary","rotate actual cookie then click submit","private draft removed; zero A16 requests; PG order count unchanged");
  await closeContext(context);
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
 await closeContext(context);
 // Switch only this owned issuer/subject to a real narrow staff principal; authenticate through normal clicks.
 const switched=await fetch(`${api}/__test/drawer-principal`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({limited:true})});
 assert.equal(switched.status,204);
 try {
  for(const locale of ["en","zh-TW","zh-CN"]) {
   current=`DU3-no-catalog-${locale}-390`;
   const limited=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:390,height:844}})),page=await limited.newPage();
   const requests=[];
   page.on("request",request=>{
    if(/\/tools\/(?:inbox\/order-prefill|orders\/(?:manual\/options|for-buyer))$/.test(new URL(request.url()).pathname)) requests.push(request.method());
   });
   try {
    await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
    await expect(page.getByTestId("dashboard-page")).toBeVisible();
    // G-UI8 audit [READ/MEASURE]: real browser fetch carries Chromium's secure loopback cookies;
    // APIRequestContext uses a different HTTP cookie transport and would inspect an anonymous 401.
    const storesReply=await page.evaluate(async()=>{
     const response=await fetch("/api/stores",{credentials:"same-origin",cache:"no-store"});
     return {status:response.status,data:await response.json()};
    });
    assert.equal(storesReply.status,200);
    const stores=storesReply.data.items;
    assert(Array.isArray(stores),"real store list shape");
    for(const id of [store,process.env.LC_DRAWER_COMMENT_STORE]) {
     const selected=stores.find(value=>value.id===id);assert(selected,"real narrow store is visible");
     assert.equal(selected.role,"fulfilment","owner bypass cannot satisfy negative permission test");
     for(const permission of ["orders:read","inventory:reserve","inbox:read","live:read"]) assert(selected.permissions.includes(permission));
     assert(!selected.permissions.includes("catalog:read"));
    }
    for(const reload of [false,true]) {
     const buyer=page.waitForResponse(response=>response.request().method()==="GET"&&new URL(response.url()).pathname===`/api/stores/${store}/inbox/buyer-panel`&&new URL(response.url()).searchParams.get("conversation_id")===fixtures[0].Linked);
     if(reload) await page.reload();else await page.goto(`${origin}/${locale}/messages?store=${store}`);
     const [loaded]=await Promise.all([buyer,page.getByTestId(`conversation-${fixtures[0].Linked}`).click()]);
     assert.equal(loaded.status(),200);
     await expect(page.getByTestId("buyer-panel")).toBeVisible();
     await expect(page.getByTestId("buyer-create-order")).toBeDisabled();
     await expect(page.getByTestId("buyer-panel")).toContainText(locale==="en"?"Catalog-read":locale==="zh-TW"?"商品目錄":"商品目录");
     assert.deepEqual(requests,[],"real narrow role selection/reload sends no A15/options/A16");
    }
    await page.goto(`${origin}/${locale}/studio/console?store=${process.env.LC_DRAWER_COMMENT_STORE}&scene=${process.env.LC_DRAWER_COMMENT_SCENE}`);
    await expect(page.getByTestId(`comment-row-${process.env.LC_DRAWER_COMMENT_REF}`)).toBeVisible();
    await expect(page.getByTestId(`comment-create-order-${process.env.LC_DRAWER_COMMENT_REF}`)).toBeDisabled();
    await expect(page.getByTestId(`comment-row-${process.env.LC_DRAWER_COMMENT_REF}`)).toContainText(locale==="en"?"Catalog-read":locale==="zh-TW"?"商品目錄":"商品目录");
    await page.reload();
    await expect(page.getByTestId(`comment-row-${process.env.LC_DRAWER_COMMENT_REF}`)).toBeVisible();
    await expect(page.getByTestId(`comment-create-order-${process.env.LC_DRAWER_COMMENT_REF}`)).toBeDisabled();
    assert.deepEqual(requests,[],"real narrow CommentStream entry/reload sends no A15/options/A16");
    record("catalog permission entry gates","real signed login/conversation click/reload/console navigation","real fulfilment grants; buyer retained; create controls disabled; zero A15/options/A16");
    cases++;
   } finally { await closeContext(limited); }
  }
 } finally {
  const restored=await fetch(`${api}/__test/drawer-principal`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({limited:false})});
  assert.equal(restored.status,204,"restore owned fixture identity");
 }
 assert.equal(cases,29,"preserve every original case before adding partial recovery");
 current="DU3-partial-begin-recovery";
 const partialContext=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),partialPage=await partialContext.newPage();
 try {
  await partialPage.goto(`${origin}/en`);await partialPage.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
  await expect(partialPage.getByTestId("dashboard-page")).toBeVisible();
  await runPartialRecovery({page:partialPage,api,origin,store,cvs,sku,fixture:partialFixture,routePath,openConversation,record,results});
  cases++;
 } finally {await closeContext(partialContext);}
 assert.equal(cases,30,"preserve partial recovery and every original case before capability recovery");
 current="DU3-capability-after-unknown-recovery";
 const capabilityContext=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),capabilityPage=await capabilityContext.newPage();
 try {
  await capabilityPage.goto(`${origin}/en`);await capabilityPage.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
  await expect(capabilityPage.getByTestId("dashboard-page")).toBeVisible();
  await runCapabilityRecovery({page:capabilityPage,api,origin,store,home,sku,fixture:capabilityFixture,routePath,openConversation,record,results});cases++;
 } finally {await closeContext(capabilityContext);}
 assert.equal(cases,31,"preserve every existing case before adding ManualOrder recovery");
 for(const partial of [false,true]) {
  current=partial?"DU3-manual-partial-recovery":"DU3-manual-absence-recovery";
  const manualContext=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),manualPage=await manualContext.newPage();
  try {
   await manualPage.goto(`${origin}/en`);await manualPage.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
   await expect(manualPage.getByTestId("dashboard-page")).toBeVisible();
   await runManualRecovery({page:manualPage,api,origin,store,cvs,addCatalog,fillEmpty,record,results,partial});cases++;
  } finally {await closeContext(manualContext);}
 }
 assert.equal(cases,33,"exact additive ManualOrder recovery matrix");
 // Final money regression B: inject exactly one invalid generator value, then run the actual client's pre-dispatch validation.
 {
  current="DU3-never-sent-key-validation";
  const context=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),page=await context.newPage();
  let posts=0;
  const observe=request=>{if(request.method()==="POST"&&new URL(request.url()).pathname===routePath)posts++;};
  page.on("request",observe);
  try {
   await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();await expect(page.getByTestId("dashboard-page")).toBeVisible();
   let drawer=await openConversation(page,"en",fixtures[0].Unlinked);
   await addCatalog(drawer);await fillEmpty(drawer);await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
   const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
   // Disclosed key-generator fault only; do not change DOM, fetch, BFF responses or client code.
   await page.evaluate(()=>{const original=crypto.randomUUID;window.__lcInvalidKeyCalls=0;Object.defineProperty(crypto,"randomUUID",{configurable:true,value:()=>{window.__lcInvalidKeyCalls++;Object.defineProperty(crypto,"randomUUID",{configurable:true,value:original});return "invalid key";}});});
   await drawer.getByTestId("drawer-submit").click();
   await expect(drawer.getByTestId("drawer-error")).toBeVisible();
   assert.equal(await page.evaluate(()=>window.__lcInvalidKeyCalls),1,"scoped generator fault must actually be consumed by the first click");
   await expect(drawer.getByTestId("drawer-error")).toContainText("Some details are not valid. Check the form.");
   await expect(drawer.getByTestId("drawer-error")).not.toContainText("The order may already exist.");
   await expect(drawer.getByTestId("drawer-close")).toBeEnabled();
   assert.equal(posts,0,"actual client invalid key is rejected before any A16 POST");
   assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0,"never-sent attempt must not leave an armed guard");
   assert.equal((await (await fetch(`${api}/__test/drawer-facts`)).json()).orders,before.orders);
   record("never sent invalid key","inject one generator fault/click actual submit","actualclient invalid_request; zero A16 POST; non-UNKNOWN error; guard0; closeenabled");
   await drawer.getByTestId("drawer-close").click();await expect(page.getByTestId("create-order-drawer")).toHaveCount(0);
   drawer=await openConversation(page,"en",fixtures[0].Unlinked);await addCatalog(drawer);await fillEmpty(drawer);await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
   const created=await submit(page,drawer);assert.equal(created.status(),201);const value=await created.json();
   await expect(drawer.getByTestId("drawer-result")).toBeVisible();assert.equal(posts,1,"valid reopened intent sends exactly one A16 POST");
   const facts=await fetch(`${api}/__test/drawer-final-request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:[created.request().headers()["idempotency-key"]]})});
   assert.equal(facts.status,200);assert.deepEqual(await facts.json(),{orders:1,receipts:1});
   assert.equal((await (await fetch(`${api}/__test/drawer-facts`)).json()).orders-before.orders,1);
   assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0);
   await drawer.getByTestId("drawer-view-order").click();
   await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();await page.reload();
   await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
   results.push({case:current,invalid_generator_calls:1,invalid_posts:0,uncertain:false,guard_armed:false,valid_posts:1,pg_delta:1,reload_persisted:true});
   record("never sent close reopen persistence","click close/create/fill/valid submit/vieworder/reload","newvalidintent actual201; one persisted order/receipt; no guard or duplicate");cases++;
  } finally {page.off("request",observe);await closeContext(context);}
 }
 assert.equal(cases,34,"all33 previouscases plus actual pre-dispatch validation");
 // Final money regression A: a real lost successful reply and real401 must not be escaped by re-login/rotated CSRF.
 {
  current="DU3-unknown-auth-relogin-guard";
  const context=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),page=await context.newPage();
  const attempts=[];let posts=0,created;
  const observe=request=>{if(request.method()==="POST"&&new URL(request.url()).pathname===routePath)posts++;};
  const scopeFor=async id=>{const observed=await page.evaluate(async id=>{const reply=await fetch("/api/stores",{credentials:"same-origin",cache:"no-store"});const data=await reply.json();return {status:reply.status,scope:data.items?.find(item=>item.id===id)?.order_guard_scope};},id);assert.equal(observed.status,200);assert.match(observed.scope,/^[a-f0-9]{64}$/);return observed.scope;};
  const exact=async()=>{const response=await fetch(`${api}/__test/drawer-final-request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:[attempts[0].key]})});assert.equal(response.status,200);return response.json();};
  const guarded=async reload=>{
   if(reload)await page.reload();else await page.goto(`${origin}/en/messages?store=${store}`);
   await page.getByTestId(`conversation-${fixtures[0].Unlinked}`).click();await expect(page.getByTestId("buyer-panel")).toBeVisible();
   const [prefill]=await Promise.all([page.waitForResponse(r=>r.request().method()==="GET"&&new URL(r.url()).pathname===`/api/stores/${store}/tools/inbox/order-prefill`&&new URL(r.url()).searchParams.get("conversation_id")===fixtures[0].Unlinked),page.getByTestId("buyer-create-order").click()]);
   assert.equal(prefill.status(),200,"guard checks run only after real successful prefill");
   const drawer=page.getByTestId("create-order-drawer");await expect(drawer).toBeVisible();
   await expect(drawer.getByTestId("drawer-unresolved-guard")).toBeVisible();
   await expect(drawer.getByTestId("drawer-unresolved-guard")).toContainText("An earlier order request is unresolved.");
   await expect(drawer.getByTestId("drawer-form")).toHaveCount(0);await expect(drawer.getByTestId("drawer-submit")).toHaveCount(0);await expect(drawer.getByTestId("drawer-new-attempt")).toHaveCount(0);
   await expect(drawer.getByTestId("drawer-close")).toBeEnabled();
   assert.equal(posts,2,"guarded new drawer/reload cannot send fresh A16");assert.deepEqual(await exact(),{orders:1,receipts:1});
   return drawer;
  };
  page.on("request",observe);
  try {
   await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();await expect(page.getByTestId("dashboard-page")).toBeVisible();
   const originalScope=await scopeFor(store),before=await (await fetch(`${api}/__test/drawer-facts`)).json();
   const oldCSRF=(await context.cookies()).filter(cookie=>cookie.name==="__Host-commerce_csrf");assert.equal(oldCSRF.length,1);
   let drawer=await openConversation(page,"en",fixtures[0].Unlinked);await addCatalog(drawer);await fillEmpty(drawer);await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
   await page.route(`**${routePath}`,async route=>{attempts.push({key:route.request().headers()["idempotency-key"],body:route.request().postData()});const real=await route.fetch();if(attempts.length===1){assert.equal(real.status(),201);created=await real.json();await real.dispose();await route.abort("failed");}else await route.fulfill({response:real});});
   await drawer.getByTestId("drawer-submit").click();await expect(drawer.getByTestId("drawer-error")).toContainText("The order may already exist.");await expect(drawer.getByTestId("drawer-close")).toBeDisabled();
   assert.deepEqual(await exact(),{orders:1,receipts:1});
   // Disclosed authentication fault: remove only the browser's session cookie; next real BFF command returns401.
   await context.clearCookies({name:"__Host-commerce_session"});
   const [denied]=await Promise.all([page.waitForResponse(isSubmit),drawer.getByTestId("drawer-submit").click()]);assert.equal(denied.status(),401);
   assert.equal(attempts.length,2);assert.equal(attempts[1].key,attempts[0].key);assert.equal(attempts[1].body,attempts[0].body);
   await expect.poll(()=>page.evaluate(()=>{const surfaces=[document.documentElement.outerHTML,...[...document.querySelectorAll("input,textarea")].map(el=>el.value)];return ["SYNTHETIC Drawer Buyer","0912345678","drawer@example.invalid","Taipei Region","1 Synthetic Drawer Road","Synthetic Floor 1"].every(value=>surfaces.every(surface=>!surface.includes(value)));}),{message:"real401 must clear every private contact/address field"}).toBe(true);
   assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),1,"UNKNOWN auth clear preserves opaque guard");assert.equal(posts,2);assert.deepEqual(await exact(),{orders:1,receipts:1});
   await page.unroute(`**${routePath}`);
   record("unknown real auth clear","real201/drop reply/remove session cookie/click original retry","actual401 samekey/body; privatefields cleared; opaqueguard1; one durable order/receipt");
   await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();await expect(page.getByTestId("dashboard-page")).toBeVisible();
   const newCSRF=(await context.cookies()).filter(cookie=>cookie.name==="__Host-commerce_csrf");assert.equal(newCSRF.length,1);assert.match(newCSRF[0].value,/^[A-Za-z0-9_-]{43}$/);assert.notEqual(newCSRF[0].value,oldCSRF[0].value,"actual OIDC login must rotate valid CSRF");
   assert.equal(await scopeFor(store),originalScope,"same actual issuer/subject/principal/store must keep stable order authority scope");
   drawer=await guarded(false);record("relogin stable unresolved guard","actual OIDC sign-in/newvalidCSRF/click buyer create","same serverguardscope; positive loadedguard; noform/newkey/POST; exactone order");
   drawer=await guarded(true);record("reloaded stable unresolved guard","reload/click conversation/create","positive loadedguard afterreload; nofresh A16; exactone originalorder/receipt");
   await drawer.getByTestId("drawer-unresolved-guard").getByRole("link").click();await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${created.order_id}`)).toBeVisible();
   // The same authenticated principal can still load an independent store's real create form.
   const otherStore=process.env.LC_DRAWER_COMMENT_STORE;assert.notEqual(await scopeFor(otherStore),originalScope);
   await page.goto(`${origin}/en/studio/console?store=${otherStore}&scene=${process.env.LC_DRAWER_COMMENT_SCENE}`);await page.getByTestId(`comment-create-order-${process.env.LC_DRAWER_COMMENT_REF}`).click();
   drawer=page.getByTestId("create-order-drawer");await expect(drawer).toBeVisible();await expect(drawer.getByTestId("drawer-form")).toBeVisible();await expect(drawer.getByTestId("drawer-unresolved-guard")).toHaveCount(0);await drawer.getByTestId("drawer-close").click();
   assert.equal(posts,2);assert.deepEqual(await exact(),{orders:1,receipts:1});assert.equal((await (await fetch(`${api}/__test/drawer-facts`)).json()).orders-before.orders,1);
   results.push({case:current,attempts:2,same_key:true,same_body:true,real_auth_status:401,csrf_rotated:true,stable_scope:true,relogin_guard_loaded:true,reload_guard_loaded:true,other_store_unaffected:true,pg_delta:1,request_orders:1,request_receipts:1});
   record("guard scope and persisted order","click guard vieworders/otherstore comment create/close","original ordervisible; independentstore formloaded; zero newwrites; PGdelta1");cases++;
  }finally{await page.unroute(`**${routePath}`);page.off("request",observe);await closeContext(context);}
 }
 assert.equal(cases,35,"exact additive final money matrix preservesall33previouscases");
 const priorCreated=await fetch(`${api}/__test/drawer-facts`);assert.equal(priorCreated.status,200);
 assert.equal((await priorCreated.json()).orders,20,"preserve exact previous35-case PG20 before created recovery");
 current="DU3-manual-created-recovery";
 const createdContext=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),createdPage=await createdContext.newPage();
 try {
  await createdPage.goto(`${origin}/en`);await createdPage.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
  await expect(createdPage.getByTestId("dashboard-page")).toBeVisible();
  await runManualCreatedRecovery({page:createdPage,api,origin,store,cvs,addCatalog,fillEmpty,record,results});cases++;
 } finally {await closeContext(createdContext);}
 assert.equal(cases,36,"preserve all35 prior cases and add created ManualOrder recovery detail persistence");
 // PR32 pre-dispatch fence: fault only the browser's CSRF primitive, then click the production submit.
 // All request parsing, session hashing, clients, BFF and business writes remain the actual deployed flow.
 for(const kind of ["drawer","manual"]) for(const priorUnknown of [false,true]) {
  current=`DU3-${kind}-prefetch-fence-${priorUnknown?"retained-unknown":"never-sent"}`;
  const context=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:{width:1440,height:900}})),page=await context.newPage();
  const commandPath=kind==="drawer"?routePath:`/api/stores/${store}/tools/orders/manual`;
  const submitId=kind==="drawer"?"drawer-submit":"manual-order-submit",errorId=kind==="drawer"?"drawer-error":"manual-order-error";
  let posts=0,first;
  const observe=request=>{if(request.method()==="POST"&&new URL(request.url()).pathname===commandPath)posts++;};
  const guardCount=()=>page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length);
  const openEntry=async()=>{
   if(kind==="drawer") return openConversation(page,"en",fixtures[0].Unlinked);
   await page.goto(`${origin}/en/orders/new?store=${store}`);
   const form=page.getByTestId("manual-order-form");await expect(form).toBeVisible();return form;
  };
  page.on("request",observe);
  try {
   await page.goto(`${origin}/en`);await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();await expect(page.getByTestId("dashboard-page")).toBeVisible();
   const oldCSRF=(await context.cookies()).filter(cookie=>cookie.name==="__Host-commerce_csrf");assert.equal(oldCSRF.length,1);
   const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
   let form=await openEntry();await addCatalog(form,kind==="drawer"?"drawer":"mo");await fillEmpty(form,kind==="drawer"?"drawer":"mo");await expect(form.getByTestId(submitId)).toBeEnabled();
   if(priorUnknown) {
    // Drop only a real successful response. The original key/body remain in the component's attempt.
    await page.route(`**${commandPath}`,async route=>{const real=await route.fetch();assert.equal(real.status(),201);first={key:route.request().headers()["idempotency-key"],body:route.request().postData()};await real.dispose();await route.abort("failed");});
    await form.getByTestId(submitId).click();await expect(form.getByTestId(errorId)).toBeVisible();assert.equal(posts,1);assert.equal(await guardCount(),1);
    await page.unroute(`**${commandPath}`);
    await context.addCookies([{...oldCSRF[0],value:"B".repeat(43)}]);
   } else {
    // Disclosed one-shot fault: a valid UUID is returned, but CSRF changes between prepare and fetch.
    await page.evaluate(()=>{const original=crypto.randomUUID;window.__lcPrefetchFenceCalls=0;Object.defineProperty(crypto,"randomUUID",{configurable:true,value:()=>{const key=original.call(crypto);window.__lcPrefetchFenceCalls++;Object.defineProperty(crypto,"randomUUID",{configurable:true,value:original});document.cookie=`__Host-commerce_csrf=${"B".repeat(43)}; Path=/; Secure; SameSite=Lax`;return key;}});});
   }
   await form.getByTestId(submitId).click();
   await expect.poll(()=>page.evaluate(()=>[...document.querySelectorAll("input,textarea")].every(el=>!el.value.includes("SYNTHETIC Drawer Buyer"))),{message:"pre-fetch session loss must clear private fields"}).toBe(true);
   if(!priorUnknown) assert.equal(await page.evaluate(()=>window.__lcPrefetchFenceCalls),1,"fault actually consumed inside prepare");
   await expect.poll(guardCount,{message:"fresh never-sent must discharge before revoke; existing UNKNOWN must remain"}).toBe(priorUnknown?1:0);
   assert.equal(posts,priorUnknown?1:0,"fence failure dispatches no additional command");
   assert.equal((await (await fetch(`${api}/__test/drawer-facts`)).json()).orders-before.orders,priorUnknown?1:0,"server order count matches only actually dispatched intent");
   await context.addCookies(oldCSRF);
   if(priorUnknown) {
    if(kind==="manual") {await page.goto(`${origin}/en/orders/new?store=${store}`);await expect(page.getByTestId("manual-order-guard")).toBeVisible();await page.reload();await expect(page.getByTestId("manual-order-guard")).toBeVisible();await expect(page.getByTestId("manual-order-form")).toHaveCount(0);}
    else {await page.goto(`${origin}/en/messages?store=${store}`);await page.getByTestId(`conversation-${fixtures[0].Unlinked}`).click();await expect(page.getByTestId("buyer-panel")).toBeVisible();await page.getByTestId("buyer-create-order").click();await expect(page.getByTestId("drawer-unresolved-guard")).toBeVisible();await expect(page.getByTestId("drawer-form")).toHaveCount(0);await page.reload();await page.getByTestId(`conversation-${fixtures[0].Unlinked}`).click();await expect(page.getByTestId("buyer-panel")).toBeVisible();await page.getByTestId("buyer-create-order").click();await expect(page.getByTestId("drawer-unresolved-guard")).toBeVisible();}
    const exact=await fetch(`${api}/__test/${kind==="drawer"?"drawer":"manual"}-prefetch-request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:[first.key]})});assert.equal(exact.status,200);assert.deepEqual(await exact.json(),{orders:1,receipts:1});
   } else {
    form=await openEntry();await addCatalog(form,kind==="drawer"?"drawer":"mo");await fillEmpty(form,kind==="drawer"?"drawer":"mo");await expect(form.getByTestId(submitId)).toBeEnabled();
    await page.reload();form=await openEntry();await expect(form).toBeVisible();assert.equal(await guardCount(),0,"no orphan UX guard after restore/reload");
   }
   assert.equal(posts,priorUnknown?1:0);
   record(`${kind} pre-fetch session fence`,"real submit/CSRF fault/restore/reload",priorUnknown?"one real lost201; retry sends0; UNKNOWN guard1 survives reload; exactone order":"new intent fetch0/order0; guard0 before privacy teardown; restored form survives reload");
   results.push({case:current,prior_unknown:priorUnknown,posts,preflight_posts:0,guard_count:priorUnknown?1:0,pg_delta:priorUnknown?1:0,reload_persisted:true});cases++;
  } finally {await page.unroute(`**${commandPath}`);page.off("request",observe);await closeContext(context);}
 }
 assert.equal(cases,40,"all36 prior cases plus drawer/manual fresh and UNKNOWN pre-dispatch fence clicks");
} catch (error) {
 recordFailure(error);
} finally {
 // Preserve the primary failure before teardown can dispose a still-running route.fetch.
 try {
  await saveEvidence();
  if(browser) {
   try {for(const context of browser.contexts()) await drainContext(context);}
   catch(error){recordFailure(error,"route drain");}
   finally {try {await browser.close();}catch(error){recordFailure(error,"browser close");}}
  }
 } finally {
  try {
   if(child && child.exitCode===null) {child.kill("SIGTERM");await Promise.race([new Promise(resolve=>child.once("exit",resolve)),wait(1500)]);if(child.exitCode===null) child.kill("SIGKILL");}
   await saveEvidence();
  } finally {log.end();}
 }
}
for(const name of failed) console.error(`FAIL ${name}`);
console.log(`drawer cases=${cases} failed=${failed.length}`);
if(failed.length) process.exitCode=1;
