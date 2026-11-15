// Purpose: native member profile PATCH acceptance, separate from immutable create/adjust cases.
// Depends on: W6 signed identity, production Next/Go/PG and isolated synthetic edit store.
// Used by: --browser-customers-billing; only fault envelopes are MOCK at the BFF boundary.
import { test, expect, type Page, type Browser } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
const required = (key: string) => { const v = process.env[key]; if (!v) throw Error(`${key} required`); return v; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_W6UI_EDIT_STORE"), member = required("LC_W6UI_EDIT_MEMBER");
const control = required("LC_W6UI_CONTROL_URL"), key = required("LC_W6UI_CONTROL_KEY");
const ledger: {case: string; action: string; result: string}[] = [];
const geometry: unknown[] = [];
let active = "";
test.describe.configure({ mode: "serial" });
test.beforeEach(async ({}, info) => { active = info.title; });
test.afterEach(async ({}, info) => {
  ledger.push({case: active, action: "native edit journey", result: info.status ?? "unknown"});
  await writeFile(path.join(evidence, "members-edit-click-ledger.json"), JSON.stringify(ledger, null, 2));
  await writeFile(path.join(evidence, "members-edit-geometry.json"), JSON.stringify(geometry, null, 2));
});
async function ctl(resource: string, mode = "") {
  const r = await fetch(`${control}/${resource}`, {method:"POST", headers:{"Content-Type":"application/json","X-W6UI-Gate-Key":key},body:JSON.stringify({mode})});
  expect(r.status).toBe(200); return r;
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
const dialog = (page: Page) => page.getByTestId("member-edit-dialog");
async function open(page: Page) {
  await page.getByTestId("member-edit-open").click();
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
  expect(m.x).toBeGreaterThanOrEqual(0); expect(m.width).toBeLessThanOrEqual(page.viewportSize()!.width);
  for (const f of m.fields) expect(f.height).toBeGreaterThanOrEqual(44);
  for (const b of m.buttons) expect(b.height).toBeGreaterThanOrEqual(44);
  const file = `members-edit-${active.replace(/[^a-zA-Z0-9-]/g,"-")}-${role}.png`;
  const png = await d.screenshot({path:path.join(evidence,file)});
  geometry.push({case:active,role,measurement:m,file,sha256:createHash("sha256").update(png).digest("hex")});
}
for (const locale of ["en","zh-TW","zh-CN"] as const) for (const width of [1440,375]) {
  test(`REAL_PG edit ${locale}-${width}`, async ({browser}) => {
    const {page,context} = await signed(browser,locale,width);
    try {
      let writes = 0; page.on("request",r=>{if(r.method()==="PATCH" && r.url().includes(`/members/${member}`)) writes++;});
      await open(page); await capture(page,"Edit");
      await dialog(page).locator('[name="display_name"]').fill("");
      await page.getByTestId("member-edit-save").click();
      await expect(dialog(page).locator('[name="display_name"]')).toHaveAttribute("aria-invalid","true");
      await expect(dialog(page).locator('[name="display_name"]')).toBeFocused();
      expect(writes).toBe(0); await capture(page,"EditEmptyName");
      const name = `Synthetic edit ${locale} ${width}`;
      await dialog(page).locator('[name="display_name"]').fill(name);
      await dialog(page).locator('[name="contact_note"]').fill("Synthetic contact");
      await dialog(page).locator('[name="notes"]').fill("Synthetic private note");
      await page.getByTestId("member-edit-save").click();
      await expect(dialog(page)).not.toBeVisible();
      await expect(page.getByRole("heading",{name,exact:true})).toBeVisible();
      await page.reload(); await expect(page.getByRole("heading",{name,exact:true})).toBeVisible();
      await open(page); await expect(dialog(page).locator('[name="contact_note"]')).toHaveValue("Synthetic contact");
      await expect(dialog(page).locator('[name="notes"]')).toHaveValue("Synthetic private note");
      expect(writes).toBe(1);
      await page.keyboard.press("Escape"); await expect(dialog(page)).not.toBeVisible();
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
      await dialog(page).locator('[name="display_name"]').fill("Synthetic retained intent");
      const other = await context.newPage(); await other.goto(url(locale)); await open(other);
      await dialog(other).locator('[name="notes"]').fill(`Concurrent ${locale} ${width}`);
      await other.getByTestId("member-edit-save").click(); await expect(dialog(other)).not.toBeVisible(); await other.close();
      await page.getByTestId("member-edit-save").click();
      await expect(page.getByTestId("member-edit-load-current")).toBeVisible(); await capture(page,"Conflict");
      await page.getByTestId("member-edit-load-current").click();
      await expect(dialog(page).locator('[name="notes"]')).toHaveValue(`Concurrent ${locale} ${width}`);
      await expect(dialog(page).locator('[name="display_name"]')).not.toHaveValue("Synthetic retained intent");
      let writes=0;
      await page.route(`**/api/stores/${store}/members/${member}`,async route=>{
        if(route.request().method()!=="PATCH") return route.continue();
        writes++; const response=await route.fetch(); expect(response.status()).toBe(200);
        await route.fulfill({response,body:"{"});
      });
      await dialog(page).locator('[name="display_name"]').fill(`Unknown ${locale} ${width}`);
      await page.getByTestId("member-edit-save").click();
      await expect(page.getByTestId("member-edit-unknown")).toBeVisible();
      await capture(page,"EditUnknown");
      for(const f of await dialog(page).locator("input,textarea").all()) await expect(f).toHaveAttribute("readonly","");
      await page.getByTestId("member-edit-refresh").click();
      await expect(page.getByTestId("member-edit-unknown")).toBeVisible();
      await expect(page.getByTestId("member-edit-save")).toHaveCount(0); expect(writes).toBe(1);
      await page.keyboard.press("Escape"); await expect(dialog(page)).toBeVisible();
    } finally { await context.close(); }
  });
}
test("REAL_PG reader has no edit and erased clears private draft",async({browser})=>{
  const reader=await signed(browser,"en",1440,"forms-reader");
  await expect(reader.page.getByTestId("member-edit-open")).toHaveCount(0); await reader.context.close();
  const {page,context}=await signed(browser);
  try{
    await open(page); await dialog(page).locator('[name="notes"]').fill("Synthetic unsaved private draft");
    await ctl("edit-erase",member); await page.getByTestId("member-edit-save").click();
    await expect(dialog(page)).not.toBeVisible();
    await expect(page.getByText("[erased]",{exact:true}).first()).toBeVisible();
    await expect(page.getByTestId("member-edit-open")).toBeDisabled();
    await expect(page.locator('input,textarea')).toHaveCount(0);
    expect(await page.content()).not.toContain("Synthetic unsaved private draft");
  }finally{await context.close();}
});
