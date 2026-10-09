// Purpose: LC-U2a closed transport and cursor/privacy counterexamples (MOCK).
// Depends on: comment-request and comment-model; node:test/assert.
// Used by: test-node and console stream acceptance; no real comment payloads.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  commentRoute,
  validCommentRequest,
  validCommentBody,
  commentErrorCode,
} from "../../apps/admin/src/features/live/comment-request.ts";
import {
  applyCommentPage,
  commentViewResource,
  commentDelay,
  emptyComments,
  parseCommentPage,
  commentSendState,
} from "../../apps/admin/src/features/live/comment-model.ts";

const sid = "22222222-2222-4222-8222-222222222222";
const path = `live-sessions/${sid}/comments`;
const deletionRows=[1,2,3,4].map(n=>({ref:String(n),created_at:new Date(n*1000).toISOString(),text:`Synthetic ${n}`}));
const deletionPage=(items:unknown[])=>({epoch:1,reset:false,items,next:{epoch:1,seq:4},older_cursor:"history",stream:{source_platform:"facebook"}}) as never;
test("PR18 P1 IG earliest page and unknown authority never delete by absence",()=>{
  const rows=Array.from({length:100},(_,i)=>({ref:String(i+1),created_at:new Date(1000*i).toISOString(),text:`Synthetic ${i+1}`}));
  const initial={...deletionPage(rows) as any,next:{epoch:1,seq:100},stream:{source_platform:"instagram"}};
  const old=applyCommentPage(emptyComments(),initial,false);
  for(const platform of ["instagram",undefined]){
    const earliest={...initial,items:rows.slice(0,50),next:{epoch:1,seq:50},stream:platform?{source_platform:platform}:undefined};
    assert.deepEqual(applyCommentPage(old,earliest,false,true).items.map(r=>r.ref),rows.map(r=>r.ref));
  }
});
test("PR18 short complete HEAD removes an absent oldest live row",()=>{
  const old=applyCommentPage(emptyComments(),deletionPage(deletionRows),false);
  const head={...deletionPage(deletionRows.slice(1)) as any,older_cursor:null};
  assert.deepEqual(applyCommentPage(old,head,false,true).items.map(r=>r.ref),["2","3","4"]);
});
test("PR18 full or cursor-bearing HEAD keeps the floor rule",()=>{
  const old=applyCommentPage(emptyComments(),deletionPage(deletionRows),false);
  const full={...deletionPage(deletionRows.slice(1)) as any,older_cursor:null};
  assert.deepEqual(applyCommentPage(old,full,false,true,3).items.map(r=>r.ref),["1","2","3","4"]);
  assert.deepEqual(applyCommentPage(old,deletionPage(deletionRows.slice(1)),false,true).items.map(r=>r.ref),["1","2","3","4"]);
});
test("PR18 loaded history stays protected from short complete HEAD",()=>{
  let old=applyCommentPage(emptyComments(),deletionPage(deletionRows.slice(1)),false);
  old=applyCommentPage(old,deletionPage([deletionRows[0]]),true);
  const head={...deletionPage([deletionRows[3]]) as any,older_cursor:null};
  assert.deepEqual(applyCommentPage(old,head,false,true).items.map(r=>r.ref),["1","2","3","4"]);
});
test("PR18 deletion HEAD removes absent refs inside its window",()=>{
  const old=applyCommentPage(emptyComments(),deletionPage(deletionRows),false);
  const result=applyCommentPage(old,deletionPage([deletionRows[2]]),false,true);
  assert.deepEqual(result.items.map(r=>r.ref),["1","2","3"]);
});
test("PR18 deletion HEAD keeps paged history below its oldest tuple",()=>{
  const old=applyCommentPage(emptyComments(),deletionPage(deletionRows),false);
  const result=applyCommentPage(old,deletionPage([deletionRows[3]]),false,true);
  assert.deepEqual(result.items.map(r=>r.ref),["1","2","3","4"]);
});
test("PR18 deletion is never inferred from incremental or older pages",()=>{
  const old=applyCommentPage(emptyComments(),deletionPage(deletionRows),false);
  for(const older of [false,true]){
    const result=applyCommentPage(old,deletionPage([deletionRows[2]]),older);
    assert.deepEqual(result.items.map(r=>r.ref),["1","2","3","4"]);
  }
  assert.deepEqual(applyCommentPage(old,deletionPage([deletionRows[2]]),true,true).items,old.items);
});
test("PR18 empty deletion HEAD is not evidence that any ref was deleted",()=>{
  const old=applyCommentPage(emptyComments(),deletionPage(deletionRows),false);
  assert.deepEqual(applyCommentPage(old,deletionPage([]),false,true).items,old.items);
});
test("PR18 deletion floor compares equal timestamp refs and timezone instants",()=>{
  const items=deletionRows.map(r=>({...r,created_at:"2026-10-09T00:00:00Z"}));
  const old=applyCommentPage(emptyComments(),deletionPage(items),false);
  const result=applyCommentPage(old,deletionPage([{...items[2],created_at:"2026-10-09T08:00:00+08:00"}]),false,true);
  assert.deepEqual(result.items.map(r=>r.ref),["1","2","3"]);
});
test("PR18 mixed FB and IG page orders normalize chronologically after dedupe", () => {
  const row=(ref:string,created_at:string)=>({ref,created_at,text:ref});
  const a=row("1","2026-10-09T00:00:01Z"),b=row("2","2026-10-09T00:00:02Z"),c=row("3","2026-10-09T08:00:02+08:00"),d=row("4","2026-10-09T00:00:03Z");
  const page=(items:unknown[])=>({epoch:1,reset:false,items,next:{epoch:1,seq:4},older_cursor:null}) as never;
  let fb=applyCommentPage(emptyComments(),page([c,b]),false);
  assert.deepEqual(fb.items.map(r=>r.ref),["2","3"],"equal instants use ref order, not provider insertion order");
  fb=applyCommentPage(fb,page([d,{...b,text:"updated"}]),false);
  assert.deepEqual(fb.items.map(r=>r.ref),["2","3","4"]);
  fb=applyCommentPage(fb,page([a,b]),true);
  const ig=applyCommentPage(emptyComments(),page([a,b,c,d]),false);
  assert.deepEqual(fb.items.map(r=>r.ref),["1","2","3","4"]);
  assert.deepEqual(fb.items.map(r=>r.ref),ig.items.map(r=>r.ref));
  assert.equal(fb.items.find(r=>r.ref==="2")?.text,"updated","older overlap must not replace newer marks/text");
});
for(const older of [false,true])test(`PR18 cap keeps newest1000 with mixed page order older=${older}`,()=>{
  const rows=Array.from({length:1002},(_,i)=>({ref:String(i+1),created_at:new Date(1000*i).toISOString()}));
  const page=(items:unknown[])=>({epoch:1,reset:false,items,next:{epoch:1,seq:1002},older_cursor:"more"}) as never;
  let old=emptyComments();
  for(let offset=3;offset<rows.length;offset+=100)
    old=applyCommentPage(old,page(rows.slice(offset,offset+100).reverse()),false);
  const merged=applyCommentPage(old,page([rows[2],rows[0],rows[1]]),older);
  assert.deepEqual(merged.items.map(r=>r.ref),rows.slice(2).map(r=>r.ref));
  const recent=applyCommentPage(merged,page([{ref:"1003",created_at:new Date(1002000).toISOString()}]),false);
  assert.equal(recent.items[0].ref,"4");assert.equal(recent.items.at(-1)?.ref,"1003");assert.equal(recent.items.length,1000);
});
test("PR18 A3 invalid_ref remains a definite 422 refusal", () => {
  assert.equal(commentErrorCode(422, { code: "invalid_ref" }), "invalid_ref");
});
test("PR18 a full history buffer stops paging without discarding rows or advancing its cursor", () => {
  const old = { ...emptyComments(), epoch: 1, items: Array.from({length:1000}, (_,i)=>({ref:String(i+100)})), next:{epoch:1,seq:1200}, older:"original" };
  const incoming = { epoch:1,reset:false,items:[{ref:"98"},{ref:"99"}],next:{epoch:1,seq:99},older_cursor:"skipped" };
  const result=applyCommentPage(old as never,incoming as never,true);
  assert.deepEqual(result.items,old.items);assert.equal(result.older,"original");assert.deepEqual(result.next,old.next);
});
test("K3 reply text rejects every C0/C1 control except newline",()=>{
  const p=`${path}/123/private-reply`;
  for(const n of [...Array(32).keys(),...Array.from({length:33},(_,i)=>127+i)]) {
    assert.equal(validCommentBody(p,JSON.stringify({text:`before${String.fromCharCode(n)}after`})),n===10,`control U+${n.toString(16)}`);
  }
});
const signedCursor = `eyJ0ZXN0IjoxfQ.${"a".repeat(43)}`;
test("PR18 approved seq DTO parser accepts safe numbers and explicit history null",()=>{
  const row={ref:"1",seq:null,parent_ref:null,created_at:"2026-10-09T00:00:00Z",author_name:null,text:"Synthetic DTO",is_page:false,has_attachment:false,marks:{intake:null,claim:null,private_reply:null,printed:null,public_replies:0,private_reply_available:true}};
  const envelope=(item:unknown)=>({epoch:1,reset:false,items:[item],next:{epoch:1,seq:1},older_cursor:null,stream:{source_platform:"facebook",video_embeddable:true}});
  for(const seq of [0,1,Number.MAX_SAFE_INTEGER,null])
    assert.equal(parseCommentPage(envelope({...row,seq})).items[0].seq,seq);
  for(const seq of [undefined,-1,1.5,"1",NaN,Infinity,Number.MAX_SAFE_INTEGER+1])
    assert.throws(()=>parseCommentPage(envelope({...row,seq})),/invalid_comment_response/);
  const {seq,...missing}=row;assert.throws(()=>parseCommentPage(envelope(missing)),/invalid_comment_response/);
});
test("SQL operation marks map to every merchant-visible delivery state",()=>{
  assert.deepEqual(["READY","DISPATCHING","SUCCEEDED","ACKNOWLEDGED","FAILED_FINAL","BLOCKED_POLICY","STALE_BINDING","CANCELLED","UNKNOWN"].map(commentSendState),["queued","queued","sent","sent","failed","blocked","blocked","blocked","unknown"]);
});
test("real bridge sealCursor uses payload.signature rather than plain base64url", () => {
  assert.equal(
    validCommentRequest(req(`?before_cursor=${signedCursor}`), path),
    true,
  );
  const value = {
    epoch: 1,
    reset: false,
    items: [],
    next: { epoch: 1, seq: 0 },
    older_cursor: signedCursor,
    stream: {
      state: "live",
      source_platform: "facebook",
      video_embeddable: true,
    },
  };
  assert.equal(parseCommentPage(value).older_cursor, signedCursor);
});
const req = (query = "", method = "GET") =>
  new Request(`https://console.invalid/${path}${query}`, {
    method,
    ...(method === "POST"
      ? { headers: { "Idempotency-Key": "synthetic-key" } }
      : {}),
  });
test("A2 grammar refuses private/duplicate/ambiguous cursors and keys on reads", () => {
  assert.equal(
    validCommentRequest(req("?after_epoch=0&after_seq=7"), path),
    true,
    "Instagram webhook fallback epoch is zero",
  );
  assert.equal(commentRoute("GET", path), true);
  for (const q of [
    "",
    "?limit=50",
    "?after_epoch=2&after_seq=0",
    "?before_cursor=eyJ0ZXN0IjoxfQ&limit=100",
  ])
    assert.equal(validCommentRequest(req(q), path), true, q);
  for (const q of [
    "?text=private",
    "?limit=0",
    "?limit=101",
    "?limit=1&limit=2",
    "?after_epoch=2",
    "?after_seq=1",
    "?after_epoch=2&after_seq=1&before_cursor=abc",
    "?before_cursor=a/b",
    "?",
    "?limit=%xx",
  ])
    assert.equal(validCommentRequest(req(q), path), false, q);
  assert.equal(
    validCommentRequest(
      new Request(req(), { headers: { "Idempotency-Key": "synthetic-key" } }),
      path,
    ),
    false,
  );
});
test("A3-A5 exact comment ref and bodies; explicit preemption private only", () => {
  for (const suffix of ["print", "private-reply", "public-reply"]) {
    assert.equal(commentRoute("POST", `${path}/123_456/${suffix}`), true);
    assert.equal(commentRoute("GET", `${path}/123_456/${suffix}`), false);
  }
  assert.equal(commentRoute("POST", `${path}/${sid}/private-reply`), false);
  assert.equal(validCommentBody(`${path}/123/print`, "{}"), true);
  for (const mode of ["private-reply", "public-reply"]) {
    const p = `${path}/123/${mode}`;
    assert.equal(
      validCommentBody(p, JSON.stringify({ text: "Synthetic reply" })),
      true,
    );
    assert.equal(
      validCommentBody(
        p,
        JSON.stringify({ template_id: "reply/thanks", template_version: 1 }),
      ),
      true,
    );
    for (const body of [
      { text: "", tenant_id: sid },
      { text: "x", unexpected: true },
      { text: "x", template_id: "x", template_version: 1 },
      { text: "x", confirm_preempt_auto: false },
    ])
      assert.equal(validCommentBody(p, JSON.stringify(body)), false);
    assert.equal(
      validCommentBody(
        p,
        JSON.stringify({ text: "x", confirm_preempt_auto: true }),
      ),
      mode === "private-reply",
    );
  }
});
test("A4/A5 preserve frozen refusal codes, never provider/debug text", () => {
  for (const code of [
    "used",
    "auto_pending",
    "auto_pending_confirm",
    "expired_7d",
    "ig_live_ended",
    "capability",
    "duplicate_recent",
    "comment_facts_unavailable",
    "comment_unknown",
    "page_comment",
    "reply_comment_unsupported",
    "ig_live_unsupported",
  ])
    assert.equal(
      commentErrorCode(409, { code, message: "not forwarded" }),
      code,
    );
  assert.equal(
    commentErrorCode(422, { code: "public_reply_forbidden_content" }),
    "public_reply_forbidden_content",
  );
  assert.equal(
    commentErrorCode(422, { code: "ig_live_unsupported" }),
    "ig_live_unsupported",
  );
  assert.equal(commentErrorCode(503, { code: "raw_secret" }), null);
});
test("reset invalidates old buffers and cursor before reread; older page cannot advance live cursor", () => {
  const old = {
    ...emptyComments(),
    epoch: 1,
    next: { epoch: 1, seq: 9 },
    items: [{ ref: "101" }],
  };
  const reset = applyCommentPage(
    old as never,
    {
      epoch: 2,
      reset: true,
      items: [],
      next: { epoch: 2, seq: 0 },
      older_cursor: null,
    } as never,
    false,
  );
  assert.deepEqual(reset.items, []);
  assert.equal(reset.next, null);
  assert.equal(reset.reset, true);
  const fresh = applyCommentPage(
    emptyComments(),
    {
      epoch: 2,
      reset: false,
      items: [{ ref: "102" }],
      next: { epoch: 2, seq: 7 },
      older_cursor: "older",
    } as never,
    false,
  );
  const older = applyCommentPage(
    fresh,
    {
      epoch: 2,
      reset: false,
      items: [{ ref: "100" }],
      next: { epoch: 2, seq: 3 },
      older_cursor: null,
    } as never,
    true,
  );
  assert.deepEqual(older.next, { epoch: 2, seq: 7 });
  assert.equal(older.items.length, 2);
  assert.equal(
    applyCommentPage(
      fresh,
      {
        epoch: 3,
        reset: false,
        items: [],
        next: { epoch: 3, seq: 0 },
        older_cursor: null,
      } as never,
      true,
    ).items.length,
    0,
  );
});
test("filter resources have only authorised opaque ids; backoff is capped and success resets", () => {
  assert.equal(commentViewResource("all", sid), null);
  assert.equal(commentViewResource("keyword", sid), null);
  assert.equal(
    commentViewResource("private", sid),
    `inbox/conversations?filter=live_comment&session_id=${sid}`,
  );
  assert.equal(
    commentViewResource("unreplied", sid),
    `inbox/conversations?filter=live_comment&session_id=${sid}`,
  );
  assert.deepEqual(
    [0, 1, 2, 3, 4, 5, 6].map(commentDelay),
    [3000, 3000, 6000, 12000, 24000, 30000, 30000],
  );
});
