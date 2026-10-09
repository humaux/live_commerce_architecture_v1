// Purpose: W3-U2 independent real-click gate across three locales and desktop/mobile, including CAS and UNKNOWN fences.
// Depends on: Playwright, private-evidence guard, native-device fixture and signed Next/Go/PG harness env LC_BROWSER_SETTINGS_*.
// Used by: TestBrowserLiveSettingsUIRealChain and --browser-live-settings; business facts/providers explicitly MOCK.
// Invariants: I01/I02/I06/I11/I14/I18; no browser response substitution, private DOM trace or note-bearing evidence.
import { expect, type Page, type APIRequestContext, type BrowserContext } from "@playwright/test";
import { test } from "./inbox-private-evidence";
import { writeFile } from "node:fs/promises";
import { randomBytes } from "node:crypto";
import { nativePage } from "./fixtures/native-device";

const required = (name: string): string => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), api = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE"), control = required("LC_BROWSER_SETTINGS_CONTROL");
const f: {store:string;other_store:string;scene:string;other_scene:string;bundle:string;restricted_bundle:string;entry:string;next_entry:string;comment:string} = JSON.parse(required("LC_BROWSER_SETTINGS_FIXTURE"));
const locales = ["zh-TW", "zh-CN", "en"] as const;
const viewports = [{ width:1440,height:992 },{ width:390,height:844 }];
const reminderRule = "只有在 24 小時內傳過訊息給粉專的買家會收到提醒；其他買家會列在待跟進清單";
const soldRule = "會用掉這則留言唯一一次私訊回覆";
const privateText = /PRIVATE_[A-Z_]+/;
type Receipt = {action:string;key_hash:string;body_hash:string;status:number;effect:boolean};
type Facts = {class:"MOCK";receipts:Receipt[];enabled:boolean;version:number;template_id:string;template_version:number;blocked:boolean;removed:boolean;bad_authority:number;reads:number;held:boolean};
type Ledger = {control:string;operation:string;expected:string;actual:string;result:"PASS"|"FAIL"}[];
const ledgers = new WeakMap<Page,Ledger>();
const responseFacts = new WeakMap<Page, {resource:string;status:number}[]>();
const leakFlags = new WeakMap<Page,{console:boolean;url:boolean}>();
function privateMonitors(page:Page) {
  const flags={console:false,url:false};leakFlags.set(page,flags);
  const responses:{resource:string;status:number}[]=[];responseFacts.set(page,responses);
  page.on("response",response=>{const path=new URL(response.url()).pathname;const resource=["sold-out-reply","reminders","blocklist","live-sessions","session"].find(x=>path.endsWith(`/${x}`));if(resource)responses.push({resource,status:response.status()});});
  page.on("console",message=>{if(privateText.test(message.text()))flags.console=true;});
  page.on("request",request=>{if(privateText.test(request.url()))flags.url=true;});
}

// Credentials and authorized private rows must never enter automatic traces, screenshots or failure DOM.
test.use({baseURL:origin,trace:"off",screenshot:"off",video:"off"});
test.setTimeout(90_000);
async function fixture(request:APIRequestContext, action:string, body?:{mode:string}) {
  // FIXTURE/SETUP: loopback-only control prepares MOCK state; never replaces real browser/BFF responses.
  const response = action === "facts"
    ? await request.get(`${api}/__test/live-settings/facts`,{headers:{"X-Settings-Control":control}})
    : await request.post(`${api}/__test/live-settings/${action}`,{headers:{"X-Settings-Control":control},data:body});
  expect(response.status()).toBe(200);
  return response.json();
}
async function facts(request:APIRequestContext):Promise<Facts> {
  const result = await fixture(request,"facts") as Facts;
  expect(result.class).toBe("MOCK");expect(result.bad_authority).toBe(0);return result;
}
async function step(page:Page, controlID:string, operation:string, expected:string, action:()=>Promise<void>) {
  const ledger = ledgers.get(page) ?? [];ledgers.set(page,ledger);
  const row:Ledger[number] = {control:controlID,operation,expected,actual:"pending",result:"FAIL"};ledger.push(row);
  await action();row.actual="observed expected result";row.result="PASS";
}
async function login(page:Page) {
  await page.goto(`${origin}/en/`);
  await step(page,"identity-signin","click","server-authenticated store selector",async()=>{
    await page.getByRole("button",{name:"Sign in with identity service",exact:true}).click();
    await expect(page.getByTestId("shell-store-selector")).toBeAttached();
  });
}
const settings = (locale:string,store=f.store,scene=f.scene) => `${origin}/${locale}/studio/settings?store=${store}${scene ? `&scene=${scene}` : ""}`;
async function privacyCheck(page:Page) {
  // READ/MEASURE: inspect storage/URLs without changing DOM, visibility or business state; expose booleans only.
  const leaked = await page.evaluate(() => {
    const values=[location.href,...Object.entries(localStorage).flat(),...Object.entries(sessionStorage).flat()];
    return values.some(value=>/PRIVATE_[A-Z_]+/.test(value));
  });
  expect(leaked,"private fields persisted in browser storage or URL").toBe(false);
  expect(leakFlags.get(page)?.console ?? false,"private fields entered console").toBe(false);
  expect(leakFlags.get(page)?.url ?? false,"private fields entered request URL").toBe(false);
}
async function snapshot(page:Page, name:string) {
  // READ/MEASURE: screenshots supplement clicks; authorized rows and editable drafts are masked.
  expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({path:`${evidence}/${name}.png`,fullPage:true,animations:"disabled",mask:[page.getByTestId("followup-row"),page.getByTestId("blocklist-row"),page.getByTestId("sold-out-template"),page.getByTestId("sold-out-preview"),page.getByTestId("buyer-panel"),page.getByTestId("buyer-block-note"),page.getByTestId("comment-rows")]});
}
// Parent owns markup; opaque row attributes select authority handles without deriving identity from labels.
function blockRow(page:Page,id:string) {return page.locator(`[data-testid="blocklist-row"][data-entry-id="${id}"]`);}
function followRow(page:Page,id:string) {return page.locator(`[data-testid="followup-row"][data-bundle-id="${id}"]`);}
async function waitWrite(page:Page,path:string,method:string,click:()=>Promise<void>) {
  const pending=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith(path)&&r.request().method()===method);
  await click();return pending;
}

test.beforeEach(async({page,request})=>{
  await fixture(request,"reset");ledgers.set(page,[]);privateMonitors(page);
});
test.afterEach(async({page,request},info)=>{
  const rows=ledgers.get(page) ?? [];
  const caseID=info.testId.replace(/[^a-zA-Z0-9_-]/g,"");
  const errors=info.errors.map(error=>({locations:[...(error.stack??"").matchAll(/live-settings\.spec\.ts:[0-9]+:[0-9]+/g)].map(x=>x[0]), matcher:(error as {matcherResult?:{name?:string}}).matcherResult?.name}));
  await writeFile(`${evidence}/click-ledger-${caseID}.json`,JSON.stringify({page:"/[locale]/studio/settings",status:info.status,rows,errors,responses:responseFacts.get(page)}));
  await writeFile(`${evidence}/mock-receipts-${caseID}.json`,JSON.stringify(await fixture(request,"facts")));
});

for (const locale of locales) for (const viewport of viewports) {
  test(`manual reminders template and restriction real clicks ${locale}-${viewport.width}`,async({page,request,context})=>{
    await page.setViewportSize(viewport);await login(page);await page.goto(settings(locale));
    await expect(page.getByTestId("live-settings")).toBeVisible();
    if(locale==="zh-TW") {await expect(page.getByTestId("live-settings")).toContainText(reminderRule);await expect(page.getByTestId("live-settings")).toContainText(soldRule);}
    await expect(page.getByTestId("live-settings").getByRole("spinbutton")).toHaveCount(0);
    await expect(page.getByTestId("live-settings").locator('input[type="checkbox"], [role="switch"]')).toHaveCount(1);
    await step(page,"reminder-trigger","click","bodyless keyed trigger reports 2 queued and 2 follow-ups",async()=>{
      const response=await waitWrite(page,"/reminders","POST",()=>page.getByTestId("reminder-trigger").click());
      expect(response.status()).toBe(200);expect(response.request().postData()).toBeNull();
      expect(response.request().headers()["idempotency-key"]).toBeTruthy();
      await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
      await expect(page.getByTestId("followup-row")).toHaveCount(2);
      expect((await facts(request)).receipts.filter(r=>r.action==="remind"&&r.effect)).toHaveLength(1);
    });
    await step(page,"live-settings-refresh","click","read-only refresh preserves authoritative reminder counts",async()=>{
      const response=await waitWrite(page,"/reminders","GET",()=>page.getByTestId("live-settings-refresh").click());
      expect(response.status()).toBe(200);await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
    });
    await expect(followRow(page,f.restricted_bundle).getByTestId("copy")).toHaveCount(0);
    await context.grantPermissions(["clipboard-read","clipboard-write"],{origin});
    await step(page,"copy","click","actual clipboard has authorized checkout URL",async()=>{
      await followRow(page,f.bundle).getByTestId("copy").click();
      // READ/MEASURE: reading the browser clipboard proves the click's real side effect.
      expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe("https://fixture.invalid/zh-TW/checkout");
    });
    await page.reload();await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
    await step(page,"sold-out-enabled","click","disabled merchant sold-out setting draft",async()=>{
      await page.getByTestId("sold-out-enabled").click();await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();
    });
    await step(page,"sold-out-template","fill","editable merchant template preview",async()=>{
      await page.getByTestId("sold-out-template").fill("MOCK merchant {{product.name}} has sold out");
      await expect(page.getByTestId("sold-out-preview")).toContainText("MOCK merchant");
      if(locale==="zh-TW")await expect(page.getByTestId("live-settings")).toContainText(soldRule);
    });
    await step(page,"sold-out-save","click","published exact template receipt then CAS save persists",async()=>{
      const published=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/message-templates")&&r.request().method()==="POST");
      const saved=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/live-settings/sold-out-reply")&&r.request().method()==="PUT");
      await page.getByTestId("sold-out-save").click();const p=await published,s=await saved;
      expect(p.status()).toBe(200);expect(s.status()).toBe(200);const receipt=await p.json();
      expect(receipt.template_id).toMatch(/^merchant-sold-out-/);expect(receipt.version).toBe(7);
      const publishedBody=p.request().postDataJSON();expect(publishedBody.template_id).not.toBe("sold-out-reply/v1");expect(publishedBody.kinds).toEqual(["private_reply"]);expect(publishedBody.public_safe).toBe(false);
      expect(s.request().postDataJSON()).toEqual({enabled:false,template_id:receipt.template_id,template_version:receipt.version,expected_version:3});
      expect(s.request().headers()["idempotency-key"]).toBeTruthy();
      await page.reload();await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();
      const persisted=await facts(request);expect(persisted.template_id).toBe(receipt.template_id);expect(persisted.template_version).toBe(7);expect(persisted.version).toBe(4);
    });
    await expect(blockRow(page,f.entry)).toContainText("facebook");await expect(blockRow(page,f.entry)).toContainText(f.bundle);
    expect((await blockRow(page,f.entry).innerText()).includes("PRIVATE_NOTE_SENTINEL")).toBe(true);
    await expect(blockRow(page,f.entry)).toContainText("2030");
    // The list exposes source metadata, never invented display names.
    expect(await blockRow(page,f.entry).innerText()).not.toMatch(/PRIVATE_(BUYER|FOLLOWUP|RESTRICTED)_SENTINEL/);
    await step(page,"blocklist-next","click","next cursor page replaces first page",async()=>{
      await page.getByTestId("blocklist-next").click();await expect(blockRow(page,f.next_entry)).toBeVisible();await expect(blockRow(page,f.entry)).toHaveCount(0);
    });
    await step(page,"blocklist-previous","click","first page restored",async()=>{
      await page.getByTestId("blocklist-previous").click();await expect(blockRow(page,f.entry)).toBeVisible();
    });
    await step(page,"blocklist-remove/cancel","click","cancel retains row without command",async()=>{
      await blockRow(page,f.entry).getByTestId("blocklist-remove").click();await expect(page.getByTestId("blocklist-confirm")).toBeVisible();
      await page.getByTestId("blocklist-cancel").click();await expect(blockRow(page,f.entry)).toBeVisible();expect((await facts(request)).removed).toBe(false);
    });
    await step(page,"blocklist-remove/confirm","click","keyed bodyless delete persists",async()=>{
      await blockRow(page,f.entry).getByTestId("blocklist-remove").click();
      const response=await waitWrite(page,`/entries/${f.entry}`,"DELETE",()=>page.getByTestId("blocklist-confirm").click());
      expect(response.status()).toBe(200);expect(response.request().postData()).toBeNull();expect(response.request().headers()["idempotency-key"]).toBeTruthy();
      await expect(blockRow(page,f.entry)).toHaveCount(0);await page.reload();await expect(blockRow(page,f.entry)).toHaveCount(0);expect((await facts(request)).removed).toBe(true);
    });
    await snapshot(page,`settings-${locale}-${viewport.width}`);await privacyCheck(page);
    await page.goto(`${origin}/${locale}/studio/console?store=${f.store}&scene=${f.scene}`);
    await page.getByTestId(`comment-select-${f.comment}`).click();await expect(page.getByTestId("buyer-panel")).toBeVisible();
    await step(page,"buyer-block-note","fill","200 character note admitted, 201 rejected",async()=>{
      await page.getByTestId("buyer-block-note").fill("x".repeat(201));
      // Native maxlength clipping or UI rejection are both valid, but no 201-character write may be sent.
      const length=(await page.getByTestId("buyer-block-note").inputValue()).length;
      if(length>200)await expect(page.getByTestId("buyer-block-add")).toBeDisabled();
      await page.getByTestId("buyer-block-note").fill("PRIVATE_BUYER_NOTE_SENTINEL");
    });
    await step(page,"buyer-block-add","click","bundle reference plus note -> restricted badge persists",async()=>{
      const response=await waitWrite(page,"/claims/blocklist","POST",()=>page.getByTestId("buyer-block-add").click());expect(response.status()).toBe(200);
      expect(response.request().postDataJSON()).toEqual({bundle_id:f.bundle,note:"PRIVATE_BUYER_NOTE_SENTINEL"});
      await expect(page.getByTestId("buyer-restricted")).toBeVisible();await page.reload();await page.getByTestId(`comment-select-${f.comment}`).click();await expect(page.getByTestId("buyer-restricted")).toBeVisible();
    });
    await privacyCheck(page);
  });
}

test("scene selector real click when scene absent",async({page})=>{
  await login(page);await page.goto(settings("zh-TW",f.store,""));
  await step(page,"scene-select","selectOption","chosen session mounts reminder and list controls",async()=>{
    await page.getByTestId("scene-select").selectOption(f.scene);await expect(page.getByTestId("reminder-trigger")).toBeVisible();await expect(blockRow(page,f.entry)).toBeVisible();
  });
});
test("409 is visible and never auto overwrites newer settings",async({page,request})=>{
  await login(page);await page.goto(settings("zh-TW"));await expect(page.getByTestId("sold-out-enabled")).toBeChecked();
  await fixture(request,"fault",{mode:"conflict"});await page.getByTestId("sold-out-enabled").click();
  await step(page,"sold-out-template","fill","valid new merchant draft before CAS conflict",async()=>{
    await page.getByTestId("sold-out-template").fill("MOCK conflict {{product.name}} has sold out");
    await expect(page.getByTestId("sold-out-preview")).toContainText("MOCK conflict");
  });
  await step(page,"sold-out-save","click","409 exposed with authoritative settings unchanged",async()=>{
    const published=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/message-templates")&&r.request().method()==="POST");
    const response=await waitWrite(page,"/live-settings/sold-out-reply","PUT",()=>page.getByTestId("sold-out-save").click());expect(response.status()).toBe(409);
    const publication=await published;expect(publication.status()).toBe(200);const receipt=await publication.json();
    expect(receipt.template_id).toMatch(/^merchant-sold-out-/);expect(receipt.version).toBe(7);
    expect(response.request().postDataJSON()).toEqual({enabled:false,template_id:receipt.template_id,template_version:receipt.version,expected_version:3});
    await expect(page.getByTestId("sold-out-conflict")).toBeVisible();
    const after=await facts(request);expect(after.enabled).toBe(true);expect(after.version).toBe(4);expect(after.receipts.filter(r=>r.action==="sold-out")).toHaveLength(1);
    expect(after.template_id).toBe("sold-out-reply/v1");expect(after.receipts.filter(r=>r.action==="publish"&&r.effect)).toHaveLength(1);
    await page.reload();await expect(page.getByTestId("sold-out-enabled")).toBeChecked();expect((await facts(request)).receipts.filter(r=>r.action==="sold-out")).toHaveLength(1);
  });
});
test("UNKNOWN retry keeps exact key and body, effect once",async({page,request})=>{
  await login(page);await page.goto(settings("en"));await expect(page.getByTestId("reminder-trigger")).toBeVisible();await fixture(request,"fault",{mode:"unknown"});
  await step(page,"reminder-trigger","click","uncertain ACK remains explicit with no automatic second send",async()=>{
    await page.getByTestId("reminder-trigger").click();await expect(page.getByTestId("live-settings-unknown")).toBeVisible();
    await expect(page.getByTestId("live-settings-refresh")).toBeDisabled();
    await expect(page.getByTestId("reminder-trigger")).toBeDisabled();
    await expect(page.getByTestId("sold-out-save")).toBeDisabled();
    expect((await facts(request)).receipts.filter(r=>r.action==="remind")).toHaveLength(1);
  });
  await step(page,"live-settings-retry","click","same immutable command receipt replayed",async()=>{
    await page.getByTestId("live-settings-retry").click();await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
    await expect.poll(async()=>(await facts(request)).receipts.filter(r=>r.action==="remind").length).toBe(2);
    const receipts=(await facts(request)).receipts.filter(r=>r.action==="remind");expect(receipts).toHaveLength(2);expect(receipts[1]!.key_hash).toBe(receipts[0]!.key_hash);expect(receipts[1]!.body_hash).toBe(receipts[0]!.body_hash);expect(receipts.filter(r=>r.effect)).toHaveLength(1);
  });
});
test("store selector discards private old-store rows",async({page,request})=>{
  await login(page);await page.goto(settings("en"));await expect(blockRow(page,f.entry)).toBeVisible();
  await fixture(request,"fault",{mode:"hold-report"});const pending=page.reload();await expect.poll(async()=>(await facts(request)).held).toBe(true);
  await pending;await step(page,"shell-store-selector","selectOption","private rows cannot repaint after old read completes",async()=>{
    await page.getByTestId("shell-store-selector").selectOption(f.other_store);await fixture(request,"fault",{mode:"release"});
    await expect(blockRow(page,f.entry)).toHaveCount(0);await expect.poll(()=>page.evaluate(()=>document.body.textContent?.includes("PRIVATE_NOTE_SENTINEL")??false)).toBe(false);
  });await privacyCheck(page);
});
test("native hidden page clears private rows and rejects late response",async({request})=>{
  const {page,close}=await nativePage(evidence,"live-settings-native-");ledgers.set(page,[]);
  privateMonitors(page);let passed=false;
  try {
    await login(page);await page.goto(settings("en"));await expect(blockRow(page,f.entry)).toBeVisible();
    await fixture(request,"fault",{mode:"hold-report"});const pending=page.reload();await expect.poll(async()=>(await facts(request)).held).toBe(true);await pending;
    const cover=await page.context().newPage();await cover.goto("about:blank");await cover.bringToFront();await expect.poll(()=>page.evaluate(()=>document.visibilityState)).toBe("hidden");
    await fixture(request,"fault",{mode:"release"});
    await expect(blockRow(page,f.entry)).toHaveCount(0);await expect.poll(()=>page.evaluate(()=>/PRIVATE_[A-Z_]+/.test(document.body.textContent??""))).toBe(false);
    await page.bringToFront();await expect(blockRow(page,f.entry)).toBeVisible();await cover.close();await privacyCheck(page);
    passed=true;
  } finally {
    await writeFile(`${evidence}/click-ledger-native.json`,JSON.stringify({page:"/[locale]/studio/settings",rows:ledgers.get(page),visibility:"native hidden -> visible",result:passed?"PASS":"FAIL"}));
    await close();
  }
});

test("read-only signed merchant cannot write; CSRF denies owner command",async({page,request,browser})=>{
  await login(page);await page.goto(settings("en"));await expect(page.getByTestId("live-settings")).toBeVisible();
  // SECURITY/NEGATIVE: explicit API probes exercise actual Next CSRF and real PG permission checks; they are not click acceptance.
  const cookies=await page.context().cookies(origin.replace(/^http:/,"https:"));
  const cookieHeader=cookies.filter(c=>["__Host-commerce_session","__Host-commerce_csrf"].includes(c.name)).map(c=>`${c.name}=${c.value}`).join("; ");
  expect(cookies.some(c=>c.name==="__Host-commerce_session")).toBe(true);
  const denied=await request.put(`${origin}/api/stores/${f.store}/live-settings/sold-out-reply`,{
    headers:{Cookie:cookieHeader,Origin:origin,"Idempotency-Key":randomBytes(16).toString("hex")},
    data:{enabled:false,template_id:"sold-out-reply/v1",template_version:1,expected_version:3},
  });expect(denied.status()).toBe(403);expect((await facts(request)).receipts).toHaveLength(0);
  const context=await browser.newContext({ignoreHTTPSErrors:true});
  try {
    const csrf=randomBytes(32).toString("base64url"),token=required("LC_BROWSER_SETTINGS_READ_TOKEN");
    await context.addCookies([
      {name:"__Host-commerce_session",value:token,url:origin.replace(/^http:/,"https:"),secure:true,httpOnly:true,sameSite:"Lax"},
      {name:"__Host-commerce_csrf",value:csrf,url:origin.replace(/^http:/,"https:"),secure:true,httpOnly:false,sameSite:"Lax"},
    ]);
    const readPage=await context.newPage();await readPage.goto(settings("en"));await expect(readPage.getByTestId("live-settings")).toBeVisible();
    for(const id of ["reminder-trigger","sold-out-save","blocklist-remove"]) {
      const controls=readPage.getByTestId(id);
      if(await controls.count())await expect(controls.first()).toBeDisabled();
    }
    const response=await request.put(`${origin}/api/stores/${f.store}/live-settings/sold-out-reply`,{
      headers:{Cookie:`__Host-commerce_session=${token}; __Host-commerce_csrf=${csrf}`,Origin:origin,"X-CSRF-Token":csrf,"Idempotency-Key":randomBytes(16).toString("hex")},
      data:{enabled:false,template_id:"sold-out-reply/v1",template_version:1,expected_version:3},
    });expect(response.status()).toBe(403);expect((await facts(request)).receipts).toHaveLength(0);
  } finally {await context.close();}
});
