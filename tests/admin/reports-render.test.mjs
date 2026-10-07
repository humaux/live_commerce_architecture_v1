// Purpose: execute actual report view rendering and report BFF leaf with independent synthetic boundaries.
// Depends on: existing TypeScript transpiler, React SSR, Node test/assert and report pure modules.
// Used by: focused Node gate; MOCK transport only, not a substitute for browser/Go/PG gates.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { test } from "node:test";
import ts from "typescript-api";

// Compile in memory: no build/server/browser, no new dependencies or temporary source files.
function load(source, overrides = {}) {
  const filename = new URL(source, import.meta.url);
  const require = createRequire(filename);
  const code = ts.transpileModule(readFileSync(filename, "utf8"), {
    compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  const module = { exports: {} };
  new Function("require", "module", "exports", code)((name) => Object.hasOwn(overrides, name) ? overrides[name] : require(name), module, module.exports);
  return module.exports;
}
const adminRequire = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const { createElement } = adminRequire("react");
const { renderToStaticMarkup } = adminRequire("react-dom/server");
const views = load("../../apps/admin/components/ReportViews.tsx");
const copy = adminRequire("./lib/reports-copy.ts").reportsCopy;
const id = "11111111-1111-4111-8111-111111111111";
const from = "2026-09-01", to = "2026-09-30", head = { from, to, timezone: "Asia/Taipei" };
const render = (view, props) => renderToStaticMarkup(createElement(view, props));
const amount = (environment, captured_minor = 10000) => ({environment, captured_count: 1, captured_minor, refunded_minor: 2000, net_minor: captured_minor - 2000, offline_count: 1, offline_minor: 900});

test("actual product table renders offline separately, both modes, no UUID copy and sortable headers", () => {
  const row = { sku_id: id, product_id: id, name: "Cup <script>", code: "CUP-1", currency: "TWD", environment: "LIVE", units: 2,
    captured_minor: 10000, refunded_minor: 2000, net_minor: 8000, offline_units: 1, offline_minor: 900 };
  const html = render(views.ProductReportTable, { locale: "en", report: {...head, truncated:false, rows:[row, {...row, environment:"SANDBOX"}]}, sort:"net_minor", ascending:false, onSort:()=>{} });
  assert.match(html, /aria-sort="descending"/);
  assert.match(html, /Offline collected/); assert.match(html, /Online net/);
  assert.match(html, /Live payments/); assert.match(html, /Test payments/);
  assert.match(html, /Cup &lt;script&gt;/); assert.doesNotMatch(html, /<script>|11111111/);
  assert.match(html, /NT\$100/); assert.match(html, /NT\$9/);
});

test("actual channel chart exposes exact numbers and both money modes using SVG", () => {
  const html = render(views.ChannelReportChart, { locale:"en", report:{...head, rows:[{ channel:"facebook_live", currency:"TWD", orders:12, cancelled_orders:2, money:[amount("LIVE"),amount("SANDBOX", 6000)] }]} });
  assert.match(html, /<svg/); assert.match(html, /role="img"/); assert.match(html, /Orders 12/);
  assert.match(html, /Test payments/); assert.match(html, /Offline collected/);
});

test("actual funnel has four stages, nested widths and truthful zero-denominator text", () => {
  const report = {...head, session_id:null, claimed:100, link_sent:50, ordered:25, paid:10, ordered_without_link:8};
  const html = render(views.FunnelReportChart, {locale:"en", report});
  assert.equal((html.match(/<svg/g)??[]).length,4);
  for (const width of [100,50,25,10]) assert.match(html,new RegExp(`width="${width}" height="10" class="reports-bar-fill"`));
  for (const ratio of ["50%","40%","10%"] ) assert.ok(html.includes(ratio), ratio);
  const empty = render(views.FunnelReportChart,{locale:"en",report:{...report,claimed:0,link_sent:0,ordered:0,paid:0,ordered_without_link:0}});
  assert.match(empty,/No starting count/); assert.doesNotMatch(empty,/NaN|Infinity/);
});

test("manual table resolves authorized labels, has honest fallback, and never presents UUIDs", () => {
  const report={...head,rows:[{principal_id:id,session_id:id,currency:"TWD",orders:2,cancelled_orders:1,money:[amount("LIVE"),amount("SANDBOX")]}]};
  const html = render(views.ManualReportTable,{locale:"en",report,staff:{[id]:"operator@example.invalid"},sessions:{[id]:"Autumn sale"}});
  assert.match(html,/operator@example.invalid/);assert.match(html,/Autumn sale/);assert.match(html,/rowSpan="2"|rowspan="2"/);
  assert.doesNotMatch(html,/11111111/);
  const fallback=render(views.ManualReportTable,{locale:"zh-TW",report,staff:{},sessions:{}});
  assert.ok(fallback.includes(copy["zh-TW"].unknownCreator)); assert.ok(fallback.includes(copy["zh-TW"].unknownSession));
  assert.doesNotMatch(fallback,/11111111/);
});

test("all report views render localized empty states", () => {
  for (const locale of ["zh-TW","zh-CN","en"]) for (const name of ["ProductReportTable","ChannelReportChart","ManualReportTable"]) {
    const html=render(views[name],{locale,report:{...head,truncated:false,rows:[]},sort:"name",ascending:true,onSort:()=>{},staff:{},sessions:{}});
    assert.ok(html.includes(copy[locale].empty),`${name} ${locale}`);
  }
});

function router({token="trusted-session", stores=[{id,permissions:["orders:read","orders:export","live:read"]}], configured=true, upstream} = {}) {
  const calls=[],fetchCalls=[];
  const auth={authConfig: configured ? {publicOrigin:"https://admin.example.invalid",apiOrigin:"https://go.example.invalid"} : null,
    sessionToken:()=>token, clearAuthCookies:(headers)=>headers.set("Set-Cookie","commerce_session=; Max-Age=0"),
    requireOrigin:(req)=>req.headers.get("origin")==="https://admin.example.invalid",
    requireCSRF:(req)=>req.headers.get("x-csrf-token")==="csrf-pair",
    authenticatedStores:async()=>({stores,response:Response.json({code:"unauthorized"},{status:401})}),
    safeError:async(response)=>Response.json({code:response.status===401?"unauthorized":"retry_later"},{status:response.status,headers:{"Cache-Control":"private, no-store"}}),
    localError:(status,code)=>Response.json({code},{status,headers:{"Cache-Control":"private, no-store"}}),
  };
  const route=load("../../apps/admin/app/api/stores/[store]/reports/[report]/route.ts",{
    "@/lib/auth":auth,
    "@/lib/orders-model":adminRequire("./lib/orders-model.ts"),
    "@/lib/reports-request":adminRequire("./lib/reports-request.ts"),
    "@/lib/reports-response":adminRequire("./lib/reports-response.ts"),
  });
  const send=async(report="products",query=`from=${from}&to=${to}`,headers={},signal)=>{
    const previous=globalThis.fetch;
    globalThis.fetch=async(...args)=>{fetchCalls.push(args);calls.push([new URL(args[0]).pathname.split(`/stores/${id}/`)[1]+new URL(args[0]).search,{method:args[1].method,cache:args[1].cache},args[1].headers.Authorization.slice(7),id]);return upstream(...args);};
    try {return await route.GET(new Request(`https://admin.example.invalid/api/stores/${id}/reports/${report}?${query}`,{headers,signal}),{params:Promise.resolve({store:id,report})});}
    finally {globalThis.fetch=previous;}
  };
  return {calls,fetchCalls,route,send};
}

test("actual BFF refuses malformed query, unknown store/session, disabled auth and missing CSV fences before Go",async()=>{
  for(const options of [{token:null},{stores:[]},{configured:false}]) {
    const r=router(options);const result=await r.send();assert.ok([401,404].includes(result.status));assert.equal(r.calls.length,0);
  }
  const r=router();
  for (const query of [`from=${from}&to=${to}&tenant_id=${id}`,`from=${from}&to=${to}&environment=LIVE`,`from=${from}&to=${to}&from=${from}`]) {
    assert.equal((await r.send("products",query)).status,422);
  }
  for(const headers of [{},{origin:"https://admin.example.invalid"},{"x-csrf-token":"csrf-pair"}])
    assert.equal((await r.send("products.csv",`from=${from}&to=${to}`,headers)).status,403);
  assert.equal(r.calls.length,0);
  assert.equal((await r.route.HEAD()).status,405);
});

test("actual BFF closed JSON projection and CSV scope/query preserve trusted token; no forwarded client authority",async()=>{
  const r=router({upstream:()=>Response.json({...head,truncated:false,rows:[]},{headers:{"Cache-Control":"private, no-store"}})});
  const response=await r.send("products",`to=${to}&from=${from}`,{Authorization:"Bearer attacker","X-Tenant-ID":"attacker"});
  assert.equal(response.status,200);assert.equal(response.headers.get("cache-control"),"private, no-store");
  assert.equal(r.calls[0][0],`reports/products?to=${to}&from=${from}`);
  assert.deepEqual(r.calls[0].slice(1),[{method:"GET",cache:"no-store"},"trusted-session",id]);
  const invalid=router({upstream:()=>Response.json({...head,truncated:false,rows:[],secret:"bad"},{headers:{"Cache-Control":"private, no-store"}})});
  assert.equal((await invalid.send()).status,503);
  const csv=router({upstream:()=>new Response("principal_id,session_id\n",{headers:{"Cache-Control":"private, no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":`attachment; filename="report-manual_orders-${from}-${to}.csv"`}})});
  const downloaded=await csv.send("manual-orders.csv",`from=${from}&to=${to}`,{origin:"https://admin.example.invalid","x-csrf-token":"csrf-pair"});
  assert.equal(downloaded.status,200);assert.equal(downloaded.headers.get("x-content-type-options"),"nosniff");
  assert.equal(csv.calls.length,1);assert.equal(await downloaded.text(),"principal_id,session_id\n");
});

test("actual BFF permission and malformed CSV response fences do not masquerade as a download",async()=>{
  const denied=router({stores:[{id,permissions:["orders:read"]}]});
  assert.equal((await denied.send("funnel")).status,403);
  assert.equal((await denied.send("products.csv",`from=${from}&to=${to}`,{origin:"https://admin.example.invalid","x-csrf-token":"csrf-pair"})).status,403);
  assert.equal(denied.calls.length,0);
  const html=router({upstream:()=>new Response("private diagnostic",{headers:{"Content-Type":"text/html","Cache-Control":"public"}})});
  const result=await html.send("products.csv",`from=${from}&to=${to}`,{origin:"https://admin.example.invalid","x-csrf-token":"csrf-pair"});
  assert.equal(result.status,503);assert.doesNotMatch(await result.text(),/diagnostic/);
});

test("audited browser GET accepts same-origin Fetch Metadata plus CSRF, never cross-site or forwarded Host",async()=>{
  const make=()=>router({upstream:()=>new Response("units\n1",{headers:{"Cache-Control":"private, no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":`attachment; filename="report-products-${from}-${to}.csv"`}})});
  const r=make();
  assert.equal((await r.send("products.csv",`from=${from}&to=${to}`,{"sec-fetch-site":"same-origin","x-csrf-token":"csrf-pair"})).status,200);
  for(const headers of [
    {"sec-fetch-site":"cross-site","x-csrf-token":"csrf-pair"},
    {"sec-fetch-site":"same-origin","origin":"https://attacker.invalid","x-csrf-token":"csrf-pair"},
    {"x-forwarded-host":"admin.example.invalid","x-csrf-token":"csrf-pair"},
  ]) {const denied=make();assert.equal((await denied.send("products.csv",`from=${from}&to=${to}`,headers)).status,403);assert.equal(denied.calls.length,0);}
});

function downloadClient(fence) {
  return load("../../apps/admin/lib/reports-client.ts", {
    "./settings-client":{csrfCookie:()=>fence.cookie,sessionBoundary:async()=>fence.check ? fence.check() : fence.boundary},
    "./customers-client":{get:()=>{throw Error("not called");},ReadError:class extends Error{}},
    "./team-client":{},"./studio-client":{},
    "./reports-model":adminRequire("./lib/reports-model.ts"),
    "./reports-response":adminRequire("./lib/reports-response.ts"),
  });
}

async function browserFixture(run) {
  const previous={fetch:globalThis.fetch,document:globalThis.document,createURL:URL.createObjectURL,revokeURL:URL.revokeObjectURL};
  const clicks=[],urls=[];
  URL.createObjectURL=(blob)=>{urls.push(blob);return "blob:reports-test";};URL.revokeObjectURL=()=>{};
  globalThis.document={visibilityState:"visible",body:{append:()=>{}},createElement:()=>({href:"",download:"",click(){clicks.push(this.download);},remove(){}})};
  try {await run(clicks,urls);} finally {globalThis.fetch=previous.fetch;URL.createObjectURL=previous.createURL;URL.revokeObjectURL=previous.revokeURL;if(previous.document===undefined)delete globalThis.document;else globalThis.document=previous.document;}
}

test("actual CSV client preflight sends nothing after session change, and a lost audit response is uncertain with no retry",async()=>{
  await browserFixture(async(clicks)=>{
    const fence={cookie:"csrf-pair",boundary:"new-session"},client=downloadClient(fence);let calls=0;
    globalThis.fetch=async()=>{calls++;throw Error("lost response");};
    assert.equal(await client.downloadReport(id,"products",from,to,"old-session",new AbortController().signal),"signed-out");
    assert.equal(calls,0);
    fence.boundary="old-session";
    assert.equal(await client.downloadReport(id,"products",from,to,"old-session",new AbortController().signal),"uncertain");
    assert.equal(calls,1);assert.deepEqual(clicks,[]);
  });
});

test("actual CSV client refuses stale-session bytes and downloads only a closed private response",async()=>{
  await browserFixture(async(clicks)=>{
    const fence={cookie:"csrf-pair",boundary:"session"},client=downloadClient(fence);let calls=0;
    const ok=()=>new Response("units\n1",{headers:{"Cache-Control":"private, no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":`attachment; filename="report-products-${from}-${to}.csv"`}});
    globalThis.fetch=async(_url,init)=>{calls++;assert.equal(init.method,"GET");assert.equal(init.headers["X-CSRF-Token"],"csrf-pair");fence.boundary="changed";return ok();};
    assert.equal(await client.downloadReport(id,"products",from,to,"session",new AbortController().signal),"uncertain");
    assert.equal(calls,1);assert.deepEqual(clicks,[]);
    fence.boundary="session";globalThis.fetch=async()=>{calls++;return ok();};
    assert.equal(await client.downloadReport(id,"products",from,to,"session",new AbortController().signal),"done");
    assert.deepEqual(clicks,[`report-products-${from}-${to}.csv`]);
    globalThis.fetch=async()=>Response.json({code:"forbidden"},{status:403});
    assert.equal(await client.downloadReport(id,"products",from,to,"session",new AbortController().signal),"forbidden");
  });
});

test("report leaf gives the actual trusted fetch 65 seconds, rejects redirects and forwards only server bearer",async()=>{
 const original=AbortSignal.timeout, budgets=[];
 AbortSignal.timeout=(ms)=>{budgets.push(ms);return new AbortController().signal;};
 try {
  const r=router({upstream:()=>Response.json({...head,truncated:false,rows:[]},{headers:{"Cache-Control":"private, no-store"}})});
  assert.equal((await r.send("products",`to=${to}&from=${from}`,{Authorization:"Bearer attacker","Cookie":"bad","X-Commerce-BFF-Key":"bad"})).status,200);
  assert.equal(r.fetchCalls.length,1);
  const [url,init]=r.fetchCalls[0];
  assert.equal(url,`https://go.example.invalid/v1/admin/stores/${id}/reports/products?to=${to}&from=${from}`);
  assert.deepEqual(budgets,[65000]);assert.equal(init.redirect,"error");assert.equal(init.cache,"no-store");
  assert.deepEqual(init.headers,{Authorization:"Bearer trusted-session",Accept:"application/json"});
 } finally {AbortSignal.timeout=original;}
});

test("incoming request cancellation aborts the real report fetch once, returns safe unavailable and never retries",async()=>{
 const controller=new AbortController();
 const r=router({upstream:async(_url,init)=>{
  queueMicrotask(()=>controller.abort());
  await new Promise((resolve,reject)=>init.signal.addEventListener("abort",()=>reject(Error("cancelled")),{once:true}));
 }});
 assert.equal((await r.send("products",`from=${from}&to=${to}`,{},controller.signal)).status,503);
 assert.equal(r.fetchCalls.length,1);assert.equal(r.fetchCalls[0][1].signal.aborted,true);
});

test("CSV final synchronous fence conceals bytes when cancellation, hide or cookie rotation happens inside final await",async()=>{
 for(const mode of ["abort","hidden","cookie"]) await browserFixture(async(clicks,urls)=>{
  const controller=new AbortController();let checks=0,calls=0;
  const fence={cookie:"csrf-pair",boundary:"session",check:async()=>{
   if(++checks===2){await Promise.resolve();if(mode==="abort")controller.abort();else if(mode==="hidden")document.visibilityState="hidden";else fence.cookie="rotated-cookie";}
   return "session";
  }};
  const client=downloadClient(fence);
  globalThis.fetch=async()=>{calls++;return new Response("units\n1",{headers:{"Cache-Control":"private, no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":`attachment; filename="report-products-${from}-${to}.csv"`}});};
  assert.equal(await client.downloadReport(id,"products",from,to,"session",controller.signal),"uncertain",mode);
  assert.deepEqual(clicks,[],mode);assert.deepEqual(urls,[],mode);assert.equal(calls,1,mode);
 });
});

test("CSV end-to-end deadline covers store lookup and report budget; aborted preflight sends no audited request",async()=>{
 await browserFixture(async(clicks)=>{
  const controller=new AbortController();let calls=0;
  const fence={cookie:"csrf-pair",boundary:"session",check:async()=>{controller.abort();return "session";}};
  globalThis.fetch=async()=>{calls++;throw Error("must not send");};
  assert.equal(await downloadClient(fence).downloadReport(id,"products",from,to,"session",controller.signal),"unavailable");
  assert.equal(calls,0);assert.deepEqual(clicks,[]);
  delete fence.check;
  const original=AbortSignal.timeout,budgets=[];
  AbortSignal.timeout=(ms)=>{budgets.push(ms);return new AbortController().signal;};
  try {
   globalThis.fetch=async()=>new Response("units\n1",{headers:{"Cache-Control":"private, no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":`attachment; filename="report-products-${from}-${to}.csv"`}});
   assert.equal(await downloadClient(fence).downloadReport(id,"products",from,to,"session",new AbortController().signal),"done");
   assert.deepEqual(budgets,[75000]);
  } finally {AbortSignal.timeout=original;}
 });
});

// Codex review P2 (PR #3): an uncertain export stays locked for its scope even after another tab exports (I06: no resubmission
// of a keyless audited GET that may already have committed); only a new server render clears it.
test("an uncertain export stays locked for its scope after another scope's export", () => {
  const client = downloadClient({cookie:"csrf-pair",boundary:"session"});
  const a = "render|en|store|2026-09-01|2026-09-30|products", b = "render|en|store|2026-09-01|2026-09-30|channels";
  const uncertain = [a];
  assert.equal(client.exportOutcome(a, {scope:b,outcome:"done"}, uncertain), "uncertain");
  assert.equal(client.exportOutcome(b, {scope:b,outcome:"done"}, uncertain), "done");
  assert.equal(client.exportOutcome(b, null, []), null);
  assert.equal(client.exportOutcome(a, {scope:a,outcome:"busy"}, []), "busy");
});

// Codex review P2 (PR #3): the UNKNOWN-export lock survives server renders and refresh (tab sessionStorage by store, no PII);
// storage failure keeps the in-memory behaviour instead of throwing.
function memoryStorage(seed = {}) {
  const data = new Map(Object.entries(seed));
  return { getItem: (k) => data.get(k) ?? null, setItem: (k, v) => { data.set(k, String(v)); }, data };
}
test("an uncertain export scope persists per store in tab storage and re-locks a fresh render", () => {
  const client = downloadClient({cookie:"csrf-pair",boundary:"session"});
  const storage = memoryStorage();
  const scope = `${id}|${from}|${to}|products`, other = `${id}|2026-09-02|${to}|products`;
  assert.deepEqual(client.loadUncertainScopes(id, storage), []);
  assert.deepEqual(client.rememberUncertainScope(id, scope, storage), [scope]);
  assert.deepEqual(client.rememberUncertainScope(id, scope, storage), [scope]);
  assert.deepEqual(client.rememberUncertainScope(id, other, storage), [scope, other]);
  // A new render (fresh workspace state) reads the stored set and keeps the same scope locked.
  assert.equal(client.exportOutcome(scope, null, client.loadUncertainScopes(id, storage)), "uncertain");
  assert.equal(client.exportOutcome(`${id}|${from}|${to}|channels`, null, client.loadUncertainScopes(id, storage)), null);
  assert.deepEqual(client.loadUncertainScopes("22222222-2222-4222-8222-222222222222", storage), []);
  for (const raw of ["not json", "{}", "[1,null]"]) assert.deepEqual(client.loadUncertainScopes(id, memoryStorage({[`lc.reports.uncertain.${id}`]: raw})), []);
  assert.doesNotMatch([...storage.data.values()].join(), /@|render/);
});
test("unavailable tab storage fails closed to the in-memory lock without throwing", () => {
  const client = downloadClient({cookie:"csrf-pair",boundary:"session"});
  const broken = { getItem() { throw new Error("denied"); }, setItem() { throw new Error("denied"); } };
  assert.deepEqual(client.loadUncertainScopes(id, broken), []);
  assert.deepEqual(client.rememberUncertainScope(id, "s", broken), ["s"]);
  assert.deepEqual(client.rememberUncertainScope(id, "s", null), ["s"]);
});
