// Purpose: real buyer checkout/order/history gate, including stable locale targets and complete context privacy audits.
// Depends on: production Next/Go/PG, Playwright, shared Taipei formatting, browser-engine and shop-helpers.
// Used by: TestBrowserBuyerOrderUI in --browser-order and the WebKit order scenario; only synthetic TLS/fixture faults.
import assert from "node:assert/strict";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import {spawn, execFileSync} from "node:child_process";
import {once} from "node:events";
import {readFile, writeFile, mkdtemp, rm, mkdir, copyFile} from "node:fs/promises";
import {createWriteStream} from "node:fs";
import {tmpdir} from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { reachCheckout, switchLocale } from "./shop-helpers.mjs";
import { isWebkitCancelledFetch, launch, ctxOpts } from "./browser-engine.mjs"; // LC_BROWSER_ENGINE=chromium|webkit; chromium behaviour is unchanged
import { displayTime } from "../../packages/format/src/index.ts";

const root=process.cwd(), evidence=process.env.LC_ORDER_EVIDENCE;
assert(evidence && /^http:\/\/127\.0\.0\.1:\d+$/.test(process.env.LC_ORDER_CONTROL));
const origin="https://buyer.example", checkoutPath="/en/checkout";
const children=new Set(), sockets=new Set(), logs=[], contexts=[], orders=[], observations=[], storageWrites=[], consoleText=[], requestURLs=[], cartRetries=[], cartRefreshes=[], clickLedger=[];
const pii={recipient_name:"Synthetic Gate Recipient",phone:"+886900000091",region:"Synthetic Region",city:"Synthetic City",postal_code:"99991",line1:"Synthetic Address Ninety One",line2:"Synthetic Unit Ninety Two"};
const secrets=[], pageErrors=[], closedContextStates=new Map();
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return{promise,resolve};};
const pause=ms=>new Promise(r=>setTimeout(r,ms));
const listen=async s=>{s.listen(0,"127.0.0.1");await once(s,"listening");return s.address().port;};
const pass=name=>{observations.push(name);console.log(`PASS ${name}`);};
const certDir=await mkdtemp(path.join(tmpdir(),"lc-order-edge-"));
const review=path.join(root,"output/playwright/review/buyer-order");
const historyReview=path.join(root,"output/playwright/review/buyer-history");
let browser,edge,proxy,hook,sessionResets=0;
const calls=[], bo01Warmups=new Map(), bo01Reads=[], bo01Probes=[];
async function control(resource,method="GET") {
  const response=await fetch(`${process.env.LC_ORDER_CONTROL}/${resource}`,{method,headers:{"X-Gate-Key":process.env.LC_ORDER_CONTROL_KEY}});
  assert.equal(response.status,200,`fixture control ${resource.split("/")[0]}`);return response.json();
}
function relay(port,req,body) {
  return new Promise((resolve,reject)=>{
    const headers={...req.headers};delete headers.connection;delete headers["transfer-encoding"];
    if(body.length)headers["content-length"]=String(body.length);else delete headers["content-length"];
    const call=http.request({hostname:"127.0.0.1",port,path:req.url,method:req.method,headers},res=>{
      const chunks=[];res.on("data",x=>chunks.push(x));res.on("error",reject);res.on("end",()=>resolve({status:res.statusCode,headers:res.headers,body:Buffer.concat(chunks)}));
    });call.setTimeout(15000,()=>call.destroy(new Error("fixture relay deadline")));call.on("error",reject);call.end(body);
  });
}
async function startNext() {
  const reserve=net.createServer(),port=await listen(reserve);await new Promise(r=>reserve.close(r));
  const log=createWriteStream(path.join(evidence,"next.log"),{flags:"wx",mode:0o600});logs.push(log);await once(log,"open");
  const env={...process.env,NODE_ENV:"production",NEXT_TELEMETRY_DISABLED:"1"};
  for(const name of Object.keys(env))if(name.startsWith("LC_ORDER_"))delete env[name];
  const child=spawn(process.execPath,[path.join(root,"apps/storefront/node_modules/next/dist/bin/next"),"start","--hostname","127.0.0.1","--port",String(port)],{cwd:path.join(root,"apps/storefront"),env,stdio:["ignore",log,log]});children.add(child);
  for(let i=0;i<100;i++){
    if(child.exitCode!==null)throw new Error("owned Next failed readiness");
    try{if((await relay(port,{url:"/api/buyer/session",method:"GET",headers:{host:"buyer.example"}},Buffer.alloc(0))).status===200)return port;}catch{}
    await pause(50);
  }throw new Error("owned Next readiness timeout");
}
function arm(suffix,fields={}) {
  return hook={path:`/api/buyer/${suffix}`,entered:deferred(),release:deferred(),result:deferred(),...fields};
}
async function newContext(mobile=false,timezoneId="America/Los_Angeles") {
  const c=await browser.newContext(ctxOpts({ignoreHTTPSErrors:true,viewport:mobile?{width:390,height:844}:{width:1440,height:900},timezoneId}));contexts.push(c);
  // A short-lived context still belongs to BO06: retain its audit before Playwright disposes its pages/cookies.
  const close=c.close.bind(c);
  c.close=async (...args)=>{
    if(closedContextStates.has(c))return close(...args);
    const states=await captureContextState(c);
    await close(...args);
    closedContextStates.set(c,states);
  };
  await c.exposeBinding("__gateStorageWrite",(_,value)=>storageWrites.push(value));
  await c.addInitScript(()=>{
    const native=Storage.prototype.setItem;
    const remove=Storage.prototype.removeItem;
    Storage.prototype.removeItem=function(key){
      if(window.__keepQuoteLocator && this===sessionStorage && String(key).startsWith("commerce-purchase-quote-v1:"))return;
      return remove.call(this,key);
    };
    Storage.prototype.setItem=function(key,value){
      void window.__gateStorageWrite({kind:this===localStorage?"local":"session",key:String(key),value:String(value)});
      if(window.__failOrderLocator && String(key).startsWith("commerce-purchase-order-v1:"))throw new DOMException("Synthetic gate storage failure","QuotaExceededError");
      return native.call(this,key,value);
    };
  });
  c.on("page",p=>{
    p.on("pageerror",e=>pageErrors.push({name:e.name,message:e.message}));
    p.on("console",m=>consoleText.push(m.text()));
    p.on("request",r=>requestURLs.push(r.url()));
  });return c;
}
const requestIs=(response,suffix,method)=>new URL(response.url()).pathname===`/api/buyer/${suffix}`&&response.request().method()===method;
const historyRead=(p)=>p.waitForResponse(r=>r.status()===200&&r.request().method()==="GET"&&new URL(r.url()).pathname==="/api/buyer/orders"&&new URL(r.url()).searchParams.get("limit")==="20");
const historyTimeZone={en:"Times are Taipei time (UTC+8).","zh-CN":"时间均为台北时间（UTC+8）。","zh-TW":"時間皆為台北時間（UTC+8）。"};
async function assertHistoryTimes(p,locale,response) {
  const body=await response.json();
  assert.equal(body.items.length,2,"actual UI-triggered history GET returns both orders");
  await expect(p.getByTestId("order-history").locator("p.order-note")).toContainText(historyTimeZone[locale]);
  for(const item of body.items) {
    const row=p.locator(".history-list li").filter({has:p.locator(`button[data-order-id="${item.order_id}"]`)});
    await expect(row).toHaveCount(1);
    await expect(row.locator("time[datetime]")).toHaveAttribute("datetime",item.created_at);
    await expect(row.locator("time[datetime]")).toHaveText(displayTime(locale,item.created_at));
  }
}
// Both calibration barriers fail locally; a lost read must not wait for Go's 180 s kill.
async function waitBo01Warmup(promise,phase,page) {
  let deadline;
  try {
    return await Promise.race([promise,new Promise((_,reject)=>{
      deadline=setTimeout(()=>reject(new Error(`BO01 warmup ${phase} exceeded 10000ms: GET /api/buyer/destination at ${page.url()}`)),10000);
    })]);
  } finally {clearTimeout(deadline);}
}
async function quotePage(c,clock=false,p,bo01=false) {
  if(!p){p=await c.newPage();if(clock)await p.clock.install();}
  if(bo01&&process.env.LC_BO01_EARLY_HEAD==="1") {
    const warmup={entered:deferred(),release:deferred(),served:deferred()};bo01Warmups.set(p,warmup);
    let waiting=true;
    await p.route("**/api/buyer/destination",async route=>{
      const request=route.request();
      if(waiting&&request.method()==="GET"&&new URL(request.headers().referer||origin).pathname==="/en/checkout") {
        waiting=false;warmup.entered.resolve();await warmup.release.promise;
      }
      await route.continue();
    });
  }
  await reachCheckout(p,origin,"en",process.env.LC_ORDER_PRODUCT); // product page -> Add to cart -> /en/checkout (also for a page that continued shopping: its cart is empty)
  await p.getByRole("button",{name:"Choose delivery",exact:true}).click();
  const pending=p.waitForResponse(r=>requestIs(r,"quotes","POST"));
  await p.getByRole("button",{name:"Get current total",exact:true}).click();
  const response=await pending;assert.equal(response.status(),200);
  const quote=await response.json();
  await expect(p.getByTestId("address-section")).toBeVisible();
  if(bo01Warmups.has(p))await waitBo01Warmup(bo01Warmups.get(p).entered.promise,"request interception",p);
  return {p,quote};
}
async function fill(p,values=pii) {for(const [key,value] of Object.entries(values))await p.locator(`input[name="${key}"]`).fill(value);}
async function confirm(p) {
  await p.getByTestId("confirm-address").click();
  await expect(p.getByTestId("create-order")).toBeEnabled();
  await expect(p.getByRole("button",{name:"View quotation",exact:true})).not.toHaveClass(/\bprimary\b/);
}
async function created(p,quote,ownerOrders=1) {
  await expect(p.getByTestId("order-section")).toBeVisible();
  await expect(p.getByTestId("order-state")).toHaveAttribute("data-state","DRAFT");
  await expect(p.getByTestId("order-state")).toHaveText(/Not paid/i);
  const id=(await p.getByTestId("order-id").innerText()).trim();assert.match(id,/^[a-f0-9-]{36}$/);
  const f=(await control("facts")).find(x=>x.id===id);assert(f);
  for(const key of ["orders","holds","jobs","receipts","reserve_lines"])assert.equal(f[key],ownerOrders,`per-buyer ${key}`);
  assert.equal(f.hold_state,"HELD");
  assert.equal(f.total,quote.amount.total_minor);assert.equal(f.currency,quote.currency);assert.equal(f.country,quote.country);
  // SF-12: only the amounts that moved the total are listed ("Delivery NT$0 / Tax NT$0 / Discount NT$0" is not); no row at all when none did.
  const shown=["shipping_minor","tax_minor","discount_minor"].filter(key=>quote.amount[key]!==0);
  if(shown.length===0)await expect(p.getByTestId("order-breakdown")).toHaveCount(0);
  else {
    const breakdown=p.getByTestId("order-breakdown").locator("div");
    await expect(breakdown).toHaveCount(shown.length);
    for(const [index,key] of shown.entries()) {
      await expect(breakdown.nth(index).locator("dd")).toHaveText(new Intl.NumberFormat("en",{style:"currency",currency:quote.currency,minimumFractionDigits:quote.amount[key]%100===0?0:2}).format(quote.amount[key]/100)); // storefront money: whole amounts carry no ".00" (lib/money.ts)
    }
  }
  await expect(p.getByTestId("create-order")).toHaveCount(0);
  // This legacy fixture does not enable buyer payment. New payment UI must not
  // manufacture an available method merely because an order was created.
  await expect(p.getByTestId("pay-order")).toHaveCount(0);
  if(!orders.includes(id))orders.push(id);return id;
}
async function stored(p,prefix="commerce-purchase-pending-v1:") {
  return p.evaluate(prefix=>{const key=Object.keys(localStorage).find(k=>k.startsWith(prefix));return key?JSON.parse(localStorage.getItem(key)):null;},prefix);
}
async function api(p,method,suffix,body,key) {
  return p.evaluate(async({method,suffix,body,key})=>{
    const session=await(await fetch("/api/buyer/session",{cache:"no-store"})).json();
    const response=await fetch(`/api/buyer/${suffix}`,{method,headers:{"Content-Type":"application/json","X-Buyer-Context":session.context,...(key?{"Idempotency-Key":key}:{})},...(body?{body:JSON.stringify(body)}:{})});
    return {status:response.status,body:await response.json()};
  },{method,suffix,body,key});
}
async function rememberCookie(c) {
  const cookie=(await c.cookies(origin))[0];assert(cookie?.httpOnly&&cookie.secure&&cookie.sameSite==="Lax");
  secrets.push(cookie.value);const payload=JSON.parse(Buffer.from(cookie.value.split(".")[0],"base64url").toString());if(payload.token)secrets.push(payload.token);
}
async function captureContextState(c) {
  await rememberCookie(c);
  const states=[];
  for(const p of c.pages()){
    assert.equal(await p.evaluate(()=>document.cookie),"");
    states.push(await p.evaluate(()=>JSON.stringify({local:{...localStorage},session:{...sessionStorage}})));
  }
  return states;
}
async function stableLocaleTarget(p,mobile=false) {
  // Causal layout gate: release the real destination-head read only after the footer link is positioned.
  // A buyer can press a language link while this read completes; removing the loading paragraph must not move it.
  const repeats=Number(process.env.LC_BO01_REPEAT||1);assert(Number.isInteger(repeats)&&repeats>=1&&repeats<=10);
  for(let iteration=1;iteration<=repeats;iteration++) {
  const cookie=(await p.context().cookies(origin))[0];assert(cookie?.httpOnly&&cookie.secure);
  // The previous English document can still be issuing its mount/focus read.
  // Match the real new document and observed owner cookie, not whichever GET arrives first.
  const headLoading=arm("destination",{method:"GET",after:true,sourcePath:"/zh-TW/checkout",cookie:`${cookie.name}=${cookie.value}`});
  const warmup=bo01Warmups.get(p);
  if(warmup){warmup.cookie=headLoading.cookie;warmup.release.resolve();await waitBo01Warmup(warmup.served.promise,"upstream completion",p);bo01Warmups.delete(p);}
  // Enter the probed document from a settled old form; its loading is measured below.
  await expect(p.locator('input[name="recipient_name"]')).toBeEnabled();
  if(mobile&&process.env.LC_BROWSER_ENGINE==="webkit") {
    await p.locator('footer nav a[hreflang="zh-TW"]').tap();
    await expect(p).toHaveURL(`${origin}/zh-TW/checkout`);
    await expect(p.locator('html[lang="zh-TW"]')).toBeVisible();
  } else await switchLocale(p,"zh-TW");
  let headDeadline;
  try {
    const status=await Promise.race([headLoading.result.promise, new Promise((_, reject) => {
      headDeadline = setTimeout(() => reject(new Error(`BO01 ${mobile ? "mobile" : "desktop"} destination-head wait exceeded 10000ms: GET /api/buyer/destination at ${p.url()}`)), 10000);
    })]);
    assert.equal(status,200);assert.equal(headLoading.receivedSourcePath,"/zh-TW/checkout");assert.equal(headLoading.ownerMatched,true);
  } finally { clearTimeout(headDeadline); }
  await expect(p.getByTestId("address-section")).toBeVisible();
  await expect(p.getByTestId("cart-line")).toHaveCount(1);
  await expect(p.getByRole("status").filter({hasText:"正在載入收件資訊…"})).toBeVisible();
  const languageLink=p.locator('footer nav a[hreflang="en"]');await languageLink.scrollIntoViewIfNeeded();
  const languagePosition=()=>languageLink.evaluate(link=>({top:link.getBoundingClientRect().top+scrollY,height:link.getBoundingClientRect().height,documentHeight:document.documentElement.scrollHeight}));
  const beforeHead=await languagePosition();headLoading.release.resolve();
  await expect(p.locator('input[name="recipient_name"]')).toBeEnabled();
  const afterHead=await languagePosition();
  assert.equal(afterHead.top,beforeHead.top,"loading completion must not move the footer language target");
  if(mobile&&process.env.LC_BROWSER_ENGINE==="webkit")await languageLink.tap();else await languageLink.click();await expect(p).toHaveURL(`${origin}/en/checkout`);
  await expect(p.getByTestId("address-section")).toBeVisible();
  bo01Probes.push({device:mobile?"mobile":"desktop",iteration,sourcePath:headLoading.receivedSourcePath,ownerMatched:headLoading.ownerMatched,before:beforeHead,after:afterHead});
  await writeFile(path.join(evidence,"bo01-probes.json"),JSON.stringify(bo01Probes,null,2));
  }
  pass(`BO01 ${mobile?"mobile":"desktop"} delivery-head completion keeps the language target stable`);
}
async function capture(p,name,fullPage=true,directory=review) {
  await mkdir(directory,{recursive:true});
  await p.screenshot({path:path.join(evidence,name),fullPage});await copyFile(path.join(evidence,name),path.join(directory,name));
}
const cartCopy={en:{retry:"Retry",increase:"Increase quantity",view:"View cart"},"zh-CN":{retry:"重试",increase:"增加数量",view:"查看购物车"},"zh-TW":{retry:"重試",increase:"增加數量",view:"查看購物車"}};
async function cartClick(p,caseID,control,locator,expected,verify) {
  const row={case_id:caseID,page:p.url(),control,action:"click",expected,actual:"NOT_RUN",status:"NOT_RUN"};clickLedger.push(row);
  try {await locator.click();await verify();row.actual=expected;row.status="PASS";}
  catch(error){row.actual=String(error.message).slice(0,1000);row.status="FAIL";throw error;}
  finally {await writeFile(path.join(evidence,"cart-click-ledger.json"),JSON.stringify(clickLedger,null,2),{mode:0o600});}
}
// BC02 measures DOM/focus and native promise settlement. It never changes app state, locks, requests or replies.
async function measureCart(p,surface) {
  await p.evaluate(surface=>{
    const state=window.__cartRace={focus:[],samples:[],empty:0,line_loss:0,seen_line:false,sequence:0,fetches:[],locks:[]};
    // Return the exact native Promise and pass every argument/callback unchanged. Passive observers
    // complete normally, leaving the original rejected Promise/error for the production caller.
    const nativeFetch=window.fetch;
    window.fetch=function(...args){
      const result=Reflect.apply(nativeFetch,this,args);
      if(args[0]==="/api/buyer/session"&&(args[1]?.method||"GET")==="GET")result.then(
        response=>state.fetches.push({status:response.status,settlement:"resolved",sequence:++state.sequence}),
        ()=>state.fetches.push({status:0,settlement:"rejected",sequence:++state.sequence})
      );
      return result;
    };
    const manager=navigator.locks,nativeRequest=manager.request,lockName="commerce-buyer-session-v1";
    manager.request=function(...args){
      const result=Reflect.apply(nativeRequest,this,args);
      if(args[0]===lockName)result.then(
        ()=>state.locks.push({name:lockName,settlement:"resolved",sequence:++state.sequence}),
        error=>state.locks.push({name:lockName,settlement:"rejected",code:error?.code,status:error?.status,sequence:++state.sequence})
      );
      return result;
    };
    // A real queued native grant/release proves the failed session lock is free. No app callback
    // is wrapped or simulated; this no-op uses the public API and the original native request.
    window.__cartRaceBarrier=async()=>{
      let acquired;
      await Reflect.apply(nativeRequest,manager,[lockName,{mode:"exclusive"},()=>{acquired=++state.sequence;}]);
      return {name:lockName,acquired_sequence:acquired,settled_sequence:++state.sequence};
    };
    const sample=()=>{
      const host=document.querySelector(`[data-testid="${surface}"]`);
      if(!host||!host.getClientRects().length)return;
      const lines=host.querySelectorAll('[data-testid="cart-line"]');
      const title=host.querySelector('.sf-empty__title')?.textContent;
      const empty=title==="Your cart is empty";
      if(empty)state.empty++;
      if(state.seen_line&&!lines.length)state.line_loss++;
      if(lines.length)state.seen_line=true;
      // CartLines renders this specific visible status while actual cartDetails hydrates.
      // A blank cart grid is never interchangeable with that explicit loading state.
      const detailsStatus=host.querySelector('.sf-cartpage__grid > p.sf-muted[role="status"]');
      const box=detailsStatus?.getBoundingClientRect();
      const detailLoading=detailsStatus&&box.width>0&&box.height>0&&getComputedStyle(detailsStatus).visibility==="visible"&&detailsStatus.textContent==="Loading…"?detailsStatus.textContent:null;
      const row={title:title||null,lines:lines.length,qty:host.querySelector('[data-testid="cart-line-qty"]')?.textContent||null,detail_loading:detailLoading};
      if(JSON.stringify(row)!==JSON.stringify(state.samples.at(-1)))state.samples.push(row);
    };
    window.addEventListener("focus",e=>state.focus.push({type:"focus",trusted:e.isTrusted}));
    window.addEventListener("blur",e=>state.focus.push({type:"blur",trusted:e.isTrusted}));
    new MutationObserver(sample).observe(document.documentElement,{childList:true,subtree:true,characterData:true});
    sample();
  },surface);
}
async function focusFixture(p,surface) {
  // Controlled same-origin HTML is only a physical focus boundary. It contains no application/cart state.
  await p.evaluate(surface=>{
    const frame=document.createElement("iframe");frame.dataset.testid="native-focus-fixture";
    frame.title="Browser gate native focus boundary";frame.src="/__gate/native-focus";
    frame.style.cssText="position:fixed;right:8px;bottom:8px;width:170px;height:60px;z-index:9999;background:white";
    document.querySelector(`[data-testid="${surface}"]`).append(frame);
  },surface);
}
async function nativeFocus(p,caseID,label,parent) {
  const before=await p.evaluate(()=>window.__cartRace.focus.length);
  await cartClick(p,caseID,`${label} frame button`,p.frameLocator('[data-testid="native-focus-fixture"]').getByRole("button",{name:"Native focus boundary",exact:true}),"actual iframe click emits a trusted parent-window blur",async()=>{
    await expect.poll(()=>p.evaluate(()=>window.__cartRace.focus.at(-1))).toEqual({type:"blur",trusted:true});
    await expect.poll(()=>p.evaluate(()=>document.activeElement?.tagName)).toBe("IFRAME");
  });
  await cartClick(p,caseID,`${label} cart text`,parent,"actual cart text click emits a new trusted parent-window focus",async()=>{
    await expect.poll(()=>p.evaluate(()=>window.__cartRace.focus.at(-1))).toEqual({type:"focus",trusted:true});
    assert((await p.evaluate(()=>window.__cartRace.focus.length))>before,"a NEW trusted native focus event starts the refresh");
  });
}
async function cartRefreshRace() {
  const caseID="BC02 native focus failure preserves held real cart on retained drawer and fresh mount";
  const c=await newContext(),p=await c.newPage();
  await p.goto(`${origin}/en/products/${process.env.LC_ORDER_PRODUCT}`);
  await expect(p.getByTestId("add-to-cart")).toBeEnabled();
  const created=p.waitForResponse(r=>requestIs(r,"cart","PUT")&&r.status()===200);
  await cartClick(p,caseID,"add-to-cart",p.getByTestId("add-to-cart"),"real UI creates one persisted cart line",async()=>expect(p.getByTestId("cart-drawer").getByTestId("cart-line-qty")).toHaveText("1"));
  const actual=await(await created).json(),write=calls.filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT").at(-1);
  assert.equal(actual.items.length,1);
  const factPath=`cart-facts/${actual.id}/${write.key}`,before=await control(factPath);
  assert.equal(before.version,1);assert.equal(before.receipts,1);assert.equal(before.writes,1);
  const phases=[];
  for(const surface of ["drawer","mount"]){
    let held;
    const phase={surface,held_status:0,session_status:0,measurement:null};phases.push(phase);
    try {
      if(surface==="drawer") {
        await measureCart(p,"cart-drawer");
        await focusFixture(p,"cart-drawer");
        held=arm("cart",{method:"GET",after:true});
        await nativeFocus(p,caseID,"drawer held",p.getByTestId("cart-drawer").getByTestId("cart-line-qty"));
      } else {
        // Full navigation mounts a new Provider with ready=false and no in-memory cart.
        // Unlike a loaded same-context cart, it exposes premature ready+empty while the persisted GET is held.
        held=arm("cart",{method:"GET",after:true});
        await p.goto(`${origin}/en/cart`);
        await measureCart(p,"cart-page");
        await expect(p.getByTestId("cart-empty")).toContainText("Loading…");
        await focusFixture(p,"cart-page");
      }
      phase.held_status=await held.result.promise;
      assert.equal(phase.held_status,200,"held cart response is the actual production Next/Go/PG read");
      phase.held_cart=JSON.parse(held.out.body.toString());
      assert.deepEqual(phase.held_cart,actual,"cart payload is unmodified and nonempty");
      const failedStart=calls.length;
      const failed=arm("session",{method:"GET",session503:true});
      const reply=p.waitForResponse(r=>requestIs(r,"session","GET")&&r.status()===503);
      await nativeFocus(p,caseID,`${surface} failed`,surface==="drawer"?p.getByTestId("cart-drawer").getByTestId("cart-line-qty"):p.getByTestId("cart-empty").locator(".sf-empty__title"));await reply;
      phase.session_status=await failed.result.promise;
      phase.session_failures=calls.slice(failedStart).filter(x=>x.path==="/api/buyer/session"&&x.method==="GET"&&x.status===503).length;
      // Headers/edge completion are insufficient: wait for the actual native fetch AND native
      // session-lock rejection. The native lock request settles only after its callback releases.
      await expect.poll(()=>p.evaluate(()=>({
        fetches:window.__cartRace.fetches.filter(x=>x.status===503&&x.settlement==="resolved").length,
        locks:window.__cartRace.locks.filter(x=>x.status===503&&x.code==="request_failed"&&x.settlement==="rejected").length
      })),{message:"actual failed session fetch and native lock rejection must settle before releasing cart"}).toEqual({fetches:1,locks:1});
      phase.settlement=await p.evaluate(async()=>{
        const fetches=window.__cartRace.fetches.filter(x=>x.status===503&&x.settlement==="resolved");
        const locks=window.__cartRace.locks.filter(x=>x.status===503&&x.code==="request_failed"&&x.settlement==="rejected");
        const barrier=await window.__cartRaceBarrier();
        return {fetch:fetches[0],lock:locks[0],fetch_count:fetches.length,lock_count:locks.length,barrier};
      });
      assert(phase.settlement.fetch.sequence<phase.settlement.lock.sequence&&phase.settlement.lock.sequence<phase.settlement.barrier.acquired_sequence&&phase.settlement.barrier.acquired_sequence<phase.settlement.barrier.settled_sequence,"actual 503 fetch, native session lock rejection, and queued native lock grant/release finish in order");
      // Request/lock completion is already proven; frames now serve only as a DOM render checkpoint.
      await p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
      phase.measurement=await p.evaluate(()=>window.__cartRace);
      await writeFile(path.join(evidence,"cart-refresh-race-facts.json"),JSON.stringify({case_id:caseID,cart_id:actual.id,key:write.key,before,phases},null,2),{mode:0o600});
      assert.equal(phase.session_failures,1,"exactly one session GET failure follows the native focus click");
      assert.equal(phase.session_status,503,"one later native-focus session request fails");
      assert.equal(phase.measurement.empty,0,`BC02 ${surface}: visible cart must NEVER render empty while valid cart is held`);
      if(surface==="drawer")assert.equal(phase.measurement.line_loss,0,"rendered line stays continuous throughout the failed refresh");
      else await expect(p.getByTestId("cart-empty")).toContainText("Loading…");
      held.release.resolve();
      const host=p.getByTestId(surface==="drawer"?"cart-drawer":"cart-page");
      await expect(host.getByTestId("cart-line-qty")).toHaveText("1");
      phase.measurement=await p.evaluate(()=>window.__cartRace);
      assert.equal(phase.measurement.empty,0,"released actual response produces the line without an empty flash");
      if(surface==="drawer")assert.equal(phase.measurement.line_loss,0);
    } finally {
      held?.release.resolve();hook=null;
      await p.getByTestId("native-focus-fixture").evaluate(frame=>frame.remove()).catch(()=>{});
      await writeFile(path.join(evidence,"cart-refresh-race-facts.json"),JSON.stringify({case_id:caseID,cart_id:actual.id,key:write.key,before,phases},null,2),{mode:0o600});
    }
  }
  await cartClick(p,caseID,"header-cart",p.getByTestId("header-cart"),"drawer still shows the persisted line after both real read races",async()=>expect(p.getByTestId("cart-drawer").getByTestId("cart-line-qty")).toHaveText("1"));
  const writes=calls.filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT").length;
  await p.reload();await expect(p.getByTestId("cart-page").getByTestId("cart-line-qty")).toHaveText("1");
  await cartClick(p,caseID,"header-cart after reload",p.getByTestId("header-cart"),"reloaded drawer keeps exactly one persisted line",async()=>expect(p.getByTestId("cart-drawer").getByTestId("cart-line-qty")).toHaveText("1"));
  assert.equal(calls.filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT").length,writes,"races and reload issue no writes");
  const after=await control(factPath);assert.deepEqual(after,before);
  cartRefreshes.push({case_id:caseID,cart_id:actual.id,key:write.key,sku_id:actual.items[0].sku_id,before,after,phases});
  await writeFile(path.join(evidence,"cart-refresh-race-facts.json"),JSON.stringify(cartRefreshes,null,2),{mode:0o600});
  await c.close();pass(caseID);
}
async function cartRetry(p,locale,surface,initiate) {
  const caseID=`BC01 ${locale} ${surface} visible cart Retry preserves key/body and one committed write after reload`;
  const host=()=>p.getByTestId(surface==="drawer"?"cart-drawer":"cart-page");
  const start=calls.length,lost=arm("cart",{method:"PUT",drop:true,repeat:true});
  // Keep dropping transparent transport retries too: the first visible recovery must be the buyer's Retry click.
  await initiate(caseID);
  assert.equal(await lost.result.promise,200,"lost cart reply follows a real committed Go/PG write");
  if(surface==="drawer")await cartClick(p,caseID,"header-cart",p.getByTestId("header-cart"),"uncertain cart notice is visible",async()=>expect(host().getByTestId("cart-problem")).toBeVisible());
  const notice=host().getByTestId("cart-problem"),retry=notice.getByRole("button",{name:cartCopy[locale].retry,exact:true});
  await expect(notice).toBeVisible();await expect(retry).toBeVisible();await expect(retry).toBeEnabled();
  const pending=await stored(p);assert.equal(pending.kind,"cart");
  const committed=JSON.parse(lost.out.body.toString()),quantity=surface==="drawer"?1:2;
  assert.equal(committed.version,quantity);assert.equal(committed.items.length,1);assert.equal(committed.items[0].quantity,quantity);
  assert.deepEqual(pending.body,{expected_version:quantity-1,items:[{sku_id:committed.items[0].sku_id,quantity}]});
  const factPath=`cart-facts/${committed.id}/${pending.key}`,before=await control(factPath);
  assert.equal(before.receipts,1);assert.equal(before.receipt_version,quantity);assert.equal(before.version,quantity);assert.equal(before.writes,quantity);
  assert.deepEqual(before.items,pending.body.items);assert.match(before.request_hash,/^[a-f0-9]{64}$/);
  const firstCalls=calls.slice(start).filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT");
  assert(firstCalls.length>=1);for(const call of firstCalls){assert.equal(call.key,pending.key);assert.deepEqual(JSON.parse(call.body),pending.body);assert.equal(call.status,200);assert.equal(call.dropped,true);}
  const explicitStart=calls.length;hook=null;
  await cartClick(p,caseID,"cart-problem > Retry",retry,"uncertainty clears and committed quantity appears",async()=>{
    await expect(notice).toHaveCount(0);await expect(host().getByTestId("cart-line-qty")).toHaveText(String(quantity));
    await expect.poll(()=>stored(p)).toBeNull();
  });
  const explicit=calls.slice(explicitStart).filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT");
  assert.equal(explicit.length,1,"one real cart PUT is triggered by the visible Retry button");
  assert.equal(explicit[0].status,200);assert.equal(explicit[0].key,pending.key);assert.equal(explicit[0].body,firstCalls[0].body);assert.equal(explicit[0].dropped,false);
  const after=await control(factPath);assert.deepEqual(after,before,"Retry replays one receipt without a second cart.updated event/version");
  const writesBeforeReload=calls.filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT").length;
  await p.reload();
  if(surface==="drawer")await cartClick(p,caseID,"header-cart after reload",p.getByTestId("header-cart"),"persisted cart drawer opens with the committed quantity",async()=>expect(host().getByTestId("cart-line-qty")).toHaveText(String(quantity)));
  await expect(host().getByTestId("cart-line-qty")).toHaveText(String(quantity));await expect(host().getByTestId("cart-problem")).toHaveCount(0);
  assert.equal(calls.filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT").length,writesBeforeReload,"reload must only read the retained cart");
  const reloaded=await control(factPath);assert.deepEqual(reloaded,before);
  cartRetries.push({case_id:caseID,locale,surface,key:pending.key,request_body:firstCalls[0].body,cart_id:committed.id,sku_id:committed.items[0].sku_id,quantity,before,after,reloaded});
  await writeFile(path.join(evidence,"cart-retry-facts.json"),JSON.stringify(cartRetries,null,2),{mode:0o600});
  pass(caseID);
}
try {
  execFileSync("openssl",["req","-x509","-newkey","rsa:2048","-nodes","-keyout",path.join(certDir,"key.pem"),"-out",path.join(certDir,"cert.pem"),"-days","1","-subj","/CN=buyer.example"],{stdio:"ignore"});
  const port=await startNext();
  edge=https.createServer({key:await readFile(path.join(certDir,"key.pem")),cert:await readFile(path.join(certDir,"cert.pem"))},async(req,res)=>{
    try {
      if(req.url==="/__gate/native-focus"){
        res.writeHead(200,{"content-type":"text/html","cache-control":"no-store"});
        res.end('<!doctype html><html lang="en"><button type="button">Native focus boundary</button></html>');return;
      }
      const chunks=[];for await(const x of req)chunks.push(x);const body=Buffer.concat(chunks);
      if(req.url.startsWith("/api/buyer/session/")&&req.url.includes("prepare")&&body.toString().includes('"reset"'))sessionResets++;
      const call={path:req.url,method:req.method,key:req.headers["idempotency-key"],body:body.length?body.toString():null,sourcePath:new URL(req.headers.referer||origin).pathname};
      if(req.url.startsWith("/api/buyer/"))calls.push(call);
      const cookiePairs=(req.headers.cookie||"").split(/;\s*/);
      const active=hook&&hook.path===req.url&&(!hook.method||hook.method===req.method)&&(!hook.sourcePath||hook.sourcePath===call.sourcePath)&&(!hook.cookie||cookiePairs.includes(hook.cookie))?hook:null;
      if(active){active.receivedSourcePath=call.sourcePath;active.ownerMatched=!!active.cookie&&cookiePairs.includes(active.cookie);if(!active.repeat)hook=null;active.entered.resolve();if(active.before)await active.release.promise;}
      // Documented BC02 fixture outage: fail one session GET at the synthetic edge.
      // Cart GET/PUT always relay the actual production response and are never replaced.
      const out=active?.session503?{status:503,headers:{"content-type":"application/json","cache-control":"no-store"},body:Buffer.from('{"error":"synthetic_session_unavailable"}')} : await relay(port,req,body);call.status=out.status;
      if(req.url==="/api/buyer/destination"&&req.method==="GET") {
        bo01Reads.push({sourcePath:call.sourcePath,status:out.status,held:!!active?.after});
        await writeFile(path.join(evidence,"bo01-head-reads.json"),JSON.stringify(bo01Reads,null,2));
        for(const [page,warmup] of bo01Warmups)if(call.sourcePath==="/en/checkout"&&page.url()===origin+"/en/checkout"&&cookiePairs.includes(warmup.cookie))warmup.served.resolve();
      }
      if(active){active.out=out;active.result.resolve(out.status);}
      call.dropped=!!active?.drop;
      if(active?.drop){res.destroy();return;}
      if(active?.after)await active.release.promise;
      const headers={...out.headers};delete headers.connection;delete headers["transfer-encoding"];
      res.writeHead(out.status,headers);res.end(out.body);
    }catch{if(!res.headersSent)res.writeHead(502);res.end();}
  });
  const edgePort=await listen(edge);proxy=http.createServer((_,r)=>{r.writeHead(403);r.end();});
  proxy.on("connect",(req,socket,head)=>{
    if(req.url!=="buyer.example:443"){socket.destroy();return;}
    const upstream=net.connect(edgePort,"127.0.0.1",()=>{socket.write("HTTP/1.1 200 Connection Established\r\n\r\n");if(head.length)upstream.write(head);socket.pipe(upstream).pipe(socket);});
    for(const s of [socket,upstream]){sockets.add(s);s.on("close",()=>sockets.delete(s));s.on("error",()=>{socket.destroy();upstream.destroy();});}
  });
  browser=await launch({headless:true,proxy:{server:`http://127.0.0.1:${await listen(proxy)}`}});

  // BC01: first write in a fresh session (product drawer), then an edit on the cart page.
  // All business requests run through production Next -> Go -> PG; only committed edge replies are lost.
  for(const locale of ["en","zh-CN","zh-TW"]){
    const context=await newContext(),p=await context.newPage();
    await p.goto(`${origin}/${locale}/products/${process.env.LC_ORDER_PRODUCT}`);
    await expect(p.getByTestId("add-to-cart")).toBeEnabled();assert.equal(await stored(p),null);
    await cartRetry(p,locale,"drawer",async caseID=>cartClick(p,caseID,"add-to-cart",p.getByTestId("add-to-cart"),"first cart write settles as uncertain",async()=>expect(p.getByTestId("add-to-cart")).toBeEnabled()));
    const routeCase=`BC01 ${locale} cart-page visible cart Retry preserves key/body and one committed write after reload`;
    await cartClick(p,routeCase,"View cart",p.getByTestId("cart-drawer").getByRole("link",{name:cartCopy[locale].view,exact:true}),"cart page shows the retained first item",async()=>{
      await expect(p).toHaveURL(`${origin}/${locale}/cart`);await expect(p.getByTestId("cart-page").getByTestId("cart-line-qty")).toHaveText("1");
    });
    await cartRetry(p,locale,"cart-page",async caseID=>cartClick(p,caseID,"Increase quantity",p.getByTestId("cart-page").getByRole("button",{name:cartCopy[locale].increase,exact:true}),"cart page shows uncertain committed edit",async()=>expect(p.getByTestId("cart-page").getByTestId("cart-problem")).toBeVisible()));
    await context.close();
  }

  await cartRefreshRace();

  // BO01/BO03: native form, all locales, in-memory PII and causal lost PUT.
  const c1=await newContext(),{p:a,quote:q1}=await quotePage(c1,false,undefined,true);await rememberCookie(c1);
  await stableLocaleTarget(a); // Run before filling PII so the original language/unsaved-address assertions retain their input.
  for(const name of Object.keys(pii))await expect(a.locator(`input[name="${name}"]`)).toHaveCount(1);
  await expect(a.locator('input[name="phone"]')).toHaveAttribute("type","tel");
  await fill(a);
  await capture(a,"desktop-address.png");
  await a.getByTestId("address-section").scrollIntoViewIfNeeded();await capture(a,"desktop-address-viewport.png",false);
  for(const locale of ["zh-CN","zh-TW","en"]){
    // The shell's language link is a full navigation (the old header select swapped the locale in place): the pinned quote survives it
    // through the journal, the unsaved address form must NOT (no PII in the URL or storage), so the fields come back empty.
    await switchLocale(a,locale);
    await expect(a).toHaveURL(`${origin}/${locale}/checkout`);
    await expect(a.getByTestId("address-section")).toBeVisible();
    for(const name of Object.keys(pii))await expect(a.locator(`input[name="${name}"]`)).toHaveValue("");
  }
  await fill(a);
  await expect(a.getByTestId("create-order")).toBeDisabled();
  pass("BO01 native address; three locales keep the pinned quote and carry no unsaved PII across the language switch");
  const destinationStart=calls.length,dropDest=arm("destination",{method:"PUT",drop:true,repeat:true});
  await a.getByTestId("confirm-address").click();assert.equal(await dropDest.result.promise,200);
  await expect(a.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const destinationPending=await stored(a);assert.equal(destinationPending.kind,"destination");
  assert.deepEqual(Object.keys(destinationPending.body).sort(),["cart_version","country","expected_version","kind"]);
  await a.getByTestId("recover-purchase").click();await confirm(a);
  const attempts=calls.slice(destinationStart).filter(x=>x.path==="/api/buyer/destination"&&x.method==="PUT");
  assert(attempts.length>=2);assert.equal(new Set(attempts.map(x=>x.key)).size,1);assert.equal(new Set(attempts.map(x=>x.body)).size,1);
  pass("BO03 committed destination lost reply retries exact key and body");

  await a.locator('input[name="line2"]').fill("Synthetic Changed Unit");
  const dropReloadDest=arm("destination",{method:"PUT",drop:true,repeat:true});
  await a.getByTestId("confirm-address").click();assert.equal(await dropReloadDest.result.promise,200);
  await expect(a.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const pendingReloadDestination=await stored(a),beforeReloadDestination=calls.filter(x=>x.path==="/api/buyer/destination"&&x.method==="PUT").length;
  await a.reload();await expect(a.getByTestId("address-section")).toBeVisible();
  await expect(a.locator('input[name="recipient_name"]')).toHaveValue(pii.recipient_name);
  await expect(a.getByTestId("create-order")).toBeDisabled();
  assert.equal(calls.filter(x=>x.path==="/api/buyer/destination"&&x.method==="PUT").length,beforeReloadDestination,"reload must not replay PII");
  await confirm(a);const replacedDestination=calls.filter(x=>x.path==="/api/buyer/destination"&&x.method==="PUT").at(-1);
  assert.notEqual(replacedDestination.key,pendingReloadDestination.key);assert(JSON.parse(replacedDestination.body).expected_version>pendingReloadDestination.body.expected_version);
  pass("BO03 lost destination reply reload reads head and explicitly replaces metadata intent");
  await a.locator('input[name="line2"]').fill(pii.line2);await expect(a.getByTestId("create-order")).toBeDisabled();await confirm(a);
  const sourceDest=(await api(a,"GET","destination")).body.destination;
  const external={expected_version:sourceDest.version,cart_version:sourceDest.cart_version,kind:"home",country:"TW",recipient_name:pii.recipient_name,phone:pii.phone,home_address:{...sourceDest.home_address,line1:"Synthetic Concurrent Address"}};
  const other=await c1.newPage();await other.goto(origin+checkoutPath);
  assert.equal((await api(other,"PUT","destination",external,crypto.randomUUID())).status,200);
  const beforeConflict=(await control("facts")).length;
  const conflict=a.waitForResponse(r=>requestIs(r,"checkout","POST"));await a.getByTestId("create-order").click();assert.equal((await conflict).status(),409);await expect(a.locator(".purchase-error")).toContainText("changed");assert.equal((await control("facts")).length,beforeConflict);
  await a.reload();await expect(a.locator('input[name="line1"]')).toHaveValue(external.home_address.line1);await expect(a.getByTestId("create-order")).toBeDisabled();await confirm(a);
  assert.equal((await api(other,"PUT","destination",external,crypto.randomUUID())).status,409,"late lower-CAS address must fail");
  pass("BO03 reload reconfirmation, edits and concurrent/late address CAS fail closed");
  await a.getByTestId("create-order").click();const order1=await created(a,q1);
  await capture(a,"desktop-order.png");
  pass("BO04 actual confirmed form creates authoritative DRAFT and exactly one order/hold/job/receipt");

  // Historical receipt is insufficient: expire through the real worker then GET.
  await control(`expire/${order1}`,"POST");
  const getOrder=a.waitForResponse(r=>requestIs(r,`orders/${order1}`,"GET"));
  await a.getByRole("button",{name:"Refresh order",exact:true}).click();assert.equal((await getOrder).status(),200);
  await expect(a.getByTestId("order-state")).toHaveAttribute("data-state","CANCELLED");
  await expect(a.getByTestId("order-id")).toHaveText(order1);await expect(a.getByTestId("create-order")).toHaveCount(0);
  pass("BO05 owned current-state GET renders real expiry rather than historical DRAFT receipt");

  // BO02: the server rejects an expired quote even if the page still has it.
  const c2=await newContext(),{p:stale,quote:q2}=await quotePage(c2);await fill(stale);await confirm(stale);
  await control(`expire-quote/${q2.id}`,"POST");const beforeExpired=(await control("facts")).length;
  const denied=stale.waitForResponse(r=>requestIs(r,"checkout","POST"));await stale.getByTestId("create-order").click();assert.equal((await denied).status(),409);
  await expect(stale.locator(".purchase-error")).toContainText("changed");assert.equal((await control("facts")).length,beforeExpired);
  await expect(stale.getByTestId("order-section")).toHaveCount(0);
  pass("BO02 expired authoritative quote fails closed with no order or hold");

  // BO04: actual UI invalidates the stale second-tab confirmation. Never force
  // a disabled button enabled to manufacture a UI race the product prevents.
  const c3=await newContext(),{p:tab1,quote:q3}=await quotePage(c3);await fill(tab1);await confirm(tab1);
  const popup=tab1.waitForEvent("popup");await tab1.evaluate(url=>window.open(url,"_blank"),origin+checkoutPath);const tab2=await popup;
  await expect(tab2.getByTestId("address-section")).toBeVisible();await confirm(tab2);
  // tab2's confirmation advanced the head; explicitly inspect and reconfirm in
  // tab1 so both tabs start with the same displayed current destination.
  await tab1.reload();await expect(tab1.getByTestId("address-section")).toBeVisible();await confirm(tab1);
  await expect(tab2.getByTestId("create-order")).toBeDisabled();
  const queuedStart=calls.length,held=arm("checkout",{method:"POST",after:true});
  await tab1.getByTestId("create-order").click();assert.equal(await held.result.promise,200);
  await expect(tab1.getByTestId("create-order")).toBeDisabled();
  await expect(tab2.getByTestId("create-order")).toHaveCount(0);
  await expect(tab2.getByTestId("recover-purchase")).toBeVisible();
  await tab2.getByTestId("recover-purchase").click();
  await expect.poll(()=>tab2.evaluate(async()=> (await navigator.locks.query()).pending.filter(x=>x.name==="commerce-purchase-write-v1").length)).toBeGreaterThan(0);
  held.release.resolve();const order3=await created(tab1,q3);assert.equal(await created(tab2,q3),order3);
  assert.equal(calls.slice(queuedStart).filter(x=>x.path==="/api/buyer/checkout"&&x.method==="POST").length,1);
  pass("BO04 actual second-tab recovery queues on Web Lock and resumes same order without second POST");

  // BO05: a committed checkout with no reply survives document death/reload.
  const c4=await newContext(true),{p:lost,quote:q4}=await quotePage(c4,false,undefined,true);await stableLocaleTarget(lost,true);await fill(lost);await confirm(lost);
  await capture(lost,"mobile-address.png");
  const lostStart=calls.length,dropCheckout=arm("checkout",{method:"POST",drop:true,repeat:true});
  await lost.getByTestId("create-order").click();assert.equal(await dropCheckout.result.promise,200);await expect(lost.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const lostPending=await stored(lost);assert.equal(lostPending.kind,"checkout");
  await expect(lost.getByTestId("continue-shopping")).toHaveCount(0);
  assert.deepEqual(Object.keys(lostPending.body).sort(),["allocation_version","cart_version","destination_id","quote_id","service_version"]);
  await lost.reload();await expect(lost.getByTestId("recover-purchase")).toBeVisible();assert.deepEqual(await stored(lost),lostPending);
  await lost.getByTestId("recover-purchase").click();await created(lost,q4);
  const replays=calls.slice(lostStart).filter(x=>x.path==="/api/buyer/checkout"&&x.method==="POST");assert(replays.length>=2);assert.equal(new Set(replays.map(x=>x.key)).size,1);assert.equal(new Set(replays.map(x=>x.body)).size,1);
  assert.equal(await stored(lost),null);await capture(lost,"mobile-order.png");
  assert(await lost.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  pass("BO05 lost checkout reply reload replays exact five-field intent and original key on mobile");

  // Success storage failure keeps the original intent until locator readback.
  const c5=await newContext(),{p:fault,quote:q5}=await quotePage(c5);await fill(fault);await confirm(fault);
  await fault.evaluate(()=>window.__failOrderLocator=true);const storageStart=calls.length;
  await fault.getByTestId("create-order").click();await expect(fault.getByTestId("recover-purchase")).toBeVisible();
  const storagePending=await stored(fault);assert.equal(storagePending.kind,"checkout");assert.equal(await stored(fault,"commerce-purchase-order-v1:"),null);
  await fault.reload();await expect(fault.getByTestId("recover-purchase")).toBeVisible();await fault.getByTestId("recover-purchase").click();await created(fault,q5);
  const storageAttempts=calls.slice(storageStart).filter(x=>x.path==="/api/buyer/checkout"&&x.method==="POST");assert.equal(storageAttempts.length,2);assert.equal(storageAttempts[0].key,storageAttempts[1].key);
  pass("BO05 locator storage failure retains and reloads same checkout without duplicate hold");

  // A later denial after commit is not evidence of no order. Keep owner/journal.
  const c6=await newContext(),{p:revoked,quote:q6}=await quotePage(c6);await fill(revoked);await confirm(revoked);
  const dropRevoked=arm("checkout",{method:"POST",drop:true,repeat:true});await revoked.getByTestId("create-order").click();assert.equal(await dropRevoked.result.promise,200);
  await expect(revoked.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const unresolved=await stored(revoked),cookieBefore=(await c6.cookies(origin))[0].value,resetsBefore=sessionResets;
  await control("unpublish","POST");await revoked.getByTestId("recover-purchase").click();await expect(revoked.getByTestId("recover-purchase")).toBeVisible();
  assert.deepEqual(await stored(revoked),unresolved);assert.equal((await c6.cookies(origin))[0].value,cookieBefore);assert.equal(sessionResets,resetsBefore);
  await expect(revoked.getByRole("button",{name:/new guest|new session|start over/i})).toHaveCount(0);
  await control("publish","POST");await revoked.reload();await expect(revoked.getByTestId("recover-purchase")).toBeVisible();await revoked.getByTestId("recover-purchase").click();await created(revoked,q6);
  pass("BO05 later publication revocation preserves uncertain committed order and never resets owner");

  // R2: late options hydration must never overwrite a newly confirmed address.
  const c8=await newContext(),{p:slow,quote:q8}=await quotePage(c8);await fill(slow);
  const dropA=arm("destination",{method:"PUT",drop:true,repeat:true});await slow.getByTestId("confirm-address").click();assert.equal(await dropA.result.promise,200);
  await expect(slow.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const slowOptions=arm(`checkout-options?market_id=${q8.market_id}&country=TW&limit=100`,{method:"GET",after:true});
  await slow.reload();await slowOptions.entered.promise;
  await expect(slow.locator('input[name="line1"]')).toHaveValue(pii.line1);
  await slow.locator('input[name="line1"]').fill("Synthetic Confirmed New Address");
  await slow.getByTestId("confirm-address").click();await expect.poll(()=>stored(slow)).toBeNull();
  slowOptions.release.resolve();await expect(slow.getByTestId("create-order")).toBeEnabled();
  await expect(slow.locator('input[name="line1"]')).toHaveValue("Synthetic Confirmed New Address");
  await slow.getByTestId("create-order").click();const order8=await created(slow,q8);
  assert.equal((await api(slow,"GET",`orders/${order8}`)).body.snapshot.destination.home_address.line1,"Synthetic Confirmed New Address");
  await expect(slow.getByTestId("order-section")).toContainText("Synthetic Confirmed New Address");
  pass("BO03 delayed real options cannot replace edited confirmed address or order snapshot");

  // R1: expiry cannot remove the only recovery surface; a fresh tab has no
  // session quote locator and still must recover the shared destination intent.
  const c9=await newContext(),{p:expiredDest,quote:q9}=await quotePage(c9,true);await fill(expiredDest);
  const dropExpired=arm("destination",{method:"PUT",drop:true,repeat:true});await expiredDest.getByTestId("confirm-address").click();assert.equal(await dropExpired.result.promise,200);
  await expect(expiredDest.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const expiredMarker=await stored(expiredDest);
  await control(`expire-quote/${q9.id}`,"POST");await expiredDest.clock.fastForward(61000);
  await expect(expiredDest.getByRole("button",{name:"Reload quotation",exact:true})).toBeDisabled();
  await expect(expiredDest.getByTestId("confirm-address")).toBeEnabled();
  const noQuoteTab=await c9.newPage();await noQuoteTab.goto(origin+checkoutPath);
  assert.equal(await noQuoteTab.evaluate(()=>Object.keys(sessionStorage).filter(k=>k.startsWith("commerce-purchase-quote-v1:")).length),0);
  await expect(noQuoteTab.getByTestId("address-section")).toBeVisible();
  await expect(noQuoteTab.locator('input[name="line1"]')).toHaveValue(pii.line1);
  await expect(noQuoteTab.getByTestId("confirm-address")).toBeEnabled();await noQuoteTab.getByTestId("confirm-address").click();
  await expect.poll(()=>stored(noQuoteTab)).toBeNull();
  const replacement=calls.filter(x=>x.path==="/api/buyer/destination"&&x.method==="PUT").at(-1);assert.notEqual(replacement.key,expiredMarker.key);
  assert(JSON.parse(replacement.body).expected_version>expiredMarker.body.expected_version);
  await expect(noQuoteTab.getByTestId("order-section")).toHaveCount(0);
  pass("BO03 expired pending address retains recovery; quote-less new tab explicitly repairs shared intent");

  // BH01/02: delay the new tab's INITIAL owned GET while the original tab
  // commits continuation, loses its reply, reloads and recovers the same key.
  const originalOrder=(await api(a,"GET",`orders/${order1}`)).body;
  const originalFact=(await control("facts")).find(x=>x.id===order1);
  const originalCheckout=calls.find(x=>x.path==="/api/buyer/checkout"&&x.method==="POST"&&x.status===200&&JSON.parse(x.body).quote_id===q1.id);
  assert(originalCheckout);
  const initialOrder={entered:deferred(),release:deferred()};
  const late=await c1.newPage();
  // Hold this tab's actual responses, including storage-event refreshes. If an
  // intermediate refresh could render A, later pointer removal would invalidate
  // the initial load and accidentally hide the original race from this gate.
  const lateOrderRoute=async route=>{
    const response=await route.fetch();assert.equal(response.status(),200);
    initialOrder.entered.resolve();await initialOrder.release.promise;await route.fulfill({response});
  };
  await late.route(`${origin}/api/buyer/orders/${order1}`,lateOrderRoute);
  await late.goto(origin+checkoutPath);await initialOrder.entered.promise;
  await expect(late.getByTestId("order-section")).toHaveCount(0);
  const continuationStart=calls.length,dropCart=arm("cart",{method:"PUT",drop:true,repeat:true});
  await a.getByTestId("continue-shopping").click();assert.equal(await dropCart.result.promise,200);
  await expect(a.getByTestId("recover-purchase")).toBeVisible();hook=null;
  const nextIntent=await stored(a);assert.equal(nextIntent.kind,"next-cart");
  assert.deepEqual(nextIntent.body,{expected_version:originalOrder.cart_version,items:[]});
  await a.reload();await expect(a.getByTestId("recover-purchase")).toBeVisible();assert.deepEqual(await stored(a),nextIntent);
  await a.getByTestId("recover-purchase").click();await expect(a.getByTestId("checkout-empty")).toBeVisible(); // continuation recovered: empty cart, no stuck journal
  assert.equal(await stored(a),null);assert.equal(await stored(a,"commerce-purchase-order-v1:"),null);
  const continuationCalls=calls.slice(continuationStart).filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT");
  assert(continuationCalls.length>=2);assert.equal(new Set(continuationCalls.map(x=>x.key)).size,1);assert.equal(new Set(continuationCalls.map(x=>x.body)).size,1);
  assert.equal((await control("facts")).find(x=>x.id===order1).cart_receipts,originalFact.cart_receipts+1);
  const continuedCart=(await api(a,"GET","cart")).body;assert.equal(continuedCart.version,originalOrder.cart_version+1);assert.deepEqual(continuedCart.items,[]);
  pass("BH01 lost next-cart reply reload keeps original key/body and exactly one cart receipt");
  await expect(late.getByTestId("order-section")).toHaveCount(0);
  initialOrder.release.resolve();
  await expect(late.getByTestId("checkout-empty")).toBeVisible();
  await expect(late.getByTestId("order-section")).toHaveCount(0);
  assert.equal(await stored(late,"commerce-purchase-order-v1:"),null);
  assert.deepEqual((await api(late,"GET","cart")).body,continuedCart);
  await late.unroute(`${origin}/api/buyer/orders/${order1}`,lateOrderRoute);
  pass("BH02 delayed initial owned GET cannot restore the old order after another tab continues");

  const {quote:qB}=await quotePage(c1,false,a);await fill(a);await confirm(a);
  await a.getByTestId("create-order").click();const orderB=await created(a,qB,2);assert.notEqual(orderB,order1);
  assert.deepEqual((await api(a,"GET",`orders/${order1}`)).body,originalOrder);
  assert.equal((await control("facts")).find(x=>x.id===order1).snapshot_hash,originalFact.snapshot_hash);
  const replayA=await api(a,"POST","checkout",JSON.parse(originalCheckout.body),originalCheckout.key);
  assert.equal(replayA.status,200);assert.equal(replayA.body.order_id,order1);
  assert.equal((await stored(a,"commerce-purchase-order-v1:")).order_id,orderB);
  for(const f of (await control("facts")).filter(x=>[order1,orderB].includes(x.id)))for(const key of ["orders","holds","jobs","receipts","reserve_lines"])assert.equal(f[key],2);
  pass("BH03 same buyer creates distinct B with two exact order facts and unchanged A snapshot/key replay");

  const firstHistory=await api(a,"GET","orders?limit=1");assert.equal(firstHistory.status,200);
  assert.deepEqual(firstHistory.body.items.map(x=>x.order_id),[orderB]);assert(firstHistory.body.next_cursor);
  const secondHistory=await api(a,"GET",`orders?limit=1&cursor=${encodeURIComponent(firstHistory.body.next_cursor)}`);
  assert.equal(secondHistory.status,200);assert.deepEqual(secondHistory.body.items.map(x=>x.order_id),[order1]);assert.equal(secondHistory.body.next_cursor,"");
  const summaryKeys=["order_id","created_at","cart_id","cart_version","commercial_state","fulfillment_state","currency","total_minor"].sort();
  for(const summary of [...firstHistory.body.items,...secondHistory.body.items])assert.deepEqual(Object.keys(summary).sort(),summaryKeys);
  const pointerB=await stored(a,"commerce-purchase-order-v1:"),cartB=(await api(a,"GET","cart")).body;
  assert.equal(await a.evaluate(()=>Intl.DateTimeFormat().resolvedOptions().timeZone),"America/Los_Angeles");
  const historyLoading=arm("orders?limit=20",{method:"GET",after:true}),firstHistoryUI=historyRead(a);await a.getByTestId("toggle-order-history").click();
  assert.equal(await historyLoading.result.promise,200);await expect(a.getByTestId("order-history").getByRole("status")).toBeVisible();historyLoading.release.resolve();
  await expect(a.locator(".history-list li")).toHaveCount(2);
  await assertHistoryTimes(a,"en",await firstHistoryUI);
  await expect(a.locator(".history-list li").first().locator("button")).toHaveAttribute("data-order-id",orderB);
  await capture(a,"desktop-history.png",true,historyReview);
  await a.setViewportSize({width:390,height:844});await capture(a,"mobile-history.png",true,historyReview);
  assert(await a.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));await a.setViewportSize({width:1440,height:900});
  await a.locator(`button[data-order-id="${order1}"]`).click();await expect(a.getByTestId("order-id")).toHaveText(order1);
  await expect(a.getByTestId("order-state")).toHaveAttribute("data-state","CANCELLED");
  assert.deepEqual(await stored(a,"commerce-purchase-order-v1:"),pointerB);assert.deepEqual((await api(a,"GET","cart")).body,cartB);
  await a.getByRole("button",{name:"Back to orders",exact:true}).click();
  for(const [locale,title] of [["zh-CN","我的订单"],["zh-TW","我的訂單"],["en","Your orders"]]){
    await switchLocale(a,locale);const localeHistory=historyRead(a);await a.getByTestId("toggle-order-history").click(); // full navigation: the history view is reopened from the pinned order
    await expect(a.getByTestId("order-history").getByRole("heading",{name:title,exact:true})).toBeVisible();
    await expect(a.locator(".history-list li")).toHaveCount(2);await assertHistoryTimes(a,locale,await localeHistory);assert.deepEqual(await stored(a,"commerce-purchase-order-v1:"),pointerB);
  }
  await a.getByTestId("toggle-order-history").click();await expect(a.getByTestId("order-id")).toHaveText(orderB);
  pass("BH04 owned history renders each server created_at in Taipei across locales under Los Angeles browser time");

  // Lose only the convenience locator, retaining the same HttpOnly credential.
  const cookieHistory=(await c1.cookies(origin))[0].value;
  await a.evaluate(()=>{for(const key of Object.keys(localStorage))if(key.startsWith("commerce-purchase-order-v1:"))localStorage.removeItem(key);});
  await a.reload();await expect(a.getByTestId("toggle-order-history")).toBeEnabled();await expect(a.getByTestId("order-section")).toHaveCount(0);
  const reloadedHistory=historyRead(a);await a.getByTestId("toggle-order-history").click();await expect(a.locator(".history-list li")).toHaveCount(2);await assertHistoryTimes(a,"en",await reloadedHistory);
  await a.locator(`button[data-order-id="${order1}"]`).click();await expect(a.getByTestId("order-id")).toHaveText(order1);
  assert.equal(await stored(a,"commerce-purchase-order-v1:"),null);assert.equal((await c1.cookies(origin))[0].value,cookieHistory);
  await a.getByTestId("toggle-order-history").click();await expect(a.getByTestId("order-section")).toHaveCount(0);
  pass("BH05 authoritative Taipei history survives reload and noncredential locator loss without repinning an old order");

  // Same owned session, read-only history in UTC: no extra order or fixture row.
  const utcContext=await newContext(false,"UTC");await utcContext.addCookies(await c1.cookies(origin));
  const utcPage=await utcContext.newPage();await utcPage.goto(origin+checkoutPath);
  assert.equal(await utcPage.evaluate(()=>Intl.DateTimeFormat().resolvedOptions().timeZone),"UTC");
  await expect(utcPage.getByTestId("toggle-order-history")).toBeEnabled();
  const utcHistory=historyRead(utcPage);await utcPage.getByTestId("toggle-order-history").click();
  await expect(utcPage.locator(".history-list li")).toHaveCount(2);await assertHistoryTimes(utcPage,"en",await utcHistory);
  await utcPage.reload();await expect(utcPage.getByTestId("toggle-order-history")).toBeEnabled();
  const utcReloadHistory=historyRead(utcPage);await utcPage.getByTestId("toggle-order-history").click();
  await expect(utcPage.locator(".history-list li")).toHaveCount(2);await assertHistoryTimes(utcPage,"en",await utcReloadHistory);
  await utcContext.close();
  pass("BH05 same owned history renders Taipei times in UTC browser context");

  // Preserve an already advanced cart, including another tab's selected items.
  const previousCart=(await api(tab1,"GET","cart")).body;
  const advanced=await api(tab2,"PUT","cart",{expected_version:previousCart.version,items:previousCart.items.map(x=>({...x,quantity:2}))},crypto.randomUUID());assert.equal(advanced.status,200);
  const advancedStart=calls.length;await tab1.getByTestId("continue-shopping").click();
  await expect(tab1.getByRole("button",{name:"Choose delivery",exact:true})).toBeEnabled();
  assert.deepEqual((await api(tab1,"GET","cart")).body,advanced.body);
  assert.equal(calls.slice(advancedStart).filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT").length,0);
  pass("BH06 explicit continuation preserves a newer authoritative cart without issuing a clearing PUT");

  // A quote removal no-op must retain the journal and old order pointer.
  await fault.evaluate(()=>window.__keepQuoteLocator=true);
  const quoteFailureStart=calls.length;await fault.getByTestId("continue-shopping").click();await expect(fault.getByTestId("recover-purchase")).toBeVisible();
  const failedNext=await stored(fault);assert.equal(failedNext.kind,"next-cart");assert(await stored(fault,"commerce-purchase-order-v1:"));
  await fault.reload();await expect(fault.getByTestId("recover-purchase")).toBeVisible();assert.deepEqual(await stored(fault),failedNext);
  await fault.getByTestId("recover-purchase").click();await expect(fault.getByTestId("checkout-empty")).toBeVisible();
  const quoteFailureCalls=calls.slice(quoteFailureStart).filter(x=>x.path==="/api/buyer/cart"&&x.method==="PUT");assert.equal(quoteFailureCalls.length,2);
  assert.equal(quoteFailureCalls[0].key,quoteFailureCalls[1].key);assert.equal(quoteFailureCalls[0].body,quoteFailureCalls[1].body);
  assert.equal(await stored(fault),null);assert.equal(await stored(fault,"commerce-purchase-order-v1:"),null);
  pass("BH07 failed quote removal preserves continuation recovery and original key across reload");

  // Cross-owner read is a real HTTP denial and must never display a snapshot.
  const c7=await newContext(),{p:foreign}=await quotePage(c7);const otherOrder=await api(foreign,"GET",`orders/${order1}`);assert.equal(otherOrder.status,404);
  await expect(foreign.getByTestId("order-section")).toHaveCount(0);
  const foreignHistory=await api(foreign,"GET","orders?limit=1");assert.equal(foreignHistory.status,200);assert.deepEqual(foreignHistory.body,{items:[],next_cursor:""});
  assert.equal((await api(foreign,"GET",`orders?cursor=${encodeURIComponent(firstHistory.body.next_cursor)}`)).status,422);
  assert.equal((await api(a,"GET","orders?cursor=bad")).status,422);
  await foreign.getByTestId("toggle-order-history").click();await expect(foreign.getByTestId("order-history")).toContainText("No orders in this shopping session.");
  await foreign.getByTestId("toggle-order-history").click();await expect(foreign.getByTestId("address-section")).toBeVisible();
  pass("BH08 foreign buyer history is empty and foreign or malformed cursors are denied");
  await fill(foreign);await confirm(foreign);const serviceBefore=(await control("facts")).length;
  await control("service-drift","POST");const serviceDenied=foreign.waitForResponse(r=>requestIs(r,"checkout","POST"));
  await foreign.getByTestId("create-order").click();assert.equal((await serviceDenied).status(),409);
  await expect(foreign.locator(".purchase-error")).toContainText("changed");assert.equal((await control("facts")).length,serviceBefore);
  await expect(foreign.getByTestId("create-order")).toBeDisabled();
  pass("BO02 service revision drift rejects a previously confirmed quotation without an order");
  // Regression: this buyer is finished, like PR7's short-lived UTC context; BO06 must still audit its state.
  await c7.close();
  const contextStates=[];
  for(const c of contexts)contextStates.push(...(closedContextStates.get(c)??await captureContextState(c)));
  // Compare every open/closed snapshot only after all contexts contributed their bearer canaries.
  for(const state of contextStates){
    for(const value of [...Object.values(pii),"Synthetic Changed Unit","Synthetic Concurrent Address","Synthetic Confirmed New Address",...secrets])assert(!state.includes(value),"PII/bearer leaked into persistent storage");
  }
  const attempted=JSON.stringify(storageWrites),urls=requestURLs.join("\n"),messages=consoleText.join("\n");
  for(const value of [...Object.values(pii),"Synthetic Changed Unit","Synthetic Concurrent Address","Synthetic Confirmed New Address",...secrets]){
    assert(!attempted.includes(value),"PII/bearer attempted persistent write");assert(!urls.includes(value)&&!urls.includes(encodeURIComponent(value)),"PII/bearer URL leak");assert(!messages.includes(value),"PII/bearer console leak");
  }
  assert.deepEqual(pageErrors.filter(e=>!isWebkitCancelledFetch(e)).map(e=>e.name),[],"browser application exception"); // WebKit cancelled-fetch console noise: browser-engine.mjs
  pass("BO06 all attempted local/session writes, URLs and console exclude PII/bearer; other owner denied");
  await writeFile(path.join(evidence,"result.json"),JSON.stringify({cases:observations.length,orders,repeated_orders:[order1,orderB],observations,cart_retries:cartRetries,cart_refreshes:cartRefreshes,click_ledger:clickLedger,storage_write_attempts:storageWrites.length,scope:"actual UI/Next/Go/isolated PG; synthetic TLS and buyer data; no PSP/production",not_run:["full foundation/race/vet and existing browser regression are separate root gates","independent visual review"]},null,2),{mode:0o600});
}catch(error){
  // A failed step leaves what the buyer was looking at (screenshot + visible text of every page), so an intermittent failure is diagnosable from its own run.
  if(browser)for(const context of browser.contexts())for(const [index,page] of context.pages().entries()){
    await page.screenshot({path:path.join(evidence,`failure-${index}.png`),fullPage:true}).catch(()=>{});
    await writeFile(path.join(evidence,`failure-${index}.txt`),`${page.url()}\n${await page.locator("body").innerText().catch(()=>"")}`.slice(0,6000)).catch(()=>{});
  }
  throw error;
}finally{
  if(hook?.release)hook.release.resolve();
  if(browser)await browser.close();for(const s of sockets)s.destroy();
  for(const server of [proxy,edge])if(server)await new Promise(r=>server.close(r));
  for(const child of children){if(child.exitCode===null){const ended=once(child,"exit");child.kill("SIGTERM");await Promise.race([ended,pause(2000)]);if(child.exitCode===null){child.kill("SIGKILL");await ended;}}}
  await Promise.all(logs.map(log=>new Promise(r=>log.end(r))));await rm(certDir,{recursive:true,force:true});
}
