# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: live-settings.spec.ts >> scene selector real click when scene absent
- Location: tests/admin/live-settings.spec.ts:187:1

# Error details

```
Error: expect(locator).toBeDisabled() failed

Locator:  getByTestId('scene-select').locator('option[value=""]')
Expected: disabled
Received: enabled
Timeout:  10000ms

Call log:
  - Expect "toBeDisabled" getByTestId('scene-select').locator('option[value=""]') with timeout 10000ms
  - waiting for getByTestId('scene-select').locator('option[value=""]')
    23 × locator resolved to <option value="" disabled>選擇場次</option>
       - unexpected value "enabled"

```

# Test source

```ts
  95  |   const errors=info.errors.map(error=>({locations:[...(error.stack??"").matchAll(/live-settings\.spec\.ts:[0-9]+:[0-9]+/g)].map(x=>x[0]), matcher:(error as {matcherResult?:{name?:string}}).matcherResult?.name}));
  96  |   await writeFile(`${evidence}/click-ledger-${caseID}.json`,JSON.stringify({page:"/[locale]/studio/settings",status:info.status,rows,errors,responses:responseFacts.get(page)}));
  97  |   await writeFile(`${evidence}/mock-receipts-${caseID}.json`,JSON.stringify(await fixture(request,"facts")));
  98  | });
  99  | 
  100 | for (const locale of locales) for (const viewport of viewports) {
  101 |   test(`manual reminders template and restriction real clicks ${locale}-${viewport.width}`,async({page,request,context})=>{
  102 |     await page.setViewportSize(viewport);await login(page);await page.goto(settings(locale));
  103 |     await expect(page.getByTestId("live-settings")).toBeVisible();
  104 |     if(locale==="zh-TW") {await expect(page.getByTestId("live-settings")).toContainText(reminderRule);await expect(page.getByTestId("live-settings")).toContainText(soldRule);}
  105 |     await expect(page.getByTestId("live-settings").getByRole("spinbutton")).toHaveCount(0);
  106 |     await expect(page.getByTestId("live-settings").locator('input[type="checkbox"], [role="switch"]')).toHaveCount(1);
  107 |     await step(page,"reminder-trigger","click","bodyless keyed trigger reports 2 queued and 2 follow-ups",async()=>{
  108 |       const response=await waitWrite(page,"/reminders","POST",()=>page.getByTestId("reminder-trigger").click());
  109 |       expect(response.status()).toBe(200);expect(response.request().postData()).toBeNull();
  110 |       expect(response.request().headers()["idempotency-key"]).toBeTruthy();
  111 |       await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
  112 |       await expect(page.getByTestId("followup-row")).toHaveCount(2);
  113 |       expect((await facts(request)).receipts.filter(r=>r.action==="remind"&&r.effect)).toHaveLength(1);
  114 |     });
  115 |     await step(page,"live-settings-refresh","click","read-only refresh preserves authoritative reminder counts",async()=>{
  116 |       const response=await waitWrite(page,"/reminders","GET",()=>page.getByTestId("live-settings-refresh").click());
  117 |       expect(response.status()).toBe(200);await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
  118 |     });
  119 |     await expect(followRow(page,f.restricted_bundle).getByTestId("copy")).toHaveCount(0);
  120 |     await context.grantPermissions(["clipboard-read","clipboard-write"],{origin});
  121 |     await step(page,"copy","click","actual clipboard has authorized checkout URL",async()=>{
  122 |       await followRow(page,f.bundle).getByTestId("copy").click();
  123 |       // READ/MEASURE: reading the browser clipboard proves the click's real side effect.
  124 |       expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe("https://fixture.invalid/zh-TW/checkout");
  125 |     });
  126 |     await page.reload();await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
  127 |     await step(page,"sold-out-enabled","click","disabled merchant sold-out setting draft",async()=>{
  128 |       await page.getByTestId("sold-out-enabled").click();await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();
  129 |     });
  130 |     await step(page,"sold-out-template","fill","editable merchant template preview",async()=>{
  131 |       await page.getByTestId("sold-out-template").fill("MOCK merchant {{product.name}} has sold out");
  132 |       await expect(page.getByTestId("sold-out-preview")).toContainText("MOCK merchant");
  133 |       if(locale==="zh-TW")await expect(page.getByTestId("live-settings")).toContainText(soldRule);
  134 |     });
  135 |     await step(page,"sold-out-save","click","published exact template receipt then CAS save persists",async()=>{
  136 |       const published=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/message-templates")&&r.request().method()==="POST");
  137 |       const saved=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/live-settings/sold-out-reply")&&r.request().method()==="PUT");
  138 |       await page.getByTestId("sold-out-save").click();const p=await published,s=await saved;
  139 |       expect(p.status()).toBe(200);expect(s.status()).toBe(200);const receipt=await p.json();
  140 |       expect(receipt.template_id).toMatch(/^merchant-sold-out-/);expect(receipt.version).toBe(7);
  141 |       const publishedBody=p.request().postDataJSON();expect(publishedBody.template_id).not.toBe("sold-out-reply/v1");expect(publishedBody.kinds).toEqual(["private_reply"]);expect(publishedBody.public_safe).toBe(false);
  142 |       expect(s.request().postDataJSON()).toEqual({enabled:false,template_id:receipt.template_id,template_version:receipt.version,expected_version:3});
  143 |       expect(s.request().headers()["idempotency-key"]).toBeTruthy();
  144 |       await page.reload();await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();
  145 |       const persisted=await facts(request);expect(persisted.template_id).toBe(receipt.template_id);expect(persisted.template_version).toBe(7);expect(persisted.version).toBe(4);
  146 |     });
  147 |     await expect(blockRow(page,f.entry)).toContainText("facebook");await expect(blockRow(page,f.entry)).toContainText(f.bundle);
  148 |     expect((await blockRow(page,f.entry).innerText()).includes("PRIVATE_NOTE_SENTINEL")).toBe(true);
  149 |     await expect(blockRow(page,f.entry)).toContainText("2030");
  150 |     // The list exposes source metadata, never invented display names.
  151 |     expect(await blockRow(page,f.entry).innerText()).not.toMatch(/PRIVATE_(BUYER|FOLLOWUP|RESTRICTED)_SENTINEL/);
  152 |     await step(page,"blocklist-next","click","next cursor page replaces first page",async()=>{
  153 |       await page.getByTestId("blocklist-next").click();await expect(blockRow(page,f.next_entry)).toBeVisible();await expect(blockRow(page,f.entry)).toHaveCount(0);
  154 |     });
  155 |     await step(page,"blocklist-previous","click","first page restored",async()=>{
  156 |       await page.getByTestId("blocklist-previous").click();await expect(blockRow(page,f.entry)).toBeVisible();
  157 |     });
  158 |     await step(page,"blocklist-remove/cancel","click","cancel retains row without command",async()=>{
  159 |       await blockRow(page,f.entry).getByTestId("blocklist-remove").click();await expect(page.getByTestId("blocklist-confirm")).toBeVisible();
  160 |       await page.getByTestId("blocklist-cancel").click();await expect(blockRow(page,f.entry)).toBeVisible();expect((await facts(request)).removed).toBe(false);
  161 |     });
  162 |     await step(page,"blocklist-remove/confirm","click","keyed bodyless delete persists",async()=>{
  163 |       await blockRow(page,f.entry).getByTestId("blocklist-remove").click();
  164 |       const response=await waitWrite(page,`/entries/${f.entry}`,"DELETE",()=>page.getByTestId("blocklist-confirm").click());
  165 |       expect(response.status()).toBe(200);expect(response.request().postData()).toBeNull();expect(response.request().headers()["idempotency-key"]).toBeTruthy();
  166 |       await expect(blockRow(page,f.entry)).toHaveCount(0);await page.reload();await expect(blockRow(page,f.entry)).toHaveCount(0);expect((await facts(request)).removed).toBe(true);
  167 |     });
  168 |     await snapshot(page,`settings-${locale}-${viewport.width}`);await privacyCheck(page);
  169 |     await page.goto(`${origin}/${locale}/studio/console?store=${f.store}&scene=${f.scene}`);
  170 |     await page.getByTestId(`comment-select-${f.comment}`).click();await expect(page.getByTestId("buyer-panel")).toBeVisible();
  171 |     await step(page,"buyer-block-note","fill","200 character note admitted, 201 rejected",async()=>{
  172 |       await page.getByTestId("buyer-block-note").fill("x".repeat(201));
  173 |       // Native maxlength clipping or UI rejection are both valid, but no 201-character write may be sent.
  174 |       const length=(await page.getByTestId("buyer-block-note").inputValue()).length;
  175 |       if(length>200)await expect(page.getByTestId("buyer-block-add")).toBeDisabled();
  176 |       await page.getByTestId("buyer-block-note").fill("PRIVATE_BUYER_NOTE_SENTINEL");
  177 |     });
  178 |     await step(page,"buyer-block-add","click","bundle reference plus note -> restricted badge persists",async()=>{
  179 |       const response=await waitWrite(page,"/claims/blocklist","POST",()=>page.getByTestId("buyer-block-add").click());expect(response.status()).toBe(200);
  180 |       expect(response.request().postDataJSON()).toEqual({bundle_id:f.bundle,note:"PRIVATE_BUYER_NOTE_SENTINEL"});
  181 |       await expect(page.getByTestId("buyer-restricted")).toBeVisible();await page.reload();await page.getByTestId(`comment-select-${f.comment}`).click();await expect(page.getByTestId("buyer-restricted")).toBeVisible();
  182 |     });
  183 |     await privacyCheck(page);
  184 |   });
  185 | }
  186 | 
  187 | test("scene selector real click when scene absent",async({page})=>{
  188 |   await login(page);await page.goto(settings("zh-TW",f.store,""));
  189 |   await expect(page.getByTestId("scene-select")).toBeEnabled();
  190 |   await step(page,"nav-live-settings","click","same explicit-store settings navigation remains selectable",async()=>{
  191 |     await page.getByTestId("nav-live-settings").click();await expect(page.getByTestId("scene-select")).toBeEnabled();
  192 |   });
  193 |   await step(page,"scene-select","selectOption","chosen session mounts reminder and list controls; placeholder disabled",async()=>{
  194 |     await page.getByTestId("scene-select").selectOption(f.scene);await expect(page.getByTestId("reminder-trigger")).toBeVisible();await expect(blockRow(page,f.entry)).toBeVisible();
> 195 |     await expect(page.getByTestId("scene-select").locator('option[value=""]')).toBeDisabled();
      |                                                                                ^ Error: expect(locator).toBeDisabled() failed
  196 |   });
  197 |   // FIXTURE/AUTH ONLY: real signed PG principal has exactly one store, making the no-store default deterministic.
  198 |   await signedStaff(page,"LC_BROWSER_SETTINGS_READ_TOKEN");
  199 |   await page.goto(`${origin}/en/studio/settings`);
  200 |   await expect(page.getByTestId("shell-store-selector")).toHaveValue(f.store);
  201 |   await expect(page.getByTestId("scene-select")).toBeEnabled();
  202 |   await step(page,"nav-live-settings","click","same default-store URL navigation preserves selectable page",async()=>{
  203 |     await page.getByTestId("nav-live-settings").click();await expect(page.getByTestId("scene-select")).toBeEnabled();
  204 |   });
  205 |   await step(page,"scene-select","selectOption","default-store page can choose a session after same-address navigation",async()=>{
  206 |     await page.getByTestId("scene-select").selectOption(f.scene);await expect(blockRow(page,f.entry)).toBeVisible();
  207 |     await expect(page.getByTestId("scene-select").locator('option[value=""]')).toBeDisabled();
  208 |   });await privacyCheck(page);
  209 | });
  210 | 
  211 | test("409 is visible and never auto overwrites newer settings",async({page,request})=>{
  212 |   await login(page);await page.goto(settings("zh-TW"));await expect(page.getByTestId("sold-out-enabled")).toBeChecked();
  213 |   await fixture(request,"fault",{mode:"conflict"});await page.getByTestId("sold-out-enabled").click();
  214 |   await step(page,"sold-out-template","fill","valid new merchant draft before CAS conflict",async()=>{
  215 |     await page.getByTestId("sold-out-template").fill("MOCK conflict {{product.name}} has sold out");
  216 |     await expect(page.getByTestId("sold-out-preview")).toContainText("MOCK conflict");
  217 |   });
  218 |   await step(page,"sold-out-save","click","409 exposed with authoritative settings unchanged",async()=>{
  219 |     const published=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/message-templates")&&r.request().method()==="POST");
  220 |     const response=await waitWrite(page,"/live-settings/sold-out-reply","PUT",()=>page.getByTestId("sold-out-save").click());expect(response.status()).toBe(409);
  221 |     const publication=await published;expect(publication.status()).toBe(200);const receipt=await publication.json();
  222 |     expect(receipt.template_id).toMatch(/^merchant-sold-out-/);expect(receipt.version).toBe(7);
  223 |     expect(response.request().postDataJSON()).toEqual({enabled:false,template_id:receipt.template_id,template_version:receipt.version,expected_version:3});
  224 |     await expect(page.getByTestId("sold-out-conflict")).toBeVisible();
  225 |     const after=await facts(request);expect(after.enabled).toBe(true);expect(after.version).toBe(4);expect(after.receipts.filter(r=>r.action==="sold-out")).toHaveLength(1);
  226 |     expect(after.template_id).toBe("sold-out-reply/v1");expect(after.receipts.filter(r=>r.action==="publish"&&r.effect)).toHaveLength(1);
  227 |     await page.reload();await expect(page.getByTestId("sold-out-enabled")).toBeChecked();expect((await facts(request)).receipts.filter(r=>r.action==="sold-out")).toHaveLength(1);
  228 |   });
  229 | });
  230 | test("UNKNOWN retry keeps exact key and body, effect once",async({page,request})=>{
  231 |   await login(page);await page.goto(settings("en"));await expect(page.getByTestId("reminder-trigger")).toBeVisible();await fixture(request,"fault",{mode:"unknown"});
  232 |   await step(page,"reminder-trigger","click","uncertain ACK remains explicit with no automatic second send",async()=>{
  233 |     await page.getByTestId("reminder-trigger").click();await expect(page.getByTestId("live-settings-unknown")).toBeVisible();
  234 |     await expect(page.getByTestId("live-settings-refresh")).toBeDisabled();
  235 |     await expect(page.getByTestId("reminder-trigger")).toBeDisabled();
  236 |     await expect(page.getByTestId("sold-out-save")).toBeDisabled();
  237 |     expect((await facts(request)).receipts.filter(r=>r.action==="remind")).toHaveLength(1);
  238 |   });
  239 |   await step(page,"live-settings-retry","click","same immutable command receipt replayed",async()=>{
  240 |     await page.getByTestId("live-settings-retry").click();await expect(page.getByTestId("reminder-report")).toContainText(/(?:^|\D)2(?:\D|$)/);
  241 |     await expect.poll(async()=>(await facts(request)).receipts.filter(r=>r.action==="remind").length).toBe(2);
  242 |     const receipts=(await facts(request)).receipts.filter(r=>r.action==="remind");expect(receipts).toHaveLength(2);expect(receipts[1]!.key_hash).toBe(receipts[0]!.key_hash);expect(receipts[1]!.body_hash).toBe(receipts[0]!.body_hash);expect(receipts.filter(r=>r.effect)).toHaveLength(1);
  243 |   });
  244 | });
  245 | test("store selector discards private old-store rows",async({page,request})=>{
  246 |   await login(page);await page.goto(settings("en"));await expect(blockRow(page,f.entry)).toBeVisible();
  247 |   await fixture(request,"fault",{mode:"hold-report"});const pending=page.reload();await expect.poll(async()=>(await facts(request)).held).toBe(true);
  248 |   await pending;await step(page,"shell-store-selector","selectOption","private rows cannot repaint after old read completes",async()=>{
  249 |     await page.getByTestId("shell-store-selector").selectOption(f.other_store);await fixture(request,"fault",{mode:"release"});
  250 |     await expect(blockRow(page,f.entry)).toHaveCount(0);await expect.poll(()=>page.evaluate(()=>document.body.textContent?.includes("PRIVATE_NOTE_SENTINEL")??false)).toBe(false);
  251 |   });await privacyCheck(page);
  252 | });
  253 | test("native hidden page clears private rows and rejects late response",async({request})=>{
  254 |   const {page,close}=await nativePage(evidence,"live-settings-native-");ledgers.set(page,[]);
  255 |   privateMonitors(page);let passed=false;
  256 |   try {
  257 |     await login(page);await page.goto(settings("en"));await expect(blockRow(page,f.entry)).toBeVisible();
  258 |     await fixture(request,"fault",{mode:"hold-report"});const pending=page.reload();await expect.poll(async()=>(await facts(request)).held).toBe(true);await pending;
  259 |     const cover=await page.context().newPage();await cover.goto("about:blank");await cover.bringToFront();await expect.poll(()=>page.evaluate(()=>document.visibilityState)).toBe("hidden");
  260 |     await fixture(request,"fault",{mode:"release"});
  261 |     await expect(blockRow(page,f.entry)).toHaveCount(0);await expect.poll(()=>page.evaluate(()=>/PRIVATE_[A-Z_]+/.test(document.body.textContent??""))).toBe(false);
  262 |     await page.bringToFront();await expect(blockRow(page,f.entry)).toBeVisible();await cover.close();await privacyCheck(page);
  263 |     passed=true;
  264 |   } finally {
  265 |     await writeFile(`${evidence}/click-ledger-native.json`,JSON.stringify({page:"/[locale]/studio/settings",rows:ledgers.get(page),visibility:"native hidden -> visible",result:passed?"PASS":"FAIL"}));
  266 |     await close();
  267 |   }
  268 | });
  269 | 
  270 | test("read-only signed merchant cannot write; CSRF denies owner command",async({page,request,browser})=>{
  271 |   await login(page);await page.goto(settings("en"));await expect(page.getByTestId("live-settings")).toBeVisible();
  272 |   // SECURITY/NEGATIVE: explicit API probes exercise actual Next CSRF and real PG permission checks; they are not click acceptance.
  273 |   const cookies=await page.context().cookies(origin.replace(/^http:/,"https:"));
  274 |   const cookieHeader=cookies.filter(c=>["__Host-commerce_session","__Host-commerce_csrf"].includes(c.name)).map(c=>`${c.name}=${c.value}`).join("; ");
  275 |   expect(cookies.some(c=>c.name==="__Host-commerce_session")).toBe(true);
  276 |   const denied=await request.put(`${origin}/api/stores/${f.store}/live-settings/sold-out-reply`,{
  277 |     headers:{Cookie:cookieHeader,Origin:origin,"Idempotency-Key":randomBytes(16).toString("hex")},
  278 |     data:{enabled:false,template_id:"sold-out-reply/v1",template_version:1,expected_version:3},
  279 |   });expect(denied.status()).toBe(403);expect((await facts(request)).receipts).toHaveLength(0);
  280 |   const context=await browser.newContext({ignoreHTTPSErrors:true});
  281 |   try {
  282 |     const csrf=randomBytes(32).toString("base64url"),token=required("LC_BROWSER_SETTINGS_READ_TOKEN");
  283 |     await context.addCookies([
  284 |       {name:"__Host-commerce_session",value:token,url:origin.replace(/^http:/,"https:"),secure:true,httpOnly:true,sameSite:"Lax"},
  285 |       {name:"__Host-commerce_csrf",value:csrf,url:origin.replace(/^http:/,"https:"),secure:true,httpOnly:false,sameSite:"Lax"},
  286 |     ]);
  287 |     const readPage=await context.newPage();await readPage.goto(settings("en"));await expect(readPage.getByTestId("live-settings")).toBeVisible();
  288 |     for(const id of ["reminder-trigger","sold-out-save","blocklist-remove"]) {
  289 |       const controls=readPage.getByTestId(id);
  290 |       if(await controls.count())await expect(controls.first()).toBeDisabled();
  291 |     }
  292 |     const response=await request.put(`${origin}/api/stores/${f.store}/live-settings/sold-out-reply`,{
  293 |       headers:{Cookie:`__Host-commerce_session=${token}; __Host-commerce_csrf=${csrf}`,Origin:origin,"X-CSRF-Token":csrf,"Idempotency-Key":randomBytes(16).toString("hex")},
  294 |       data:{enabled:false,template_id:"sold-out-reply/v1",template_version:1,expected_version:3},
  295 |     });expect(response.status()).toBe(403);expect((await facts(request)).receipts).toHaveLength(0);
```