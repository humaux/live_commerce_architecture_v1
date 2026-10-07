// Purpose: independently exercise bundle recovery and conversation capability controls through real components.
// Depends on: generic Node hook/VNode host, real Inbox/Thread/BuyerPanel and existing claims/inbox clients.
// Used by: LC-U2b review2 Node gate; all server fields below are explicit MOCK existing contract projections.
// Invariants: I02/I06 exact receipt; I07 scoped server authority; I11 token only in memory and clipboard.
import test from "node:test";
import assert from "node:assert/strict";
import {
  environment,
  nodes,
  textOf,
  node,
  change,
  submit,
  store,
  conversation,
  thread,
  response,
  transport,
  mountThread,
  setHealth,
  type Host,
} from "./inbox-review-host.test.ts";
const { Inbox } = await import("../../apps/admin/components/Inbox.tsx");

const sid = "30000000-0000-4000-8000-000000000003";
const bid = "40000000-0000-4000-8000-000000000004";
const otherBid = "50000000-0000-4000-8000-000000000005";
const product = "60000000-0000-4000-8000-000000000006";
const token = "M".repeat(42) + "A"; // canonical test token; never a real customer claim credential.
const origin = "https://mock-shop.invalid";
const issuedURL = `${origin}/en/claim#t=${token}`;
const bundleRow = {
  ...conversation,
  conversation_id: null,
  bundle_id: bid,
  session_id: sid,
  display_name: null,
  mode: null,
  link_pending_manual: true,
};
const buyer = {
  platform: "facebook",
  purchase_ordinal: 0,
  claims: [],
  claim_total_minor: 0,
  orders: [],
  link_pending_manual: true,
};
const bundle = (bundle_id = bid) => ({
  bundle_id,
  ref: "ABCD1234",
  platform: "facebook",
  label: "",
  bound: false,
  version: 4,
  link: { state: "ACTIVE", generation: 9, expires_at: "2099-01-01T00:00:00Z" },
  lines: [],
  created_at: "2030-01-01T00:00:00Z",
  updated_at: "2030-01-01T00:00:00Z",
});
const issued = (replayed = false) => ({
  token: replayed ? null : token,
  generation: 10,
  expires_at: "2099-01-01T00:00:00Z",
  released: false,
  replayed,
});
const privateResponse = (value: unknown) =>
  new Response(JSON.stringify(value), {
    headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store" },
  });
type Call = { path: string; method: string; body: string; key: string | null };

function bundleTransport(
  post: (index: number) => Response | Promise<Response>,
  options: { missingSession?: boolean; noBundle?: boolean } = {},
) {
  const calls: Call[] = [];
  let posts = 0;
  globalThis.fetch = async (input, init) => {
    const path = String(input),
      method = init?.method ?? "GET";
    calls.push({
      path,
      method,
      body: String(init?.body ?? ""),
      key: new Headers(init?.headers).get("Idempotency-Key"),
    });
    if (path.includes("/inbox/conversations?"))
      return response({
        items: [
          { ...bundleRow, ...(options.missingSession ? { session_id: null } : {}) },
          { ...bundleRow, bundle_id: otherBid },
        ],
        next_cursor: "",
        unread_total: 0,
      });
    if (path.includes("/inbox/buyer-panel?")) return response(buyer);
    // Exact existing M6 wire shape; the first cursor page is deliberately the wrong bundle.
    if (path.includes(`/live-sessions/${sid}/claims/bundles?`))
      return privateResponse({
        items: path.includes("cursor=") ? (options.noBundle ? [] : [bundle()]) : [bundle(otherBid)],
        next_cursor: path.includes("cursor=") ? "" : "MOCK_NEXT_PAGE",
      });
    if (path.includes("/products?"))
      return response({ items: [{ id: product, name: "MOCK_PRODUCT", status: "active" }], next_cursor: "" });
    if (path.includes(`/products/${product}/purchase-entry?`))
      return response({
        product_id: product,
        locale: "en",
        state: "configured",
        url: `${origin}/en/products/${product}`,
      });
    if (method === "POST" && path === `/api/stores/${store.id}/live-sessions/${sid}/claims/bundles/${bid}/link`)
      return post(++posts);
    throw new Error(`unexpected MOCK recovery route ${method} ${path}`);
  };
  return calls;
}
function privateSinks(t: any, denyClipboard = false) {
  const clipboard: string[] = [],
    storage: string[] = [],
    logs: string[] = [],
    navigation: string[] = [];
  const descriptors = Object.fromEntries(
    ["navigator", "localStorage", "sessionStorage"].map((name) => [
      name,
      Object.getOwnPropertyDescriptor(globalThis, name),
    ]),
  );
  let denied = denyClipboard;
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: {
      clipboard: {
        async writeText(value: string) {
          if (denied) {
            denied = false;
            throw new Error("MOCK_CLIPBOARD_DENIED");
          }
          clipboard.push(value);
        },
      },
    },
  });
  for (const name of ["localStorage", "sessionStorage"])
    Object.defineProperty(globalThis, name, {
      configurable: true,
      value: {
        getItem() {
          return null;
        },
        setItem(key: string, value: string) {
          storage.push(`${key}:${value}`);
        },
        removeItem() {},
      },
    });
  const win = (globalThis as any).window;
  win.location = {
    get href() {
      return "https://mock-admin.invalid/en/messages";
    },
    set href(value) {
      navigation.push(String(value));
    },
    assign(value: string) {
      navigation.push(value);
    },
    replace(value: string) {
      navigation.push(value);
    },
  };
  win.history = {
    pushState(_state: unknown, _title: string, value: string) {
      navigation.push(value);
    },
    replaceState(_state: unknown, _title: string, value: string) {
      navigation.push(value);
    },
  };
  const oldLog = console.log,
    oldWarn = console.warn,
    oldError = console.error;
  console.log =
    console.warn =
    console.error =
      (...args: unknown[]) => {
        logs.push(args.map(String).join(" "));
      };
  t.after(() => {
    console.log = oldLog;
    console.warn = oldWarn;
    console.error = oldError;
    for (const [name, descriptor] of Object.entries(descriptors)) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else delete (globalThis as any)[name];
    }
  });
  return { clipboard, storage, logs, navigation };
}
async function openBundle(env: ReturnType<typeof environment>, authorizedStore: any = store) {
  const host = env.mount(() => Inbox({ store: authorizedStore, locale: "en", initialError: null }));
  await host.settle();
  node(host, (n) => n.props["data-testid"] === `conversation-${bid}`).props.onClick();
  await host.settle();
  return host;
}
function copyAction(host: Host) {
  const action = node(
    host,
    (n) => n.props["data-testid"] === "bundle-copy-link" || (n.type === "button" && /copy.*link/i.test(textOf(n))),
  );
  assert.equal(action.props.disabled, false, "bundle-only recovery has an enabled explicit claim-link action");
  action.props.onClick();
  host.flush();
}
const posts = (calls: Call[]) => calls.filter((call) => call.method === "POST");
function assertExactIssue(calls: Call[]) {
  const sends = posts(calls);
  assert.equal(sends.length, 1);
  assert.equal(sends[0].path, `/api/stores/${store.id}/live-sessions/${sid}/claims/bundles/${bid}/link`);
  assert.deepEqual(
    JSON.parse(sends[0].body),
    { expected_generation: 9, release_binding: false },
    "generation is real matching M6 server field, never guessed zero",
  );
  assert.match(sends[0].key ?? "", /^[0-9a-f-]{36}$/);
  assert.ok(
    calls.some((call) => call.path.includes("cursor=MOCK_NEXT_PAGE")),
    "matching bundle found through actual cursor page",
  );
  assert.equal(
    calls.some((call) => call.path.includes(token)),
    false,
    "claim token stays out of request URLs",
  );
}

test("review2 P1 bundle-only row opens real A13 and issues existing M7 claim link to clipboard", async (t) => {
  const env = environment(t),
    sinks = privateSinks(t);
  const calls = bundleTransport(() => privateResponse(issued()));
  const host = await openBundle(env);
  assert.ok(
    nodes(host.output).some((n) => n.props["data-testid"] === "buyer-panel"),
    "real mounted BuyerPanel opened",
  );
  assert.ok(
    calls.some((call) => call.path.endsWith(`inbox/buyer-panel?bundle_id=${bid}`)),
    "A13 bundle scope read",
  );
  assert.equal(
    nodes(host.output).some((n) => n.props["data-testid"] === "reply-send"),
    false,
    "bundle-only cannot DM",
  );
  assert.equal(
    nodes(host.output).some((n) => n.type === "input" || /^(Link|Unlink) customer$/.test(textOf(n))),
    false,
    "A14 controls never guessed for bundle-only row",
  );
  copyAction(host);
  await host.settle();
  assertExactIssue(calls);
  assert.deepEqual(sinks.clipboard, [issuedURL]);
  assert.equal(
    JSON.stringify(host.output).includes(token),
    false,
    "issued token never rendered in VNode attributes or text",
  );
  assert.deepEqual(sinks.storage, []);
  assert.equal(
    sinks.logs.some((value) => value.includes(token)),
    false,
  );
  assert.deepEqual(sinks.navigation, [], "claim credential never navigates the admin URL/history");
});

test("review2 P1 clipboard denial recopy reuses cached issued URL without another M7 POST", async (t) => {
  const env = environment(t),
    sinks = privateSinks(t, true);
  const calls = bundleTransport(() => privateResponse(issued()));
  const host = await openBundle(env);
  copyAction(host);
  await host.settle();
  assert.equal(posts(calls).length, 1);
  assert.deepEqual(sinks.clipboard, []);
  copyAction(host);
  await host.settle();
  assertExactIssue(calls);
  assert.deepEqual(sinks.clipboard, [issuedURL]);
});

test("review2 P1 lost M7 ACK retries exact receipt and null replay token never mints a fresh key", async (t) => {
  const env = environment(t),
    sinks = privateSinks(t);
  const calls = bundleTransport((index) => {
    if (index === 1) throw new Error("MOCK_LOST_ACK");
    return privateResponse(issued(true));
  });
  const host = await openBundle(env);
  copyAction(host);
  await host.settle();
  assert.equal(posts(calls).length, 1, "uncertainty never triggers automatic reissue");
  const readsBeforeRetry = calls.filter((call) => call.method === "GET").length;
  const retry = node(host, (n) => n.props["data-testid"] === "bundle-link-retry");
  assert.equal(retry.props.disabled, false);
  retry.props.onClick();
  await host.settle();
  const sends = posts(calls);
  assert.equal(sends.length, 2);
  assert.equal(
    calls.filter((call) => call.method === "GET").length,
    readsBeforeRetry,
    "explicit unknown retry does not refresh generation or origin",
  );
  assert.equal(sends[1].path, sends[0].path);
  assert.equal(sends[1].body, sends[0].body);
  assert.equal(sends[1].key, sends[0].key);
  assert.deepEqual(sinks.clipboard, [], "one-time replay has no recoverable credential");
  assert.ok(
    nodes(host.output).some(
      (n) => ["alert", "status"].includes(n.props.role) && /already|replay|recover|unavailable|issued/i.test(textOf(n)),
    ),
    "fixed unrecoverable outcome visible",
  );
  const actions = nodes(host.output).filter((n) =>
    ["bundle-copy-link", "bundle-link-retry"].includes(n.props["data-testid"]),
  );
  assert.ok(
    actions.every((n) => n.props.disabled === true),
    "no new-key action within this scope after null replay",
  );
  await host.settle();
  assert.equal(posts(calls).length, 2);
});

test("review2 P1 missing CSRF before cached recopy cannot copy or issue again", async (t) => {
  const env = environment(t),
    sinks = privateSinks(t);
  const calls = bundleTransport(() => privateResponse(issued()));
  const host = await openBundle(env);
  copyAction(host);
  await host.settle();
  assert.deepEqual(sinks.clipboard, [issuedURL]);
  assertExactIssue(calls);
  const recopy = node(host, (n) => n.props["data-testid"] === "bundle-copy-link");
  assert.equal(recopy.props.disabled, false);
  env.document.cookie = ""; // Actual sessionBoundary must reject even without a focus notification.
  recopy.props.onClick();
  await host.settle();
  assert.deepEqual(sinks.clipboard, [issuedURL], "cached credential cannot bypass missing session");
  assert.equal(posts(calls).length, 1);
  assert.equal(JSON.stringify(host.output).includes(token), false);
});

test("review2 P1 missing CSRF before uncertain retry rejects preserved receipt dispatch", async (t) => {
  const env = environment(t),
    sinks = privateSinks(t);
  const calls = bundleTransport(() => {
    throw new Error("MOCK_LOST_ACK");
  });
  const host = await openBundle(env);
  copyAction(host);
  await host.settle();
  assert.equal(posts(calls).length, 1);
  const retry = node(host, (n) => n.props["data-testid"] === "bundle-link-retry");
  assert.equal(retry.props.disabled, false);
  env.document.cookie = "";
  retry.props.onClick();
  await host.settle();
  assert.equal(posts(calls).length, 1, "uncertain receipt is never dispatched with missing CSRF/session");
  assert.deepEqual(sinks.clipboard, []);
});

for (const boundary of ["hide", "selection", "unmount", "logout"] as const) {
  test(`review2 P1 late issued-link completion after ${boundary} cannot write clipboard`, async (t) => {
    const env = environment(t),
      sinks = privateSinks(t);
    let complete!: (value: Response) => void;
    const calls = bundleTransport(
      () =>
        new Promise<Response>((done) => {
          complete = done;
        }),
    );
    const host = await openBundle(env);
    copyAction(host);
    await host.settle();
    assert.equal(posts(calls).length, 1);
    assert.ok(complete, "real issueClaimLink reached held M7 transport");
    if (boundary === "hide") {
      env.document.visibilityState = "hidden";
      env.document.dispatchEvent(new Event("visibilitychange"));
      host.flush();
    } else if (boundary === "selection") {
      node(host, (n) => n.props["data-testid"] === `conversation-${otherBid}`).props.onClick();
      host.flush();
    } else if (boundary === "logout") {
      env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
      env.window.dispatchEvent(new Event("focus"));
      host.flush();
    } else host.dispose();
    complete(privateResponse(issued()));
    await host.settle();
    assert.deepEqual(sinks.clipboard, [], "obsolete authorization never copies late credential");
    assert.equal(JSON.stringify(host.output).includes(token), false);
    assert.deepEqual(sinks.storage, []);
  });
}

for (const permissions of [
  ["inbox:read", "live:read"],
  ["inbox:read", "live:manage"],
]) {
  test(`review2 P1 claim-link issuance denied without both live read/manage (${permissions.join(",")})`, async (t) => {
    const env = environment(t);
    privateSinks(t);
    const calls = bundleTransport(() => privateResponse(issued()));
    const host = await openBundle(env, { ...store, role: "staff", permissions });
    const controls = nodes(host.output).filter(
      (n) => n.props["data-testid"] === "bundle-copy-link" || (n.type === "button" && /copy.*link/i.test(textOf(n))),
    );
    assert.ok(
      controls.every((n) => n.props.disabled === true),
      "existing live claims permissions gate issueLink",
    );
    assert.equal(posts(calls).length, 0);
  });
}

for (const options of [{ missingSession: true }, { noBundle: true }]) {
  test(`review2 P1 missing authoritative recovery identity never guesses generation ${JSON.stringify(options)}`, async (t) => {
    const env = environment(t);
    privateSinks(t);
    const calls = bundleTransport(() => privateResponse(issued()), options);
    const host = await openBundle(env);
    const action = nodes(host.output).find(
      (n) => n.props["data-testid"] === "bundle-copy-link" || (n.type === "button" && /copy.*link/i.test(textOf(n))),
    );
    if (action && action.props.disabled !== true) {
      action.props.onClick();
      await host.settle();
    }
    assert.equal(posts(calls).length, 0, "no issue without server session and matching bundle generation");
  });
}

// B1 fixtures use actual existing binding_id/provider/capability/state fields, with no invented conversation binding.
const bindingA = "70000000-0000-4000-8000-000000000007",
  bindingB = "80000000-0000-4000-8000-000000000008";
function cap(binding_id: string, state: string, provider = "facebook", capability = "dm_session") {
  return {
    binding_id,
    provider,
    capability,
    state,
    reason: "MOCK_REASON",
    evidence: "MOCK",
    checked_at: "2030-01-01T00:00:00Z",
  };
}
function health(rows: ReturnType<typeof cap>[]) {
  return {
    pages: rows.map((row, i) => ({
      page_id: `MOCK_PAGE_${i}`,
      page_name: "MOCK_PAGE",
      status: "active",
      severity: "ok",
      capabilities: [row],
    })),
  };
}
const advisory = (host: Host) => nodes(host.output).find((n) => n.props["data-testid"] === "inbox-capability-advisory");
for (const state of ["ok", "review_required", "missing_permission"] as const) {
  test(`review2 P2 exactly one matching binding uses named DM hard gate ${state}`, async (t) => {
    const env = environment(t);
    setHealth(health([cap(bindingA, state), cap(bindingB, "missing_permission", "instagram")]));
    transport(() => response(thread));
    const host = mountThread(env);
    await host.settle();
    change(host, "reply-text", "MOCK_REPLY");
    assert.equal(
      node(host, (n) => n.props["data-testid"] === "reply-send").props.disabled,
      state === "missing_permission",
    );
    assert.equal(!!advisory(host), false, "different-provider binding is irrelevant");
    assert.equal(textOf(host.output).includes("App-role test accounts only"), state === "review_required");
  });
}
for (const rows of [
  [cap(bindingA, "ok"), cap(bindingB, "missing_permission")],
  [cap(bindingA, "missing_permission"), cap(bindingB, "reauth_required")],
  [cap(bindingA, "ok"), cap(bindingB, "missing_permission", "facebook", "read_comment")],
  [cap(bindingA, "review_required"), cap(bindingB, "ok")],
]) {
  test(`review2 P2 multiple same-provider bindings advisory only (${rows.map((r) => `${r.capability}:${r.state}`).join(",")})`, async (t) => {
    const env = environment(t);
    setHealth(health(rows));
    transport(() => response(thread));
    const host = mountThread(env);
    await host.settle();
    change(host, "reply-text", "MOCK_SCOPED_REPLY");
    assert.equal(
      node(host, (n) => n.props["data-testid"] === "reply-send").props.disabled,
      false,
      "unrelated Page state cannot block current conversation",
    );
    assert.ok(advisory(host), "ambiguity has a visible advisory instead of inferred binding authority");
    assert.ok(textOf(advisory(host)).trim().length > 0);
    assert.equal(
      textOf(host.output).includes("App-role test accounts only"),
      false,
      "ambiguous binding cannot infer a scoped review-only badge",
    );
  });
}
test("review2 P2 actual scoped A12 409 capability refusal remains visible after advisory submit", async (t) => {
  const env = environment(t);
  setHealth(health([cap(bindingA, "ok"), cap(bindingB, "missing_permission")]));
  const calls = transport(
    () => response(thread),
    () => response({ code: "capability" }, 409),
  );
  const host = mountThread(env);
  await host.settle();
  change(host, "reply-text", "MOCK_SCOPED_REPLY");
  submit(host);
  await host.settle();
  const send = calls.find((call) => call.method === "POST" && call.path.endsWith("/messages"));
  assert.ok(send);
  assert.equal(send.path, `/api/stores/${store.id}/inbox/conversations/${conversation.conversation_id}/messages`);
  assert.deepEqual(JSON.parse(send.body!), { text: "MOCK_SCOPED_REPLY", expected_generation: 7 });
  assert.ok(
    nodes(host.output).some(
      (n) => n.props.role === "alert" && /permission|capability|available|reply|connected/i.test(textOf(n)),
    ),
    "server-scoped capability denial is shown",
  );
});
test("review2 P2 missing health and single binding without named DM remain disabled", async (t) => {
  const env = environment(t);
  setHealth(health([cap(bindingA, "ok", "facebook", "read_comment")]));
  transport(() => response(thread));
  const host = mountThread(env);
  await host.settle();
  change(host, "reply-text", "MOCK_REPLY");
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-send").props.disabled, true);
  setHealth(null);
  host.dirty = true;
  host.flush();
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-send").props.disabled, true);
});
