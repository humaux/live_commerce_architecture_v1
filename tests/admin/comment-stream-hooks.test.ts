// Purpose: run actual LC-U2a hooks/composer with fake timers and privacy counterexamples (MOCK).
// Depends on: inbox real-source hook host, production comment hook/reply component and Node timers.
// Used by: test-node; complements real Go/PG browser gates rather than replacing them.
import test from "node:test";
import assert from "node:assert/strict";
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
const { commentCopy } =
  await import("../../apps/admin/src/features/live/comment-copy.ts");
const sid = "22222222-2222-4222-8222-222222222222";
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
