// Purpose: persisted A9 UNKNOWN must retain the operator warning and refuse another reply after reload.
// Depends on: actual InboxThread, inbox client and ReplyReceipt under the generic MOCK hook host.
// Used by: LC-U2b Node gate; A12 queued ACK is not evidence of a final delivery outcome.
// Invariants: I07 no blind resend; I11 only synthetic DM fixtures; every wait observes committed state.
import test from "node:test";
import assert from "node:assert/strict";
import {
  environment, nodes, node, textOf, change, submit, clickText,
  store, conversation, thread, response, transport, ReplyReceipt, type Host,
} from "./inbox-review-host.test.ts";
const { InboxThread } = await import("../../apps/admin/components/InboxThread.tsx");
const warning: Record<string, string> = {
  en: "Delivery is uncertain. Check Messenger before taking another action; this reply is not automatically resent.",
  "zh-TW": "送達狀態不確定。請先到 Messenger 確認；系統不會自動重送。",
  "zh-CN": "送达状态不确定。请先到 Messenger 确认；系统不会自动重发。",
};
const projection = (state: string) => ({
  ...thread,
  items: [...thread.items, {
    direction: "out", at: "2030-01-01T00:00:01Z", text: `MOCK_OUTBOUND_${state}`,
    attachments: null, send_state: state, send_code: state === "unknown" ? "timeout" : null,
  }],
});
function mount(env: ReturnType<typeof environment>, locale = "en") {
  return env.mount(() => InboxThread({
    store, conversation, locale, onUnauthorized() {}, onRead() {},
  } as any));
}
const idle = (host: Host) => host.output?.props["aria-busy"] === false;
const sendControl = (host: Host) => node(host, (n) => n.props["data-testid"] === "reply-send");
const sends = (calls: ReturnType<typeof transport>) =>
  calls.filter((call) => call.method === "POST" && call.path.endsWith("/messages"));
async function loaded(host: Host, state: string) {
  await host.waitFor(() => idle(host) && textOf(host.output).includes(`MOCK_OUTBOUND_${state}`),
    "A9 projection and A10 authority read committed");
}

for (const locale of Object.keys(warning)) {
  test(`persisted A9 UNKNOWN after page entry warns and blocks the real composer in ${locale}`, async (t) => {
    const env = environment(t), calls = transport(() => response(projection("unknown")));
    const host = mount(env, locale);
    await loaded(host, "unknown");
    assert.ok(nodes(host.output).some((n) => n.props.role === "status" && textOf(n) === warning[locale]),
      "persisted UNKNOWN must show the full verify-Messenger warning, not only its short badge");
    change(host, "reply-text", "MOCK_NEW_DRAFT");
    assert.equal(sendControl(host).props.disabled, true, "loaded final UNKNOWN blocks new sends");
    node(host, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
    assert.equal(host.ref(ReplyReceipt).pending(), null, "disabled-form bypass cannot mint a new send receipt");
    assert.equal(sends(calls).length, 0);
  });
}

test("A12 queued ACK followed by final A9 UNKNOWN overrides the local queued notice and blocks another send", async (t) => {
  const env = environment(t);
  let reads = 0;
  const calls = transport(() => response(++reads === 1 ? projection("sent") : projection("unknown")),
    () => response({ send_state: "queued" }));
  const host = mount(env);
  await loaded(host, "sent");
  change(host, "reply-text", "MOCK_FIRST_SEND");
  submit(host);
  await loaded(host, "unknown");
  assert.equal(sends(calls).length, 1);
  assert.equal(reads, 2, "real send finally reloaded persisted delivery authority");
  assert.ok(textOf(host.output).includes(warning.en));
  change(host, "reply-text", "MOCK_NEXT_SEND");
  assert.equal(sendControl(host).props.disabled, true);
  node(host, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
  assert.equal(host.ref(ReplyReceipt).pending(), null);
  assert.equal(sends(calls).length, 1, "no second A12 for a persisted final UNKNOWN");
});

test("a captured previously enabled composer cannot bypass final UNKNOWN learned by Refresh", async (t) => {
  const env = environment(t);
  let reads = 0;
  const calls = transport(() => response(++reads === 1 ? projection("sent") : projection("unknown")));
  const host = mount(env);
  await loaded(host, "sent");
  change(host, "reply-text", "MOCK_CAPTURED_DRAFT");
  assert.equal(sendControl(host).props.disabled, false);
  const captured = node(host, (n) => n.type === "form").props.onSubmit;
  clickText(host, "Refresh");
  await loaded(host, "unknown");
  captured({ preventDefault() {} });
  assert.equal(host.ref(ReplyReceipt).pending(), null,
    "action-time authority must reject a callback captured before the final UNKNOWN read");
  assert.equal(sends(calls).length, 0);
});

for (const state of ["queued", "sent", "failed", "blocked"]) {
  test(`persisted ${state} is not mislabeled UNKNOWN and retains ordinary composer eligibility`, async (t) => {
    const env = environment(t);
    transport(() => response(projection(state)));
    const host = mount(env);
    await loaded(host, state);
    assert.equal(textOf(host.output).includes(warning.en), false);
    change(host, "reply-text", "MOCK_ELIGIBLE_DRAFT");
    assert.equal(sendControl(host).props.disabled, false);
  });
}
