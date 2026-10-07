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
  assert.equal(client.rememberUncertainScope(id, scope, storage), true);
  assert.equal(client.rememberUncertainScope(id, scope, storage), true);
  assert.deepEqual(client.loadUncertainScopes(id, storage), [scope]);
  assert.equal(client.rememberUncertainScope(id, other, storage), true);
  assert.deepEqual(client.loadUncertainScopes(id, storage), [scope, other]);
  // A new render (fresh workspace state) reads the stored set and keeps the same scope locked.
  assert.equal(client.exportOutcome(scope, null, client.loadUncertainScopes(id, storage)), "uncertain");
  assert.equal(client.exportOutcome(`${id}|${from}|${to}|channels`, null, client.loadUncertainScopes(id, storage)), null);
  assert.deepEqual(client.loadUncertainScopes("22222222-2222-4222-8222-222222222222", storage), []);
  // Corrupt storage reads as none (a fresh tab has no in-memory locks either).
  for (const raw of ["not json", "{}", "[1,null]"]) assert.deepEqual(downloadClient({cookie:"csrf-pair",boundary:"session"}).loadUncertainScopes(id, memoryStorage({[`lc.reports.uncertain.${id}`]: raw})), []);
  assert.doesNotMatch([...storage.data.values()].join(), /@|render/);
});
test("unavailable tab storage fails closed to the in-memory lock without throwing", () => {
  const client = downloadClient({cookie:"csrf-pair",boundary:"session"});
  const broken = { getItem() { throw new Error("denied"); }, setItem() { throw new Error("denied"); } };
  assert.deepEqual(client.loadUncertainScopes(id, broken), []);
  // Not persisted -> false (callers must not dispatch), yet the in-memory lock still holds for the tab.
  assert.equal(client.rememberUncertainScope(id, "s", broken), false);
  assert.equal(client.rememberUncertainScope(id, "t", null), false);
  assert.deepEqual(client.loadUncertainScopes(id, broken), ["s", "t"]);
});

// Codex review P2 (PR #3): the lock is PENDING before the audited GET dispatches, so a refresh/close mid-request (the page never
// sees a result) still counts as uncertain on the next mount; only a definitive outcome releases it.
const pendingKey = `lc.reports.uncertain.${id}`;
const csv = () => new Response("units\n1", {headers:{"Cache-Control":"private, no-store","Content-Type":"text/csv; charset=utf-8","Content-Disposition":`attachment; filename="report-products-${from}-${to}.csv"`}});
test("export lock is stored before dispatch; refused/delivered outcomes release it, uncertain and cut-short keep it", async () => {
  await browserFixture(async () => {
    const scope = `${id}|${from}|${to}|products`;
    const run = (client, storage) => client.exportWithLock(id, scope, () => client.downloadReport(id, "products", from, to, "session", new AbortController().signal), storage);
    const cases = [["done", () => csv(), false], ["forbidden", () => new Response("{}", {status:403}), false], ["not-found", () => new Response("{}", {status:404}), false],
      ["signed-out", () => new Response("{}", {status:401}), false], ["unavailable", () => new Response("{}", {status:422}), false],
      ["uncertain", () => new Response("{}", {status:503}), true], ["uncertain", () => { throw new Error("aborted by pagehide"); }, true]];
    for (const [expected, respond, locked] of cases) {
      const storage = memoryStorage(); const client = downloadClient({cookie:"csrf-pair",boundary:"session"}); let atDispatch = null;
      globalThis.fetch = async () => { atDispatch = storage.getItem(pendingKey); return respond(); };
      assert.equal(await run(client, storage), expected);
      assert.deepEqual(JSON.parse(atDispatch), [scope], `${expected}: pending lock must exist when the GET is dispatched`);
      assert.deepEqual(client.loadUncertainScopes(id, storage), locked ? [scope] : [], expected);
    }
    // Pre-dispatch refusal (no session cookie) never reached the server and is released too.
    const storage = memoryStorage(); const client = downloadClient({cookie:"",boundary:"session"});
    globalThis.fetch = async () => { throw new Error("must not send"); };
    assert.equal(await run(client, storage), "signed-out"); assert.deepEqual(client.loadUncertainScopes(id, storage), []);
    // Page goes away mid-request: the run never resolves, nothing releases the lock, and a fresh mount sees it as uncertain.
    const cut = memoryStorage(); const fresh = downloadClient({cookie:"csrf-pair",boundary:"session"});
    void fresh.exportWithLock(id, scope, () => new Promise(() => {}), cut);
    assert.equal(fresh.exportOutcome(scope, null, fresh.loadUncertainScopes(id, cut)), "uncertain");
  });
});
test("releasing one scope keeps the other uncertain scopes of the store", () => {
  const client = downloadClient({cookie:"csrf-pair",boundary:"session"}); const storage = memoryStorage();
  client.rememberUncertainScope(id, "a", storage); client.rememberUncertainScope(id, "b", storage);
  client.forgetUncertainScope(id, "a", storage); assert.deepEqual(client.loadUncertainScopes(id, storage), ["b"]);
  client.forgetUncertainScope(id, "a", { getItem() { throw new Error("denied"); }, setItem() { throw new Error("denied"); } });
});

// Codex review P2 (PR #3): if the lock cannot be persisted BEFORE dispatch, the audited GET is not sent at all.
test("export is refused and nothing is dispatched when the lock cannot be saved (denied, full or no storage)", async () => {
  await browserFixture(async (clicks) => {
    const scope = `${id}|${from}|${to}|products`;
    const denied = { getItem: () => null, setItem() { throw new DOMException("quota", "QuotaExceededError"); } };
    for (const storage of [denied, null]) {
      const client = downloadClient({cookie:"csrf-pair",boundary:"session"}); let sent = 0;
      globalThis.fetch = async () => { sent++; return csv(); };
      const result = await client.exportWithLock(id, scope, () => client.downloadReport(id, "products", from, to, "session", new AbortController().signal), storage);
      assert.equal(result, "lock-failed"); assert.equal(sent, 0, "no request may be sent without a persisted lock"); assert.deepEqual(clicks, []);
      assert.equal(client.exportOutcome(scope, {scope, outcome: result}, client.loadUncertainScopes(id, storage)), "lock-failed");
    }
  });
});
test("copy for an unsaved export lock exists in every locale", () => {
  for (const locale of ["en", "zh-CN", "zh-TW"]) assert.match(copy[locale].csvLockFailed, /\S/);
});

// Codex review P2 (PR #3): the product sort is encoded in the page URL, validated against the sortable fields.
test("product sort query parses strictly and defaults to net_minor descending", () => {
  const { parseProductSort, productSorts } = adminRequire("./lib/reports-presentation.ts");
  assert.deepEqual(parseProductSort(undefined, undefined), { sort: "net_minor", ascending: false });
  assert.deepEqual(parseProductSort("name", undefined), { sort: "name", ascending: true });
  assert.deepEqual(parseProductSort("units", undefined), { sort: "units", ascending: false });
  assert.deepEqual(parseProductSort("units", "asc"), { sort: "units", ascending: true });
  assert.deepEqual(parseProductSort(undefined, "asc"), { sort: "net_minor", ascending: true });
  for (const key of productSorts) for (const dir of ["asc", "desc"]) assert.deepEqual(parseProductSort(key, dir), { sort: key, ascending: dir === "asc" });
  for (const [sort, dir] of [["", undefined], ["Units", "asc"], ["sku_id", "asc"], ["__proto__", "asc"], ["units", ""], ["units", "ASC"], ["units", "up"]]) assert.equal(parseProductSort(sort, dir), null, `${sort}/${dir}`);
});
