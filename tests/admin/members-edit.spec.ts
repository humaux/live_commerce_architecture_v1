// Purpose: native member profile PATCH acceptance, separate from immutable create/adjust cases.
// Depends on: W6 signed identity, production Next/Go/PG and isolated synthetic edit store.
// Used by: --browser-customers-billing; only fault envelopes are MOCK at the BFF boundary.
import { test, expect, type Page, type Browser } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
const required = (key: string) => { const v = process.env[key]; if (!v) throw Error(`${key} required`); return v; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE");
const alt = required("LC_W6UI_EDIT_ALT"), foreignMember=required("LC_W6UI_FOREIGN_MEMBER");
const store = required("LC_W6UI_EDIT_STORE"), member = required("LC_W6UI_EDIT_MEMBER");
const control = required("LC_W6UI_CONTROL_URL"), key = required("LC_W6UI_CONTROL_KEY");
const ledger: {case:string;page?:string;control?:string;action:string;expected?:string;actual?:string;result:string}[] = [];
const sourceRoles: Record<string,{node:string;pngSHA256:string;responseSHA256:string}> = {
  "Edit-desktop": {
    "node": "1782:75",
    "pngSHA256": "dca79404654b3e2422e05a040481e2f64661cc5fef6c5ec226512955973fb541",
    "responseSHA256": "6077f65b30b55cbc6f4914898e7ed5d1ccb179495e7bc4f3f0b21b0951293f64"
  },
  "Edit-mobile": {
    "node": "1782:190",
    "pngSHA256": "8d3312bb0481ad3b6d32faea70ea1009579fb2aceb0cb71b410d2aed23a4bc73",
    "responseSHA256": "c4c3ebc0ffa6d80eb7b4950964e4fed19b6514c76180ff26700c8a213a0ada3a"
  },
  "Conflict-desktop": {
    "node": "1782:251",
    "pngSHA256": "dd84fe75a09130aefa89818ea7f228ddcc1c42452dc5f817be6d5d5ba816dff9",
    "responseSHA256": "a03ba52aae00b73400713b07ee411876681d57cf7079e9b0b11bf6e0195e4a77"
  },
  "Conflict-mobile": {
    "node": "1782:366",
    "pngSHA256": "e754eebf3a6243a9e566da9cf3d32d99a63c1b30e06db3ac44d42c2699f04588",
    "responseSHA256": "e14eebf844ca63195ab87e93fa7b0237980c14ce1f61933ff2df45fe0d82e38a"
  },
  "EditUnknown-desktop": {
    "node": "1556:80743",
    "pngSHA256": "76fc042bd53ebd3c194469b59d5ad38ef46b1cd925f44aa6cc420d567064bb7a",
    "responseSHA256": "6def0b0de680e438190ebaaacb45cf327677a55d0d46141221dca6b45da88d24"
  },
  "EditUnknown-mobile": {
    "node": "1556:80868",
    "pngSHA256": "f73af856af8580a3bedf976a436e8f37857a9afa661c62f5683847b8793684ab",
    "responseSHA256": "9577d02cf79824969443049290b3aad8037d4528ce8d89dfb74fa90c25011c22"
  },
  "EditEmptyName-desktop": {
    "node": "1784:50",
    "pngSHA256": "bc0f548e73c490b64100f820943e88e606eeca2829ebbd79eb431a696330afd1",
    "responseSHA256": "b85da220a45bb85ed56ddd35b75d32208c856236cf559c15843962a47a8c45bc"
  },
  "EditEmptyName-mobile": {
    "node": "1784:165",
    "pngSHA256": "8c31192c82f3fee79385b105ddd16a5e24b80780626834cf53faae7255908cf4",
    "responseSHA256": "2c3d1c64c5e4dc026518f19af97ddad00b8f379700798dddef8fdbbbe939bd98"
  }
};
const geometry: unknown[] = [];
let active = "";
test.describe.configure({ mode: "serial" });
test.beforeEach(async ({}, info) => { active = info.title; });
test.afterEach(async ({}, info) => {
  ledger.push({case:active,action:"native edit journey",expected:"all visible case assertions pass",actual:info.status??"unknown",result:info.status==="passed"?"pass":"fail"});
  await writeFile(path.join(evidence, "members-edit-click-ledger.json"), JSON.stringify(ledger, null, 2));
  await writeFile(path.join(evidence, "members-edit-geometry.json"), JSON.stringify(geometry, null, 2));
});
async function ctl(resource: string, mode = "") {
  const r = await fetch(`${control}/${resource}`, {method:"POST", headers:{"Content-Type":"application/json","X-W6UI-Gate-Key":key},body:JSON.stringify({mode})});
  expect(r.status).toBe(204); return r;
}
const url = (locale = "en") => `${origin}/${locale}/members/${member}?store=${store}`;
async function signed(browser: Browser, locale = "en", width = 1440, actor = "forms-points") {
  await ctl("actor", actor);
  const context = await browser.newContext({ignoreHTTPSErrors:true,viewport:{width,height:900}});
  const page = await context.newPage();
  await page.goto(`${origin}/en/`);
  await page.getByRole("button",{name:"Sign in with identity service"}).click();
  await expect(page.getByTestId("shell-store-selector")).toBeVisible();
  await page.goto(url(locale));
  await expect(page.getByTestId("member-detail")).toBeVisible();
  return {page,context};
}
async function click(page:Page,id:string){
  const row={case:active,page:new URL(page.url()).pathname,control:id,action:"click",expected:"native control accepts interaction; case asserts visible outcome",actual:"pending",result:"fail"};ledger.push(row);
  await page.getByTestId(id).click();row.actual="native click settled";row.result="pass";
}
async function fill(page:Page,field:string,value:string){
  const row={case:active,page:new URL(page.url()).pathname,control:field,action:"fill",expected:"field accepts synthetic input",actual:"pending",result:"fail"};ledger.push(row);
  const input=dialog(page).locator(`[name="${field}"]`);await input.fill(value);await expect(input).toHaveValue(value);row.actual="visible field value matched";row.result="pass";
}
async function dismiss(page:Page){
  const row={case:active,page:new URL(page.url()).pathname,control:"member-edit-dismiss",action:"click",expected:"dialog closes",actual:"pending",result:"fail"};ledger.push(row);
  await dialog(page).getByRole("button").last().click();await expect(dialog(page)).not.toBeVisible();row.actual="dialog closed";row.result="pass";
}
async function escape(page:Page,retained=false){
  const row={case:active,page:new URL(page.url()).pathname,control:"member-edit-dialog",action:"keyboard Escape",expected:retained?"unconfirmed intent remains visible":"local draft closes",actual:"pending",result:"fail"};ledger.push(row);
  await page.keyboard.press("Escape");if(retained)await expect(dialog(page)).toBeVisible();else await expect(dialog(page)).not.toBeVisible();row.actual="visible state matched";row.result="pass";
}
const dialog = (page: Page) => page.getByTestId("member-edit-dialog");
async function open(page: Page) {
  await expect(page.getByTestId("member-edit-open")).toBeVisible();
  await click(page,"member-edit-open");
  await expect(dialog(page)).toBeVisible();
  await expect(dialog(page).locator('[name="display_name"]')).toBeFocused();
}
async function capture(page: Page, role: string) {
  const d = dialog(page);
  const m = await d.evaluate(el => {
    const r = el.getBoundingClientRect(), css = getComputedStyle(el);
    return {width:r.width,height:r.height,x:r.x,y:r.y,padding:css.padding,borderRadius:css.borderRadius,
      fields:Array.from(el.querySelectorAll<HTMLInputElement|HTMLTextAreaElement>("input,textarea")).map(f=>({name:f.name,height:f.getBoundingClientRect().height,border:getComputedStyle(f).borderColor,readonly:f.readOnly})),
      buttons:Array.from(el.querySelectorAll("button")).map(b=>({text:b.textContent,height:b.getBoundingClientRect().height}))};
  });
  const mobile=page.viewportSize()!.width===375;
  expect(m.width).toBe(mobile?375:520);
  expect(m.padding).toBe(mobile?"20px 16px 28px":"28px");
  expect(m.borderRadius).toBe(mobile?"16px 16px 0px 0px":"12px");
  expect(m.fields.length).toBe(role==="EditUnknown"?0:3);
  for(const f of m.fields){
    if(f.name==="notes")expect(f.height).toBe(88);
    if(role!=="EditEmptyName"||f.name!=="display_name")expect(f.border).toBe("rgb(140, 140, 140)");
    if(role==="Conflict")expect(f.readonly).toBe(true);
  }
  expect(m.buttons.length).toBe(2);
  expect(m.x).toBeGreaterThanOrEqual(0); expect(m.width).toBeLessThanOrEqual(page.viewportSize()!.width);
  for (const f of m.fields) expect(f.height).toBeGreaterThanOrEqual(44);
  for (const b of m.buttons) expect(b.height).toBeGreaterThanOrEqual(44);
  const file = `members-edit-${active.replace(/[^a-zA-Z0-9-]/g,"-")}-${role}.png`;
  const png = await d.screenshot({path:path.join(evidence,file)});
  const viewportFile=file.replace(/\.png$/,"-viewport.png");
  const viewportPNG=await page.screenshot({path:path.join(evidence,viewportFile)});
  geometry.push({case:active,role,checkout:required("LC_M7_SOURCE_SHA"),sourcePromotionSHA256:"fe61c66adacd2888c152f905d9e6a293ee04d886ae008b64c5b505aa895ccc73",source:sourceRoles[`${role}-${mobile?"mobile":"desktop"}`],measurement:m,viewport:{file:viewportFile,sha256:createHash("sha256").update(viewportPNG).digest("hex")},file,sha256:createHash("sha256").update(png).digest("hex")});
}
for (const locale of ["en","zh-TW","zh-CN"] as const) for (const width of [1440,375]) {
  test(`REAL_PG edit ${locale}-${width}`, async ({browser}) => {
    const {page,context} = await signed(browser,locale,width);
    try {
      let writes = 0; page.on("request",r=>{if(r.method()==="PATCH" && r.url().includes(`/members/${member}`)) writes++;});
      await open(page); await capture(page,"Edit");
      await dismiss(page);
      await expect(page.getByTestId("member-edit-open")).toBeFocused();await open(page);
      await fill(page,"display_name","");
      await click(page,"member-edit-save");
      await expect(dialog(page).locator('[name="display_name"]')).toHaveAttribute("aria-invalid","true");
      await expect(dialog(page).locator('[name="display_name"]')).toBeFocused();
      expect(writes).toBe(0);
      await expect(dialog(page).getByRole("alert")).toHaveText(locale==="zh-TW"?"請填寫顯示名稱。":locale==="zh-CN"?"请填写显示名称。":"Enter a display name.");
      await capture(page,"EditEmptyName");
      const name = `Synthetic edit ${locale} ${width}`;
      await fill(page,"display_name",name);
      await fill(page,"contact_note","Synthetic contact");
      await fill(page,"notes","Synthetic private note");
      await click(page,"member-edit-save");
      await expect(dialog(page)).not.toBeVisible();
      await expect(page.getByRole("heading",{name,exact:true})).toBeVisible();
      await page.reload(); await expect(page.getByRole("heading",{name,exact:true})).toBeVisible();
      await open(page); await expect(dialog(page).locator('[name="contact_note"]')).toHaveValue("Synthetic contact");
      await expect(dialog(page).locator('[name="notes"]')).toHaveValue("Synthetic private note");
      expect(writes).toBe(1);
      await escape(page);
      await expect(page.getByTestId("member-edit-open")).toBeFocused();
      expect(page.url()).not.toContain("Synthetic");
    } finally { await context.close(); }
  });
}
for (const locale of ["en","zh-TW","zh-CN"] as const) for (const width of [1440,375]) {
  test(`REAL_PG CAS and MOCK BFF UNKNOWN ${locale}-${width}`, async ({browser}) => {
    const {page,context} = await signed(browser,locale,width);
    try {
      await open(page);
      await fill(page,"display_name","Synthetic retained intent");
      const other = await context.newPage(); await other.goto(url(locale)); await open(other);
      await fill(other,"notes",`Concurrent ${locale} ${width}`);
      await click(other,"member-edit-save"); await expect(dialog(other)).not.toBeVisible(); await other.close();
      await click(page,"member-edit-save");
      await expect(page.getByTestId("member-edit-load-current")).toBeVisible(); await capture(page,"Conflict");
      await click(page,"member-edit-load-current");
      await expect(dialog(page).locator('[name="notes"]')).toHaveValue(`Concurrent ${locale} ${width}`);
      await expect(dialog(page).locator('[name="display_name"]')).not.toHaveValue("Synthetic retained intent");
      let writes=0;
      await page.route(`**/api/stores/${store}/members/${member}`,async route=>{
        if(route.request().method()!=="PATCH") return route.continue();
        writes++; const response=await route.fetch(); expect(response.status()).toBe(200);
        await route.fulfill({response,body:"{"});
      });
      await fill(page,"display_name",`Unknown ${locale} ${width}`);
      await click(page,"member-edit-save");
      await expect(page.getByTestId("member-edit-unknown")).toBeVisible();
      await capture(page,"EditUnknown");
      // Corrected author defect: formal UNKNOWN has no fields; an empty readonly loop proves nothing.
      await expect(dialog(page).locator("input,textarea")).toHaveCount(0);
      await click(page,"member-edit-refresh");
      await expect(page.getByTestId("member-edit-unknown")).toBeVisible();
      await expect(page.getByTestId("member-edit-save")).toHaveCount(0); expect(writes).toBe(1);
      await escape(page,true);
      await dismiss(page);
      await click(page,"member-edit-open");
      await expect(page.getByTestId("member-edit-unknown")).toBeVisible();expect(writes).toBe(1);
    } finally { await context.close(); }
  });
}
test("REAL_PG store session and unavailable read fence private draft",async({browser})=>{
  const {page,context}=await signed(browser);
  try{
    // Native store-selection history creates a genuine same-tab scope change while the modal is open.
    await page.getByTestId("shell-store-selector").selectOption(alt);
    await expect(page.getByTestId("members-list")).toBeVisible();
    // Enter the original detail directly so Back targets the alternate store, not another original-store entry.
    await page.goto(url());
    await open(page);await fill(page,"notes","Synthetic scope private draft");
    await page.goBack();await expect(page.getByTestId("shell-store-selector")).toHaveValue(alt);await expect(dialog(page)).not.toBeVisible();
    await page.goto(url());await open(page);
    await expect(dialog(page).locator('[name="notes"]')).not.toHaveValue("Synthetic scope private draft");
    await fill(page,"notes","Synthetic member private draft");
    // Deep-link navigation is read/scope setup only; all mutations still use native Save.
    await page.goto(`${origin}/en/members/${foreignMember}?store=${store}`);
    await expect(dialog(page)).not.toBeVisible();
    await expect(page.locator('input[name="display_name"],textarea[name="notes"]')).toHaveCount(0);
    expect(await page.content()).not.toContain("Synthetic member private draft");
    await page.goto(url());await open(page);
    await expect(dialog(page).locator('[name="notes"]')).not.toHaveValue("Synthetic member private draft");
    await fill(page,"notes","Synthetic session private draft");
    const other=await context.newPage();await other.goto(url());
    await other.locator("summary").filter({hasText:"Account"}).click();
    await other.getByTestId("workspace-sign-out").click();
    await expect(dialog(page)).not.toBeVisible();
    expect(await page.content()).not.toContain("Synthetic session private draft");
    await other.close();
  }finally{await context.close();}
});
test("MOCK BFF refused read boundary conceals and reloads private draft",async({browser})=>{
  const {page,context}=await signed(browser);
  try{
    await open(page);await fill(page,"notes","Synthetic unavailable private draft");
    await page.route(`**/api/stores/${store}/members/${member}`,async route=>{
      if(route.request().method()==="PATCH")return route.fulfill({status:409,contentType:"application/json",headers:{"cache-control":"private, no-store"},body:JSON.stringify({code:"conflict"})});
      return route.fulfill({status:503,contentType:"application/json",headers:{"cache-control":"private, no-store"},body:JSON.stringify({code:"retry_later"})});
    });
    await click(page,"member-edit-save");await expect(page.getByTestId("member-edit-load-current")).toBeVisible();
    await click(page,"member-edit-load-current");await expect(dialog(page)).not.toBeVisible();
    await expect(page.locator('input[name="display_name"],textarea[name="notes"]')).toHaveCount(0);
    expect(await page.content()).not.toContain("Synthetic unavailable private draft");
  }finally{await context.close();}
});
test("REAL_PG revoked points permission clears draft without PATCH effect",async({browser})=>{
  const {page,context}=await signed(browser);
  try{
    await open(page);await fill(page,"notes","Synthetic forbidden private draft");
    await ctl("forms-points-grant","deny");
    await click(page,"member-edit-save");
    await expect(dialog(page)).not.toBeVisible();await expect(page.getByTestId("member-edit-open")).toHaveCount(0);
    expect(await page.content()).not.toContain("Synthetic forbidden private draft");
  }finally{await ctl("forms-points-grant","restore");await context.close();}
});
test("REAL_PG reader has no edit and erased clears private draft",async({browser})=>{
  const reader=await signed(browser,"en",1440,"forms-reader");
  await expect(reader.page.getByTestId("member-edit-open")).toHaveCount(0); await reader.context.close();
  const {page,context}=await signed(browser);
  try{
    await open(page); await fill(page,"notes","Synthetic unsaved private draft");
    await ctl("edit-erase",member); await click(page,"member-edit-save");
    await expect(dialog(page)).not.toBeVisible();
    await expect(page.getByText("[erased]",{exact:true}).first()).toBeVisible();
    await expect(page.getByTestId("member-edit-open")).toBeDisabled();
    await expect(page.locator('input,textarea')).toHaveCount(0);
    expect(await page.content()).not.toContain("Synthetic unsaved private draft");
  }finally{await context.close();}
});
