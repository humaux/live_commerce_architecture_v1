// Exercises the storefront gate's fail-closed WebKit pageerror boundary without network traffic.
import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";

process.env.LC_BROWSER_ENGINE = "webkit";
const { isOwnRscPageError } = await import("./c7-rsc-pageerror.mjs");
const origin = "https://shop.example";
const signature = (message, name = "Fetch API cannot load https") => ({ name, message });
const good = signature("/shop.example/en/search?_rsc=abc123 due to access control checks.");
const errors = [
  ["uncaught TypeError", { name: "TypeError", message: "Unexpected application exception" }],
  ["wrong name", signature(good.message, "Error")],
  ["foreign origin", signature("/evil.example/en/search?_rsc=abc123 due to access control checks.")],
  ["non-RSC API", signature("/shop.example/api/buyer/contact due to access control checks.")],
  ["API with RSC query", signature("/shop.example/api/buyer/contact?_rsc=abc123 due to access control checks.")],
  ["unmatched error", signature("/shop.example/en/search?_rsc=abc123 different error.")],
  ["other query data", signature("/shop.example/en/search?other=value&_rsc=abc123 due to access control checks.")],
  ["preview query", signature("/shop.example/en/search?preview=fixture&_rsc=abc123 due to access control checks.")],
  ["different port", signature("/shop.example:8443/en/search?_rsc=abc123 due to access control checks.")],
];
test("recognizes only the exact same-origin RSC WebKit signature", () => assert.equal(isOwnRscPageError(good, origin), true));
for (const [name, error] of errors) test(`${name} remains an unexpected error and fails the gate`, () => {
  const unexpected = [];
  if (!isOwnRscPageError(error, origin)) unexpected.push(error);
  assert.throws(() => assert.deepEqual(unexpected, []));
});
test("Chromium never classifies the WebKit signature", () => {
  const run = spawnSync(process.execPath, ["--input-type=module", "-e", `import { isOwnRscPageError } from './apps/storefront/tests/c7-rsc-pageerror.mjs'; if (isOwnRscPageError(${JSON.stringify(good)}, ${JSON.stringify(origin)})) process.exit(1);`], { cwd: process.cwd(), env: { ...process.env, LC_BROWSER_ENGINE: "chromium" }, encoding: "utf8" });
  assert.equal(run.status, 0, run.stderr);
});

test("actual storefront gate admits every pageerror into the zero-error assertion", async () => {
  const { readFile } = await import("node:fs/promises");
  const vm = await import("node:vm");
  const source = await readFile("tests/storefront/shop-real-gate.mjs", "utf8");
  const begin = source.indexOf('  p.on("pageerror", (e) => {');
  const end = source.indexOf("  if (rscDiagnostic) {", begin);
  assert.ok(begin >= 0 && end > begin, "actual pageerror listener must be present");
  let listener;
  const problems = [];
  vm.runInNewContext(source.slice(begin, end), {
    p: { on: (event, callback) => { assert.equal(event, "pageerror"); listener = callback; } },
    problems, safeObserver: null, isOwnRscPageError, origin, recognizedRscPageErrors: [],
    observedUrl: raw => { const u = new URL(raw); return { host: u.host, path: u.pathname, rsc: true }; },
  });
  for (const [label, error] of [["matching same-origin RSC signature", good], ...errors]) {
    const before = problems.length;
    listener(error);
    assert.equal(problems.length, before + 1, `${label} was omitted from the actual gate`);
  }
  assert.throws(() => assert.deepEqual(problems, []), "unchanged zero-error assertion must fail");
});

test("actual storefront gate retains hydration warning failures", async () => {
  const { readFile } = await import("node:fs/promises");
  const vm = await import("node:vm");
  const source = await readFile("tests/storefront/shop-real-gate.mjs", "utf8");
  const line = source.split("\n").find(x => x.includes('p.on("console", (m) =>') && x.includes("problems.push"));
  assert.ok(line, "actual hydration console listener must be present");
  let listener; const problems = [];
  vm.runInNewContext(line, { p: { on: (_, cb) => { listener = cb; } }, problems, safeObserver: null });
  listener({ type: () => "error", text: () => "Warning: hydration mismatch" });
  assert.equal(problems.length, 1);
  assert.throws(() => assert.deepEqual(problems, []));
});

// Execute the actual observation boundary; hostile inputs must never reach serialized evidence.
test("C7 observer safe document shapes and bounded identities reject hostile data", async () => {
  const { readFile } = await import("node:fs/promises"); const vm = await import("node:vm");
  const source = await readFile("tests/storefront/shop-real-gate.mjs", "utf8");
  const code = source.slice(source.indexOf("function createSafeObserver("), source.indexOf("// END C7_SAFE_OBSERVER"));
  const observer = vm.runInNewContext(`${code}; createSafeObserver("https://shop.example")`, { URL });
  const secret = "UNIQUE_PRIVATE_7f193", raw = `https://shop.example/en/products/${secret}?preview=${secret}&_rsc=${secret}&${secret}=${secret}`;
  const target = observer.identify(raw); observer.attach(target, 1); observer.attach(target, 2);
  const events = [target, observer.document(raw), observer.document(`https://${secret}.example/${secret}`), observer.document("about:blank"), observer.error({ name: secret, message: secret }), observer.error({ name: "Fetch API cannot load https", message: `/shop.example/en/products/${secret}?_rsc=${secret} due to access control checks.` }), observer.fail(secret)];
  assert.equal(observer.document(raw).same_origin, true);
  assert.equal(observer.document(`https://${secret}.example/${secret}`).same_origin, false);
  assert.equal(observer.document("about:blank").same_origin, "UNKNOWN");
  assert.equal(JSON.stringify(events).includes(secret), false);
  assert.deepEqual(Object.keys(target).sort(), ["path", "query_names", "url_id"]);
  assert.equal(observer.identify(`https://user:${secret}@shop.example/en?_rsc=x`), null);
  assert.equal(observer.error({ name: "TypeError", message: secret }).message, "[redacted]");
  for (let i = 0; i < 3000; i++) observer.identify(`https://shop.example/en?_rsc=${i}`);
  assert.equal(observer.identify("https://shop.example/en?_rsc=overflow"), null);
});

for (const enabled of [false, true]) test(`actual serialized errors stay private with observer ${enabled ? 'on' : 'off'}`, async () => {
  const { readFile } = await import('node:fs/promises'); const vm = await import('node:vm');
  const source = await readFile('tests/storefront/shop-real-gate.mjs', 'utf8');
  const code = source.slice(source.indexOf('function createSafeObserver('), source.indexOf('// END C7_SAFE_OBSERVER'));
  const observer = vm.runInNewContext(`${code}; createSafeObserver('https://shop.example')`, { URL });
  const begin = source.indexOf('  p.on("pageerror", (e) => {');
  const end = source.indexOf('  if (rscDiagnostic) {', begin);
  const line = source.split('\n').find(x => x.includes('p.on("console", (m) =>') && x.includes('problems.push'));
  const listeners = {}, problems = [];
  vm.runInNewContext(source.slice(begin, end) + '\n' + line, {
    p: { on: (event, cb) => { listeners[event] = cb; } }, problems, safeObserver: enabled ? observer : null,
  });
  const canary = 'PRIVATE_C7_WIRING_8720';
  const messages = [canary, `/shop.example/en/products/${canary}?_rsc=${canary} due to access control checks.`,
    `https://shop.example/${canary}?preview=${canary}#${canary}`, `/en/${canary}?q=${canary}`, `Warning: hydration ${canary}`];
  for (const message of messages) {
    listeners.pageerror({ name: canary, message });
    listeners.console({ type: () => 'error', text: () => `Warning: hydration ${message}` });
  }
  assert.equal(problems.length, messages.length * 2, 'every unexpected error remains counted');
  // Read the actual strict assertion, not a substitute success condition.
  const strict = source.split('\n').find(x => x.includes('assert.deepEqual(problems, []'));
  assert.ok(strict);
  let failure;
  try { vm.runInNewContext(strict, { assert, problems }); } catch (e) { failure = e; }
  assert.ok(failure, 'real strict gate must fail');
  const serialized = JSON.stringify(problems) + String(failure);
  assert.equal(serialized.includes(canary), false, 'serialized diagnostics leaked a synthetic canary');
});

// Exercise the actual registration, actual diagnosticEvent and actual final serialization; no browser/PG.
async function actualWebErrorListeners(enabled = true, catalogOptions = null) {
  const { readFile } = await import('node:fs/promises'); const vm = await import('node:vm');
  const source = await readFile('tests/storefront/shop-real-gate.mjs', 'utf8');
  let assetCatalog = null;
  if (enabled && catalogOptions && source.includes('createClientAssetCatalog')) { const mod = await import('../../../tests/storefront/c7-asset-catalog.mjs'); assetCatalog = mod.createClientAssetCatalog(catalogOptions); }
  const helper = source.slice(source.indexOf('function createSafeObserver('), source.indexOf('// END C7_SAFE_OBSERVER'));
  const state = source.slice(source.indexOf('const safeObserver ='), source.indexOf('let browser, edge,'));
  const start = source.indexOf('  const pc = await phone()');
  const registration = source.slice(start, source.indexOf('  // ---- SFR01', start));
  const serialization = source.split('\n').find(x => x.includes('const events = rscDiagnostic.map'));
  const finish = source.split('\n').find(x => x.trim() === 'assetCatalog?.finish();') ?? '';
  const contextListeners = {}, pageListeners = {}, order = [];
  let url = 'https://shop.example/en';
  const page = { url: () => typeof url === 'function' ? url() : url, on: (name, cb) => { (pageListeners[name] ??= []).push(cb); }, mainFrame: () => page };
  const context = { on: (name, cb) => { order.push(name); (contextListeners[name] ??= []).push(cb); }, newPage: async () => { order.push('newPage'); return page; } };
  const run = await vm.runInNewContext(`(async () => { ${helper}\n${state}\n${registration}\nreturn {problems, observer:safeObserver, serialize:()=>{if(!rscDiagnostic)return '[]'; ${finish} ${serialization}; return JSON.stringify(events);}}; })()`, {
    URL, origin, assetCatalog, rscDiagnostic: enabled ? [] : null, observedAt: () => 1, phone: async () => context,
  });
  return { ...run, page, order, setURL: value => { url = value; },
    web: event => { for (const cb of contextListeners.weberror ?? []) cb(event); },
    error: error => { for (const cb of pageListeners.pageerror ?? []) cb(error); },
    events: () => JSON.parse(run.serialize()).filter(e => e.event === 'pageerror'),
  };
}
const publicError = (page, error, location = { url: '', line: 0, column: 0 }) => ({ page: () => page, error: () => error, location: () => location });

test('actual public WebError bridge shares same-object metadata and registers before page creation', async () => {
  const h = await actualWebErrorListeners();
  const e = new Error('same text'); e.stack = 'Error\n    at synthetic (native)';
  h.web(publicError(h.page, e, { url: '/_next/static/chunks/synthetic.js', line: 0, column: 0 }));
  h.web(publicError({}, e, {url:'native',line:9,column:9})); // Cross-page delivery cannot overwrite the main-page snapshot.
  h.setURL('https://shop.example/zh-CN/cart'); h.error(e); h.error(e);
  const [a,b] = h.events();
  assert.equal(a.web_error_available, 'PRESENT');
  assert.equal(a.error_id, b.error_id); assert.equal(typeof a.error_id, 'number');
  assert.equal(a.page_id, 1); assert.equal(typeof a.document_delivery_id, 'number');
  assert.equal(a.web_error_document.shape, '/en');
  assert.equal(a.location_category, 'app-static');
  assert.equal(a.location_line, 0); assert.equal(a.location_column, 0);
  assert.equal(a.coordinate_basis, 'public-zero-based'); assert.equal(a.zero_coordinate_ambiguity, true);
  assert.equal(a.stack_present, true); assert.equal(a.frame_present, true);
  assert.equal(a.source_asset_id, 'UNKNOWN'); assert.equal(a.source_frame, 'UNKNOWN');
  assert.equal(h.order.indexOf('weberror') < h.order.indexOf('newPage'), true);
  assert.equal(h.problems.length, 2); assert.equal(a.exact_request_id, 'UNKNOWN');
});

test('distinct same-text Errors and later URL candidates never become one error identity', async () => {
  const h = await actualWebErrorListeners(); const a = signature(good.message), b = signature(good.message);
  h.web(publicError(h.page,a)); h.web(publicError(h.page,b)); h.error(a); h.error(b);
  const es = h.events(); assert.equal(typeof es[0].error_id,'number'); assert.equal(es[0].error_id === es[1].error_id,false);
  const id = es[0].error_id;
  const unjoined=signature(good.message); h.error(unjoined);
  const before=h.events()[2].error_id;
  h.observer.attach(h.observer.identify('https://shop.example/en/search?_rsc=abc123'), 99);
  h.error(unjoined);
  assert.equal(h.events()[0].error_id,id); assert.equal(h.events()[0].exact_request_id,'UNKNOWN');
  assert.equal(h.events()[3].error_id,before); assert.equal(h.events()[3].web_error_available,'UNKNOWN');
  assert.equal(h.events()[3].candidate_request_ids.includes(99),true); assert.equal(h.events()[3].exact_request_id,'UNKNOWN');
  assert.equal(h.problems.length,4);
});

test('cross-page, absent and throwing public metadata retain UNKNOWN without hiding pageerrors', async () => {
  const h = await actualWebErrorListeners();
  const cases = [
    e => h.web(publicError({},e)), e => h.web(publicError(null,e)),
    e => h.web({page:()=>{throw new Error('private');},error:()=>e}),
    e => h.web({page:()=>h.page,error:()=>{throw new Error('private');}}),
    () => {},
  ];
  for (const send of cases) { const e = new Error('same text'); send(e); h.error(e); }
  for (const e of h.events()) { assert.equal(e.web_error_available,'UNKNOWN'); assert.equal(e.location_category,'UNKNOWN'); }
  const bad = {};
  for (const key of ['name','message','stack']) Object.defineProperty(bad,key,{get(){throw new Error('private');}});
  let threw = false;
  try { h.web({page:()=>h.page,error:()=>bad,location:()=>{throw new Error('private');}}); h.error(bad); } catch { threw=true; }
  assert.equal(threw,false,'diagnostics must not throw'); assert.equal(h.problems.length,6);
  const last=h.events().at(-1); assert.equal(last.stack_present,'UNKNOWN'); assert.equal(last.location_category,'UNKNOWN');
});

test('missing location defaults stay UNKNOWN; legitimate zero-based API values stay observable', async () => {
  const h=await actualWebErrorListeners();
  const fixtures=[
    [{url:'',line:0,column:0},'empty','UNKNOWN','UNKNOWN'],
    [undefined,'UNKNOWN','UNKNOWN','UNKNOWN'],
    [{url:'native',line:0,column:0},'native',0,0],
    [{url:'/source.js',line:3,column:4},'other',3,4],
    [{url:'/source.js',line:-1,column:0},'other','UNKNOWN','UNKNOWN'],
    [{url:'/source.js',line:NaN,column:1},'other','UNKNOWN','UNKNOWN'],
  ];
  for(const [location,category,line,column] of fixtures){const e=new Error('same');e.stack='';h.web({page:()=>h.page,error:()=>e,location:()=>location});h.error(e);const r=h.events().at(-1);assert.equal(r.location_category,category);assert.equal(r.location_line,line);assert.equal(r.location_column,column);assert.equal(r.frame_present,false);assert.equal(r.source_frame,'UNKNOWN');}
});

test('hostile source URL, stack and function text never serialize; observer off remains passive', async () => {
  const h=await actualWebErrorListeners(), canary='PRIVATE_WEBERROR_10479';
  const e={name:canary,message:canary,stack:`${canary}\n    at ${canary} (https://shop.example/${canary}?${canary}#${canary})`};
  h.setURL(`https://shop.example/en/products/${canary}?${canary}#${canary}`);
  h.web(publicError(h.page,e,{url:`https://shop.example/_next/static/${canary}?${canary}#${canary}`,line:0,column:1}));h.error(e);
  assert.equal(h.serialize().includes(canary),false);assert.equal(JSON.stringify(h.problems).includes(canary),false);
  const off=await actualWebErrorListeners(false);off.web(publicError(off.page,e));off.error(e);
  assert.equal(off.order.includes('weberror'),false);assert.equal(off.problems.length,1);assert.equal(off.serialize(),'[]');
});


test('throwing location properties and page URL leave availability UNKNOWN and preserve the original error', async () => {
  const h=await actualWebErrorListeners(); const e=new Error('same'); e.stack='';
  const location={}; for(const key of ['url','line','column']) Object.defineProperty(location,key,{get(){throw new Error('private');}});
  h.setURL(()=>{throw new Error('private');});
  let threw=false;try{h.web(publicError(h.page,e,location));h.error(e);}catch{threw=true;}
  assert.equal(threw,false);assert.equal(h.problems.length,1);
  const event=h.events()[0]; assert.equal(event.web_error_available,'PRESENT');
  assert.equal(event.document_available,'UNKNOWN');assert.equal(event.document_identity,'UNKNOWN');
  assert.equal(event.location_category,'UNKNOWN');assert.equal(event.location_line,'UNKNOWN');assert.equal(event.location_column,'UNKNOWN');
});


async function assetFixture(run) {
  const fs = await import('node:fs/promises'); const path = await import('node:path');
  const base=path.resolve('output/c7-observer-wiring-20261107/asset-catalog'); await fs.mkdir(base,{recursive:true});
  const dir=await fs.mkdtemp(path.join(base,'fixture-'));
  const appRoot=path.join(dir,'app'), evidenceRoot=path.join(dir,'evidence');
  await fs.mkdir(path.join(appRoot,'.next','static','chunks'),{recursive:true}); await fs.mkdir(evidenceRoot);
  await fs.writeFile(path.join(appRoot,'.next','BUILD_ID'),'synthetic-build-1');
  await fs.writeFile(path.join(appRoot,'.next','static','chunks','one.js'),'// inert fixture one\nvoid 0;\n');
  await fs.writeFile(path.join(appRoot,'.next','static','chunks','two.js'),'// inert fixture two\nvoid 1;\n');
  try { return await run({fs,path,dir,appRoot,evidenceRoot,origin}); } finally { await fs.rm(dir,{recursive:true,force:true}); }
}

test('actual handler matches same-consuming fixture asset and retains exact recoverable bytes', async () => assetFixture(async f => {
  const h=await actualWebErrorListeners(true,f); const e=new Error('same');
  h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);
  const event=h.events()[0]; assert.equal(event.asset_match?.status,'MATCHED');
  assert.equal(event.source_asset_id,event.asset_match.asset_id); assert.equal(event.source_frame,'UNKNOWN');
  const raw=await f.fs.readFile(f.path.join(f.evidenceRoot,event.asset_match.evidence_path));
  const {createHash}=await import('node:crypto'); assert.equal(createHash('sha256').update(raw).digest('hex'),event.asset_match.content_sha256);
  assert.equal(raw.equals(await f.fs.readFile(f.path.join(f.appRoot,'.next/static/chunks/one.js'))),true);
  const manifest=JSON.parse(await f.fs.readFile(f.path.join(f.evidenceRoot,'c7-client-assets/manifest.json'),'utf8'));
  assert.equal(manifest.matches[0].error_id,event.error_id); assert.equal(manifest.matches[0].asset_id,event.source_asset_id);
  assert.equal(h.problems.length,1); assert.equal(event.exact_request_id,'UNKNOWN');
}));

test('same coordinates on two catalog assets retain distinct identities and unit uncertainty', async () => assetFixture(async f=>{
  const h=await actualWebErrorListeners(true,f);
  for(const name of ['one','two']){const e=new Error('identical');h.web(publicError(h.page,e,{url:origin+`/_next/static/chunks/${name}.js`,line:0,column:1}));h.error(e);}
  const es=h.events();assert.equal(es[0].asset_match.status,'MATCHED');assert.equal(es[1].asset_match.status,'MATCHED');
  assert.equal(es[0].source_asset_id===es[1].source_asset_id,false);
  assert.equal(es[0].asset_match.coordinate_units,'UNKNOWN');assert.equal(es[0].zero_coordinate_ambiguity,true);
  const m=JSON.parse(await f.fs.readFile(f.path.join(f.evidenceRoot,'c7-client-assets/manifest.json'),'utf8'));
  assert.equal(m.sealed,true);assert.equal(m.matches.length,2);assert.equal(h.problems.length,2);
}));

for(const [label,url] of [
 ['foreign','https://else.example/_next/static/chunks/one.js'],['credentials','https://a:b@shop.example/_next/static/chunks/one.js'],
 ['query',origin+'/_next/static/chunks/one.js?PRIVATE_ASSET_908'],['fragment',origin+'/_next/static/chunks/one.js#PRIVATE_ASSET_908'],
 ['empty query',origin+'/_next/static/chunks/one.js?'],['traversal',origin+'/_next/static/chunks/../chunks/one.js'],
 ['encoded dot',origin+'/_next/static/chunks/%2e%2e/chunks/one.js'],['encoded alias',origin+'/_next/static/chunks/%6Fne.js'],
 ['encoded slash',origin+'/_next/static/chunks%2Fone.js'],['NUL',origin+'/_next/static/chunks/one.js\0'],
 ['missing',origin+'/_next/static/chunks/missing.js'],['port alias','https://shop.example:443/_next/static/chunks/one.js'],
]) test(`catalog rejects ${label} through actual handler without private serialization`,async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,f),e={name:'PRIVATE_ASSET_908',message:'PRIVATE_ASSET_908',stack:'PRIVATE_ASSET_908\n    at PRIVATE_ASSET_908'};
 h.web(publicError(h.page,e,{url,line:0,column:1}));h.error(e);const event=h.events()[0];
 assert.equal(event.asset_match.status,'UNKNOWN');assert.equal(event.source_asset_id,'UNKNOWN');assert.equal(h.problems.length,1);
 assert.equal(h.serialize().includes('PRIVATE_ASSET_908'),false);
}));

test('official per-segment encoding admits literal space bracket Unicode and percent filenames only canonically',async()=>assetFixture(async f=>{
 const name='sp ace[é]%.js';await f.fs.writeFile(f.path.join(f.appRoot,'.next/static/chunks',name),'// inert encoded fixture\n');
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/'+encodeURIComponent(name),line:0,column:0}));h.error(e);
 const a=h.events()[0];assert.equal(a.asset_match.status,'MATCHED');assert.equal(a.asset_match.coordinate_units,'UNKNOWN');
 const manifest=await f.fs.readFile(f.path.join(f.evidenceRoot,'c7-client-assets/manifest.json'),'utf8');assert.equal(manifest.includes(name),false);
}));

for(const [label,limits] of [['filecount',{files:1}],['entrycount',{entries:1}],['individual bytes',{fileBytes:1}],['total bytes',{totalBytes:1}],['path length',{pathBytes:1}]])
 test(`incomplete ${label} catalog fails closed with no partial COMPLETE`,async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,{...f,limits}),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);
 assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal(h.events()[0].asset_match.status,'UNKNOWN');assert.equal(h.problems.length,1);
 }));

for(const kind of ['build-id','source-bytes','missing-build','symlink-file','symlink-tree','fifo','duplicate-next-alias'])
 test(`catalog rejects ${kind} rather than mixing epochs or escaping`,async()=>assetFixture(async f=>{
 const staticDir=f.path.join(f.appRoot,'.next/static'),one=f.path.join(staticDir,'chunks/one.js');
 if(kind==='missing-build')await f.fs.unlink(f.path.join(f.appRoot,'.next/BUILD_ID'));
 if(kind==='symlink-file'){await f.fs.unlink(one);await f.fs.symlink(f.path.join(f.dir,'outside.js'),one);await f.fs.writeFile(f.path.join(f.dir,'outside.js'),'// outside inert');}
 if(kind==='symlink-tree'){await f.fs.rename(staticDir,f.path.join(f.dir,'outside-static'));await f.fs.symlink(f.path.join(f.dir,'outside-static'),staticDir);}
 if(kind==='fifo'){const {execFileSync}=await import('node:child_process');execFileSync('mkfifo',[f.path.join(staticDir,'pipe')]);}
 if(kind==='duplicate-next-alias'){
  const {normalizePathSep}=await import('../../../apps/storefront/node_modules/next/dist/shared/lib/page-path/normalize-path-sep.js');
  const {encodeURIPath}=await import('../../../apps/storefront/node_modules/next/dist/shared/lib/encode-uri-path.js');
  assert.equal(encodeURIPath(normalizePathSep('chunks\\one.js')),encodeURIPath(normalizePathSep('chunks/one.js')));
  await f.fs.writeFile(f.path.join(staticDir,'chunks\\one.js'),'// alias inert');
 }
 const h=await actualWebErrorListeners(true,f);
 if(kind==='build-id')await f.fs.writeFile(f.path.join(f.appRoot,'.next/BUILD_ID'),'different-build');
 if(kind==='source-bytes')await f.fs.writeFile(one,'// changed fixture bytes\n');
 const e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);
 assert.equal(h.events()[0].asset_match.status,'UNKNOWN');assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal(h.problems.length,1);
}));

test('late epoch/file change invalidates earlier success in final serialization and manifest',async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);
 await f.fs.writeFile(f.path.join(f.appRoot,'.next/static/chunks/two.js'),'// changed later\n');
 assert.equal(h.events()[0].source_asset_id,'UNKNOWN');
 const m=JSON.parse(await f.fs.readFile(f.path.join(f.evidenceRoot,'c7-client-assets/manifest.json'),'utf8'));assert.equal(m.status,'UNKNOWN');assert.equal(m.sealed,false);
}));

for(const [line,column] of [[-1,0],[NaN,0],[Infinity,0],[0,-1],[0,1.5],[99,0],[0,99999]])
 test(`invalid or out-of-representation coordinate pair remains UNKNOWN (${String(line)},${String(column)})`,async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line,column}));h.error(e);
 assert.equal(h.events()[0].asset_match.status,'UNKNOWN');assert.equal(h.problems.length,1);
 }));

test('retained-byte cap and preexisting destination conflict never claim success',async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,{...f,limits:{retainedBytes:1}}),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);assert.equal(h.events()[0].asset_match.status,'UNKNOWN');
 assert.equal((await f.fs.readdir(f.path.join(f.evidenceRoot,'c7-client-assets'))).some(x=>x.endsWith('.js')),false);
}));

test('unsafe evidence destination is not followed or overwritten',async()=>assetFixture(async f=>{
 const dir=f.path.join(f.evidenceRoot,'c7-client-assets');await f.fs.mkdir(dir);await f.fs.writeFile(f.path.join(dir,'KEEP'),'KEEP');
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);
 assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal((await f.fs.readFile(f.path.join(dir,'KEEP'),'utf8'))==='KEEP',true);
}));

test('tampered retained bytes invalidate event identity without overwriting the file',async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);const event=h.events()[0];
 const file=f.path.join(f.evidenceRoot,event.asset_match.evidence_path);await f.fs.writeFile(file,'KEEP');assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal((await f.fs.readFile(file,'utf8'))==='KEEP',true);
}));

test('only matched JS is retained; maps HTML server and facts never become source artifacts',async()=>assetFixture(async f=>{
 const secret='PRIVATE_ASSET_908';await f.fs.mkdir(f.path.join(f.appRoot,'.next/server'));await f.fs.writeFile(f.path.join(f.appRoot,'.next/server/private.js'),secret);
 for(const name of ['one.js.map','index.html','facts.json'])await f.fs.writeFile(f.path.join(f.appRoot,'.next/static',name),secret);
 const h=await actualWebErrorListeners(true,f),e=new Error(secret);h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);h.events();
 const names=await f.fs.readdir(f.path.join(f.evidenceRoot,'c7-client-assets'));assert.equal(names.filter(x=>x.endsWith('.js')).length,1);
 assert.equal(h.serialize().includes(secret),false);assert.equal((await f.fs.readFile(f.path.join(f.evidenceRoot,'c7-client-assets/manifest.json'),'utf8')).includes(secret),false);
}));

test('actual observer-off catalog initializer performs zero build/evidence IO',async()=>{
 const {readFile}=await import('node:fs/promises'),vm=await import('node:vm');const source=await readFile('tests/storefront/shop-real-gate.mjs','utf8');
 const init=source.split('\n').find(x=>x.startsWith('const assetCatalog ='));assert.ok(init);let calls=0;
 await vm.runInNewContext(`(async()=>{${init}})()`,{rscDiagnostic:null},{importModuleDynamically:()=>{calls++;throw new Error('unexpected IO');}});
 assert.equal(calls,0);
});

test('evidence symlink is rejected without following destination',async()=>assetFixture(async f=>{
 const real=f.path.join(f.dir,'real-evidence');await f.fs.rename(f.evidenceRoot,real);await f.fs.symlink(real,f.evidenceRoot);
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);
 assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal((await f.fs.readdir(real)).length,0);
}));

test('retained symlink and conflicting manifest are never overwritten',async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);const a=h.events()[0];
 const file=f.path.join(f.evidenceRoot,a.asset_match.evidence_path),outside=f.path.join(f.dir,'KEEP.js');await f.fs.writeFile(outside,'KEEP');await f.fs.unlink(file);await f.fs.symlink(outside,file);
 assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal((await f.fs.readFile(outside,'utf8'))==='KEEP',true);
}));

test('manifest conflict is preserved and cannot yield a MATCHED final event',async()=>assetFixture(async f=>{
 const h=await actualWebErrorListeners(true,f),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);h.events();
 const file=f.path.join(f.evidenceRoot,'c7-client-assets/manifest.json');await f.fs.writeFile(file,'KEEP');assert.equal(h.events()[0].source_asset_id,'UNKNOWN');assert.equal((await f.fs.readFile(file,'utf8'))==='KEEP',true);
}));

test('depth limit rejects incomplete enumeration',async()=>assetFixture(async f=>{
 await f.fs.mkdir(f.path.join(f.appRoot,'.next/static/chunks/deeper'));const h=await actualWebErrorListeners(true,{...f,limits:{depth:1}}),e=new Error('same');h.web(publicError(h.page,e,{url:origin+'/_next/static/chunks/one.js',line:0,column:1}));h.error(e);assert.equal(h.events()[0].source_asset_id,'UNKNOWN');
}));
