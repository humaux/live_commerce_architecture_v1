// Purpose: verify definitive 409 capability after UNKNOWN exposes authoritative recovery of completed A16.
// Depends on: real drawer clicks, signed synthetic eighth conversation and scoped PG grant/readback controls.
// Used by: create-order-drawer-gate.mjs after its unchanged 30-case matrix.
// Invariants: I02/I06/I11; original body/key stay private, and evidence contains only counts/status/booleans.
import assert from "node:assert/strict";
import { expect } from "@playwright/test";

/** Recover a lost completed order response after only inbox:reply is revoked; no fabricated API responses. */
export async function runCapabilityRecovery({page,api,origin,store,home,sku,fixture,routePath,openConversation,record,results}) {
 const control=async(path,body={})=>{
  const reply=await fetch(`${api}/__test/drawer-capability/${path}`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify(body)});
  assert.equal(reply.status,200,`owned capability ${path} control`);return reply.json();
 };
 const drawer=await openConversation(page,"en",fixture.Linked);
 await drawer.getByTestId("drawer-option").selectOption(home);
 await drawer.getByTestId("drawer-mode-bank_transfer").check();
 await expect(drawer.getByTestId(`drawer-quantity-${sku}`)).toHaveValue("2");
 await expect(drawer.getByTestId("drawer-send-payment-link")).toBeEnabled();await drawer.getByTestId("drawer-send-payment-link").check();
 await expect(drawer.getByTestId("drawer-submit")).toBeEnabled();
 assert.deepEqual(await control("arm"),{reply_grants:1});
 const attempts=[];let created;
 await page.route(`**${routePath}`,async route=>{
  attempts.push({key:route.request().headers()["idempotency-key"],body:route.request().postData()});
  const real=await route.fetch();
  if(attempts.length===1) {
   assert.equal(real.status(),201,"first real A16 finishes before its reply is dropped");created=await real.json();
   assert.equal(created.send.state,"queued");assert(created.send.operation_id);
   await real.dispose();await route.abort("failed");
  } else await route.fulfill({response:real});
 });
 try {
  await drawer.getByTestId("drawer-submit").click();await expect(drawer.getByTestId("drawer-error")).toBeVisible();
  await expect(drawer.getByTestId("drawer-close")).toBeDisabled();await expect(drawer.getByTestId("drawer-new-attempt")).toHaveCount(0);
  const finishedFacts=await control("facts",{key:attempts[0].key});
  // The original Place link is superseded once by atomic DM issuance, before the lost response.
  // Recovery must preserve both historical rows and exactly one redeemable sealed link thereafter.
  assert.deepEqual(finishedFacts,{begin:1,bindings:1,orders:1,manual_source:1,links:2,valid_links:1,manual_receipts:1,for_buyer_receipts:1,completion_receipts:1,recovery_receipts:0,closures:0,manual_audits:1,for_buyer_audits:1,pending_bundles:0,placed_bundles:1,operations:1,secrets:1,jobs:1,dm_valid_links:1});
  record("capability lost completed reply","submit then drop actual201","durable full A16 completion and queued DM; original opaque guard retained");
  assert.deepEqual(await control("revoke"),{reply_grants:0});
  const [refusal]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===routePath),drawer.getByTestId("drawer-submit").click()]);
  assert.equal(refusal.status(),409);assert.equal((await refusal.json()).code,"capability");
  await expect(drawer.getByTestId("drawer-error")).toBeVisible();
  results.push({case:"DU3-capability-after-unknown-recovery",refusal_status:409,replay_same_key:attempts[1].key===attempts[0].key,replay_same_body:attempts[1].body===attempts[0].body});
  // RED target4250747441: the current Drawer retains this409 but fails to offer its existing A16R control.
  await expect(drawer.getByTestId("drawer-recover-attempt")).toBeVisible();
  await expect(drawer.getByTestId("drawer-close")).toBeDisabled();await expect(drawer.getByTestId("drawer-new-attempt")).toHaveCount(0);
  assert.deepEqual(await control("facts",{key:attempts[0].key}),finishedFacts,"refused replay changes no order/DM/receipt");
  const [recovery]=await Promise.all([page.waitForResponse(r=>r.request().method()==="POST"&&new URL(r.url()).pathname===`${routePath}/recover`),drawer.getByTestId("drawer-recover-attempt").click()]);
  assert.equal(recovery.status(),200);assert(recovery.headers()["cache-control"]?.includes("no-store"));
  assert.deepEqual(await recovery.json(),{state:"created",order_id:created.order_id},"A16R resolves completed original despite missing inbox:reply");
  assert.equal(recovery.request().headers()["idempotency-key"],attempts[0].key);assert.equal(recovery.request().postData(),attempts[0].body);
  assert.equal(attempts.length,2);assert(attempts.every(a=>a.key===attempts[0].key&&a.body===attempts[0].body));
  await expect(drawer.getByTestId("drawer-view-order")).toBeVisible();
  await expect(drawer.getByTestId("drawer-close")).toBeEnabled();
  assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(key=>key.startsWith("buyer-order-unresolved:")).length),0,"authoritative created result clears guard");
  const recoveredFacts=await control("facts",{key:attempts[0].key});assert.deepEqual(recoveredFacts,{...finishedFacts,recovery_receipts:1});
  record("capability A16R recovery","revoke exact reply grant/retry/click recover","actual409 capability; actual200 created; same key/body; guard released; exactone order and DM");
  await drawer.getByTestId("drawer-close").click();
  await page.goto(`${origin}/en/orders?store=${store}`);await page.reload();
  await expect(page.getByTestId("orders-table")).toBeVisible();await expect(page.getByTestId(`order-row-${created.order_id}`)).toBeVisible();
  await page.getByTestId(`order-expand-${created.order_id}`).click();await expect(page.getByTestId("order-detail")).toBeVisible();await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label",new RegExp(created.order_id));
  assert.deepEqual(await control("facts",{key:attempts[0].key}),recoveredFacts,"reload does not create new order/link/DM or receipt");
  results.push({case:"DU3-capability-after-unknown-recovery",attempts:2,same_key:true,same_body:true,refusal_status:409,recover_status:200,completion:recoveredFacts,reload_persisted:true});
  record("capability recovered order persistence","close/reload/click exact order","same persisted order and exactone source/link/audits/bundle/queued DM job and valid sealed link");
 } finally {await page.unroute(`**${routePath}`);assert.deepEqual(await control("restore"),{reply_grants:1});}
}
