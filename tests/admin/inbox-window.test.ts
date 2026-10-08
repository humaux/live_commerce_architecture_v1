// Purpose: independently verify the strict DM safety cutoff and quota-recovery instructions in actual components.
// Depends on: generic MOCK Node hook-host, real InboxThread/BundleRecovery, real inbox/claims clients.
// Used by: LC-U2b window/copy Node gate; SQL and Go are frozen, their runtime acceptance is NOT_RUN here.
// Invariants: I07 send eligibility rechecked at action time; I11 no real DM, customer or credential fixture.
import test from "node:test";
import assert from "node:assert/strict";
import {
  environment,
  nodes,
  node,
  textOf,
  change,
  store,
  conversation,
  thread,
  response,
  transport,
  type Host,
} from "./inbox-review-host.test.ts";
const { InboxThread } = await import("../../apps/admin/components/InboxThread.tsx");
const { BundleRecovery } = await import("../../apps/admin/src/features/messages/bundle-recovery.tsx");
const hardEnd = Date.parse("2030-01-02T00:00:00Z");
const safetyMargin = 5 * 60 * 1000; // frozen live-console-v1 §3.3 / §4.1, not a production helper.
type Deadline = { delay: number; callback: () => void; handle: ReturnType<typeof setTimeout> };

function clock(t: any, initial: number) {
  let now = initial;
  const originalNow = Date.now,
    originalTimeout = globalThis.setTimeout;
  const deadlines: Deadline[] = [];
  Date.now = () => now;
  // Capture only the known fixture's cutoff/hard-end timers. Host.waitFor's 5000ms bound stays real.
  // Holding these timers prevents a fixed fake clock from making a 1ms production timer spin forever.
  globalThis.setTimeout = ((callback: (...args: any[]) => void, delay?: number, ...args: any[]) => {
    if (typeof delay === "number" && (delay <= 1 || delay === safetyMargin)) {
      const handle = originalTimeout(() => {}, 2147483647);
      deadlines.push({ delay, handle, callback: () => callback(...args) });
      return handle;
    }
    return originalTimeout(callback, delay, ...args);
  }) as typeof setTimeout;
  t.after(() => {
    Date.now = originalNow;
    globalThis.setTimeout = originalTimeout;
    for (const deadline of deadlines) clearTimeout(deadline.handle);
  });
  return {
    deadlines,
    set(value: number) {
      now = value;
    },
    run(deadline: Deadline) {
      clearTimeout(deadline.handle);
      deadline.callback();
    },
  };
}
const sendControl = (host: Host) => node(host, (n) => n.props["data-testid"] === "reply-send");
const closingNotice = (host: Host) => nodes(host.output).find((n) => n.props["data-testid"] === "inbox-window-closing");
async function loadedThread(env: ReturnType<typeof environment>, windowEnd: string) {
  const calls = transport(
    () => response({ ...thread, window_open_until: windowEnd }),
    () => response({ code: "window_closed" }, 409),
  );
  const host = env.mount(() =>
    InboxThread({ store, conversation, locale: "en", onUnauthorized() {}, onRead() {} } as any),
  );
  await host.waitFor(
    () => host.output?.props["aria-busy"] === false && textOf(host.output).includes("MOCK_DM"),
    "actual thread and read receipt committed",
  );
  change(host, "reply-text", "MOCK_CUTOFF_REPLY");
  return { host, calls };
}

for (const remaining of [safetyMargin + 1, safetyMargin, 60_000, 0]) {
  test(`window strict eligibility with ${remaining}ms before hard 24h deadline`, async (t) => {
    const env = environment(t);
    clock(t, hardEnd - remaining);
    const { host } = await loadedThread(env, new Date(hardEnd).toISOString());
    assert.equal(
      sendControl(host).props.disabled,
      remaining <= safetyMargin,
      "SQL cutoff is strict, including equality",
    );
    if (remaining > 0 && remaining <= safetyMargin) {
      const notice = closingNotice(host);
      assert.ok(notice, "closing state has a distinct visible explanation");
      assert.match(textOf(notice), /clos|five minute|5 minute/i);
    } else assert.equal(!!closingNotice(host), false, "hard closure and open state differ from safety-margin closing");
    if (remaining === 0) assert.ok(textOf(host.output).includes("The reply window is closed."));
  });
}

test("window invalid server deadline fails closed without preparing a send", async (t) => {
  const env = environment(t);
  clock(t, hardEnd - safetyMargin - 1);
  const { host, calls } = await loadedThread(env, "not-a-date");
  assert.equal(sendControl(host).props.disabled, true);
  assert.equal(
    calls.some((call) => call.method === "POST" && call.path.endsWith("/messages")),
    false,
  );
});

test("window captured enabled form callback cannot POST after crossing safe cutoff", async (t) => {
  const env = environment(t),
    time = clock(t, hardEnd - safetyMargin - 1);
  const { host, calls } = await loadedThread(env, new Date(hardEnd).toISOString());
  assert.equal(sendControl(host).props.disabled, false);
  const captured = node(host, (n) => n.type === "form").props.onSubmit;
  time.set(hardEnd - safetyMargin); // No render or timer tick: invoke the actual earlier event handler.
  captured({ preventDefault() {} });
  await host.waitFor(() => host.output?.props["aria-busy"] === false, "captured submit decision committed");
  assert.equal(
    calls.filter((call) => call.method === "POST" && call.path.endsWith("/messages")).length,
    0,
    "the current clock must be rechecked before key preparation or POST",
  );
});

test("window deadline timer closes controls at cutoff and hard end without waiting for 15s poll", async (t) => {
  const env = environment(t),
    time = clock(t, hardEnd - safetyMargin - 1);
  const { host } = await loadedThread(env, new Date(hardEnd).toISOString());
  assert.equal(sendControl(host).props.disabled, false);
  const cutoffTimer = time.deadlines.find((deadline) => deadline.delay <= 1);
  assert.ok(cutoffTimer, "actual component schedules imminent cutoff instead of retaining enabled control for 15s");
  time.set(hardEnd - safetyMargin);
  time.run(cutoffTimer);
  host.flush();
  assert.equal(sendControl(host).props.disabled, true);
  assert.ok(closingNotice(host));
  const hardTimer = time.deadlines.find((deadline) => deadline.delay === safetyMargin);
  assert.ok(hardTimer, "closing state schedules its hard-expiry transition");
  time.set(hardEnd);
  time.run(hardTimer);
  host.flush();
  assert.equal(!!closingNotice(host), false);
  assert.ok(textOf(host.output).includes("The reply window is closed."));
});

const instructions = {
  "zh-TW": "此買家的認領連結無法送出：這則留言的私訊額度已用。買家回覆私訊後 24 小時內可用一般私訊補發認領連結",
  "zh-CN": "此买家的认领链接无法发送：这条评论的私信额度已用。买家回复私信后 24 小时内可用普通私信补发认领链接",
  en: "This buyer’s claim link could not be sent because this comment’s private-reply quota has been used. After the buyer replies by DM, you can resend the claim link using a regular DM within 24 hours.",
};
for (const [locale, expected] of Object.entries(instructions)) {
  test(`quota recovery gives exact ${locale} comment-quota and DM-window instructions beside copy only`, (t) => {
    const env = environment(t);
    const recoveryStore = { ...store, role: "staff", permissions: ["inbox:read", "live:read", "live:manage"] };
    const row = {
      ...conversation,
      conversation_id: null,
      bundle_id: "40000000-0000-4000-8000-000000000004",
      session_id: "30000000-0000-4000-8000-000000000003",
      link_pending_manual: true,
    };
    const host = env.mount(() =>
      BundleRecovery({ store: recoveryStore, conversation: row, locale, onUnauthorized() {} } as any),
    );
    const text = nodes(host.output)
      .filter((n) => n.type === "p")
      .map(textOf);
    assert.ok(text.includes(expected), "operator receives supplied literal instructions, not generic manual-link text");
    const controls = nodes(host.output).filter((n) => n.type === "button");
    assert.equal(controls.length, 1);
    assert.equal(controls[0].props["data-testid"], "bundle-copy-link");
    assert.equal(controls[0].props.disabled, false);
    assert.equal(
      nodes(host.output).some((n) => n.type === "form" || n.props["data-testid"] === "reply-send"),
      false,
    );
  });
}
