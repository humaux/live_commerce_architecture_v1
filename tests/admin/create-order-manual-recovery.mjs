// Purpose: independently verify ManualOrder's authoritative absence and partial-Begin recovery UI.
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
 const before=await counts(),attempts=[];let posts=0,recoveryPosts=0,firstFacts;
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
  const [refusal]=await Promise.all([page.waitForResponse(r=>isPost(r)),form.getByTestId("manual-order-submit").click()]);
  assert.equal(refusal.status(),422);assert.equal((await refusal.json()).code,"bank_transfer_unavailable");
  assert.equal(attempts.length,2);assert.equal(attempts[1].key,attempts[0].key);assert.equal(attempts[1].body,attempts[0].body);
  await expect(form.getByTestId("manual-order-error")).toBeVisible();
  await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);
  results.push({case:caseName,refusal_status:422,same_key:true,same_body:true,partial:partial});
  // RED target4254715306: existing ManualOrder retains the attempt but exposes no recovery control.
  await expect(form.getByTestId("manual-order-recover-attempt")).toBeVisible();
  assert.equal(await guardCount(),1,"retained manual attempt keeps opaque guard");
  if(partial)record("manual partial Begin fault","submit/terminate exact PG waiter/drop reply/edit/retry","actual503 lost reply then real422; one Begin and order; source/link/manual receipt absent");
  else record("manual frozen refusal","load options/disable bank/submit/drop reply/edit/retry","two actual422; edited draft retains exact original key/body; no new attempt");
  const [recovery]=await Promise.all([page.waitForResponse(r=>isPost(r,recoveryPath)),form.getByTestId("manual-order-recover-attempt").click()]);
  assert.equal(recovery.request().headers()["idempotency-key"],attempts[0].key);assert.equal(recovery.request().postData(),attempts[0].body);
  assert(recovery.headers()["cache-control"]?.includes("no-store"));
  const seal=await fetch(`${api}/__test/drawer-recovery-facts`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({key:attempts[0].key})});
  assert.equal(seal.status,200);
  const exactBefore=await control("request-facts",{keys:[attempts[0].key]});assert.deepEqual(exactBefore,{orders:0,receipts:0});
  if(partial){
   assert.equal(recovery.status(),503);assert.equal((await recovery.json()).code,"retry_later");
   assert.deepEqual(await seal.json(),{not_created_seals:0});
   await expect(form.getByTestId("manual-order-new-attempt")).toHaveCount(0);
   await expect(form.getByTestId("manual-order-recovered-order")).toHaveCount(0);
   await expect(page.getByTestId("manual-order-result")).toHaveCount(0);
   await expect(form.getByTestId("manual-order-submit")).toBeEnabled();
   assert.equal(await guardCount(),1,"503 recovery retains opaque guard");
   await form.getByTestId("mo-name").fill("SYNTHETIC Partial Edited Again");
   assert.equal(posts,2,"draft edit after503 cannot create a new intent or POST");assert.equal(recoveryPosts,1);
   const {killed,...initial}=firstFacts;assert.deepEqual(await control("facts",{key:attempts[0].key}),initial,"503 recovery writes no seal or false completion");
   record("manual partial authority","click recover","actual503 retry_later; no seal/result/newattempt; original request remains retryable");
   await bank(true);
  }else{
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
  assert.equal(attempts.length,3);assert.equal(posts,partial?3:4,"only explicit submits and disclosed closed-key probe");assert.equal(recoveryPosts,1,"only explicit recovery click");
  if(partial){assert.equal(attempts[2].key,attempts[0].key);assert.equal(attempts[2].body,attempts[0].body);}
  else {assert.notEqual(attempts[2].key,attempts[0].key);assert.notEqual(attempts[2].body,attempts[0].body,"explicit new attempt adopts edited draft");}
  await expect(page.getByTestId("manual-order-result")).toBeVisible();
  assert.equal(await guardCount(),0,"successful original/new intent clears opaque guard");
  await page.unroute(`**${path}`);
  assert.equal((await counts()).orders-before.orders,1);
  const completed=await control("request-facts",{keys:[attempts[2].key]});assert.deepEqual(completed,{orders:1,receipts:1});
  // Real view-orders link, reload, then expand the exact persisted row.
  await page.getByTestId("manual-order-result").getByRole("link").click();await page.reload();
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
  await page.getByTestId(`order-expand-${value.order_id}`).click();await expect(page.getByTestId("order-detail")).toBeVisible();
  await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(value.order_id));
  assert.deepEqual(await control("request-facts",{keys:[attempts[2].key]}),completed);assert.equal((await counts()).orders-before.orders,1);
  results.push({case:caseName,attempts:3,same_key:partial,new_key:!partial,recover_status:partial?503:200,seals:partial?0:1,pg_delta:1,request_orders:1,request_receipts:1,reload_persisted:true});
  record(partial?"manual partial retry persistence":"manual new intent persistence","restore bank/submit/click vieworders/reload/expand exact order","exactone persisted manual order and receipt; no duplicate onreload");
 }finally{await page.unroute(`**${path}`);page.off("request",observe);await bank(true);}
}
