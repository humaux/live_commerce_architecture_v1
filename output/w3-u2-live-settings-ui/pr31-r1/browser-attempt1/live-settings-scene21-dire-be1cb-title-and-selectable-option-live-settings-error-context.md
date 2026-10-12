# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: live-settings.spec.ts >> scene21 direct link shows authoritative title and selectable option
- Location: tests/admin/live-settings.spec.ts:362:1

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
    24 × locator resolved to <option value="" disabled>Choose a session</option>
       - unexpected value "enabled"

```

# Test source

```ts
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
  296 |   } finally {await context.close();}
  297 | });
  298 | 
  299 | // FIXTURE/AUTH ONLY: inject a server-issued signed session, as in the existing read-only staff case.
  300 | // No product request or BFF response is seeded/replaced; browser UI executes every business command.
  301 | async function signedStaff(page:Page, tokenName:string) {
  302 |   await page.context().clearCookies();
  303 |   const csrf=randomBytes(32).toString("base64url");
  304 |   await page.context().addCookies([
  305 |     {name:"__Host-commerce_session",value:required(tokenName),url:origin.replace(/^http:/,"https:"),secure:true,httpOnly:true,sameSite:"Lax"},
  306 |     {name:"__Host-commerce_csrf",value:csrf,url:origin.replace(/^http:/,"https:"),secure:true,httpOnly:false,sameSite:"Lax"},
  307 |   ]);
  308 | }
  309 | 
  310 | test("split signed staff sees reminder result without report read and local send404",async({page,request})=>{
  311 |   await signedStaff(page,"LC_BROWSER_SETTINGS_SPLIT_TOKEN");await page.goto(settings("en"));
  312 |   await expect(page.getByTestId("reminder-trigger")).toBeEnabled();
  313 |   await expect(blockRow(page,f.entry)).toBeVisible();
  314 |   const reminders:{method:string;status:number}[]=[];
  315 |   page.on("response",response=>{if(new URL(response.url()).pathname.endsWith("/reminders"))reminders.push({method:response.request().method(),status:response.status()});});
  316 |   await step(page,"reminder-trigger","click","single successful POST; aggregate receipt visible without inbox:read GET",async()=>{
  317 |     const response=await waitWrite(page,"/reminders","POST",()=>page.getByTestId("reminder-trigger").click());
  318 |     expect(response.status()).toBe(200);
  319 |     await expect(page.getByTestId("reminder-result")).toContainText("Queued 2");
  320 |     await expect(page.getByTestId("reminder-report")).toHaveCount(0);
  321 |     await expect(page.getByTestId("sold-out-enabled")).toBeVisible();await expect(blockRow(page,f.entry)).toBeVisible();
  322 |     expect((await facts(request)).receipts.filter(r=>r.action==="remind")).toHaveLength(1);
  323 |     expect(responseFacts.get(page)?.filter(r=>r.resource==="reminders"&&r.method==="GET")).toHaveLength(0);
  324 |     expect(reminders).toEqual([{method:"POST",status:200}]);
  325 |   });
  326 |   await fixture(request,"fault",{mode:"missing-reminders"});
  327 |   await step(page,"reminder-trigger","click","unmounted send route produces local unavailable panel and preserves healthy sections",async()=>{
  328 |     const response=await waitWrite(page,"/reminders","POST",()=>page.getByTestId("reminder-trigger").click());expect(response.status()).toBe(404);
  329 |     await expect(page.getByTestId("reminders-unavailable")).toBeVisible();await expect(page.getByTestId("reminder-trigger")).toBeDisabled();
  330 |     await expect(page.getByTestId("sold-out-enabled")).toBeVisible();await expect(blockRow(page,f.entry)).toBeVisible();
  331 |     expect((await facts(request)).receipts.filter(r=>r.action==="remind")).toHaveLength(1);
  332 |     expect(reminders).toEqual([{method:"POST",status:200},{method:"POST",status:404}]);
  333 |     expect(responseFacts.get(page)?.filter(r=>r.resource==="reminders"&&r.method==="GET")).toHaveLength(0);
  334 |   });await privacyCheck(page);
  335 | });
  336 | 
  337 | test("scene picker loads page21 and targets reminders and blocklist",async({page,request})=>{
  338 |   await fixture(request,"fault",{mode:"many-scenes"});await login(page);await page.goto(settings("en",f.store,""));
  339 |   await expect(page.getByTestId("scene-select").locator('option:not([value=""])')).toHaveCount(20);
  340 |   await expect(page.getByTestId("scene-select").locator(`option[value="${f.later_scene}"]`)).toHaveCount(0);
  341 |   await step(page,"scene-more","click","actual next cursor read appends the 21st scene",async()=>{
  342 |     const response=await waitWrite(page,"/live-sessions","GET",()=>page.getByTestId("scene-more").click());
  343 |     expect(response.status()).toBe(200);expect(new URL(response.url()).searchParams.get("cursor")).toBe("mock-scenes-page-two");
  344 |     await expect(page.getByTestId("scene-select").locator('option:not([value=""])')).toHaveCount(21);
  345 |     await expect(page.getByTestId("scene-select").locator(`option[value="${f.later_scene}"]`)).toHaveText("MOCK settings scene 21");
  346 |     await expect(page.getByTestId("scene-more")).toHaveCount(0);
  347 |   });
  348 |   await step(page,"scene-select","selectOption","scene21 requests its own reminder report and blocklist",async()=>{
  349 |     const report=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith(`/live-sessions/${f.later_scene}/reminders`)&&r.request().method()==="GET");
  350 |     const list=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith(`/live-sessions/${f.later_scene}/claims/blocklist`)&&r.request().method()==="GET");
  351 |     await page.getByTestId("scene-select").selectOption(f.later_scene);
  352 |     expect((await report).status()).toBe(200);expect((await list).status()).toBe(200);
  353 |     await expect(page.getByTestId("scene-select")).toHaveValue(f.later_scene);await expect(blockRow(page,f.later_entry)).toBeVisible();await expect(blockRow(page,f.entry)).toHaveCount(0);
  354 |   });
  355 |   await step(page,"reminder-trigger","click","scene21 business command targets selected scene exactly once",async()=>{
  356 |     const response=await waitWrite(page,`/live-sessions/${f.later_scene}/reminders`,"POST",()=>page.getByTestId("reminder-trigger").click());expect(response.status()).toBe(200);
  357 |     await expect(page.getByTestId("reminder-result")).toContainText("Queued 2");expect((await facts(request)).receipts.filter(r=>r.action==="remind"&&r.effect)).toHaveLength(1);
  358 |   });
  359 |   await page.reload();await expect(page.getByTestId("scene-select")).toHaveValue(f.later_scene);await expect(blockRow(page,f.later_entry)).toBeVisible();await privacyCheck(page);
  360 | });
  361 | 
  362 | test("scene21 direct link shows authoritative title and selectable option",async({page,request})=>{
  363 |   await fixture(request,"fault",{mode:"many-scenes"});await login(page);
  364 |   const detail=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith(`/live-sessions/${f.later_scene}`)&&r.request().method()==="GET");
  365 |   await page.goto(settings("en",f.store,f.later_scene));expect((await detail).status()).toBe(200);
  366 |   await expect(page.getByTestId("scene-select")).toHaveValue(f.later_scene);
  367 |   await expect(page.getByTestId("scene-select").locator(`option[value="${f.later_scene}"]`)).toHaveText("MOCK settings scene 21");
> 368 |   await expect(page.getByTestId("scene-select").locator('option[value=""]')).toBeDisabled();await expect(blockRow(page,f.later_entry)).toBeVisible();
      |                                                                              ^ Error: expect(locator).toBeDisabled() failed
  369 |   await step(page,"live-settings-refresh","click","refresh retains scene21 title from actual scoped detail",async()=>{
  370 |     await page.getByTestId("live-settings-refresh").click();await expect(page.getByTestId("scene-select")).toBeEnabled();
  371 |     await expect(page.getByTestId("scene-select").locator(`option[value="${f.later_scene}"]`)).toHaveText("MOCK settings scene 21");await expect(blockRow(page,f.later_entry)).toBeVisible();
  372 |   });await privacyCheck(page);
  373 | });
  374 | 
  375 | test("unmounted reminder report stays local while settings and blocklist work",async({page,request})=>{
  376 |   await fixture(request,"fault",{mode:"missing-reminders"});await login(page);
  377 |   const missing=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith("/reminders")&&r.request().method()==="GET");
  378 |   await page.goto(settings("en"));expect((await missing).status()).toBe(404);
  379 |   await expect(page.getByTestId("reminders-unavailable")).toBeVisible();await expect(page.getByTestId("reminder-trigger")).toBeDisabled();
  380 |   await expect(page.getByTestId("sold-out-enabled")).toBeChecked();await expect(blockRow(page,f.entry)).toBeVisible();
  381 |   await step(page,"sold-out-enabled","click","sold-out draft remains operable after independent report404",async()=>{
  382 |     await page.getByTestId("sold-out-enabled").click();await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();
  383 |   });
  384 |   await step(page,"sold-out-template","fill","merchant template editor remains operable",async()=>{
  385 |     await page.getByTestId("sold-out-template").fill("MOCK unavailable {{product.name}} sold out");await expect(page.getByTestId("sold-out-preview")).toContainText("MOCK unavailable");
  386 |   });
  387 |   await step(page,"sold-out-save","click","actual publish/save persists despite optional reminder service404",async()=>{
  388 |     const response=await waitWrite(page,"/live-settings/sold-out-reply","PUT",()=>page.getByTestId("sold-out-save").click());expect(response.status()).toBe(200);
  389 |     await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();expect((await facts(request)).enabled).toBe(false);
  390 |   });
  391 |   await step(page,"blocklist-next","click","restriction list remains independently pageable",async()=>{
  392 |     await page.getByTestId("blocklist-next").click();await expect(blockRow(page,f.next_entry)).toBeVisible();
  393 |   });
  394 |   await page.reload();await expect(page.getByTestId("reminders-unavailable")).toBeVisible();await expect(page.getByTestId("sold-out-enabled")).not.toBeChecked();await expect(blockRow(page,f.entry)).toBeVisible();await privacyCheck(page);
  395 | });
  396 | 
  397 | const stateCopy = {
  398 |   "en": ["Sent","Queued","Failed","Blocked","Delivery uncertain"],
  399 |   "zh-TW": ["已傳送","排隊中","傳送失敗","已阻擋","結果未確認"],
  400 |   "zh-CN": ["已发送","排队中","发送失败","已阻止","结果未确认"],
  401 | } as const;
  402 | const sentCopy = {"en":"Sold-out reply sent","zh-TW":"沒貨已回覆","zh-CN":"没货已回复"} as const;
  403 | for(const locale of locales)for(const viewport of viewports) {
  404 |   test(`five sold-out delivery states real clicks and masked badges ${locale}-${viewport.width}`,async({page,request})=>{
  405 |     await fixture(request,"fault",{mode:"reply-states"});await page.setViewportSize(viewport);await login(page);
  406 |     await page.goto(`${origin}/${locale}/studio/console?store=${f.store}&scene=${f.scene}`);
  407 |     const assertStates=async()=>{
  408 |       await expect(page.getByTestId("comment-rows").locator('[data-testid^="comment-row-"]')).toHaveCount(5);
  409 |       await expect(page.locator('[data-testid^="comment-soldout-sent-"]')).toHaveCount(1);
  410 |       for(let i=0;i<f.reply_refs.length;i++) {
  411 |         const ref=f.reply_refs[i]!;
  412 |         await expect(page.getByTestId(`comment-send-state-${ref}`)).toHaveText(stateCopy[locale][i]!);
  413 |         if(i===0)await expect(page.getByTestId(`comment-soldout-sent-${ref}`)).toHaveText(sentCopy[locale]);
  414 |         else await expect(page.getByTestId(`comment-soldout-sent-${ref}`)).toHaveCount(0);
  415 |       }
  416 |     };
  417 |     await assertStates();
  418 |     for(const ref of f.reply_refs)await step(page,`comment-select-${ref}`,"click","selected row shows buyer panel without claiming unsent reply delivered",async()=>{
  419 |       await page.getByTestId(`comment-select-${ref}`).click();await expect(page.getByTestId(`comment-row-${ref}`)).toHaveAttribute("data-selected","true");await expect(page.getByTestId("buyer-panel")).toBeVisible();await assertStates();
  420 |     });
  421 |     await page.reload();await assertStates();
  422 |     // READ/MEASURE: custom screenshot keeps five public status/badge rows visible.
  423 |     // Only author/body fields and unrelated private panels are masked; the whole comment list is NOT masked.
  424 |     expect(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
  425 |     await page.screenshot({path:`${evidence}/soldout-states-${locale}-${viewport.width}.png`,fullPage:true,animations:"disabled",mask:[page.locator('[data-private="comment-author"], [data-private="comment-text"]'),page.getByTestId("buyer-panel"),page.getByTestId("buyer-block-note")]});
  426 |     await privacyCheck(page);
  427 |   });
  428 | }
  429 | 
```