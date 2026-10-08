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
} from "./inbox-review-host.test.ts";
const { useCommentStream } =
  await import("../../apps/admin/src/features/live/use-comment-stream.ts");
const { CommentReply } =
  await import("../../apps/admin/components/CommentReply.tsx");
const { CommentStream } =
  await import("../../apps/admin/components/CommentStream.tsx");
const { commentCopy } =
  await import("../../apps/admin/src/features/live/comment-copy.ts");
const sid = "22222222-2222-4222-8222-222222222222";
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
  for(const delay of [3000,6000,12000,24000,30000]){const previous=reads;t.mock.timers.tick(delay-1);await h.settle();assert.equal(reads,previous);t.mock.timers.tick(1);await h.settle();assert.equal(reads,previous+1);}
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
test("real SQL UNKNOWN mark blocks public-mode switching without a local receipt",async t=>{
  const env=environment(t);globalThis.fetch=async()=>response({items:[]});
  const h=env.mount(()=>CommentReply({store,session:sid,comment:{...row,marks:{...row.marks,private_reply:{kind:"manual",state:"UNKNOWN",blocked_reason:null},private_reply_available:false,private_reply_unavailable_reason:"auto_pending"}},locale:"en",platform:"facebook",capabilities:{facebook:{private_reply:{state:"ok",reason:"ok",evidence:"MOCK",checked_at:null},reply_public:{state:"ok",reason:"ok",evidence:"MOCK",checked_at:null}}},onSent(){},onDenied(){}} as any));
  await h.settle();assert.ok(textOf(h.output).includes(commentCopy("en").verify));
  assert.equal(node(h,n=>n.type==="button"&&textOf(n)==="Public reply").props.disabled,true);
});
test("UNKNOWN and queued public receipt survive selection remount without storing private identifiers", async (t) => {
  const env = environment(t);
  let sends = 0;
  globalThis.fetch = async (_url, init) => {
    if (init?.method === "POST") {
      sends++;
      return response({
        send_state: "queued",
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
  h.dispose();
  h = env.mount(() =>
    CommentReply({ ...props, comment: { ...row, ref: "987654" } } as any),
  );
  await h.settle();
  assert.equal(
    nodes(h.output).find((n) => n.type === "fieldset")?.props.disabled,
    true,
    "queued receipt blocks new replies after navigation until verified",
  );
  assert.equal(stored.size, 1);
  for (const [key, value] of stored) {
    assert.doesNotMatch(key, /123_456|987654|Synthetic/);
    assert.equal(value, "1");
  }
});
const row = {
  ref: "123_456",
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
  next: { epoch, seq: items.length },
  older_cursor: null,
  stream: {
    state: "live",
    source_platform: "facebook",
    video_embeddable: true,
  },
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
