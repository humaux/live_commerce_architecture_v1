// Purpose: independently reproduce null A9 attachments and distinguish empty A8 data from unavailable entry.
// Depends on: real Inbox/Thread/BuyerPanel/client with generic MOCK hook host; Node fs checks Go acceptance wiring.
// Used by: PR8 A/C focused Node tests. Component evidence is MOCK; Go wiring checks are STRUCTURE_ONLY, Go NOT_RUN.
// Invariants: I01 fail closed on 404; I11 synthetic fixtures only; I18 static wiring is not runtime acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  environment,
  nodes,
  node,
  textOf,
  store,
  conversation,
  thread,
  response,
  type Host,
} from "./inbox-review-host.test.ts";
const { Inbox } = await import("../../apps/admin/components/Inbox.tsx");
const buyer = {
  display_name: "MOCK_ENTRY_BUYER",
  platform: "messenger",
  purchase_ordinal: 0,
  claims: [],
  claim_total_minor: 0,
  orders: [],
  link_pending_manual: false,
};

function child(host: Host, testId: string): Host {
  for (const value of host.children.values()) {
    if (value.host.raw?.props?.["data-testid"] === testId) return value.host;
    try {
      return child(value.host, testId);
    } catch {
      /* Search the next generic component host. */
    }
  }
  throw new Error(`Actual mounted child host missing: ${testId}`);
}
function entryTransport(attachments: unknown, listStatus = 200, empty = false) {
  const calls: { path: string; method: string }[] = [];
  const observed = Promise.withResolvers<void>();
  globalThis.fetch = async (input, init) => {
    const path = String(input),
      method = init?.method ?? "GET";
    calls.push({ path, method });
    if (path.includes("/inbox/conversations?"))
      return response(
        listStatus === 200
          ? { items: empty ? [] : [conversation], next_cursor: "", unread_total: 0 }
          : { code: "not_found" },
        listStatus,
      );
    if (path.includes("/messages?")) {
      // Preserve the actual Go wire value. No fixture-side null-to-array rewrite.
      observed.resolve();
      return response({ ...thread, items: [{ ...thread.items[0], attachments }] });
    }
    if (path.includes("/inbox/buyer-panel?")) return response(buyer);
    if (path.endsWith("/message-templates")) return response({ items: [] });
    if (method === "POST" && path.endsWith("/read")) return response({ read_seq: 1 });
    throw new Error(`Unexpected MOCK entry route ${method} ${path}`);
  };
  return { calls, observed };
}
const listReads = (calls: { path: string; method: string }[]) =>
  calls.filter((call) => call.method === "GET" && call.path.includes("/inbox/conversations?")).length;
async function entry(env: ReturnType<typeof environment>, locale = "en", expectError = false) {
  const host = env.mount(() => Inbox({ locale, store, initialError: null } as any));
  await host.waitFor(
    () =>
      expectError
        ? nodes(host.output).some((n) => n.props.role === "alert")
        : nodes(host.output).some((n) => n.props["data-testid"] === "inbox-list" && n.props["aria-busy"] === false),
    "actual A8 entry response committed",
  );
  return host;
}
async function selectedThread(host: Host) {
  node(host, (n) => n.props["data-testid"] === `conversation-${conversation.conversation_id}`).props.onClick();
  host.flush();
  return child(host, "inbox-thread");
}

for (const attachments of [null, []]) {
  test(`A real Inbox selection admits Go A9 attachments ${attachments === null ? "null" : "empty array"} without losing thread`, async (t) => {
    const env = environment(t),
      transport = entryTransport(attachments);
    const host = await entry(env),
      threadHost = await selectedThread(host);
    await assert.doesNotReject(
      threadHost.waitFor(
        () => threadHost.output?.props["aria-busy"] === false && textOf(threadHost.output).includes("MOCK_DM"),
        "actual A9/A10 thread render",
      ),
      "valid null/empty attachment projection must render",
    );
    const buyerHost = child(host, "buyer-panel");
    await buyerHost.waitFor(
      () => buyerHost.output?.props["aria-busy"] === false && textOf(buyerHost.output).includes("MOCK_ENTRY_BUYER"),
      "actual selected A13 buyer render",
    );
    host.flush();
    assert.equal(listReads(transport.calls), 1);
    assert.equal(
      node(host, (n) => n.props["data-testid"] === `conversation-${conversation.conversation_id}`).props[
        "aria-pressed"
      ],
      true,
    );
    assert.ok(nodes(host.output).some((n) => n.props["data-testid"] === "inbox-thread"));
    assert.ok(textOf(host.output).includes("MOCK_DM"));
    assert.equal(
      nodes(threadHost.output).filter((n) => n.type === "li" && n.props["data-direction"] === "in").length,
      1,
    );
  });
}

test("A malformed non-array A9 attachments is refused without painting DM or crashing selected entry", async (t) => {
  const env = environment(t);
  entryTransport({ type: "MOCK_BAD_ATTACHMENT_SHAPE" });
  const host = await entry(env),
    threadHost = await selectedThread(host);
  await assert.doesNotReject(
    threadHost.waitFor(() => threadHost.output?.props["aria-busy"] === false, "malformed A9 response refused"),
    "malformed arrays fail closed, not a component exception",
  );
  host.flush();
  assert.ok(nodes(host.output).some((n) => n.props["data-testid"] === "inbox-thread"));
  assert.equal(
    textOf(host.output).includes("MOCK_DM"),
    false,
    "non-array attachment payload must not be normalized into admitted data",
  );
  assert.ok(
    nodes(threadHost.output).some((n) => n.props.role === "alert"),
    "invalid response gives a bounded safe error",
  );
});

test("C A8 HTTP200 empty list is ordinary data with no alert, selection or focus request storm", async (t) => {
  const env = environment(t),
    transport = entryTransport([], 200, true);
  const host = await entry(env);
  for (let n = 0; n < 3; n++) {
    host.dirty = true;
    host.flush();
    env.window.dispatchEvent(new Event("focus"));
    host.flush();
  }
  assert.equal(listReads(transport.calls), 1);
  assert.equal(nodes(host.output).filter((n) => n.props.role === "alert").length, 0);
  assert.equal(
    nodes(host.output).some((n) => ["inbox-thread", "buyer-panel"].includes(n.props["data-testid"])),
    false,
  );
  assert.ok(nodes(host.output).some((n) => n.props["data-testid"] === "inbox-list"));
});

const unavailable = {
  en: "The inbox is unavailable for this store.",
  "zh-TW": "目前店鋪的收件匣無法使用。",
  "zh-CN": "当前店铺的收件箱无法使用。",
};
const signedOut: Record<string, string> = {
  en: "Sign in again to read messages.",
  "zh-TW": "請重新登入以查看訊息。",
  "zh-CN": "请重新登录以查看消息。",
};
for (const [locale, expected] of Object.entries(unavailable)) {
  test(`C A8 list404 is scoped inbox unavailable in ${locale}, never an empty list or conversation denial`, async (t) => {
    const env = environment(t),
      transport = entryTransport([], 404);
    const host = await entry(env, locale, true);
    for (let n = 0; n < 3; n++) {
      host.dirty = true;
      host.flush();
      env.window.dispatchEvent(new Event("focus"));
      host.flush();
    }
    const alerts = nodes(host.output).filter((n) => n.props.role === "alert");
    assert.equal(alerts.length, 1);
    assert.equal(textOf(alerts[0]), expected);
    assert.equal(textOf(host.output).includes(signedOut[locale]), false,
      "authenticated unavailable inbox must not suggest a lost sign-in");
    assert.equal(listReads(transport.calls), 1, "scope failure remains fenced across render/focus");
    assert.equal(
      nodes(host.output).some((n) => ["inbox-list", "inbox-thread", "buyer-panel"].includes(n.props["data-testid"])),
      false,
      "404 is not fabricated as successful empty-list data",
    );
  });
}

test("C click-sweep Go harness mounts existing Inbox/templates services (STRUCTURE_ONLY, Go NOT_RUN)", () => {
  const source = readFileSync(new URL("../foundation/browser_click_sweep_test.go", import.meta.url), "utf8").replace(
    /\/\/[^\n]*/g,
    "",
  );
  const options = source.match(/options\s*:=\s*httpapi\.Options\{([\s\S]*?)\}/)?.[1] ?? "";
  assert.ok(/\bInbox:\s*(?!nil\b)\w+/.test(options), "click-sweep actual router Options must mount real inbox service");
  assert.ok(/\bMsgTemplates:\s*(?!nil\b)\w+/.test(options), "published template read service must be mounted");
  assert.ok(/inbox\.NewService\s*\(/.test(source), "use the existing inbox service constructor");
  assert.ok(/msgtemplates\.NewService\s*\(/.test(source), "use the existing templates constructor");
});
test("C click-sweep includes actual-router owner200 empty inbox assertion (STRUCTURE_ONLY, Go NOT_RUN)", () => {
  const source = readFileSync(new URL("../foundation/browser_click_sweep_test.go", import.meta.url), "utf8").replace(
    /\/\/[^\n]*/g,
    "",
  );
  const call = source.match(
    /call\(http\.MethodGet,\s*"\/inbox\/conversations[^"\n]*",\s*"",\s*nil,\s*(?:http\.StatusOK|200),\s*&(\w+)\)/,
  );
  assert.ok(call, "existing owner call helper must assert real A8 GET200 through mounted router");
  assert.ok(
    /api\s*:=\s*httpapi\.NewHandler\(f\.runtime,\s*options\)/.test(source) && /api\.ServeHTTP\(w,\s*req\)/.test(source),
    "GET proof must use the constructed real router",
  );
  assert.ok(
    /req\.Header\.Set\("Authorization",\s*"Bearer "\+e\.token\(\)\)/.test(source),
    "existing signed owner scope used",
  );
  const name = call![1];
  assert.ok(
    new RegExp(`len\\(${name}\\.Items\\)\\s*!=\\s*0`).test(source),
    "empty list must be asserted from actual response fields",
  );
  assert.ok(
    new RegExp(`${name}\\.Items\\s*==\\s*nil`).test(source),
    "Go null slice must not stand in for canonical empty items[]",
  );
});
