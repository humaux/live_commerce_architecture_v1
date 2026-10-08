// Purpose: LC-U2a closed transport and cursor/privacy counterexamples (MOCK).
// Depends on: comment-request and comment-model; node:test/assert.
// Used by: test-node and console stream acceptance; no real comment payloads.
import assert from "node:assert/strict";
import { test } from "node:test";
import { commentRoute, validCommentRequest, validCommentBody, commentErrorCode } from "../../apps/admin/src/features/live/comment-request.ts";
import { applyCommentPage, commentViewResource, commentDelay, emptyComments } from "../../apps/admin/src/features/live/comment-model.ts";

const sid = "22222222-2222-4222-8222-222222222222";
const path = `live-sessions/${sid}/comments`;
const req = (query = "", method = "GET") => new Request(`https://console.invalid/${path}${query}`, { method, ...(method === "POST" ? {headers:{"Idempotency-Key":"synthetic-key"}}:{}) });
test("A2 grammar refuses private/duplicate/ambiguous cursors and keys on reads", () => {
  assert.equal(commentRoute("GET", path), true);
  for (const q of ["", "?limit=50", "?after_epoch=2&after_seq=0", "?before_cursor=eyJ0ZXN0IjoxfQ&limit=100"]) assert.equal(validCommentRequest(req(q), path), true, q);
  for (const q of ["?text=private", "?limit=0", "?limit=101", "?limit=1&limit=2", "?after_epoch=2", "?after_seq=1", "?after_epoch=2&after_seq=1&before_cursor=abc", "?before_cursor=a/b", "?", "?limit=%xx"]) assert.equal(validCommentRequest(req(q), path), false, q);
  assert.equal(validCommentRequest(new Request(req(),{headers:{"Idempotency-Key":"synthetic-key"}}),path),false);
});
test("A3-A5 exact comment ref and bodies; explicit preemption private only", () => {
  for (const suffix of ["print","private-reply","public-reply"]) {
    assert.equal(commentRoute("POST", `${path}/123_456/${suffix}`),true);
    assert.equal(commentRoute("GET",`${path}/123_456/${suffix}`),false);
  }
  assert.equal(commentRoute("POST", `${path}/${sid}/private-reply`),false);
  assert.equal(validCommentBody(`${path}/123/print`, "{}"),true);
  for (const mode of ["private-reply","public-reply"]) {
    const p = `${path}/123/${mode}`;
    assert.equal(validCommentBody(p,JSON.stringify({text:"Synthetic reply"})),true);
    assert.equal(validCommentBody(p,JSON.stringify({template_id:"reply/thanks",template_version:1})),true);
    for (const body of [{text:"",tenant_id:sid},{text:"x",unexpected:true},{text:"x",template_id:"x",template_version:1},{text:"x",confirm_preempt_auto:false}]) assert.equal(validCommentBody(p,JSON.stringify(body)),false);
    assert.equal(validCommentBody(p,JSON.stringify({text:"x",confirm_preempt_auto:true})), mode === "private-reply");
  }
});
test("A4/A5 preserve frozen refusal codes, never provider/debug text", () => {
  for(const code of ["used","auto_pending","auto_pending_confirm","expired_7d","ig_live_ended","capability","duplicate_recent","comment_facts_unavailable","comment_unknown","page_comment","reply_comment_unsupported","ig_live_unsupported"])
    assert.equal(commentErrorCode(409,{code,message:"not forwarded"}),code);
  assert.equal(commentErrorCode(422,{code:"public_reply_forbidden_content"}),"public_reply_forbidden_content");
  assert.equal(commentErrorCode(422,{code:"ig_live_unsupported"}),"ig_live_unsupported");
  assert.equal(commentErrorCode(503,{code:"raw_secret"}),null);
});
test("reset invalidates old buffers and cursor before reread; older page cannot advance live cursor", () => {
  const old = {...emptyComments(),epoch:1,next:{epoch:1,seq:9},items:[{ref:"101"}]};
  const reset = applyCommentPage(old as never,{epoch:2,reset:true,items:[],next:{epoch:2,seq:0},older_cursor:null} as never,false);
  assert.deepEqual(reset.items,[]); assert.equal(reset.next,null); assert.equal(reset.reset,true);
  const fresh = applyCommentPage(emptyComments(),{epoch:2,reset:false,items:[{ref:"102"}],next:{epoch:2,seq:7},older_cursor:"older"} as never,false);
  const older = applyCommentPage(fresh,{epoch:2,reset:false,items:[{ref:"100"}],next:{epoch:2,seq:3},older_cursor:null} as never,true);
  assert.deepEqual(older.next,{epoch:2,seq:7}); assert.equal(older.items.length,2);
  assert.equal(applyCommentPage(fresh,{epoch:3,reset:false,items:[],next:{epoch:3,seq:0},older_cursor:null} as never,true).items.length,0);
});
test("filter resources have only authorised opaque ids; backoff is capped and success resets", () => {
  assert.equal(commentViewResource("all",sid),null); assert.equal(commentViewResource("keyword",sid),null);
  assert.equal(commentViewResource("private",sid),`inbox/conversations?filter=live_comment&session_id=${sid}`);
  assert.equal(commentViewResource("unreplied",sid),`inbox/conversations?filter=live_comment&session_id=${sid}`);
  assert.deepEqual([0,1,2,3,4,5,6].map(commentDelay),[3000,3000,6000,12000,24000,30000,30000]);
});
