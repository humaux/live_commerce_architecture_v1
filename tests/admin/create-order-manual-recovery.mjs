// Purpose: independently verify ManualOrder's authoritative absence, partial-Begin and created recovery UI.
// Depends on: real signed admin session, existing manual form click helpers and scoped Go/PG fixture controls.
// Used by: create-order-drawer-gate.mjs after all unchanged 31 cases.
// Invariants: I02/I03/I06/I11; frozen bodies/keys remain in memory, retained evidence contains counts/status/booleans only.
import assert from "node:assert/strict";
import { expect } from "@playwright/test";

export async function runManualRecovery({page,api,origin,store,cvs,addCatalog,fillEmpty,record,results,partial=false}) {
 const path=`/api/stores/${store}/tools/orders/manual`,recoveryPath=`${path}/recover`;
 const isPost=(r,p=path)=>r.request().method()==="POST"&&new URL(r.url()).pathname===p;
 const control=async(name,body={})=>{
  const reply=await fetch(`${api}/__test/manual-recovery/${name}`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify(body)});
  assert.equal(reply.status,200,`owned manual ${name} control`);return reply.json();
 };
 const bank=async enabled=>assert.equal((await fetch(`${api}/__test/drawer-bank-transfer`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({enabled})})).status,204);
 const counts=async()=>{const reply=await fetch(`${api}/__test/drawer-facts`);assert.equal(reply.status,200);return reply.json();};
 const caseName=partial?"DU3-manual-partial-recovery":"DU3-manual-absence-recovery";
 const guardCount=()=>page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length);
 await page.goto(`${origin}/en/orders/new?store=${store}`);
 const form=page.getByTestId("manual-order-form");await expect(form).toBeVisible();
 await addCatalog(form,"mo");await fillEmpty(form,"mo");
 // Real CVS remains a valid delivery row via pay_at_pickup when only bank transfer is disabled.
 await form.getByTestId("mo-option").selectOption(cvs);
 await form.getByTestId("mo-store-code").fill("123456");
 await form.getByTestId("mo-store-name").fill("SYNTHETIC Manual Recovery Pickup");
 await form.getByTestId("mo-store-address").fill("1 Synthetic Recovery Road");
 await form.getByTestId("mo-mode-bank_transfer").check();
 await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
 const before=await counts(),attempts=[],completedReplies=[];let posts=0,recoveryPosts=0,firstFacts;
 const observe=request=>{if(request.method()==="POST"){const pathname=new URL(request.url()).pathname;if(pathname===path)posts++;if(pathname===recoveryPath)recoveryPosts++;}};
 page.on("request",observe);
 if(partial)assert.deepEqual(await control("arm"),{armed:1});else await bank(false);
 const handleAttempt=async route=>{
  const request=route.request();attempts.push({key:request.headers()["idempotency-key"],body:request.postData()});
  if(attempts.length===1){
   let real;
   if(partial){const replies=await Promise.all([route.fetch(),control("kill",{key:attempts[0].key})]);real=replies[0];firstFacts=replies[1];assert.equal(real.status(),503);assert.deepEqual(firstFacts,{killed:true,begin:1,bindings:1,orders:1,manual_source:0,links:0,valid_links:0,manual_receipts:0,for_buyer_receipts:0,completion_receipts:0,recovery_receipts:0,closures:0,manual_audits:0,for_buyer_audits:0,pending_bundles:0,placed_bundles:0,operations:0,secrets:0,jobs:0,dm_valid_links:0});}
   else {real=await route.fetch();assert.equal(real.status(),422);assert.equal((await real.json()).code,"bank_transfer_unavailable");}
   // Drop only the actual server reply; no mock HTTP refusal/recovery is substituted.
   await real.dispose();await route.abort("failed");
  }else if(partial){
   // A durable Begin resumes from its immutable checkout, even after a delivery/payment option disappears.
   // Lose only the real 201 replies so the merchant must explicitly recover the completed original request.
   const real=await route.fetch();assert.equal(real.status(),201);const value=await real.json();assert(value.order_id);
   completedReplies.push(value);await real.dispose();await route.abort("failed");
  }else await route.fulfill({response:await route.fetch()});
 };
 await page.route(`**${path}`,handleAttempt);
 try {
  await form.getByTestId("manual-order-submit").click();await expect(form.getByTestId("manual-order-error")).toBeVisible();
  await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);
  await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
  if(partial)await bank(false);
  await form.getByTestId("mo-name").fill("SYNTHETIC Recovery Edited Draft");
  await expect(form.getByTestId("mo-name")).toHaveValue("SYNTHETIC Recovery Edited Draft");
  if(partial){
   record("manual partial Begin fault","submit/terminate exact PG waiter/drop actual503/edit draft","one real Begin/order; source/link/manual receipt absent; edited draft creates no new request");
   await expect(form.getByTestId("manual-order-recover-attempt")).toBeVisible();assert.equal(await guardCount(),1);
   const [unknown]=await Promise.all([page.waitForResponse(r=>isPost(r,recoveryPath)),form.getByTestId("manual-order-recover-attempt").click()]);
   assert.equal(unknown.status(),503);assert.equal((await unknown.json()).code,"retry_later");
   assert.equal(unknown.request().headers()["idempotency-key"],attempts[0].key);assert.equal(unknown.request().postData(),attempts[0].body);
   assert(unknown.headers()["cache-control"]?.includes("no-store"));
   await expect(form.getByTestId("manual-order-error")).toBeVisible();await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
   await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);await expect(form.getByTestId("manual-order-recovered-order")).toHaveCount(0);
   await expect(page.getByTestId("manual-order-result")).toHaveCount(0);assert.equal(await guardCount(),1);
   assert.equal(posts,1);assert.equal(recoveryPosts,1);
   const {killed,...initial}=firstFacts;assert.deepEqual(await control("facts",{key:attempts[0].key}),initial,"partial recovery cannot invent upper completion or seal");
   assert.deepEqual(await control("request-facts",{keys:[attempts[0].key]}),{orders:0,receipts:0});
   record("manual partial authority","click original recovery","actual503 retry_later; no seal/result/newattempt; original frozen request remains retryable");
   const complete={...initial,manual_source:1,links:1,valid_links:1,manual_receipts:1,manual_audits:1};
   for(let index=1;index<=2;index++){
    await form.getByTestId("manual-order-submit").click();
    await expect.poll(()=>completedReplies.length).toBe(index);
    await expect(form.getByTestId("manual-order-error")).toBeVisible();await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
    assert.equal(attempts.length,index+1);assert.equal(attempts[index].key,attempts[0].key);assert.equal(attempts[index].body,attempts[0].body);
    assert.equal(completedReplies[index-1].order_id,completedReplies[0].order_id,"same frozen intent resumes/replays the original order");
    await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);await expect(page.getByTestId("manual-order-result")).toHaveCount(0);
    assert.equal(await guardCount(),1);assert.equal((await counts()).orders-before.orders,1);
    assert.deepEqual(await control("facts",{key:attempts[0].key}),complete,"resume/replay produces exactly one source/link/audit/receipt");
    assert.deepEqual(await control("request-facts",{keys:[attempts[0].key]}),{orders:1,receipts:1});
   }
   await expect(form.getByTestId("manual-order-recover-attempt")).toBeVisible();
   const [created]=await Promise.all([page.waitForResponse(r=>isPost(r,recoveryPath)),form.getByTestId("manual-order-recover-attempt").click()]);
   assert.equal(created.status(),200);assert.deepEqual(await created.json(),{state:"created",order_id:completedReplies[0].order_id});
   assert.equal(created.request().headers()["idempotency-key"],attempts[0].key);assert.equal(created.request().postData(),attempts[0].body);
   assert(created.headers()["cache-control"]?.includes("no-store"));
   await expect(page.getByTestId("manual-order-recovery-status")).toBeVisible();
   const viewOrder=page.getByTestId("manual-order-recovered-order");await expect(viewOrder).toBeVisible();await expect(viewOrder).toHaveText("View order");
   await expect(viewOrder).toHaveAttribute("href",`/en/orders?store=${store}&order=${completedReplies[0].order_id}`);
   await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);await expect(page.getByTestId("manual-order-result")).toHaveCount(0);
   assert.equal(await guardCount(),0);assert.equal(posts,3);assert.equal(recoveryPosts,2);
   await page.unroute(`**${path}`,handleAttempt);
   await viewOrder.click();await page.waitForURL(url=>url.pathname==="/en/orders"&&url.searchParams.get("store")===store&&url.searchParams.get("order")===completedReplies[0].order_id);
   await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${completedReplies[0].order_id}`)).toBeVisible();
   await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",`Order ${completedReplies[0].order_id}`);
   await page.reload();await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${completedReplies[0].order_id}`)).toBeVisible();
   await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",`Order ${completedReplies[0].order_id}`);
   assert.deepEqual(await control("facts",{key:attempts[0].key}),complete);assert.equal((await counts()).orders-before.orders,1);
   assert.deepEqual(await control("request-facts",{keys:[attempts[0].key]}),{orders:1,receipts:1});
   assert.equal(attempts.length,3);assert.equal(posts,3);assert.equal(recoveryPosts,2);
   results.push({case:caseName,attempts:3,same_key:true,same_body:true,first_recovery_status:503,completed_response_losses:2,resume_status:201,recover_status:200,recovered_original:true,seals:0,pg_delta:1,request_orders:1,request_receipts:1,source:1,links:1,audits:1,reload_detail_identity:true});
   record("manual partial retry persistence","retry same frozen intent twice/drop actual201/click created recovery/view order/reload","bank remains disabled; same original order completes/replays; source/link/audit/receipt1; exact query detail persists; no new intent");
   return;
  }
  const [refusal]=await Promise.all([page.waitForResponse(r=>isPost(r)),form.getByTestId("manual-order-submit").click()]);
  assert.equal(refusal.status(),422);assert.equal((await refusal.json()).code,"bank_transfer_unavailable");
  assert.equal(attempts.length,2);assert.equal(attempts[1].key,attempts[0].key);assert.equal(attempts[1].body,attempts[0].body);
  await expect(form.getByTestId("manual-order-error")).toBeVisible();
  await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);
  results.push({case:caseName,refusal_status:422,same_key:true,same_body:true,partial:false});
  // RED target4254715306: existing ManualOrder retains the attempt but exposes no recovery control.
  await expect(form.getByTestId("manual-order-recover-attempt")).toBeVisible();
  assert.equal(await guardCount(),1,"retained manual attempt keeps opaque guard");
  record("manual frozen refusal","load options/disable bank/submit/drop reply/edit/retry","two actual422; edited draft retains exact original key/body; no new attempt");
  const [recovery]=await Promise.all([page.waitForResponse(r=>isPost(r,recoveryPath)),form.getByTestId("manual-order-recover-attempt").click()]);
  assert.equal(recovery.request().headers()["idempotency-key"],attempts[0].key);assert.equal(recovery.request().postData(),attempts[0].body);
  assert(recovery.headers()["cache-control"]?.includes("no-store"));
  const seal=await fetch(`${api}/__test/drawer-recovery-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({key:attempts[0].key})});
  assert.equal(seal.status,200);
  const exactBefore=await control("request-facts",{keys:[attempts[0].key]});assert.deepEqual(exactBefore,{orders:0,receipts:0});
  {
   assert.equal(recovery.status(),200);assert.deepEqual(await recovery.json(),{state:"not_created"});
   assert.deepEqual(await seal.json(),{not_created_seals:1});
   await expect(form.getByTestId("manual-order-recovery-status")).toBeVisible();
   await expect(form.getByTestId("manual-order-recovered-order")).toHaveCount(0);
   await expect(form.getByTestId("manual-order-new-attempt")).toBeVisible();
   await expect(form.getByTestId("manual-order-submit")).toBeDisabled();
   assert.equal(await guardCount(),0,"verified not_created seal clears opaque guard");
   assert.equal((await counts()).orders,before.orders,"sealed absence creates zero orders");
   // Direct authenticated exact replay is only a seal probe, never a replacement for UI click acceptance.
   await page.unroute(`**${path}`,handleAttempt);
   const blocked=await page.evaluate(async({path,key,body})=>{const token=decodeURIComponent(document.cookie.split("; ").find(row=>row.startsWith("__Host-commerce_csrf="))?.split("=").slice(1).join("=")??"");const response=await fetch(path,{method:"POST",headers:{"content-type":"application/json","idempotency-key":key,"x-csrf-token":token},body});return {status:response.status,code:(await response.json()).code};},{path,key:attempts[0].key,body:attempts[0].body});
   assert.deepEqual(blocked,{status:409,code:"attempt_closed"},"original-key direct replay cannot bypass durable seal");
   await page.route(`**${path}`,handleAttempt);
   const priorPosts=posts,priorRecoveries=recoveryPosts;
   await form.getByTestId("manual-order-new-attempt").click();
   await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
   assert.equal(posts,priorPosts,"new intent is local and does not POST");assert.equal(recoveryPosts,priorRecoveries);
   assert.equal((await counts()).orders,before.orders);
   assert.deepEqual(await control("request-facts",{keys:[attempts[0].key]}),{orders:0,receipts:0});
   record("manual absence seal and new intent","click recover/probe closed replay/click new attempt","actual200 not_created; PGseal1 and zeroorders; oldkey409 attempt_closed; explicitnewintent sends noPOST");
   await bank(true);
  }
  const [created]=await Promise.all([page.waitForResponse(r=>isPost(r)),form.getByTestId("manual-order-submit").click()]);
  assert.equal(created.status(),201);const value=await created.json();assert(value.order_id);
  assert.equal(attempts.length,3);assert.equal(posts,4,"only explicit submits and disclosed closed-key probe");assert.equal(recoveryPosts,1,"only explicit recovery click");
  assert.notEqual(attempts[2].key,attempts[0].key);assert.notEqual(attempts[2].body,attempts[0].body,"explicit new attempt adopts edited draft");
  await expect(page.getByTestId("manual-order-result")).toBeVisible();
  assert.equal(await guardCount(),0,"successful original/new intent clears opaque guard");
  await page.unroute(`**${path}`);
  assert.equal((await counts()).orders-before.orders,1);
  const completed=await control("request-facts",{keys:[attempts[2].key]});assert.deepEqual(completed,{orders:1,receipts:1});
  results.push({case:caseName,checkpoint:"manual_created",request_orders:1,request_receipts:1,pg_delta:1});
  // A Next Link click can return before its client navigation commits. Wait for the real loaded order identity before reload.
  const viewOrders=page.getByTestId("manual-order-result").getByRole("link");
  await expect(viewOrders).toHaveAttribute("href",`/en/orders?store=${store}`);
  await viewOrders.click();
  await page.waitForURL(url=>url.pathname==="/en/orders"&&url.searchParams.get("store")===store);
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
  results.push({case:caseName,checkpoint:"orders_loaded",saved_row:true});
  await page.reload();
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
  results.push({case:caseName,checkpoint:"orders_reloaded",saved_row:true});
  await page.getByTestId(`order-expand-${value.order_id}`).click();await expect(page.getByTestId("order-detail")).toBeVisible();
  await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(value.order_id));
  assert.deepEqual(await control("request-facts",{keys:[attempts[2].key]}),completed);assert.equal((await counts()).orders-before.orders,1);
  results.push({case:caseName,attempts:3,same_key:false,new_key:true,recover_status:200,seals:1,pg_delta:1,request_orders:1,request_receipts:1,reload_persisted:true});
  record("manual new intent persistence","restore bank/submit/click vieworders/reload/expand exact order","exactone persisted manual order and receipt; no duplicate onreload");
 }finally{await page.unroute(`**${path}`);page.off("request",observe);await bank(true);}
}

// A completed upper receipt replays before dynamic options; bank removal cannot yield a real retry422.
// Recover the genuinely UNKNOWN lost201 directly, with its original frozen request and current bank disabled.
export async function runManualCreatedRecovery({page,api,origin,store,cvs,addCatalog,fillEmpty,record,results}) {
 const caseName="DU3-manual-created-recovery",path=`/api/stores/${store}/tools/orders/manual`,recoveryPath=`${path}/recover`;
 const isPost=(r,p=path)=>r.request().method()==="POST"&&new URL(r.url()).pathname===p;
 const bank=async enabled=>assert.equal((await fetch(`${api}/__test/drawer-bank-transfer`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({enabled})})).status,204);
 const counts=async()=>{const reply=await fetch(`${api}/__test/drawer-facts`);assert.equal(reply.status,200);return reply.json();};
 const exact=async()=>{const reply=await fetch(`${api}/__test/manual-recovery/request-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({keys:[attempts[0].key]})});assert.equal(reply.status,200);return reply.json();};
 const guardCount=()=>page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length);
 await page.goto(`${origin}/en/orders/new?store=${store}`);
 const form=page.getByTestId("manual-order-form");await expect(form).toBeVisible();
 await addCatalog(form,"mo");await fillEmpty(form,"mo");
 await form.getByTestId("mo-option").selectOption(cvs);
 await form.getByTestId("mo-store-code").fill("123456");
 await form.getByTestId("mo-store-name").fill("SYNTHETIC Created Recovery Pickup");
 await form.getByTestId("mo-store-address").fill("1 Synthetic Recovery Road");
 await form.getByTestId("mo-mode-bank_transfer").check();
 await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
 const before=await counts(),attempts=[];let committed,posts=0,recoveryPosts=0;
 const observe=request=>{if(request.method()==="POST"){const pathname=new URL(request.url()).pathname;if(pathname===path)posts++;if(pathname===recoveryPath)recoveryPosts++;}};
 const handleAttempt=async route=>{
  const request=route.request();attempts.push({key:request.headers()["idempotency-key"],body:request.postData()});
  const real=await route.fetch();
  if(attempts.length===1){
   assert.equal(real.status(),201);committed=await real.json();assert(committed.order_id);
   assert.deepEqual(await exact(),{orders:1,receipts:1});
   await real.dispose();await route.abort("failed");
  }else await route.fulfill({response:real});
 };
 page.on("request",observe);await page.route(`**${path}`,handleAttempt);
 try {
  await form.getByTestId("manual-order-submit").click();await expect(form.getByTestId("manual-order-error")).toBeVisible();
  await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);
  assert.equal(await guardCount(),1);
  await bank(false);
  await form.getByTestId("mo-name").fill("SYNTHETIC Created Recovery Edited Draft");
  await expect(form.getByTestId("mo-name")).toHaveValue("SYNTHETIC Created Recovery Edited Draft");
  assert.equal(attempts.length,1);assert.equal(posts,1);assert.equal(recoveryPosts,0,"draft edits and bank removal send no order/recovery request");
  await expect(form.getByTestId("manual-order-error")).toBeVisible();await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);
  await expect(form.getByTestId("manual-order-recover-attempt")).toBeVisible();assert.equal(await guardCount(),1);
  record("manual committed response loss","submit/drop only actual201 reply/disable bank/edit draft","genuine UNKNOWN retains original frozen request; no retry/new attempt; PG one order/receipt");
  const [recovery]=await Promise.all([page.waitForResponse(r=>isPost(r,recoveryPath)),form.getByTestId("manual-order-recover-attempt").click()]);
  assert.equal(recovery.status(),200);assert.deepEqual(await recovery.json(),{state:"created",order_id:committed.order_id});
  assert.equal(recovery.request().headers()["idempotency-key"],attempts[0].key);assert.equal(recovery.request().postData(),attempts[0].body);
  assert(recovery.headers()["cache-control"]?.includes("no-store"));
  await expect(page.getByTestId("manual-order-recovery-status")).toBeVisible();
  const viewOrder=page.getByTestId("manual-order-recovered-order");await expect(viewOrder).toBeVisible();await expect(viewOrder).toHaveText("View order");
  await expect(page.getByTestId("manual-order-new-attempt")).toHaveCount(0);await expect(page.getByTestId("manual-order-result")).toHaveCount(0);
  assert.equal(await guardCount(),0);assert.deepEqual(await exact(),{orders:1,receipts:1});assert.equal((await counts()).orders-before.orders,1);
  results.push({case:caseName,checkpoint:"recovered_created",first_status:201,recover_status:200,same_key:true,same_body:true,recovered_original:true,bank_disabled:true,created_status_visible:true,pg_delta:1,request_orders:1,request_receipts:1});
  record("manual created authority","click original recovery","real200 created matches original committed order; visible View order; guard0; PG one order/receipt");
  await viewOrder.click();await page.waitForURL(url=>url.pathname!=="/en/orders/new");
  const destination=new URL(page.url()),correctPath=destination.pathname==="/en/orders",correctStore=destination.searchParams.get("store")===store,correctOrder=destination.searchParams.get("order")===committed.order_id;
  results.push({case:caseName,checkpoint:"view_order_clicked",correct_path:correctPath,correct_store:correctStore,correct_order:correctOrder});
  assert(correctPath&&correctStore&&correctOrder,"visible recovered order link must open existing scoped orders detail route");
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${committed.order_id}`)).toBeVisible();
  await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",`Order ${committed.order_id}`);
  record("recovered order detail","click visible View order link","existing en/orders route has exact store/order query and positively loaded exact detail identity");
  await page.reload();await page.waitForURL(url=>url.pathname==="/en/orders"&&url.searchParams.get("store")===store&&url.searchParams.get("order")===committed.order_id);
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${committed.order_id}`)).toBeVisible();
  await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",`Order ${committed.order_id}`);
  const completed=await exact(),delta=(await counts()).orders-before.orders;
  assert.deepEqual(completed,{orders:1,receipts:1});assert.equal(delta,1);assert.equal(attempts.length,1);assert.equal(posts,1);assert.equal(recoveryPosts,1);
  results.push({case:caseName,attempts:attempts.length,placement_posts:posts,recovery_posts:recoveryPosts,pg_delta:delta,request_orders:completed.orders,request_receipts:completed.receipts,detail_identity:true,reload_detail_identity:true});
  record("recovered detail reload persistence","reload exact scoped detail","same exact aria identity loaded; PGdelta1/requestorder1/receipt1; placements1/recovery1; no duplicate");
 }finally{await page.unroute(`**${path}`);page.off("request",observe);await bank(true);}
}
