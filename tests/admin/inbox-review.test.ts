// Purpose: retain independent PR8 privacy and authority-loss regression assertions.
// Depends on: actual-component MOCK hook-host driver and real production refs/client.
// Used by: LC-U2b focused Node gate; BROWSER is separate and NOT_RUN here.
import test from "node:test";
import assert from "node:assert/strict";
import {
  environment,
  textOf,
  node,
  change,
  submit,
  clickText,
  store,
  conversation,
  thread,
  response,
  transport,
  mountThread,
  ReplyReceipt,
  InboxFence,
  InboxError,
} from "./inbox-review-host.test.ts";
import { useInboxPrivacy, BuyerPanel, nodes, react } from "./inbox-review-host.test.ts";
import type { Host } from "./inbox-review-host.test.ts";

// A held request changes no React state when it arrives, so observe the fixture transport itself.
function arrival(description: string) {
  const event = Promise.withResolvers<void>();
  return {
    notify: () => event.resolve(),
    wait: () => {
      let timer!: ReturnType<typeof setTimeout>;
      const deadline = new Promise<never>((_resolve, reject) => {
        timer = setTimeout(() => reject(new Error(`Timed out waiting for ${description}`)), 5000);
      });
      return Promise.race([event.promise, deadline]).finally(() => clearTimeout(timer));
    },
  };
}
const threadIdle = (host: Host) =>
  nodes(host.output).some((n) => n.props["data-testid"] === "inbox-thread" && n.props["aria-busy"] === false);
const threadLoaded = (host: Host) => threadIdle(host) && textOf(host.output).includes("MOCK_DM");
const threadGone = (host: Host) =>
  threadIdle(host) &&
  nodes(host.output).some(
    (n) => n.type === "h2" && textOf(n).includes("This conversation is unavailable in the current store."),
  );
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
  assert.equal(host.output.fence.current(ticket), false, "old completion cannot regain authority after reveal");
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
  await host.waitFor(() => threadLoaded(host), "initial A9/A10 and visible thread");
  assert.ok(textOf(host.output).includes("MOCK_DM"), "baseline DM actually rendered");
  change(host, "reply-text", "MOCK_DRAFT");
  clickText(host, "Refresh");
  await host.waitFor(() => reads === 2 && threadGone(host), "Refresh 404 terminal purge");
  assert.equal(reads, 2, "actual Refresh callback re-read A9");
  assert.equal(textOf(host.output).includes("MOCK_DM"), false, "404 must remove previously authorized private DM");
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-text").props.value, "");
  assert.equal(host.ref(ReplyReceipt).pending(), null);
});
test("P1-2 actual send-finally A9 404 removes DM, draft and uncertain immutable receipt", async (t) => {
  const env = environment(t);
  let reads = 0,
    rejectAuthority!: (value: Response) => void;
  const authorityArrived = arrival("held send-finally A9 authority request");
  const calls = transport(
    () =>
      ++reads === 1
        ? response(thread)
        : new Promise<Response>((done) => {
            rejectAuthority = done;
            authorityArrived.notify();
          }),
    () => {
      throw new Error("MOCK_LOST_ACK");
    },
  );
  const host = mountThread(env);
  await host.waitFor(() => threadLoaded(host), "initial A9/A10 before uncertain send");
  change(host, "reply-text", "MOCK_RETRY_DRAFT");
  submit(host);
  await authorityArrived.wait();
  host.flush();
  const receipt = host.ref(ReplyReceipt).pending();
  assert.ok(receipt, "real uncertain receipt exists before A9 404");
  assert.equal(receipt.body.text, "MOCK_RETRY_DRAFT");
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-text").props.value, "MOCK_RETRY_DRAFT");
  assert.ok(rejectAuthority, "real send-finally A9 request reached held transport");
  rejectAuthority(response({ code: "not_found" }, 404));
  await host.waitFor(() => threadGone(host), "send-finally 404 terminal purge");
  assert.equal(reads, 2, "real send finally performs A9 authority read");
  assert.equal(calls.filter((c) => c.method === "POST" && c.path.endsWith("/messages")).length, 1, "no blind resend");
  assert.equal(textOf(host.output).includes("MOCK_DM"), false, "404 must drop earlier private rendering");
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-text").props.value, "", "404 clears retry draft");
  assert.equal(host.ref(ReplyReceipt).pending(), null, "real receipt is retired after authority is gone");
});
for (const authorityDelay of [0, 50]) {
  test(`actual uncertain send still retains exact receipt when A9 authority remains (${authorityDelay}ms post-send A9)`, async (t) => {
    const env = environment(t);
    let reads = 0;
    const held = new Map<ReturnType<typeof setTimeout>, () => void>();
    t.after(() => {
      for (const [timer, finish] of held) {
        clearTimeout(timer);
        finish();
      }
    });
    const calls = transport(
      () => {
        if (++reads === 1 || !authorityDelay) return response(thread);
        // Controlled MOCK I/O latency, not a test waiter: the real send-finally A9 remains in flight.
        return new Promise<Response>((done) => {
          const finish = () => {
            held.delete(timer);
            done(response(thread));
          };
          const timer = setTimeout(finish, authorityDelay);
          held.set(timer, finish);
        });
      },
      () => {
        throw new Error("MOCK_LOST_ACK");
      },
    );
    const host = mountThread(env);
    const idle = () => {
      const rendered = nodes(host.output);
      return rendered.some((n) => n.props["data-testid"] === "inbox-thread" && n.props["aria-busy"] === false);
    };
    const readyToReply = () =>
      idle() && nodes(host.output).some((n) => n.props["data-testid"] === "reply-send" && n.props.disabled === false);
    await host.waitFor(
      () => reads >= 1 && idle() && textOf(host.output).includes("MOCK_DM"),
      "initial A9/A10 completion",
    );
    change(host, "reply-text", "MOCK_RETRY");
    submit(host);
    // Retry becomes safe only after send.finally's A9 and A10 have cleared the real in-flight guard.
    await host.waitFor(() => reads >= 2 && readyToReply(), "first send authority revalidation");
    const pending = host.ref(ReplyReceipt).pending();
    assert.ok(pending);
    assert.equal(pending.body.text, "MOCK_RETRY");
    submit(host);
    await host.waitFor(() => reads >= 3 && readyToReply(), "retry authority revalidation");
    const sends = calls.filter((c) => c.method === "POST" && c.path.endsWith("/messages"));
    assert.equal(sends.length, 2);
    assert.equal(sends[0].body, sends[1].body);
    assert.equal(sends[0].key, pending.key);
    assert.equal(sends[1].key, pending.key);
    assert.equal(host.ref(ReplyReceipt).pending(), pending);
  });
}
test("actual A9 stale held completion cannot paint after component cleanup", async (t) => {
  const env = environment(t);
  let complete!: (value: Response) => void;
  const readArrived = arrival("held stale A9 request");
  transport(
    () =>
      new Promise<Response>((done) => {
        complete = done;
        readArrived.notify();
      }),
  );
  const host = mountThread(env);
  await readArrived.wait();
  assert.ok(complete, "real A9 request reached held MOCK transport");
  const fence = host.ref(InboxFence),
    ticket = fence.begin();
  host.dispose();
  complete(response(thread));
  // The host is disposed: this is only an aborted-completion drain, not a readiness checkpoint.
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
  await host.waitFor(
    () => host.output.props["aria-busy"] === false && textOf(host.output).includes("MOCK_BUYER"),
    "A13 buyer fields and idle link controls",
  );
  assert.ok(textOf(host.output).includes("MOCK_BUYER"));
  const input = node(host, (n) => n.type === "input");
  assert.equal(input.props.disabled, false);
  input.props.onChange({
    target: { value: "30000000-0000-4000-8000-000000000003" },
  });
  host.flush();
  clickText(host, "Link customer");
  await host.waitFor(
    () => writes === 1 && !textOf(host.output).includes("MOCK_BUYER") && host.output.props["aria-busy"] === false,
    "A14 404 buyer and customer-draft purge",
  );
  assert.equal(writes, 1);
  assert.equal(textOf(host.output).includes("MOCK_BUYER"), false, "A14 authority-loss 404 removes old buyer facts");
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
  await host.waitFor(
    () => textOf(host.output).includes("MOCK_BUYER_NO_VERSION") && host.output.props["aria-busy"] === false,
    "A13 actual missing-version controls",
  );
  assert.equal(node(host, (n) => n.type === "input").props.disabled, true);
  assert.equal(node(host, (n) => n.type === "button" && textOf(n) === "Link customer").props.disabled, true);
  assert.equal(writes, 0, "missing A13 version never synthesizes authority or mutation");
});
for (const kind of ["takeover", "messages"] as const) {
  test(`P1-2 actual ${kind === "takeover" ? "A11 takeover" : "A12 send"} write 404 clears private state despite failing follow-up A9`, async (t) => {
    const env = environment(t);
    let reads = 0,
      writes = 0,
      followupFailures = 0;
    let rejectWrite!: (value: Response) => void;
    const postArrived = arrival(`held ${kind} POST`);
    globalThis.fetch = async (input, init) => {
      const path = String(input),
        method = init?.method ?? "GET";
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
        return new Promise<Response>((done) => {
          rejectWrite = done;
          postArrived.notify();
        });
      }
      throw new Error(`unexpected MOCK write404 route ${method} ${path}`);
    };
    const host = mountThread(env);
    await host.waitFor(() => reads === 1 && threadLoaded(host), "initial A9/A10 before write404 action");
    assert.ok(textOf(host.output).includes("MOCK_DM"), "actual authorized baseline DM rendered");
    change(host, "reply-text", "MOCK_WRITE404_DRAFT");
    if (kind === "messages") submit(host);
    else {
      const button = node(host, (n) => n.props["data-testid"] === "takeover");
      assert.equal(button.props.disabled, false);
      button.props.onClick();
      host.flush();
    }
    await postArrived.wait();
    host.flush();
    assert.equal(writes, 1, "actual A11/A12 callback reached held POST");
    const pending = host.ref(ReplyReceipt).pending();
    if (kind === "messages") {
      assert.ok(pending, "actual A12 receipt exists before definitive 404");
      assert.equal(pending.body.text, "MOCK_WRITE404_DRAFT");
    } else assert.equal(pending, null, "A11 correctly has no reply receipt");
    rejectWrite(response({ code: "not_found" }, 404));
    await host.waitFor(() => threadGone(host), "write404 terminal thread/receipt/draft purge");
    assert.ok(reads <= 2, "write404 never loops or blind-retries A9");
    if (reads === 2) assert.equal(followupFailures, 1, "follow-up A9 could not grant authority");
    assert.equal(
      textOf(host.output).includes("MOCK_DM"),
      false,
      "write404 itself clears DM even if subsequent A9 is unavailable",
    );
    assert.equal(
      node(host, (n) => n.props["data-testid"] === "reply-text").props.value,
      "",
      "definitive write404 clears draft",
    );
    assert.equal(host.ref(ReplyReceipt).pending(), null, "definitive write404 retires real receipt");
  });
}

// LC-B3b A13 intentionally omits orders without server authority; a stale shell hint cannot restore it.
for (const shape of ["omitted", "empty", "populated"] as const) {
  test(`A13 server orders omission remains distinct from authorized ${shape} data`, async (t) => {
    const env = environment(t);
    const order = {
      order_id: "30000000-0000-4000-8000-000000000003", number: "MOCK_A13_ORDER",
      state: "CONFIRMED", total_minor: 10000, created_at: "2030-01-01T00:00:00Z",
    };
    const buyer = {
      display_name: "MOCK_B3B_BUYER", platform: "messenger", purchase_ordinal: 2,
      claims: [{ session_id: store.id, offer_id: conversation.conversation_id, keyword: "MOCK_CLAIM", quantity: 1 }],
      claim_total_minor: 10000, link_pending_manual: false,
      ...(shape === "omitted" ? {} : { orders: shape === "empty" ? [] : [order] }),
    };
    let writes = 0;
    globalThis.fetch = async (input, init) => {
      if (init?.method === "POST") writes++;
      assert.ok(String(input).includes("buyer-panel?"), "actual A13 request only");
      return response(buyer);
    };
    // The owner hint is intentionally stale for omitted orders. Server omission remains authoritative.
    const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id, onOrder() {} }));
    await host.waitFor(
      () => host.output.props["aria-busy"] === false && textOf(host.output).includes("MOCK_B3B_BUYER"),
      "actual LC-B3b A13 completion",
    );
    const rendered = textOf(host.output);
    assert.ok(rendered.includes("MOCK_CLAIM"), "authorized claim facts remain visible");
    assert.equal(rendered.includes("Order details require order read access."), shape === "omitted");
    assert.equal(rendered.includes("No orders supplied."), shape === "empty", "omission must never pretend authorized emptiness");
    assert.equal(rendered.includes("MOCK_A13_ORDER"), shape === "populated");
    assert.equal(nodes(host.output).filter((n) => n.type === "button" && textOf(n) === "MOCK_A13_ORDER").length, shape === "populated" ? 1 : 0);
    assert.equal(node(host, (n) => n.type === "input").props.disabled, true);
    for (const label of ["Link customer", "Unlink customer"])
      assert.equal(node(host, (n) => n.type === "button" && textOf(n) === label).props.disabled, true, "A14 stays deferred without its supplied version");
    assert.equal(writes, 0);
  });
}
