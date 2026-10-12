// Purpose: independent LC-U3 browser gate against production Next/BFF and real Go/PG, with a named retry calibration.
// Depends on: Go-owned LC_DRAWER_* fixture, signed MOCK OIDC, standalone admin build and installed Playwright.
// Used by: TestBrowserManualOrderLink after unchanged legacy cases; no trace/HAR/body/clipboard logging.
// Invariants: I02/I03/I06/I09/I11; only fixed case names, status/counts and synthetic screenshots enter evidence.
import assert from "node:assert/strict";
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
assert(evidence && store && sku && home && fixtures.length===6);
await mkdir(evidence,{recursive:true});
const ledger=[], failed=[], results=[];
let browser, child, cases=0, current="DU3-setup";
const log=createWriteStream(path.join(evidence,"admin.log"),{flags:"wx",mode:0o600});
// spawn requires an opened file descriptor; passing a pending WriteStream throws before Next starts.
await once(log,"open");
const record=(control,action,actual)=>ledger.push({case:current,control,action,actual,status:"PASS"});
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
 let posts=0;
 const observe=request=>{if(request.method()==="POST"&&new URL(request.url()).pathname===requestPath) posts++;};
 page.on("request",observe);
 const attempts=[];
 if(unknownFirst) await form.getByTestId(`drawer-quantity-${sku}`).fill("1000");
 // UNKNOWN drops only the reply to the merchant's frozen request. The separate 422 case mutates quantity at the network edge.
 await page.route(`**${requestPath}`,async route=>{
  attempts.push({key:route.request().headers()["idempotency-key"],body:route.request().postData()});
  const body=JSON.parse(route.request().postData());
  if(unknownFirst) assert.equal(body.items[0].quantity,1000,"real UI supplied the immutable inventory-refused intent");
  else body.items[0].quantity=0;
  const real=await route.fetch(unknownFirst?{}:{postData:JSON.stringify(body)});
  assert.equal(real.status(),unknownFirst?409:422,"real BFF/Go must refuse invalid quantity or insufficient inventory");
  if(unknownFirst) assert.equal((await real.json()).code,"insufficient_inventory","actual checkout refusal after durable receipt lookup");
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
 assert.equal(refusal.status(),unknownFirst?409:422);assert(refusal.headers()["cache-control"]?.includes("no-store"));
 await expect(form.getByTestId(errorId)).toBeVisible();
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
 assert.equal(after.orders,before.orders,"explicit new-attempt action creates zero orders");
 page.off("request",observe);
 record(newAttemptId,unknownFirst?"real UNKNOWN/409 then explicit click":"real 422 then explicit click","zero merchant-manual order delta; submit remains disabled after refusal/draft edit; new-attempt enables without network write");
 if(unknownFirst) { record("UNKNOWN then coded refusal","abort real inventory 409/replay/click new attempt","same key/body; zero PG orders/receipts; OrderGuard released; close enabled");cases++; }
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
  await context.close();
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
   drawer=await openConversation(page,locale,fixture.Unlinked);
   await addCatalog(drawer);await fillEmpty(drawer);
   await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
   record("settled guard close/reopen","click close then create","new drawer can submit after refused intent");
  }
  await page.screenshot({path:path.join(evidence,`unlinked-${locale}-${width}.png`),fullPage:true});
  record("unlinked warning/empty fields/zero","edit form","no customer guessing; quantity zero removes; valid catalog line re-added");cases++;

  current=`DU3-single-order-${locale}-${width}`;
  const queued=locale==="en"&&width===1440;
  let committed, originalLink="", regeneratePosts=0;
  const observeRegenerate=request=>{if(request.method()==="POST"&&new URL(request.url()).pathname.endsWith("/orders/manual/regenerate-link")) regeneratePosts++;};
  if(queued) {
   // Fixture-only window control; acceptance itself remains real BuyerPanel/form clicks.
   assert.equal((await fetch(`${api}/__test/drawer-queued-window`,{method:"POST"})).status,204);
   await drawer.getByTestId("drawer-close").click();drawer=await openConversation(page,locale,fixture.Unlinked);
   await addCatalog(drawer);await fillEmpty(drawer);await drawer.getByTestId("drawer-send-payment-link").check();
   page.on("request",observeRegenerate);
  }
  const before=await (await fetch(`${api}/__test/drawer-facts`)).json();
  const attempts=[];let armed=true;
  await page.route(`**${routePath}`,async route=>{
   const req=route.request(), headers={...req.headers()};
   if(!armed && calibration==="new-key-on-retry" && locale==="en" && width===1440) headers["idempotency-key"]=crypto.randomUUID();
   const body=req.postData();attempts.push({key:headers["idempotency-key"],body});
   const real=await route.fetch({headers});
   if(armed){armed=false;assert.equal(real.status(),201,"lost reply first commits through real Go");
    if(queued){committed=await real.json();assert.equal(committed.send.state,"queued");assert(committed.send.operation_id);originalLink=committed.buyer_link;assert(typeof originalLink==="string");}
    await real.dispose();await route.abort("failed");}
   else await route.fulfill({response:real});
  });
  await drawer.getByTestId("drawer-submit").click();
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
   assert.equal(facts.status,200);const dm=await facts.json();assert.deepEqual(dm,{operations:1,secrets:1,original_link:true,valid_links:1,regenerations:0});assert.equal(regeneratePosts,0);
   results.push({case:current,queued_replay:true,copy_blocked:copyBlocked,regenerate_posts:regeneratePosts,...dm});
   // Reload and click the persisted order, using a positive loaded table and detail sentinel.
   await page.goto(`${origin}/${locale}/orders?store=${store}`);await page.reload();
   await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
   await page.getByTestId(`order-expand-${value.order_id}`).click();await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(value.order_id));
   record("queued order persistence","reload orders then click exact order","same persisted order row and visible detail");
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
   } finally { await limited.close(); }
  }
 } finally {
  const restored=await fetch(`${api}/__test/drawer-principal`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({limited:false})});
  assert.equal(restored.status,204,"restore owned fixture identity");
 }
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
