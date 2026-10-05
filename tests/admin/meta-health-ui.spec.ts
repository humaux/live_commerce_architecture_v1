// Purpose: MCH11 real clicks, three locales/390px, scoped reads, recheck/reconnect and explicit backend identity contract.
// Depends on: signed MOCK IdP, real BFF/Go/PG, fixture-only control and Playwright; no real Meta traffic.
// Used by: --browser-meta-health-ui; every new control has a persisted click ledger entry.
import { test, expect, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
const origin=process.env.LC_BROWSER_PUBLIC_ORIGIN!,store=process.env.LC_HEALTH_STORE!,other=process.env.LC_HEALTH_OTHER_STORE!,pageID=process.env.LC_HEALTH_PAGE!;
const evidence=process.env.LC_BROWSER_EVIDENCE!;
const ledger:{page:string;control:string;action:string;expected:string;actual:string;pass:boolean}[]=[];
async function control(value:string){
 // FIXTURE preparation/readback only; never substitutes for a merchant click.
 const r=await fetch(`${process.env.LC_HEALTH_CONTROL}/${value}`,{method:"POST",headers:{"X-Gate-Key":process.env.LC_HEALTH_KEY!}});expect(r.status).toBe(200);return r;
}
async function scenario(value:string){await control(`scenario/${value}`);}
async function login(page:Page,base=origin){
 await page.goto(`${base}/en/`);await page.getByRole("button",{name:"Sign in with identity service"}).click();
 await expect(page.getByRole("combobox",{name:"Switch store",exact:true})).toBeVisible();
}
async function click(page:Page,name:string,act:()=>Promise<unknown>,verify:()=>Promise<unknown>,expected:string){
 const item={page:new URL(page.url()).pathname,control:name,action:"real click/select/navigation",expected,actual:"",pass:false};ledger.push(item);
 try{await act();await verify();item.pass=true;item.actual=expected;}catch(e){item.actual=String(e).split("\n")[0];throw e;}
}
async function fits(page:Page){
 // READ/MEASURE only. No DOM/state mutation.
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
}
async function shot(page:Page,name:string,selector:string){await page.locator(selector).screenshot({path:path.join(evidence,name+".png")});}
const locales=["zh-TW","zh-CN","en"] as const;
const review={"zh-TW":"僅測試帳號","zh-CN":"仅测试账号",en:"Test accounts only"};
const recheck={"zh-TW":"重新檢查","zh-CN":"重新检查",en:"Check again"};
test.use({actionTimeout:15000,navigationTimeout:30000});
test.afterAll(async()=>{
 // A failed test restarts the single worker; retain preceding workers' real-click records.
 const previous = await readFile(path.join(evidence,"click-ledger.json"),"utf8").then(text=>JSON.parse(text) as typeof ledger).catch((error:NodeJS.ErrnoException)=>{if(error.code==="ENOENT")return [];throw error;});
 const complete=[...previous,...ledger];
 await writeFile(path.join(evidence,"click-ledger.json"),JSON.stringify(complete,null,2));
 await writeFile(path.join(evidence,"click-ledger.md"),"# MCH11 click ledger\n\n| Page | Control | Expected | Actual | Pass |\n|---|---|---|---|---|\n"+complete.map(x=>`| ${x.page} | ${x.control} | ${x.expected} | ${x.actual.replaceAll("|","/")} | ${x.pass} |`).join("\n"));
});
for(const locale of locales){
 test(`${locale} 390: blocking connection advice opens its settings card`,async({page})=>{
  await page.setViewportSize({width:390,height:844});await scenario("blocking");await login(page);
  for(const route of ["orders","settings","studio/claims"]){
   await page.goto(`${origin}/${locale}/${route}?store=${store}${route === "studio/claims" ? `&scene=${process.env.LC_HEALTH_SCENE}` : ""}`);
   await expect(page.getByTestId("meta-health-banner")).toHaveAttribute("data-severity","blocking");await fits(page);
  }
  await shot(page,`blocking-${locale}-390`,'[data-testid="meta-health-banner"]');
  await click(page,"banner-reconnect",()=>page.getByTestId("meta-health-settings").click(),async()=>{
   await expect(page.getByTestId("metaconnect-card")).toBeVisible();
   await expect(page).toHaveURL(new RegExp(`/${locale}/settings\\?store=${store}#facebook-instagram$`));
  },"same-store Settings connection card reached");
  await page.reload();await expect(page.getByTestId("meta-health-banner")).toBeVisible();
 });
 test(`${locale} 390: named FB/IG capabilities keep review-required restricted`,async({page})=>{
  await page.setViewportSize({width:390,height:844});await scenario("review");await login(page);await page.goto(`${origin}/${locale}/settings?store=${store}`);
  await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);
  await expect(page.getByRole("button",{name:recheck[locale],exact:true})).toBeEnabled();
  await click(page,`recheck-${locale}`,()=>page.getByRole("button",{name:recheck[locale],exact:true}).click(),()=>expect(page.getByTestId("meta-health-recheck-result")).toBeVisible(),"localized recheck acknowledges scheduling");
  // Contract assertion: a backend omission must fail here, never be guessed from array positions.
  const rows=page.locator('[data-testid^="meta-health-capability-"]');await expect(rows).toHaveCount(8);
  for(const provider of ["facebook","instagram"])for(const capability of ["read_comment","private_reply","dm_session","reply_public"]){
   await expect(page.getByTestId(`meta-health-capability-${provider}-${capability}`)).toContainText(review[locale]);
  }
  await fits(page);await shot(page,`capabilities-${locale}-390`,'.meta-health-details');
  await page.reload();await expect(rows).toHaveCount(8);
 });
}
test("desktop: warning is neutral, none and failed reads hide advice without blocking the workspace",async({page})=>{
 await scenario("warning");await login(page);await page.goto(`${origin}/en/settings?store=${store}`);
 await expect(page.getByTestId("meta-health-banner")).toHaveAttribute("data-severity","warning");await shot(page,"warning-en-desktop",'[data-testid="meta-health-banner"]');
 await scenario("none");await page.reload();await expect(page.getByTestId("metaconnect-card")).toBeVisible();await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);
 // FAULT injection: B1 read failure only, not a merchant write.
 await page.route("**/meta/health",route=>route.fulfill({status:503,json:{code:"retry_later"}}));
 await scenario("blocking");await page.reload();await expect(page.getByTestId("meta-health-unavailable")).toBeVisible();await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);
 await expect(page.getByTestId("metaconnect-reconnect")).toBeEnabled();
});
test("explicit recheck schedules, then reports the real 429 without another schedule",async({page})=>{
 await scenario("none");await login(page);await page.goto(`${origin}/en/settings?store=${store}`);
 const button=page.getByRole("button",{name:recheck.en,exact:true});await expect(button).toBeEnabled();
 const before=await(await control("facts")).json();
 await click(page,"recheck",()=>button.click(),()=>expect(page.getByTestId("meta-health-recheck-result")).toContainText("scheduled"),"B2 scheduling receipt displayed");
 const after=await(await control("facts")).json();expect(after.next_due_at).not.toBe(before.next_due_at);
 await control("probe"); // FIXTURE: execute the already-requested worker check without waiting for its periodic timer.
 const checked=await(await control("facts")).json();
 await click(page,"recheck-too-soon",()=>button.click(),()=>expect(page.getByTestId("meta-health-recheck-result")).toContainText("one minute"),"real 429 is actionable and no success is invented");
 expect((await(await control("facts")).json()).next_due_at).toBe(checked.next_due_at);
 await page.reload();await expect(button).toBeVisible();
});
test("viewer sees owner advice and capability state but no managed actions",async({browser})=>{
 await scenario("blocking");const context=await browser.newContext({viewport:{width:390,height:844}});const page=await context.newPage();
 const base=process.env.LC_HEALTH_VIEWER!;await login(page,base);await page.goto(`${base}/en/orders?store=${store}`);
 await expect(page.getByTestId("meta-health-owner")).toContainText("store owner");await expect(page.getByTestId("meta-health-settings")).toHaveCount(0);
 const before=await(await control("facts")).json();await page.goto(`${base}/en/settings?store=${store}`);await expect(page.getByTestId(`meta-health-page-${pageID}`)).toBeVisible();
 await expect(page.getByTestId("meta-health-recheck")).toHaveCount(0);await expect(page.getByTestId("metaconnect-reconnect")).toBeDisabled();
 expect((await(await control("facts")).json()).next_due_at).toBe(before.next_due_at);await context.close();
});
test("store switch discards a delayed old-store response",async({page})=>{
 await scenario("blocking");await login(page);
 let release!:()=>void,arrived!:()=>void,finished!:()=>void;
 const hold=new Promise<void>(r=>{release=r;}),seen=new Promise<void>(r=>{arrived=r;}),done=new Promise<void>(r=>{finished=r;});let held=false;
 // FAULT injection: delay one real B1 response. No store or product state is mutated here.
 await page.route(`**/stores/${store}/meta/health`,async route=>{
  if(held){await route.continue();return;}held=true;const response=await route.fetch();arrived();await hold;
  try{await route.fulfill({response});}catch{/* The store switch can abort the old request. */}finally{finished();}
 });
 await page.goto(`${origin}/en/orders?store=${store}`);await seen;
 await click(page,"store-switch",()=>page.getByRole("combobox",{name:"Switch store",exact:true}).selectOption(other),async()=>{await expect(page).toHaveURL(new RegExp(`store=${other}`));await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);},"new store has no old-store advice");
 release();await done;await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);await page.reload();await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);
});
test("reconnect uses the existing OAuth picker and restores the same Page",async({page})=>{
 await scenario("blocking");await login(page);await page.goto(`${origin}/en/orders?store=${store}`);
 // MOCK provider dialog: only the external Facebook redirect is answered; BFF/Go and user clicks remain real.
 await page.route("https://www.facebook.com/**",async route=>{
  const state=new URL(route.request().url()).searchParams.get("state")!;const {code}=await(await control("oauth/code")).json();
  await route.fulfill({status:302,headers:{Location:`${origin}/api/meta/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state)}`}});
 });
 await page.getByTestId("meta-health-settings").click();
 await click(page,"existing-reconnect",()=>page.getByTestId("metaconnect-reconnect").click(),()=>expect(page.getByTestId("metaconnect-pick-submit")).toBeVisible(),"real OAuth callback opens the existing Page picker");
 await click(page,"confirm-same-page",()=>page.getByTestId("metaconnect-pick-submit").click(),()=>expect(page.getByTestId(`metaconnect-row-${pageID}`)).toBeVisible(),"same Page is connected again");
 await control("probe");await page.reload();await expect(page.getByTestId("meta-health-banner")).toHaveCount(0);await expect(page.getByTestId("metaconnect-token")).toHaveAttribute("data-state","active");
});
