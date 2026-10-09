// Purpose: run actual LC-U2a hooks/composer with fake timers and privacy counterexamples (MOCK).
// Depends on: inbox real-source hook host, production comment hook/reply component and Node timers.
// Used by: test-node; complements real Go/PG browser gates rather than replacing them.
import test from "node:test";
import assert from "node:assert/strict";
const stored = new Map<string, string>();
Object.assign(globalThis, {
  sessionStorage: {
    getItem: (k: string) => stored.get(k) ?? null,
    setItem: (k: string, v: string) => stored.set(k, v),
    removeItem: (k: string) => stored.delete(k),
  },
});
test.beforeEach(() => stored.clear());
import {
  environment,
  store,
  response,
  nodes,
  node,
  textOf,
  BuyerPanel,
} from "./inbox-review-host.test.ts";
const { useCommentStream } =
  await import("../../apps/admin/src/features/live/use-comment-stream.ts");
const { CommentReply } =
  await import("../../apps/admin/components/CommentReply.tsx");
const { CommentStream } =
  await import("../../apps/admin/components/CommentStream.tsx");
const { commentCopy } =
  await import("../../apps/admin/src/features/live/comment-copy.ts");
const { CommentReceipt } =
  await import("../../apps/admin/src/features/live/comment-receipt.ts");
const sid = "22222222-2222-4222-8222-222222222222";
test("PR18 final public unresolved receipt survives relogin but coarse fence stays session-bound",()=>{
  const saved=new Map<string,string>();
  const storage={getItem:(key:string)=>saved.get(key)??null,setItem:(key:string,value:string)=>saved.set(key,value),removeItem:(key:string)=>saved.delete(key)};
  const publicBefore=new CommentReceipt(storage,"store-a",sid,"boundary-a","123_456");
  assert.equal(publicBefore.arm(),true);
  const publicAfter=new CommentReceipt(storage,"store-a",sid,"boundary-b","123_456");
  assert.equal(publicAfter.blocked(),true,"relogin must not turn UNKNOWN into permission to resend");
  assert.equal(publicAfter.arm(),false);
  for(const [shop,session,ref] of [["store-b",sid,"123_456"],["store-a","other-session","123_456"],["store-a",sid,"123_457"]])
    assert.equal(new CommentReceipt(storage,shop,session,"boundary-b",ref).blocked(),false,"public guards stay scope/ref isolated");
  assert.equal(new CommentReceipt(storage,"store-a",sid,"boundary-a").arm(),true);
  assert.equal(new CommentReceipt(storage,"store-a",sid,"boundary-b").blocked(),false,"coarse session semantics unchanged");
  assert.equal(publicAfter.clear(),true);
  assert.equal(publicBefore.blocked(),false,"terminal/verified clears the shared ref guard");
});
test("PR18 enabled downgrade gates hook output before passive cleanup",async t=>{
  const env=environment(t);let enabled=true;
  globalThis.fetch=async()=>response(page());
  const h=env.mount(()=>useCommentStream(store.id,sid,enabled));await h.settle();
  h.output.select({ref:row.ref});h.flush();assert.equal(h.output.buffer.items.length,1);
  // Hold queued passive effects, not the real render: inspect the first revoked commit.
  const splice=h.effects.splice;h.effects.splice=(()=>[]) as typeof splice;
  try {
    enabled=false;h.dirty=true;h.flush();
    assert.equal(h.output.buffer.items.length,0,"revoked hook cannot expose cached comments");
    assert.equal(h.output.selection,null,"revoked hook cannot expose selected buyer");
    assert.equal(h.output.busy,false);
  } finally {h.effects.splice=splice;h.dirty=true;await h.settle();}
});
for(const view of ["all","private"] as const)test(`PR18 enabled downgrade first ${view} render hides rows buyer and composer`,async t=>{
  const env=environment(t);
  let currentStore={...store,role:"staff",permissions:["live:read","inbox:read","inbox:reply"]};
  const claim={status:"ACCEPTED",reason:null,offer_id:sid,keyword:"A1",quantity:1,bundle_id:sid};
  globalThis.fetch=async input=>{
    const url=String(input);
    if(url.includes("buyer-panel?"))return response({display_name:"PRIVATE_PANEL",platform:"facebook",purchase_ordinal:1,claims:[],claim_total_minor:0,orders:[],link_pending_manual:false});
    if(url.includes("message-templates"))return response({items:[]});
    if(url.includes("inbox/conversations"))return response({items:[{conversation_id:sid,bundle_id:sid,platform:"facebook",display_name:"PRIVATE_CONVERSATION",last_at:row.created_at,unreplied:true}],next_cursor:"",unread_total:1});
    return response(page(1,[{...row,marks:{...row.marks,claim}}]));
  };
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const h=env.mount(()=>CommentStream({store:currentStore,session:sid,locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:cap,reply_public:cap}}} as any));await h.settle();
  if(view==="private"){
    node(h,n=>n.props["data-testid"]==="comment-filter-private").props.onClick();await h.settle();
    node(h,n=>n.type==="button"&&textOf(n).includes("PRIVATE_CONVERSATION")).props.onClick();
  }else node(h,n=>n.props["data-testid"]===`comment-select-${row.ref}`).props.onClick();
  await h.settle();assert.ok(textOf(h.output).includes("PRIVATE_PANEL"));
  if(view==="all")assert.ok(nodes(h.output).some(n=>n.props["data-testid"]==="comment-reply-text"));
  const splice=h.effects.splice;h.effects.splice=(()=>[]) as typeof splice;
  try {
    currentStore={...currentStore,permissions:["inbox:read","inbox:reply"]};h.dirty=true;h.flush();
    assert.equal(nodes(h.output).some(n=>n.props["data-testid"]===`comment-row-${row.ref}`),false);
    assert.equal(nodes(h.output).some(n=>n.props["data-testid"]==="comment-conversations"),false);
    assert.equal(nodes(h.output).some(n=>n.props["data-testid"]==="buyer-panel"),false);
    assert.equal(nodes(h.output).some(n=>n.props["data-testid"]==="comment-reply-text"),false);
    assert.equal(/SYNTHETIC_COMMENT|PRIVATE_PANEL|PRIVATE_CONVERSATION/.test(textOf(h.output)),false);
  } finally {h.effects.splice=splice;h.dirty=true;await h.settle();}
});
for (const deniedPath of ["a2", "a8", "older"] as const) test(`PR18 R2 scoped404 ${deniedPath} clears both private views and stops recovery reads`, async t => {
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T01:00:00Z")});
  let revoked=false, reads=0;
  const claim={status:"ACCEPTED",reason:null,offer_id:sid,keyword:"A1",quantity:1,bundle_id:sid};
  globalThis.fetch=async input=>{
    const url=String(input);
    if(url.includes("buyer-panel?"))return response({display_name:"SYNTHETIC_PANEL",platform:"facebook",purchase_ordinal:1,claims:[],claim_total_minor:0,orders:[],link_pending_manual:false});
    if(url.includes("message-templates"))return response({items:[]});
    reads++;
    const a8=url.includes("inbox/conversations");
    if(revoked && ((deniedPath==="a2"&&!a8)||(deniedPath==="a8"&&a8)||(deniedPath==="older"&&url.includes("cursor="))))return response({code:"not_found"},404);
    if(a8)return response({items:[{conversation_id:sid,bundle_id:sid,platform:"facebook",display_name:"SYNTHETIC_A8_NAME",last_at:"2026-10-09T01:00:00Z",unreplied:true}],next_cursor:"more",unread_total:1});
    return response(page(1,[{...row,author_name:"SYNTHETIC_A2_NAME",marks:{...row.marks,claim}}]));
  };
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();
  assert.ok(textOf(h.output).includes("SYNTHETIC_COMMENT"));
  node(h,n=>n.props["data-testid"]==="comment-filter-private").props.onClick();await h.settle();
  node(h,n=>n.type==="button"&&textOf(n).includes("SYNTHETIC_A8_NAME")).props.onClick();await h.settle();
  assert.ok(textOf(h.output).includes("SYNTHETIC_PANEL"));
  revoked=true;
  if(deniedPath==="older")node(h,n=>n.type==="button"&&textOf(n)===commentCopy("en").older).props.onClick();
  else t.mock.timers.tick(deniedPath==="a2"?3000:10000);
  await h.settle();
  assert.equal(node(h,n=>n.props["data-testid"]==="comment-refresh").props.disabled,true);
  assert.equal(nodes(h.output).some(n=>n.props["data-testid"]==="buyer-panel"),false);
  assert.equal(/SYNTHETIC_(PANEL|A8_NAME|A2_NAME|COMMENT)/.test(textOf(h.output)),false);
  assert.equal(h.slots.some(s=>[...(s.value?.items??[]),...(s.value?.current?.items??[])].some((r:any)=>r.text||r.display_name||r.author_name)),false,"private A2/A8 state is cleared, not merely hidden");
  const stopped=reads;
  env.window.dispatchEvent(new Event("focus"));t.mock.timers.tick(90000);await h.settle();
  assert.equal(reads,stopped,"terminal loss cannot resume polling on focus or timers");
});
test("PR18 A8 clears names synchronously and keeps loaded older pages through a head poll",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-08T01:00:00Z")});
  const item=(id:string,name:string)=>({conversation_id:id,bundle_id:null,display_name:name,platform:"facebook",last_at:"2026-10-08T01:00:00Z",unreplied:true});
  globalThis.fetch=async input=>String(input).includes("inbox/conversations") ? response({items:[String(input).includes("cursor=")?item("old","Synthetic older buyer"):item("head","Synthetic head buyer")],next_cursor:String(input).includes("cursor=")?"":"more",unread_total:2}):response(page());
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-filter-private").props.onClick();await h.settle();
  node(h,n=>n.type==="button"&&textOf(n)===commentCopy("en").older).props.onClick();await h.settle();
  assert.ok(textOf(h.output).includes("Synthetic older buyer"));
  t.mock.timers.tick(10000);await h.settle();
  assert.ok(textOf(h.output).includes("Synthetic older buyer"),"background head must not discard loaded tail");
  assert.equal(nodes(h.output).filter(n=>n.type==="button"&&textOf(n)===commentCopy("en").older).length,0,"exhausted older cursor remains exhausted");
  env.document.visibilityState="hidden";env.document.dispatchEvent(new Event("visibilitychange"));
  // Inspect React state storage before ANY next render/passive effect: privacy callback must clear it now.
  assert.equal(h.slots.some(s=>s.value?.items?.some((r:any)=>r.display_name?.includes("Synthetic"))),false,"A8 names must be synchronously cleared");
  h.flush();assert.equal(textOf(h.output).includes("Synthetic older buyer"),false);
});
test("PR18 A8 paged list stays bounded across fresh heads and stops older at cap",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  let heads=0,olderReads=0;
  const item=(n:number)=>({conversation_id:`synthetic-${n}`,bundle_id:null,display_name:`Synthetic buyer ${n}`,platform:"facebook",last_at:new Date(1000*n).toISOString(),unreplied:true});
  globalThis.fetch=async input=>{
    const url=String(input);if(!url.includes("inbox/conversations"))return response(page(1,[]));
    if(url.includes("cursor=")){olderReads++;return response({items:Array.from({length:50},(_,i)=>item(i)),next_cursor:"more",unread_total:0});}
    const start=50*(++heads);return response({items:Array.from({length:50},(_,i)=>item(start+i)),next_cursor:"more",unread_total:0});
  };
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-filter-private").props.onClick();await h.settle();
  node(h,n=>n.type==="button"&&textOf(n)===commentCopy("en").older).props.onClick();await h.settle();
  assert.equal(olderReads,1);
  for(let n=0;n<22;n++){
    t.mock.timers.tick(10000);await h.settle();
    const list=node(h,x=>x.props["data-testid"]==="comment-conversations");
    assert.ok(nodes(list).filter(x=>x.type==="li").length<=1000,`head${heads}: DOM bounded`);
    assert.ok(h.slots.every(s=>!Array.isArray(s.value?.items)||s.value.items.length<=1000),"retained A8 state bounded too");
  }
  const list=node(h,x=>x.props["data-testid"]==="comment-conversations");
  assert.equal(nodes(list).filter(x=>x.type==="li").length,1000);
  assert.ok(textOf(list).includes(`Synthetic buyer ${heads*50+49}`),"newest head kept");
  assert.equal(textOf(list).includes("Synthetic buyer 0 ·"),false,"oldest evicted");
  assert.equal(nodes(h.output).some(n=>n.type==="button"&&textOf(n)===commentCopy("en").older),false);
});
test("PR18 A8 hides clear before effects even without pagination",async t=>{
  const env=environment(t);
  globalThis.fetch=async input=>String(input).includes("inbox/conversations")?response({items:[{conversation_id:sid,bundle_id:null,display_name:"PRIVATE_A8_NAME",platform:"facebook",last_at:"2026-10-08T01:00:00Z",unreplied:true}],next_cursor:"",unread_total:1}):response(page());
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();node(h,n=>n.props["data-testid"]==="comment-filter-private").props.onClick();await h.settle();
  assert.ok(textOf(h.output).includes("PRIVATE_A8_NAME"));
  env.document.visibilityState="hidden";env.document.dispatchEvent(new Event("visibilitychange"));
  assert.equal(h.slots.some(s=>s.value?.items?.some((r:any)=>r.display_name==="PRIVATE_A8_NAME")),false);
});
for(const loadedHistory of [true,false])test(loadedHistory ? "PR18 deletion HEAD clears selected comment composer but keeps history and public guard" : "PR18 short complete HEAD clears a covered selected comment and composer",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  let deleted=false,headReads=0;
  const history={...row,ref:"100",seq:null,text:"Synthetic old history",created_at:"2026-10-08T00:00:00Z"};
  const selected={...row,seq:2};
  const left={...row,ref:"123_455",seq:1,text:"Synthetic window left",created_at:"2026-10-08T00:59:59Z"};
  const survivor={...row,ref:"123_457",seq:3,text:"Synthetic surviving recent",created_at:row.created_at};
  globalThis.fetch=async(input,init)=>{
    const url=String(input);
    if(init?.method==="POST")return response({send_state:"queued",operation_id:sid,outbound_id:sid});
    if(url.includes("message-templates"))return response({items:[]});
    if(url.includes("before_cursor"))return response(page(1,[history]));
    if(url.includes("after_seq"))return response(page(1,[]));
    headReads++;return response({...page(1,deleted?[left,survivor]:[left,selected,survivor]),older_cursor:loadedHistory?"history":null});
  };
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:cap,reply_public:cap}}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]===`comment-select-${row.ref}`).props.onClick();await h.settle();
  node(h,n=>n.type==="button"&&textOf(n)==="Public reply").props.onClick();h.flush();
  node(h,n=>n.props["data-testid"]==="comment-reply-text").props.onChange({target:{value:"Synthetic pending reply"}});h.flush();
  node(h,n=>n.type==="form").props.onSubmit({preventDefault(){}});await h.settle();
  if(loadedHistory){node(h,n=>n.props["data-testid"]==="comment-older").props.onClick();await h.settle();}
  const guard=[...stored.keys()].find(k=>k.startsWith("live-comment-public-pending:"));assert.ok(guard);
  const headBefore=headReads;deleted=true;
  for(let n=0;n<3;n++){t.mock.timers.tick(3000);await h.settle();}
  assert.ok(textOf(h.output).includes("SYNTHETIC_COMMENT"),"incremental reads cannot imply deletion");
  t.mock.timers.tick(3000);await h.settle();
  assert.equal(headReads,headBefore+1,"only the existing10s cadence re-reads the head");
  assert.equal(nodes(h.output).some(n=>n.props["data-testid"]===`comment-row-${row.ref}`),false);
  assert.equal(nodes(h.output).some(n=>n.props["data-testid"]==="comment-reply"),false);
  assert.equal(textOf(h.output).includes("Synthetic old history"),loadedHistory);assert.equal(stored.get(guard!),"1");
  assert.equal(h.slots.some(s=>s.value?.ref===row.ref),false,"selection is cleared, not only hidden");
});
test("PR18 quiet A2 stream refreshes delayed claim marks without a new sequence or user refresh",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-08T01:00:00Z")});
  let completed=false;const urls:string[]=[];
  globalThis.fetch=async input=>{const url=String(input);urls.push(url);return response(url.includes("after_seq")?page(1,[]):page(1,[completed?{...row,marks:{...row.marks,claim:{status:"ACCEPTED",reason:null,offer_id:sid,keyword:"A1",quantity:1,bundle_id:sid}}}:row]));};
  const h=env.mount(()=>useCommentStream(store.id,sid,true));await h.settle();assert.equal(h.output.buffer.items[0].marks.claim,null);
  h.output.select({ref:row.ref});h.flush();completed=true;
  for(let n=0;n<4;n++){t.mock.timers.tick(3000);await h.settle();}
  assert.equal(h.output.buffer.items[0].marks.claim?.bundle_id,sid);
  assert.equal(h.output.selection.ref,row.ref);
  assert.ok(urls.some(u=>u.includes("after_seq")),"incremental cursor still used");
  assert.ok(urls.filter(u=>!u.includes("after_seq")).length>=2,"bounded recent window automatically re-read");
});
test("PR18 late claim automatically exposes keyword view and the reused BuyerPanel",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-08T01:00:00Z")});
  let completed=false,panelReads=0;
  globalThis.fetch=async input=>{
    const url=String(input);
    if(url.includes("buyer-panel?")){panelReads++;assert.ok(url.includes(`bundle_id=${sid}`));return response({display_name:"SYNTHETIC_LATE_BUYER",platform:"facebook",purchase_ordinal:1,claims:[],claim_total_minor:0,orders:[],link_pending_manual:false});}
    if(url.includes("message-templates"))return response({items:[]});
    return response(url.includes("after_seq")?page(1,[]):page(1,[completed?{...row,marks:{...row.marks,claim:{status:"ACCEPTED",reason:null,offer_id:sid,keyword:"A1",quantity:1,bundle_id:sid}}}:row]));
  };
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]===`comment-select-${row.ref}`).props.onClick();await h.settle();assert.equal(panelReads,0);
  completed=true;for(let n=0;n<4;n++){t.mock.timers.tick(3000);await h.settle();}
  assert.equal(panelReads,1);assert.ok(textOf(h.output).includes("SYNTHETIC_LATE_BUYER"));
  node(h,n=>n.props["data-testid"]==="comment-filter-keyword").props.onClick();await h.settle();assert.ok(textOf(h.output).includes("A1"));
});
for(const filter of ["private","unreplied"])test(`K3 A8 ${filter} polls every 10s, backs off, resets, pauses hidden and stops off-filter`,async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-08T01:00:00Z")});
  let reads=0,failures=0;
  globalThis.fetch=async input=>{
    if(String(input).includes("inbox/conversations")){
      reads++;if(failures>0){failures--;return response({code:"unavailable"},503);}
      return response({items:reads>1?[{bundle_id:sid,conversation_id:null,platform:"facebook",display_name:"Synthetic new DM",last_at:"2026-10-08T01:00:00Z",unread:true,unreplied:true,mode:null,assignee:null,window_open_until:null,linked_customer_id:null}]:[],next_cursor:"",unread_total:0});
    }
    return response(page());
  };
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]===`comment-filter-${filter}`).props.onClick();await h.settle();assert.equal(reads,1);
  t.mock.timers.tick(9999);await h.settle();assert.equal(reads,1);t.mock.timers.tick(1);await h.settle();assert.equal(reads,2,"idle A8 view must refresh without a merchant action");assert.ok(textOf(h.output).includes("Synthetic new DM"));
  failures=5;t.mock.timers.tick(10000);await h.settle();assert.equal(reads,3);
  for(const delay of [10000,20000,30000,30000,30000]){const previous=reads;t.mock.timers.tick(delay-1);await h.settle();assert.equal(reads,previous);t.mock.timers.tick(1);await h.settle();assert.equal(reads,previous+1);}
  const successful=reads;t.mock.timers.tick(9999);await h.settle();assert.equal(reads,successful);t.mock.timers.tick(1);await h.settle();assert.equal(reads,successful+1);
  env.document.visibilityState="hidden";env.document.dispatchEvent(new Event("visibilitychange"));h.flush();const hidden=reads;
  t.mock.timers.tick(60000);await h.settle();assert.equal(reads,hidden);assert.equal(textOf(h.output).includes("Synthetic new DM"),false);
  env.document.visibilityState="visible";env.document.dispatchEvent(new Event("visibilitychange"));await h.settle();assert.equal(reads,hidden+1);
  node(h,n=>n.props["data-testid"]==="comment-filter-all").props.onClick();await h.settle();const stopped=reads;t.mock.timers.tick(60000);await h.settle();assert.equal(reads,stopped);
});
test("K3 pasted tab is refused with invalid_text copy before any reply POST",async t=>{
  const env=environment(t);let sends=0;
  globalThis.fetch=async(_url,init)=>{if(init?.method==="POST")sends++;return response({items:[],send_state:"queued"});};
  const h=env.mount(()=>CommentReply({store,session:sid,comment:row,locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:{state:"ok",reason:"ok",evidence:"MOCK",checked_at:null}}},onSent(){},onDenied(){}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-reply-text").props.onChange({target:{value:"Synthetic\treply"}});h.flush();node(h,n=>n.type==="form").props.onSubmit({preventDefault(){}});await h.settle();
  assert.equal(sends,0);assert.equal(stored.size,0,"invalid text must not arm an uncertain-send receipt");assert.ok(textOf(h.output).includes(commentCopy("en").invalid_text));
});
for(const locale of ["zh-TW","zh-CN","en"])test(`PR18 facts_unavailable mark uses actionable copy ${locale}`,async t=>{
  const env=environment(t);globalThis.fetch=async()=>response({items:[]});
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const h=env.mount(()=>CommentReply({store,session:sid,comment:{...row,marks:{...row.marks,private_reply_available:false,private_reply_unavailable_reason:"facts_unavailable"}},locale,platform:"facebook",capabilities:{facebook:{private_reply:cap,reply_public:cap}},onSent(){},onDenied(){}} as any));
  await h.settle();
  assert.equal(textOf(node(h,n=>n.props["data-testid"]==="comment-rule")),commentCopy(locale).comment_facts_unavailable);
  assert.equal(node(h,n=>n.type==="fieldset").props.disabled,true);
});
test("real SQL UNKNOWN mark blocks public-mode switching without a local receipt",async t=>{
  const env=environment(t);globalThis.fetch=async()=>response({items:[]});
  const h=env.mount(()=>CommentReply({store,session:sid,comment:{...row,marks:{...row.marks,private_reply:{kind:"manual",state:"UNKNOWN",blocked_reason:null},private_reply_available:false,private_reply_unavailable_reason:"auto_pending"}},locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:{state:"ok",reason:"ok",evidence:"MOCK",checked_at:null},reply_public:{state:"ok",reason:"ok",evidence:"MOCK",checked_at:null}}},onSent(){},onDenied(){}} as any));
  await h.settle();assert.ok(textOf(h.output).includes(commentCopy("en").verify));
  assert.equal(node(h,n=>n.type==="button"&&textOf(n)==="Public reply").props.disabled,true);
});
for(const state of ["unknown","queued"] as const)test(`PR18 ${state} public receipt has the correct cross-comment fence`, async (t) => {
  const env = environment(t);
  t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T01:00:00Z")});
  let sends = 0;
  globalThis.fetch = async (_url, init) => {
    if (init?.method === "POST") {
      sends++;
      return response({
        send_state: state,
        operation_id: sid,
        outbound_id: sid,
      });
    }
    return response({ items: [] });
  };
  const props = {
    store,
    session: sid,
    comment: row,
    locale: "en",
    platform: "facebook",
    capabilities: {
      facebook: {
        private_reply: {
          state: "ok",
          reason: "ok",
          evidence: "MOCK",
          checked_at: null,
        },
        reply_public: {
          state: "ok",
          reason: "ok",
          evidence: "MOCK",
          checked_at: null,
        },
      },
    },
    onSent() {},
    onDenied() {},
  };
  let h = env.mount(() => CommentReply(props as any));
  await h.settle();
  node(
    h,
    (n) => n.type === "button" && textOf(n) === "Public reply",
  ).props.onClick();
  h.flush();
  node(
    h,
    (n) => n.props["data-testid"] === "comment-reply-text",
  ).props.onChange({ target: { value: "Synthetic unresolved public" } });
  h.flush();
  node(h, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
  await h.settle();
  assert.equal(sends, 1);
  const publicKey=[...stored.keys()].find(key=>key.startsWith("live-comment-public-pending:"));
  assert.ok(publicKey,"public guard armed before losing component state");
  h.dispose();
  h = env.mount(() =>
    CommentReply({ ...props, comment: { ...row, ref: "987654" } } as any),
  );
  await h.settle();
  assert.equal(
    nodes(h.output).find((n) => n.type === "fieldset")?.props.disabled,
    state === "unknown",
    "only UNKNOWN blocks a new comment after navigation",
  );
  // R2 ruling: keep a per-comment public guard even after an acknowledged queue entry.
  assert.equal(stored.size, state === "unknown" ? 2 : 1);
  for (const [key, value] of stored) {
    assert.doesNotMatch(key, /987654|Synthetic/);
    assert.equal(value, "1");
  }
  // Worker may become UNKNOWN; A2 supplies only a count, not that state.
  // Recreate the component twice (selection return and reload), retaining only storage.
  h.dispose();h=env.mount(()=>CommentReply(props as any));await h.settle();
  h.dispose();h=env.mount(()=>CommentReply(props as any));await h.settle();
  t.mock.timers.tick(31000);await h.settle();
  assert.equal(node(h,n=>n.type==="fieldset").props.disabled,true,"public non-terminal survives reload past backend dedupe window");
  node(h,n=>n.type==="form").props.onSubmit({preventDefault(){}});await h.settle();
  assert.equal(sends,1,"cannot create a new operation without verification");
  Object.assign(env.window,{confirm:()=>true});
  node(h,n=>n.props["data-testid"]==="comment-verified").props.onClick();await h.settle();
  assert.equal(stored.size,0,"explicit verification clears both guards");
});

for(const outcome of ["sent","failed","blocked","refused"] as const)test(`PR18 public guard releases only definite ${outcome}`,async t=>{
  const env=environment(t);
  globalThis.fetch=async(_url,init)=>{
    if(init?.method!=="POST")return response({items:[]});
    assert.equal([...stored.keys()].filter(k=>k.startsWith("live-comment-public-pending:")).length,1,"armed before network dispatch");
    return outcome==="refused"?response({code:"public_reply_forbidden_content"},422):response({send_state:outcome,operation_id:sid,outbound_id:sid});
  };
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const props={store,session:sid,comment:row,locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:cap,reply_public:cap}},onSent(){},onDenied(){}};
  const h=env.mount(()=>CommentReply(props as any));await h.settle();
  node(h,n=>n.type==="button"&&textOf(n)==="Public reply").props.onClick();h.flush();
  node(h,n=>n.props["data-testid"]==="comment-reply-text").props.onChange({target:{value:"Synthetic terminal reply"}});h.flush();
  node(h,n=>n.type==="form").props.onSubmit({preventDefault(){}});await h.settle();
  assert.equal(stored.size,0);
});
const row = {
  ref: "123_456",
  seq: 1,
  parent_ref: null,
  created_at: "2026-10-08T01:00:00Z",
  author_name: null,
  text: "SYNTHETIC_COMMENT",
  is_page: false,
  has_attachment: false,
  marks: {
    intake: null,
    claim: null,
    private_reply: null,
    printed: null,
    public_replies: 0,
    private_reply_available: true,
  },
};
const page = (epoch = 1, items = [row], reset = false) => ({
  epoch,
  items,
  reset,
  scan_exhausted: false,
  next: { epoch, seq: items.length },
  older_cursor: null,
  stream: {
    state: "live",
    source_platform: "facebook",
    video_embeddable: true,
  },
});
test("PR18 P1 IG earliest50 refresh preserves100 rows selection and private draft",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  const igRows=Array.from({length:100},(_,i)=>({...row,ref:String(i+1),seq:i+1,text:`Synthetic IG ${i+1}`,created_at:new Date(Date.parse(row.created_at)+1000*i).toISOString()}));
  const igPage=(items:typeof igRows,seq:number)=>({...page(0,items),next:{epoch:0,seq},stream:{state:"live",source_platform:"instagram",video_embeddable:false}});
  let heads=0,sends=0;const afterSeqs:string[]=[];
  globalThis.fetch=async(input,init)=>{
    if(init?.method==="POST"){sends++;return response({send_state:"queued"});}
    const url=String(input);if(url.includes("message-templates"))return response({items:[]});
    const params=new URL(url,"https://example.invalid").searchParams;
    if(params.has("after_seq"))afterSeqs.push(params.get("after_seq")!);
    if(params.get("after_seq")==="50")return response(igPage(igRows.slice(50),100));
    if(params.has("after_seq"))return response(igPage([],100));
    heads++;return response(igPage(igRows.slice(0,50),50)); // Real IG SQL ordering: earliest, not newest.
  };
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"instagram",capabilities:{instagram:{private_reply:cap,reply_public:cap}}} as any));await h.settle();
  t.mock.timers.tick(3000);await h.settle();
  assert.equal(nodes(h.output).filter(n=>String(n.props["data-testid"]??"").startsWith("comment-row-")).length,100);
  node(h,n=>n.props["data-testid"]==="comment-select-75").props.onClick();await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-reply-text").props.onChange({target:{value:"Synthetic IG unsaved draft"}});h.flush();
  const initialHeads=heads;
  for(let cycle=0;cycle<3;cycle++){
    for(let tick=0;tick<4;tick++){t.mock.timers.tick(3000);await h.settle();}
    assert.equal(nodes(h.output).filter(n=>String(n.props["data-testid"]??"").startsWith("comment-row-")).length,100,`cycle${cycle}: no earliest-page absence deletion`);
    assert.equal(node(h,n=>n.props["data-testid"]==="comment-row-75").props["data-selected"],true);
    assert.equal(node(h,n=>n.props["data-testid"]==="comment-reply-text").props.value,"Synthetic IG unsaved draft");
    for(let n=51;n<=100;n++)assert.ok(nodes(h.output).some(x=>x.props["data-testid"]===`comment-row-${n}`));
    assert.equal(afterSeqs.at(-1),"100","head never rewinds incremental continuation");
  }
  assert.equal(heads,initialHeads+3,"three real hook head-read cycles exercised");
  assert.equal(sends,0,"an unsaved draft is never dispatched by a refresh");
});
test("PR18 final expired older cursor keeps live buffer selection and polling",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  let olderReads=0,incrementalReads=0;
  globalThis.fetch=async input=>{
    const url=String(input);
    if(url.includes("before_cursor=")){olderReads++;return response({code:"invalid_cursor"},400);}
    if(url.includes("after_seq=")){incrementalReads++;return response(page(1,[]));}
    return response({...page(),older_cursor:"expired-history"});
  };
  const h=env.mount(()=>useCommentStream(store.id,sid,true));await h.settle();
  h.output.select({ref:row.ref});h.flush();const next={...h.output.buffer.next};
  h.output.older();await h.settle();assert.equal(olderReads,1);
  assert.deepEqual(h.output.buffer.items.map((r:any)=>r.ref),[row.ref]);
  assert.deepEqual(h.output.selection,{ref:row.ref});
  assert.deepEqual(h.output.buffer.next,next,"older failure does not rewind live cursor");
  assert.equal(h.output.buffer.older,null);assert.equal(h.output.error,"invalid_cursor");
  h.output.older();await h.settle();assert.equal(olderReads,1,"expired pagination is not retried");
  t.mock.timers.tick(3000);await h.settle();
  assert.equal(incrementalReads,1);assert.equal(h.output.selection.ref,row.ref);
});
for(const initial of [0,1])test(`PR18 command refresh keeps epoch evidence while cursorless initial=${initial}`,async t=>{
  const env=environment(t);let changed=false;
  globalThis.fetch=async()=>response(changed?page(initial+1,[{...row,ref:"999"}]):page(initial));
  const h=env.mount(()=>useCommentStream(store.id,sid,true));await h.settle();h.output.select({ref:row.ref});h.flush();
  changed=true;h.output.refresh();await h.settle();
  assert.deepEqual(h.output.buffer.items.map((r:any)=>r.ref),["999"]);assert.equal(h.output.buffer.epoch,initial+1);assert.equal(h.output.selection,null);
});
for(const loadedHistory of [false,true])test(`PR18 bounded window refresh removes FB deletion behind60 arrivals history=${loadedHistory}`,async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  const rows=Array.from({length:61},(_,i)=>({...row,ref:String(i+1),seq:i+1,created_at:new Date(Date.parse(row.created_at)+i*1000).toISOString()}));let heads=0;
  const reply=(items:typeof rows,seq:number)=>({...page(1,items),next:{epoch:1,seq}});
  globalThis.fetch=async input=>{
    const url=new URL(String(input),"https://example.invalid"),after=url.searchParams.get("after_seq");
    if(url.searchParams.has("before_cursor"))return response(reply([{...row,ref:"100",seq:null,created_at:"2026-10-01T00:00:00Z"}] as any,1));
    if(after==="1")return response(reply(rows.slice(1,51),51));
    if(after==="51")return response(reply(rows.slice(51),61));
    if(after)return response(reply([],61));
    heads++;return response(heads===1?{...reply([rows[0]],1),older_cursor:loadedHistory?"history":null}:reply(rows.slice(11),12));
  };
  const h=env.mount(()=>useCommentStream(store.id,sid,true));await h.settle();
  if(loadedHistory){h.output.older();await h.settle();}
  for(let n=0;n<4;n++){t.mock.timers.tick(3000);await h.settle();}
  assert.equal(h.output.buffer.items.length,loadedHistory?62:61);assert.ok(h.output.buffer.items.some((r:any)=>r.ref==="1"));
  t.mock.timers.tick(227999);await h.settle();assert.ok(h.output.buffer.items.some((r:any)=>r.ref==="1"),"no premature absence inference outside the head coverage");
  t.mock.timers.tick(3000);await h.settle();
  assert.equal(h.output.buffer.items.some((r:any)=>r.ref==="1"),false,"successful full refresh removes stale live rows within5min");
  assert.equal(h.output.buffer.next.seq,61,"FB rebuild does not rewind the incremental cursor");
  if(loadedHistory){assert.ok(h.output.buffer.items.some((r:any)=>r.ref==="100"));assert.equal(h.output.buffer.historyLoaded,true);}
});
test("PR18 bounded window refresh preserves explicit history until manual refresh",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  const history={...row,ref:"100",seq:null,created_at:"2026-10-01T00:00:00Z"};
  globalThis.fetch=async input=>{
    const url=String(input);return response(url.includes("before_cursor")?page(1,[history] as any):url.includes("after_seq")?page(1,[]):{...page(),older_cursor:"history"});
  };
  const h=env.mount(()=>useCommentStream(store.id,sid,true));await h.settle();h.output.older();await h.settle();
  assert.equal(h.output.buffer.historyLoaded,true);t.mock.timers.tick(300000);await h.settle();
  assert.ok(h.output.buffer.items.some((r:any)=>r.ref===history.ref));
  h.output.refresh();await h.settle();assert.ok(h.output.buffer.items.some((r:any)=>r.ref===history.ref),"command mark refresh is not manual history reset");
  h.output.refresh(true);await h.settle();
  assert.equal(h.output.buffer.items.some((r:any)=>r.ref===history.ref),false);assert.equal(h.output.buffer.historyLoaded,false);
});
for(const deleted of [false,true])test(`PR18 bounded window refresh atomically rebuilds IG100 deleted75=${deleted}`,async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  const rows=Array.from({length:100},(_,i)=>({...row,ref:String(i+1),seq:i+1,created_at:new Date(Date.parse(row.created_at)+i*1000).toISOString()}));let loaded=false,tailReads=0;
  const reply=(items:typeof rows,seq:number)=>({...page(0,items),next:{epoch:0,seq},stream:{state:"live",source_platform:"instagram",video_embeddable:false}});
  globalThis.fetch=async input=>{
    const url=String(input);if(url.includes("message-templates"))return response({items:[]});
    const after=new URL(url,"https://example.invalid").searchParams.get("after_seq");
    if(after==="50"){tailReads++;return response(reply(rows.slice(50).filter(r=>!loaded||!deleted||r.ref!=="75"),100));}
    return response(after?reply([],100):reply(rows.slice(0,50),50));
  };
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"instagram",capabilities:{instagram:{private_reply:cap,reply_public:cap}}} as any));await h.settle();
  t.mock.timers.tick(3000);await h.settle();assert.equal(tailReads,1);loaded=true;
  node(h,n=>n.props["data-testid"]==="comment-select-75").props.onClick();await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-reply-text").props.onChange({target:{value:"Synthetic retained IG draft"}});h.flush();
  t.mock.timers.tick(236999);await h.settle();assert.equal(node(h,n=>n.props["data-testid"]==="comment-reply-text").props.value,"Synthetic retained IG draft");
  t.mock.timers.tick(3000);await h.settle();assert.ok(tailReads>=2,"full rebuild must follow the fresh continuation, not keep old100 against earliest50");
  assert.equal(nodes(h.output).filter(n=>String(n.props["data-testid"]??"").startsWith("comment-row-")).length,deleted?99:100);
  assert.equal(nodes(h.output).some(n=>n.props["data-testid"]==="comment-reply-text"),!deleted);
  if(!deleted)assert.equal(node(h,n=>n.props["data-testid"]==="comment-reply-text").props.value,"Synthetic retained IG draft");
  assert.ok(h.slots.some(s=>s.value?.next?.seq===100));
});
for(const refusal of ["stalled","budget"] as const)test(`PR18 bounded window refresh ${refusal} never publishes an incomplete IG window`,async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  const started=Date.now(),rows=Array.from({length:100},(_,i)=>({...row,ref:String(i+1),seq:i+1,created_at:new Date(Date.parse(row.created_at)+i*1000).toISOString()}));let scanReads=0;
  const reply=(items:typeof rows,seq:number)=>({...page(0,items),next:{epoch:0,seq},stream:{source_platform:"instagram",video_embeddable:false}});
  globalThis.fetch=async input=>{
    const url=String(input);if(url.includes("message-templates"))return response({items:[]});
    const after=new URL(url,"https://example.invalid").searchParams.get("after_seq");
    if(after==="100")return response(reply([],100));
    if(Date.now()-started>=240000){scanReads++;if(after)return response(refusal==="stalled"?reply([],Number(after)):reply([rows[Number(after)]],Number(after)+1));}
    return response(after?reply(rows.slice(50),100):reply(rows.slice(0,50),50));
  };
  const cap={state:"ok",reason:"ok",evidence:"MOCK",checked_at:null};
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"instagram",capabilities:{instagram:{private_reply:cap,reply_public:cap}}} as any));await h.settle();
  t.mock.timers.tick(3000);await h.settle();node(h,n=>n.props["data-testid"]==="comment-select-75").props.onClick();await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-reply-text").props.onChange({target:{value:"Synthetic retained incomplete-scan draft"}});h.flush();
  t.mock.timers.tick(236999);await h.settle();t.mock.timers.tick(3000);await h.settle();
  assert.equal(scanReads,refusal==="stalled"?2:20,"scan budget and forward progress are enforced");
  assert.equal(nodes(h.output).filter(n=>String(n.props["data-testid"]??"").startsWith("comment-row-")).length,100);
  assert.equal(node(h,n=>n.props["data-testid"]==="comment-row-75").props["data-selected"],true);
  assert.equal(node(h,n=>n.props["data-testid"]==="comment-reply-text").props.value,"Synthetic retained incomplete-scan draft");
  assert.ok(h.slots.some(s=>s.value?.next?.seq===100));assert.ok(h.slots.some(s=>s.value==="stream_unavailable"));
});
test("PR18 bounded window refresh removes deleted IG final sequence",async t=>{
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T00:00:00Z")});
  const rows=Array.from({length:100},(_,i)=>({...row,ref:String(i+1),seq:i+1,created_at:new Date(Date.parse(row.created_at)+i*1000).toISOString()}));let deleted=false;
  const reply=(items:typeof rows,seq:number)=>({...page(0,items),next:{epoch:0,seq},stream:{source_platform:"instagram",video_embeddable:false}});
  globalThis.fetch=async input=>{
    const after=new URL(String(input),"https://example.invalid").searchParams.get("after_seq");
    if(after==="50")return response({...reply(rows.slice(50).filter(r=>!deleted||r.ref!=="100"),deleted?99:100),scan_exhausted:deleted});
    if(after)return response(reply([],Number(after)));
    return response(reply(rows.slice(0,50),50));
  };
  const h=env.mount(()=>useCommentStream(store.id,sid,true));await h.settle();t.mock.timers.tick(3000);await h.settle();assert.equal(h.output.buffer.items.length,100);
  deleted=true;t.mock.timers.tick(236999);await h.settle();t.mock.timers.tick(3000);await h.settle();
  assert.equal(h.output.buffer.items.some((r:any)=>r.ref==="100"),false,"the deleted last seq must not remain indefinitely after an exhausted rebuild");
});
test("actual A2 hook fake clock: 3/6/12/24/30 backoff, success reset, hide clears and stops, 403 locks", async (t) => {
  const env = environment(t);
  t.mock.timers.enable({
    apis: ["setTimeout", "Date"],
    now: Date.parse("2026-10-08T01:00:00Z"),
  });
  let calls = 0,
    deny = false;
  globalThis.fetch = async () => {
    calls++;
    return calls <= 5
      ? response({ code: "stream_unavailable" }, 503)
      : deny
        ? response({ code: "forbidden" }, 403)
        : response(page());
  };
  const h = env.mount(() => useCommentStream(store.id, sid, true));
  await h.settle();
  assert.equal(calls, 1);
  for (const delay of [3000, 6000, 12000, 24000, 30000]) {
    const before = calls;
    t.mock.timers.tick(delay - 1);
    await h.settle();
    assert.equal(calls, before);
    t.mock.timers.tick(1);
    await h.settle();
    assert.equal(calls, before + 1);
  }
  assert.equal(h.output.buffer.items.length, 1);
  t.mock.timers.tick(3000);
  await h.settle();
  assert.equal(calls, 7);
  env.document.visibilityState = "hidden";
  env.document.dispatchEvent(new Event("visibilitychange"));
  h.flush();
  assert.equal(h.output.buffer.items.length, 0);
  t.mock.timers.tick(90000);
  await h.settle();
  assert.equal(calls, 7);
  deny = true;
  env.document.visibilityState = "visible";
  env.document.dispatchEvent(new Event("visibilitychange"));
  await h.settle();
  assert.equal(calls, 8);
  assert.equal(h.output.buffer.items.length, 0);
  assert.equal(h.output.error, "forbidden");
  h.output.refresh();
  t.mock.timers.tick(90000);
  await h.settle();
  assert.equal(calls, 8);
});
test("actual reset clears selected buyer and rereads without the old epoch cursor", async (t) => {
  const env = environment(t);
  t.mock.timers.enable({
    apis: ["setTimeout", "Date"],
    now: Date.parse("2026-10-08T01:00:00Z"),
  });
  const urls: string[] = [];
  let n = 0;
  globalThis.fetch = async (input) => {
    urls.push(String(input));
    n++;
    return response(
      n === 1
        ? page()
        : n === 2
          ? page(2, [], true)
          : page(2, [{ ...row, ref: "789" }]),
    );
  };
  const h = env.mount(() => useCommentStream(store.id, sid, true));
  await h.settle();
  h.output.select({ ref: row.ref });
  h.flush();
  t.mock.timers.tick(3000);
  await h.settle();
  assert.deepEqual(
    h.output.buffer.items.map((r: any) => r.ref),
    ["789"],
  );
  assert.equal(h.output.selection, null);
  assert.match(urls[1], /after_epoch=1/);
  assert.doesNotMatch(urls[2], /after_epoch/);
});
test("reply hints, coded error, UNKNOWN and capability state execute the actual composer", async (t) => {
  const env = environment(t);
  let sends = 0;
  globalThis.fetch = async (_input, init) => {
    if (init?.method === "POST") {
      sends++;
      return response({ code: "unavailable" }, 503);
    }
    return response({ items: [] });
  };
  const h = env.mount(() =>
    CommentReply({
      store,
      session: sid,
      comment: row,
      locale: "en",
      platform: "facebook",
      capabilities: {
        facebook: {
          private_reply: {
            state: "ok",
            reason: "ok",
            evidence: "MOCK",
            checked_at: null,
          },
        },
      },
      onSent() {},
      onDenied() {},
    } as any),
  );
  await h.settle();
  assert.ok(textOf(h.output).includes(commentCopy("en").one));
  assert.ok(textOf(h.output).includes(commentCopy("en").window));
  node(
    h,
    (n) => n.props["data-testid"] === "comment-reply-text",
  ).props.onChange({ target: { value: "Synthetic reply" } });
  h.flush();
  node(h, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
  await h.settle();
  assert.equal(sends, 1);
  assert.ok(textOf(h.output).includes(commentCopy("en").verify));
  node(h, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
  await h.settle();
  assert.equal(sends, 1);
  assert.equal(
    nodes(h.output).find((n) => n.type === "fieldset")?.props.disabled,
    true,
  );
});

// K3 PR18 r2 P2 (privacy, folded in): scope-only reads treat 404 as lost scope, like Inbox.tsx. The send POST is deliberately
// excluded: its 404 can also mean a stale offer, and the A2 poll (3 s) expires a real scope loss anyway.
test("PR18 R2 templates read 404 denies the composer", async t => {
  const env=environment(t);let denied=0;
  globalThis.fetch=async input=>String(input).includes("message-templates")?response({code:"not_found"},404):response({items:[]});
  const h=env.mount(()=>CommentReply({store,session:sid,comment:row,locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:{state:"ok",reason:"ok",evidence:"MOCK",checked_at:null}}},onSent(){},onDenied(){denied++;}} as any));
  await h.settle();assert.equal(denied,1);
});
for(const [status,lost] of [[404,0],[403,1]] as const)test(`PR18 buyer-panel read ${status}: ${lost?"revokes the console scope":"stays local (retention-purged or gone selection), console keeps access"}`, async t => {
  // Codex r4 4227138842: A13 answers not_found for a purged/missing bundle too, so only 401/403 revoke the parent scope;
  // a real scope loss still reaches the A2/A8 polls within one interval.
  const env=environment(t);let lost_=0;
  globalThis.fetch=async input=>String(input).includes("buyer-panel?")?response({code:status===404?"not_found":"forbidden"},status):response({items:[]});
  const h=env.mount(()=>BuyerPanel({store,conversationId:sid,onUnauthorized(){lost_++;}} as any));
  await h.settle();assert.equal(lost_,lost);assert.equal(textOf(h.output).includes("SYNTHETIC"),false);
});

// W3-U2 adds a private scope read below A13; its first 404 must also revoke the containing console.
test("W3-U2 restriction check 404 first expires all console views and rejects late A8", async t => {
  const env=environment(t);t.mock.timers.enable({apis:["setTimeout","Date"],now:Date.parse("2026-10-09T01:00:00Z")});
  let deny!:(value:Response)=>void, late!:(value:Response)=>void, reads=0, hold=false;
  const claim={status:"ACCEPTED",reason:null,offer_id:sid,keyword:"A1",quantity:1,bundle_id:sid};
  const a8=()=>response({items:[{conversation_id:sid,bundle_id:sid,platform:"facebook",display_name:"SYNTHETIC_A8_NAME",last_at:"2026-10-09T01:00:00Z",unreplied:true}],next_cursor:"",unread_total:1});
  globalThis.fetch=async input=>{
    const url=String(input);reads++;
    if(url.includes("blocklist/check"))return new Promise<Response>(resolve=>{deny=resolve;});
    if(url.includes("buyer-panel?"))return response({display_name:"SYNTHETIC_PANEL",platform:"facebook",purchase_ordinal:1,claims:[],claim_total_minor:0,orders:[],link_pending_manual:false});
    if(url.includes("message-templates"))return response({items:[]});
    if(url.includes("inbox/conversations"))return hold?new Promise<Response>(resolve=>{late=resolve;}):a8();
    return response(page(1,[{...row,author_name:"SYNTHETIC_A2_NAME",marks:{...row.marks,claim}}]));
  };
  const h=env.mount(()=>CommentStream({store,session:sid,locale:"en",platform:"facebook",capabilities:{}} as any));await h.settle();
  node(h,n=>n.props["data-testid"]==="comment-filter-private").props.onClick();await h.settle();
  node(h,n=>n.type==="button"&&textOf(n).includes("SYNTHETIC_A8_NAME")).props.onClick();await h.settle();
  assert.ok(textOf(h.output).includes("SYNTHETIC_PANEL"));assert.equal(typeof deny,"function");
  hold=true;t.mock.timers.tick(10000);await h.settle();assert.equal(typeof late,"function");
  deny(response({code:"not_found"},404));await h.settle();
  assert.equal(node(h,n=>n.props["data-testid"]==="comment-refresh").props.disabled,true);
  assert.equal(/SYNTHETIC_(PANEL|A8_NAME|A2_NAME|COMMENT)/.test(textOf(h.output)),false);
  const stopped=reads;late(a8());await h.settle();env.window.dispatchEvent(new Event("focus"));t.mock.timers.tick(90000);await h.settle();
  assert.equal(reads,stopped);assert.equal(/SYNTHETIC_(PANEL|A8_NAME|A2_NAME|COMMENT)/.test(textOf(h.output)),false);
});
