// Purpose: prove Begin-only UNKNOWN, cross-tab intent exclusion and server-authoritative full A16 recovery.
// Depends on: drawer gate click helpers and Go-owned scoped PG trigger/termination and readback controls.
// Used by: create-order-drawer-gate.mjs after the unchanged 29-case matrix.
// Invariants: I02/I03/I06/I11; only counts, booleans and HTTP statuses enter retained evidence.
import assert from "node:assert/strict";
import { expect } from "@playwright/test";

/** Drive real drawer clicks after killing its manual receipt transaction; never substitute recovery responses. */
export async function runPartialRecovery({page,api,origin,store,cvs,sku,fixture,routePath,openConversation,record,results}) {
 const control=async(path,body)=>{
  const response=await fetch(`${api}/__test/drawer-partial/${path}`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify(body)});
  assert.equal(response.status,200,`owned partial ${path} control`);return response.json();
 };
 const bank=async enabled=>assert.equal((await fetch(`${api}/__test/drawer-bank-transfer`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({enabled})})).status,204);
 const drawer=await openConversation(page,"en",fixture.Linked);
 await drawer.getByTestId("drawer-option").selectOption(cvs);
 await drawer.getByTestId("drawer-store-code").fill("123456");
 await drawer.getByTestId("drawer-store-name").fill("SYNTHETIC Partial Pickup");
 await drawer.getByTestId("drawer-store-address").fill("1 Synthetic Partial Road");
 await drawer.getByTestId("drawer-mode-bank_transfer").check();
 await expect(drawer.getByTestId("drawer-send-payment-link")).toBeEnabled();
 await drawer.getByTestId("drawer-send-payment-link").check();
 await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");
 await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
 const attempts=[];
 let firstFacts,otherPage;
 await control("arm",{});
 await page.route(`**${routePath}`,async route=>{
  attempts.push({key:route.request().headers()["idempotency-key"],body:route.request().postData()});
  if(attempts.length===1) {
   // Network reply loss is disclosed; the real PG fault first rolls back manual source/link/receipt writes.
   const [real,killed]=await Promise.all([route.fetch(),control("kill",{key:attempts[0].key})]);
   assert.equal(real.status(),503,"real A16 loses the manual receipt backend after committed Begin");
   firstFacts=killed;await real.dispose();await route.abort("failed");
  } else await route.fulfill({response:await route.fetch()});
 });
 try {
  await drawer.getByTestId("drawer-submit").click();
  await expect(drawer.getByTestId("drawer-error")).toBeVisible();
  assert.equal(firstFacts.killed,true,"the exact PG advisory waiter was terminated");
  results.push({case:"DU3-partial-begin-recovery",partial_before_recovery:firstFacts});
  assert.deepEqual(firstFacts,{killed:true,begin:1,bindings:1,orders:1,manual_source:0,links:0,valid_links:0,manual_receipts:0,for_buyer_receipts:0,completion_receipts:0,recovery_receipts:0,closures:0,manual_audits:0,for_buyer_audits:0,pending_bundles:1,placed_bundles:0,operations:0,secrets:0,jobs:0,dm_valid_links:0});
  record("partial Begin fault","submit/terminate exact PG waiter/drop actual reply","one committed Begin; zero source/link/upper receipt/DM; original attempt unresolved");
  await expect(drawer.getByTestId("drawer-close")).toBeDisabled();
  await expect(drawer.getByTestId("drawer-new-attempt")).toHaveCount(0);
  await expect(drawer.getByTestId("drawer-name")).toBeDisabled();
  // UNKNOWN itself permits only an explicit authoritative check; it neither finalizes nor regenerates the key.
  await expect(drawer.getByTestId("drawer-recover-attempt")).toBeVisible();await expect(drawer.getByTestId("drawer-recover-attempt")).toBeEnabled();
  const [recovery]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===`${routePath}/recover`),drawer.getByTestId("drawer-recover-attempt").click()]);
  // RED target 4250496779: old recovery incorrectly reports created from Begin alone.
  results.push({case:"DU3-partial-begin-recovery",recover_status:recovery.status()});
  assert.equal(recovery.status(),503,"partial Begin must retain UNKNOWN, not report a completed A16");
  assert.equal((await recovery.json()).code,"retry_later");assert(recovery.headers()["cache-control"]?.includes("no-store"));
  assert.equal(recovery.request().headers()["idempotency-key"],attempts[0].key);assert.equal(recovery.request().postData(),attempts[0].body);
  attempts.push({key:recovery.request().headers()["idempotency-key"],body:recovery.request().postData()});
  await expect(drawer.getByTestId("drawer-error")).toBeVisible();
  await expect(drawer.getByTestId("drawer-close")).toBeDisabled();
  await expect(drawer.getByTestId("drawer-new-attempt")).toHaveCount(0);
  await expect(drawer.getByTestId("drawer-result")).toHaveCount(0);
  await expect(drawer.getByTestId("drawer-view-order")).toHaveCount(0);
  await expect(drawer.getByTestId("drawer-recovery-result")).toHaveCount(0);
  assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),1,"opaque OrderGuard stays armed");
  const {killed,...beforeRecovery}=firstFacts;
  const unresolved=await control("facts",{key:attempts[0].key});assert.deepEqual(unresolved,beforeRecovery);
  record("partial A16R authority","click recover","actual 503 retry_later; no seal/receipt; new attempt absent; close blocked; guard retained");
  // A separate real tab shares the login but has no first tab's sessionStorage flag or frozen body.
  otherPage=await page.context().newPage();
  const otherDrawer=await openConversation(otherPage,"en",fixture.Linked);
  assert.equal(await otherPage.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0);
  await otherDrawer.getByTestId("drawer-option").selectOption(cvs);
  await otherDrawer.getByTestId("drawer-store-code").fill("123456");
  await otherDrawer.getByTestId("drawer-store-name").fill("SYNTHETIC Partial Pickup");
  await otherDrawer.getByTestId("drawer-store-address").fill("1 Synthetic Partial Road");
  await otherDrawer.getByTestId("drawer-mode-bank_transfer").check();
  await otherDrawer.getByTestId("drawer-send-payment-link").check();
  await expect(otherDrawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");await expect(otherDrawer.getByTestId("drawer-submit")).toBeEnabled();
  // Remove a dynamic option only after both real forms loaded. The original committed intent must remain completable.
  await bank(false);
  const [refused]=await Promise.all([otherPage.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===routePath),otherDrawer.getByTestId("drawer-submit").click()]);
  assert.equal(refused.status(),409);assert.equal((await refused.json()).code,"attempt_pending");
  assert.notEqual(refused.request().headers()["idempotency-key"],attempts[0].key,"a fresh tab actually attempted a different key");
  await expect(otherDrawer.getByTestId("drawer-unresolved-guard")).toBeVisible();await expect(otherDrawer.getByTestId("drawer-form")).toHaveCount(0);
  assert.deepEqual(await control("facts",{key:attempts[0].key}),beforeRecovery,"fresh tab refusal creates no second Begin, source, link, bundle placement or DM");
  record("partial cross-tab pending refusal","open real independent tab/fill actual form/click create","actual409 attempt_pending with different key; server retains exact original Begin; no second pipeline");
  const recoveryPath=`/api/stores/${store}/tools/orders/attempt/recover`;
  await expect(otherDrawer.getByTestId("drawer-recover-pending")).toBeVisible();await expect(otherDrawer.getByTestId("drawer-recover-pending")).toBeEnabled();
  const [reconciled]=await Promise.all([otherPage.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===recoveryPath),otherDrawer.getByTestId("drawer-recover-pending").click()]);
  assert.equal(reconciled.status(),200);const recovered=await reconciled.json();assert.equal(recovered.state,"created");assert.deepEqual(Object.keys(recovered).sort(),["order_id","state"]);
  assert.equal(reconciled.request().postData(),"{}");assert.notEqual(reconciled.request().headers()["idempotency-key"],attempts[0].key);assert.notEqual(reconciled.request().headers()["idempotency-key"],refused.request().headers()["idempotency-key"]);
  assert(reconciled.headers()["cache-control"]?.includes("no-store"));
  await expect(otherDrawer.getByTestId("drawer-recovery-result")).toBeVisible();await expect(otherDrawer.getByTestId("drawer-view-order")).toHaveAttribute("href",`/en/orders?store=${store}&order=${recovered.order_id}`);
  await expect(otherDrawer.getByTestId("drawer-form")).toHaveCount(0);await expect(otherDrawer.getByTestId("drawer-copy-link")).toHaveCount(0);
  assert.equal(await otherPage.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0);
  const complete=await control("facts",{key:attempts[0].key});
  // Place's original link is superseded by atomic DM issuance: exact historical rows, only one redeemable planned link.
  assert.deepEqual(complete,{begin:1,bindings:1,orders:1,manual_source:1,links:2,valid_links:1,manual_receipts:1,for_buyer_receipts:1,completion_receipts:1,recovery_receipts:0,closures:0,manual_audits:1,for_buyer_audits:1,pending_bundles:0,placed_bundles:1,operations:1,secrets:1,jobs:1,dm_valid_links:1});
  record("partial server pending recovery","click server recovery in the second tab","real200 created exact original order; original frozen pipeline resumes despite removed bank option; source/link/bundle/audit/DM exactly once");
  await otherDrawer.getByTestId("drawer-view-order").click();await expect(otherPage.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(recovered.order_id));await otherPage.reload();await expect(otherPage.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(recovered.order_id));
  assert.deepEqual(await control("facts",{key:attempts[0].key}),complete,"second-tab detail/reload never repeats any side effect");
  await otherPage.close();otherPage=null;
  await expect(drawer.getByTestId("drawer-submit")).toBeVisible();await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
  const [finished]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===routePath),drawer.getByTestId("drawer-submit").click()]);
  assert.equal(finished.status(),201);const value=await finished.json();assert.equal(value.send.state,"queued");assert(value.send.operation_id);
  assert.equal(value.order_id,recovered.order_id,"original frozen-key replay points to the order reconciled by server recovery");
  await expect(drawer.getByTestId("drawer-result")).toBeVisible();await expect(drawer.getByTestId("drawer-send-result")).toContainText("queued");
  assert.equal(attempts.length,3);assert(attempts.every(a=>a.key===attempts[0].key&&a.body===attempts[0].body),"original create, key recovery and explicit replay all retain the exact frozen body/key");
  assert.deepEqual(await control("facts",{key:attempts[0].key}),complete,"same-key replay repeats no link, receipt, source, audit, bundle or DM");
  assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0);
  await drawer.getByTestId("drawer-close").click();
  await page.goto(`${origin}/en/orders?store=${store}`);await page.reload();
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${value.order_id}`)).toBeVisible();
  await page.getByTestId(`order-expand-${value.order_id}`).click();await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(value.order_id));
  assert.deepEqual(await control("facts",{key:attempts[0].key}),complete,"reload creates no second order/link/operation or receipt");
  results.push({case:"DU3-partial-begin-recovery",attempts:3,same_key:true,same_body:true,recover_status:503,cross_tab_refusal:409,cross_tab_recovery:200,generic_request_empty:true,completion:complete,reload_persisted:true});
  record("partial original retry persistence","restore bank/click original retry/close/reload/click exact order","one source/valid sealed link/manual+for-buyer+completion receipt/audit/placed bundle and real queued DM job; exact original+atomic-issuance history; no new key");
 } finally {if(otherPage)await otherPage.close();await page.unroute(`**${routePath}`);await bank(true);}
}
