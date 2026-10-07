// Purpose: independent PR8 counterexamples execute actual inbox hook/component handlers and render state.
// Depends on: Node registerHooks/test/assert, installed typescript-api, real InboxFence/ReplyReceipt/client.
// Used by: LC-U2b focused Node gate; MOCK hook host is not React/browser or backend acceptance.
// Invariants: I02, I06, I11, I18; no production privacy rule is copied or replaced here.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks, createRequire } from "node:module";
import { readFileSync, existsSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import { resolve, dirname } from "node:path";
const require = createRequire(import.meta.url);
const ts = require("typescript-api");
const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
type VNode = { type: unknown; props: Record<string, any> };
type Slot = { value?: any; deps?: unknown[]; cleanup?: () => void };
let active: Host;
const same = (a?: unknown[], b?: unknown[]) =>
  !!a && !!b && a.length === b.length && a.every((v, i) => Object.is(v, b[i]));
// A generic synchronous hook scheduler: it only implements hook storage/dependency ordering.
// Privacy, fencing, submission receipts and all component state transitions remain real source.
class Host {
  slots: Slot[] = []; cursor = 0;
  dirty = true; effects: (() => void)[] = [];
  output: any; render: () => any;
  constructor(render: () => any) { this.render = render; }
  flush() {
    let renders = 0;
    while (this.dirty) {
      assert.ok(++renders < 50, "hook host render loop");
      this.dirty = false; this.cursor = 0; active = this;
      this.output = this.render();
      const effects = this.effects.splice(0);
      for (const effect of effects) effect();
    }
    return this.output;
  }
  async settle() {
    // WebCrypto session fences and Promise-finally reads yield to the event loop.
    for (let i = 0; i < 12; i++) {
      this.flush(); await new Promise<void>((done) => setImmediate(done));
    }
    this.flush();
  }
  ref<T>(klass: new (...args: any[]) => T): T {
    const found = this.slots.find((slot) => slot.value?.current instanceof klass);
    assert.ok(found, "real production ref exists");
    return found.value.current;
  }
  dispose() { for (const slot of this.slots) slot.cleanup?.(); }
}
const react = {
  useState(initial: any) {
    const host = active, index = host.cursor++;
    host.slots[index] ??= { value: typeof initial === "function" ? initial() : initial };
    return [host.slots[index].value, (next: any) => {
      const slot = host.slots[index];
      const value = typeof next === "function" ? next(slot.value) : next;
      if (!Object.is(value, slot.value)) { slot.value = value; host.dirty = true; }
    }];
  },
  useRef(initial: any) {
    const host = active, index = host.cursor++;
    host.slots[index] ??= { value: { current: initial } };
    return host.slots[index].value;
  },
  useCallback(callback: any, deps: unknown[]) {
    const host = active, index = host.cursor++;
    if (!same(host.slots[index]?.deps, deps)) host.slots[index] = { value: callback, deps };
    return host.slots[index].value;
  },
  useEffect(effect: () => any, deps: unknown[]) {
    const host = active, index = host.cursor++;
    const prior = host.slots[index];
    if (same(prior?.deps, deps)) return;
    host.slots[index] = { deps, cleanup: prior?.cleanup };
    host.effects.push(() => {
      prior?.cleanup?.();
      host.slots[index].cleanup = effect();
    });
  },
};
const runtime = { ...react, jsx: (type: unknown, props: any): VNode => ({ type, props }), Fragment: Symbol("fragment") };
(globalThis as any).__inboxReviewRuntime = runtime;
const mock = (code: string) => `data:text/javascript,${encodeURIComponent(code)}`;
const runtimeURL = mock(`const r=globalThis.__inboxReviewRuntime;
export const {useState,useRef,useCallback,useEffect,jsx,Fragment}=r;export const jsxs=jsx;`);
const adapters: Record<string, string> = {
  react: runtimeURL,
  "react/jsx-runtime": runtimeURL,
  "react-dom": mock("export const flushSync=callback=>callback();"),
  "next/navigation": mock('export const useParams=()=>({locale:"en"});'),
  "@live-commerce/format": mock(
    "export const displayTime=(_,value)=>value; export const money=(_,currency,value)=>`${currency} ${value}`;",
  ),
  "@/lib/meta-health-hook": mock(
    'export const useMetaHealth=()=>({data:{pages:[{capabilities:[{provider:"facebook",capability:"dm_session",state:"ok"}]}]}});',
  ),
};
registerHooks({
  resolve(specifier, context, next) {
    if (adapters[specifier]) return { url: adapters[specifier], shortCircuit: true };
    if (specifier.endsWith(".css")) return { url: mock("export default {};"), shortCircuit: true };
    let file: string | undefined;
    if (specifier.startsWith("@/")) file = resolve(root, "apps/admin", specifier.slice(2));
    else if (specifier.startsWith(".") && context.parentURL?.startsWith("file:")) {
      file = resolve(dirname(fileURLToPath(context.parentURL)), specifier);
    }
    if (file) {
      for (const suffix of ["", ".ts", ".tsx"]) {
        if (existsSync(file + suffix)) return { url: pathToFileURL(file + suffix).href, shortCircuit: true };
      }
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.startsWith("file:") && /\.tsx?$/.test(url)) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(fileURLToPath(url), "utf8"), {
          compilerOptions: {
            target: ts.ScriptTarget.ES2023,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
          fileName: fileURLToPath(url),
        }).outputText,
      };
    }
    return next(url, context);
  },
});
const { useInboxPrivacy } = await import("../../apps/admin/src/features/messages/use-privacy.ts");
const { InboxFence, ReplyReceipt } = await import("../../apps/admin/src/features/messages/privacy.ts");
const { InboxThread } = await import("../../apps/admin/components/InboxThread.tsx");
const { BuyerPanel } = await import("../../apps/admin/components/BuyerPanel.tsx");
const { InboxError } = await import("../../apps/admin/lib/inbox-client.ts");
class MockDocument extends EventTarget {
  visibilityState = "visible";
  cookie = `__Host-commerce_csrf=${"a".repeat(43)}`;
}
function environment(t: any) {
  const previous = {
    document: (globalThis as any).document,
    window: (globalThis as any).window,
    fetch: globalThis.fetch,
    BroadcastChannel: globalThis.BroadcastChannel,
  };
  const document = new MockDocument(),
    window = new EventTarget();
  Object.assign(globalThis, {
    document,
    window,
    BroadcastChannel: class extends EventTarget {
      close() {}
    },
  });
  const hosts: Host[] = [];
  t.after(() => {
    for (const host of hosts) host.dispose();
    Object.assign(globalThis, previous);
  });
  return {
    document,
    window,
    mount(render: () => any) {
      const host = new Host(render);
      hosts.push(host);
      host.flush();
      return host;
    },
  };
}
function nodes(tree: any): VNode[] {
  if (Array.isArray(tree)) return tree.flatMap(nodes);
  if (!tree || typeof tree !== "object" || !tree.props) return [];
  return [tree, ...nodes(tree.props.children)];
}
function textOf(tree: any): string {
  if (Array.isArray(tree)) return tree.map(textOf).join(" ");
  if (tree && typeof tree === "object") return textOf(tree.props?.children);
  return tree === null || tree === undefined || typeof tree === "boolean" ? "" : String(tree);
}
function node(host: Host, predicate: (node: VNode) => boolean) {
  const found = nodes(host.flush()).find(predicate);
  assert.ok(found, "actual component VNode found");
  return found;
}
function change(host: Host, id: string, value: string) {
  const input = node(host, (n) => n.props["data-testid"] === id);
  assert.equal(input.props.disabled, false, "actual input is enabled");
  input.props.onChange({ target: { value } });
  host.flush();
}
function submit(host: Host) {
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-send").props.disabled, false);
  node(host, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
  host.flush();
}
function clickText(host: Host, label: string) {
  const button = node(host, (n) => n.type === "button" && textOf(n) === label);
  assert.equal(button.props.disabled, false, "actual button is enabled");
  button.props.onClick();
  host.flush();
}
const store = {
  id: "10000000-0000-4000-8000-000000000001",
  name: "MOCK_STORE",
  currency: "TWD",
  role: "owner",
};
const conversation = {
  conversation_id: "20000000-0000-4000-8000-000000000002",
  platform: "messenger",
  display_name: "MOCK_NAME",
  last_at: "2030-01-01T00:00:00Z",
  unread: true,
  unreplied: true,
  mode: "auto",
  assignee: null,
  window_open_until: "2099-01-01T00:00:00Z",
  linked_customer_id: null,
};
const thread = {
  items: [
    {
      direction: "in",
      seq: 1,
      at: "2030-01-01T00:00:00Z",
      text: "MOCK_DM",
      attachments: [],
    },
  ],
  window_open_until: "2099-01-01T00:00:00Z",
  mode: "auto",
  takeover_generation: 7,
  human_until: null,
};
const response = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
    },
  });
function transport(read: () => any, send?: () => any) {
  const calls: {
    path: string;
    method: string;
    body?: string;
    key: string | null;
  }[] = [];
  globalThis.fetch = async (input, init) => {
    const path = String(input),
      method = init?.method ?? "GET";
    calls.push({
      path,
      method,
      body: init?.body as string | undefined,
      key: new Headers(init?.headers).get("Idempotency-Key"),
    });
    if (path.endsWith("message-templates")) return response({ items: [] });
    if (method === "POST" && path.endsWith("/read")) return response({ read_seq: 1 });
    if (method === "GET" && path.includes("/messages?")) return read();
    if (method === "POST" && path.endsWith("/messages"))
      return send?.() ?? response({ send_state: "queued" });
    throw new Error(`unexpected MOCK route ${method} ${path}`);
  };
  return calls;
}
function mountThread(env: ReturnType<typeof environment>) {
  return env.mount(() =>
    InboxThread({
      store,
      conversation,
      locale: "en",
      onUnauthorized() {},
      onRead() {},
    } as any),
  );
}
test("P1-1 actual visible focus preserves pending ticket, selection, revision and real retry receipt", async (t) => {
  const env = environment(t);
  let clears = 0,
    selection: string | null = conversation.conversation_id;
  const receipt = new ReplyReceipt(() => "MOCK_KEY");
  const first = receipt.prepare({ text: "MOCK_REPLY", expected_generation: 7 });
  const clear = () => {
    clears++;
    selection = null;
    receipt.clear();
  };
  const host = env.mount(() => useInboxPrivacy(clear));
  const ticket = host.output.fence.begin(),
    revision = host.output.revision;
  env.window.dispatchEvent(new Event("focus"));
  host.flush();
  assert.equal(ticket.signal.aborted, false, "ordinary visible focus must not abort actual pending ticket");
  assert.equal(host.output.fence.current(ticket), true);
  assert.equal(clears, 0, "visible focus does not clear selected conversation");
  assert.equal(selection, conversation.conversation_id);
  assert.equal(host.output.revision, revision, "visible focus does not refetch");
  assert.equal(receipt.pending(), first, "immutable retry key/body retained");
});
test("P1-1 actual unchanged-session pageshow checks without clearing; changed-session focus expires", (t) => {
  const env = environment(t);
  let clears = 0,
    cookieReads = 0;
  let cookie = env.document.cookie;
  Object.defineProperty(env.document, "cookie", {
    get() {
      cookieReads++;
      return cookie;
    },
    set(value) {
      cookie = value;
    },
  });
  const host = env.mount(() => useInboxPrivacy(() => clears++));
  const before = cookieReads;
  env.window.dispatchEvent(new Event("pageshow"));
  host.flush();
  assert.ok(cookieReads > before, "actual csrfCookie checked at focus/pageshow boundary");
  assert.equal(clears, 0, "same-session visible pageshow must not clear");
  env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
  env.window.dispatchEvent(new Event("focus"));
  host.flush();
  assert.equal(host.output.blocked.current, true);
  assert.equal(host.output.visible, false);
  assert.equal(clears, 1, "changed session still clears private selection");
});
test("P1-1 actual hide/reveal clears and requests fresh revision, never accepts obsolete completion", (t) => {
  const env = environment(t);
  let clears = 0,
    loads = 0;
  const clear = () => clears++;
  const host = env.mount(() => {
    const privacy = useInboxPrivacy(clear);
    react.useEffect(() => {
      if (privacy.visible) loads++;
    }, [privacy.visible, privacy.revision]);
    return privacy;
  });
  const ticket = host.output.fence.begin();
  env.document.visibilityState = "hidden";
  env.document.dispatchEvent(new Event("visibilitychange"));
  host.flush();
  assert.equal(clears, 1);
  assert.equal(host.output.visible, false);
  assert.equal(ticket.signal.aborted, true);
  env.window.dispatchEvent(new Event("focus"));
  host.flush();
  assert.equal(host.output.visible, false);
  assert.equal(loads, 1);
  env.document.visibilityState = "visible";
  env.document.dispatchEvent(new Event("visibilitychange"));
  host.flush();
  assert.equal(host.output.visible, true);
  assert.equal(loads, 2);
  assert.equal(host.output.revision, 1);
  assert.equal(
    host.output.fence.current(ticket),
    false,
    "old completion cannot regain authority after reveal",
  );
});
test("P1-1 changed-session visible focus expires even with a pending request", (t) => {
  const env = environment(t);
  let clears = 0;
  const host = env.mount(() => useInboxPrivacy(() => clears++));
  const old = host.output.fence.begin();
  env.document.cookie = `__Host-commerce_csrf=${"c".repeat(43)}`;
  env.window.dispatchEvent(new Event("focus"));
  host.flush();
  assert.equal(host.output.blocked.current, true);
  assert.equal(clears, 1);
  assert.equal(host.output.fence.current(old), false);
  assert.equal(old.signal.aborted, true);
});
test("P1-2 actual A9 refresh 404 removes rendered DM and unsent draft", async (t) => {
  const env = environment(t);
  let reads = 0;
  transport(() => (++reads === 1 ? response(thread) : response({ code: "not_found" }, 404)));
  const host = mountThread(env);
  await host.settle();
  assert.ok(textOf(host.output).includes("MOCK_DM"), "baseline DM actually rendered");
  change(host, "reply-text", "MOCK_DRAFT");
  clickText(host, "Refresh");
  await host.settle();
  assert.equal(reads, 2, "actual Refresh callback re-read A9");
  assert.equal(
    textOf(host.output).includes("MOCK_DM"),
    false,
    "404 must remove previously authorized private DM",
  );
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-text").props.value, "");
  assert.equal(host.ref(ReplyReceipt).pending(), null);
});
test("P1-2 actual send-finally A9 404 removes DM, draft and uncertain immutable receipt", async (t) => {
  const env = environment(t);
  let reads = 0,
    rejectAuthority!: (value: Response) => void;
  const calls = transport(
    () =>
      ++reads === 1
        ? response(thread)
        : new Promise<Response>((done) => {
            rejectAuthority = done;
          }),
    () => {
      throw new Error("MOCK_LOST_ACK");
    },
  );
  const host = mountThread(env);
  await host.settle();
  change(host, "reply-text", "MOCK_RETRY_DRAFT");
  submit(host);
  await host.settle();
  const receipt = host.ref(ReplyReceipt).pending();
  assert.ok(receipt, "real uncertain receipt exists before A9 404");
  assert.equal(receipt.body.text, "MOCK_RETRY_DRAFT");
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-text").props.value, "MOCK_RETRY_DRAFT");
  assert.ok(rejectAuthority, "real send-finally A9 request reached held transport");
  rejectAuthority(response({ code: "not_found" }, 404));
  await host.settle();
  assert.equal(reads, 2, "real send finally performs A9 authority read");
  assert.equal(
    calls.filter((c) => c.method === "POST" && c.path.endsWith("/messages")).length,
    1,
    "no blind resend",
  );
  assert.equal(textOf(host.output).includes("MOCK_DM"), false, "404 must drop earlier private rendering");
  assert.equal(
    node(host, (n) => n.props["data-testid"] === "reply-text").props.value,
    "",
    "404 clears retry draft",
  );
  assert.equal(host.ref(ReplyReceipt).pending(), null, "real receipt is retired after authority is gone");
});
test("actual uncertain send still retains exact receipt when A9 authority remains", async (t) => {
  const env = environment(t);
  const calls = transport(
    () => response(thread),
    () => {
      throw new Error("MOCK_LOST_ACK");
    },
  );
  const host = mountThread(env);
  await host.settle();
  change(host, "reply-text", "MOCK_RETRY");
  submit(host);
  await host.settle();
  const pending = host.ref(ReplyReceipt).pending();
  assert.ok(pending);
  assert.equal(pending.body.text, "MOCK_RETRY");
  submit(host);
  await host.settle();
  const sends = calls.filter((c) => c.method === "POST" && c.path.endsWith("/messages"));
  assert.equal(sends.length, 2);
  assert.equal(sends[0].body, sends[1].body);
  assert.equal(sends[0].key, pending.key);
  assert.equal(sends[1].key, pending.key);
  assert.equal(host.ref(ReplyReceipt).pending(), pending);
});
test("actual A9 stale held completion cannot paint after component cleanup", async (t) => {
  const env = environment(t);
  let complete!: (value: Response) => void;
  transport(
    () =>
      new Promise<Response>((done) => {
        complete = done;
      }),
  );
  const host = mountThread(env);
  await host.settle();
  assert.ok(complete, "real A9 request reached held MOCK transport");
  const fence = host.ref(InboxFence),
    ticket = fence.begin();
  host.dispose();
  complete(response(thread));
  await host.settle();
  assert.equal(fence.current(ticket), false);
  assert.equal(textOf(host.output).includes("MOCK_DM"), false);
});
test("P2 actual A14 404 clears buyer facts and customer draft (explicit future-version MOCK)", async (t) => {
  const env = environment(t);
  // A13 production lacks version. This fixture exercises an existing dormant handler,
  // and does not assert backend availability or change the missing-version production rule.
  const buyer = {
    display_name: "MOCK_BUYER",
    platform: "messenger",
    purchase_ordinal: 2,
    claims: [],
    claim_total_minor: 0,
    orders: [],
    link_pending_manual: false,
    version: 7,
  };
  let writes = 0;
  globalThis.fetch = async (input, init) => {
    if (String(input).includes("buyer-panel?")) return response(buyer);
    if (String(input).endsWith("/customer-link")) {
      writes++;
      return response({ code: "not_found" }, 404);
    }
    throw new Error("unexpected MOCK buyer route");
  };
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id } as any));
  await host.settle();
  assert.ok(textOf(host.output).includes("MOCK_BUYER"));
  const input = node(host, (n) => n.type === "input");
  assert.equal(input.props.disabled, false);
  input.props.onChange({
    target: { value: "30000000-0000-4000-8000-000000000003" },
  });
  host.flush();
  clickText(host, "Link customer");
  await host.settle();
  assert.equal(writes, 1);
  assert.equal(
    textOf(host.output).includes("MOCK_BUYER"),
    false,
    "A14 authority-loss 404 removes old buyer facts",
  );
  assert.ok(!nodes(host.output).some((n) => n.type === "input" && n.props.value), "customer draft removed");
});
test("safe authority errors remain the actual InboxError transport class", () => {
  const error = new InboxError("not_found", 404);
  assert.equal(error.status, 404);
  assert.equal(error.code, "not_found");
});
test("actual current A13 without version keeps A14 disabled; future MOCK grants no backend claim", async (t) => {
  const env = environment(t);
  let writes = 0;
  globalThis.fetch = async (_input, init) => {
    if (init?.method === "POST") writes++;
    return response({
      display_name: "MOCK_BUYER_NO_VERSION",
      platform: "messenger",
      purchase_ordinal: 0,
      claims: [],
      claim_total_minor: 0,
      orders: [],
      link_pending_manual: false,
    });
  };
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id } as any));
  await host.settle();
  assert.equal(node(host, (n) => n.type === "input").props.disabled, true);
  assert.equal(node(host, (n) => n.type === "button" && textOf(n) === "Link customer").props.disabled, true);
  assert.equal(writes, 0, "missing A13 version never synthesizes authority or mutation");
});
for (const kind of ["takeover", "messages"] as const) {
  test(`P1-2 actual ${kind === "takeover" ? "A11 takeover" : "A12 send"} write 404 clears private state despite failing follow-up A9`, async (t) => {
    const env = environment(t);
    let reads = 0, writes = 0, followupFailures = 0;
    let rejectWrite!: (value: Response) => void;
    globalThis.fetch = async (input, init) => {
      const path = String(input), method = init?.method ?? "GET";
      if (path.endsWith("message-templates")) return response({ items: [] });
      if (method === "POST" && path.endsWith("/read")) return response({ read_seq: 1 });
      if (method === "GET" && path.includes("/messages?")) {
        if (++reads === 1) return response(thread);
        followupFailures++;
        if (kind === "messages") throw new Error("MOCK_A9_NETWORK_FAILURE");
        return response({ code: "retry_later" }, 503);
      }
      if (method === "POST" && path.endsWith(`/${kind}`)) {
        writes++;
        return new Promise<Response>((done) => { rejectWrite = done; });
      }
      throw new Error(`unexpected MOCK write404 route ${method} ${path}`);
    };
    const host = mountThread(env);
    await host.settle();
    assert.ok(textOf(host.output).includes("MOCK_DM"), "actual authorized baseline DM rendered");
    change(host, "reply-text", "MOCK_WRITE404_DRAFT");
    if (kind === "messages") submit(host);
    else {
      const button = node(host, (n) => n.props["data-testid"] === "takeover");
      assert.equal(button.props.disabled, false);
      button.props.onClick(); host.flush();
    }
    await host.settle();
    assert.equal(writes, 1, "actual A11/A12 callback reached held POST");
    const pending = host.ref(ReplyReceipt).pending();
    if (kind === "messages") {
      assert.ok(pending, "actual A12 receipt exists before definitive 404");
      assert.equal(pending.body.text, "MOCK_WRITE404_DRAFT");
    } else assert.equal(pending, null, "A11 correctly has no reply receipt");
    rejectWrite(response({ code: "not_found" }, 404));
    await host.settle();
    assert.ok(reads <= 2, "write404 never loops or blind-retries A9");
    if (reads === 2) assert.equal(followupFailures, 1, "follow-up A9 could not grant authority");
    assert.equal(textOf(host.output).includes("MOCK_DM"), false, "write404 itself clears DM even if subsequent A9 is unavailable");
    assert.equal(node(host, (n) => n.props["data-testid"] === "reply-text").props.value, "", "definitive write404 clears draft");
    assert.equal(host.ref(ReplyReceipt).pending(), null, "definitive write404 retires real receipt");
  });
}
