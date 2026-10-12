// Purpose: LC-U2b2 counterexamples for actual BuyerPanel linking, CAS recovery, authority and privacy.
// Depends on: existing hook host, real inbox/customers clients and their validators; only fetch is a synthetic edge.
// Used by: test-node.sh and LC-U2b2 red/green; MOCK component host, not browser or PostgreSQL evidence.
import test from "node:test";
import assert from "node:assert/strict";
import { environment, BuyerPanel, nodes, node, textOf, store, conversation, thread, response,
  setHealth, transport, mountThread, change } from "./inbox-review-host.test.ts";
import type { Host } from "./inbox-review-host.test.ts";

const customerID = "30000000-0000-4000-8000-000000000003";
const sessionID = "40000000-0000-4000-8000-000000000004";
const bindingA = "70000000-0000-4000-8000-000000000007", bindingB = "80000000-0000-4000-8000-000000000008";
const customer = { customer_id: customerID, display_name: "MOCK_PICKED_CUSTOMER", phone_last3: "321",
  first_seen_at: "2030-01-01T00:00:00Z", last_activity_at: "2030-01-01T00:00:00Z", orders_count: 0,
  paid_orders_count: 0, captured_minor: 0, refunded_minor: 0, currency: null, claims_count: 0, platforms: [],
  consents: { marketing_messages: false, ads_personalization: false }, active: true, imported: false, tags: [] };
const buyer = { display_name: "MOCK_INBOX_BUYER", platform: "messenger", purchase_ordinal: 2,
  claims: [{ session_id: sessionID, offer_id: customerID, keyword: "MOCK_KEYWORD", quantity: 2 }],
  claim_total_minor: 24000, orders: [], auto_reply: { send_state: "sent" }, link_pending_manual: false };
const selected = (host: Host) => node(host, n => n.props["data-testid"] === "customer-link-choice");
const button = (host: Host, id: string) => node(host, n => n.props["data-testid"] === id);
function click(host: Host, id: string) { const b = button(host, id); assert.equal(b.props.disabled, false); b.props.onClick(); host.flush(); }
function pick(host: Host) { const n = selected(host); assert.equal(n.props.disabled, false); n.props.onChange({ target: { value: customerID } }); host.flush(); }
const ready = (host: Host) => host.output.props["aria-busy"] === false && textOf(host.output).includes("MOCK_INBOX_BUYER");
function server(options: { conflict?: boolean; gone?: boolean; unknown?: boolean; lostAck?: boolean; omittedOrders?: boolean; dangling?: boolean } = {}) {
  const calls: { path: string; method: string; body: any; key: string | null }[] = [];
  let version = 0, linked: string | null = options.dangling ? customerID : null, failure = false;
  const concurrentID = "60000000-0000-4000-8000-000000000006";
  globalThis.fetch = async (input, init) => {
    const path = String(input), method = init?.method ?? "GET";
    const body = init?.body ? JSON.parse(String(init.body)) : null;
    calls.push({ path, method, body, key: new Headers(init?.headers).get("idempotency-key") });
    if (path.includes("/buyer-panel?")) { const p: any = { ...buyer, linked_customer_id: linked }; if (options.omittedOrders) delete p.orders; return response(p); }
    if (path.includes("/messages?")) return response({ ...thread, link_version: version, binding_id: bindingA });
    if (path.includes("/customers?")) return response({ items: [customer], next_cursor: "" });
    if (path.endsWith(`/customers/${concurrentID}`)) return response({ ...customer, customer_id: concurrentID, display_name: "MOCK_CONCURRENT_CUSTOMER", orders: [], claims: [], consent_history: [], privacy_actions: [], tags_revision: "a".repeat(64), notes: [] });
    if (options.dangling && path.endsWith(`/customers/${customerID}`)) return response({ code: "not_found" }, 404);
    if (path.endsWith(`/customers/${customerID}`)) return response({ ...customer, orders: [], claims: [], consent_history: [], privacy_actions: [], tags_revision: "a".repeat(64), notes: [] });
    if (path.endsWith("/customer-link")) {
      if (options.gone) return response({ code: "not_found" }, 404);
      if (options.conflict && !failure) { failure = true; version = 6; linked = concurrentID; return response({ code: "version_conflict" }, 409); }
      if (options.unknown && !failure) { failure = true; throw new TypeError("synthetic lost acknowledgement"); }
      if (body.expected_version !== version) return response({ code: "version_conflict" }, 409); // real A14: no idempotent replay
      linked = body.customer_id; version++;
      if (options.lostAck && !failure) { failure = true; throw new TypeError("synthetic lost acknowledgement after commit"); }
      return response({ customer_id: linked, version });
    }
    throw new Error("unexpected synthetic route");
  };
  return { calls, writes: () => calls.filter(c => c.method === "POST" && c.path.endsWith("/customer-link")) };
}

test("A14 takes A9 version zero; CAS keeps the choice and refreshes before explicit retry, then unlink", async t => {
  const env = environment(t), wire = server({ conflict: true });
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "buyer and A9 loaded"); pick(host); click(host, "customer-link-save");
  await host.waitFor(() => !host.output.props["aria-busy"] && wire.writes().length === 1, "CAS rejection and fresh A9");
  assert.deepEqual(wire.writes()[0].body, { customer_id: customerID, expected_version: 0 });
  assert.equal(selected(host).props.value, customerID, "CAS must preserve the merchant's choice");
  assert.match(textOf(host.output), /changed/i);
  assert.ok(textOf(host.output).includes("MOCK_CONCURRENT_CUSTOMER"), "fresh CAS authority must describe the winner before allowing unlink");
  await host.settle(); assert.equal(wire.writes().length, 1, "no automatic CAS retry");
  click(host, "customer-link-save");
  await host.waitFor(() => ready(host) && textOf(host.output).includes("Manually linked"), "persisted linked name");
  assert.deepEqual(wire.writes()[1].body, { customer_id: customerID, expected_version: 6 });
  assert.notEqual(wire.writes()[0].key, wire.writes()[1].key, "new explicit command after definitive 409");
  assert.ok(textOf(host.output).includes(customer.display_name));
  click(host, "customer-link-remove");
  await host.waitFor(() => ready(host) && wire.writes().length === 3 && !textOf(host.output).includes("Manually linked"), "unlink authoritative refresh");
  assert.deepEqual(wire.writes()[2].body, { customer_id: null, expected_version: 7 });
  assert.ok(wire.calls.every(c => !c.path.includes(customer.display_name)), "customer names never enter URLs");
});

test("a 404 on the linked customer read keeps the authorized A13 panel and shows the link without a name", async t => {
  const env = environment(t); server({ dangling: true });
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "A13 stays visible after a dangling linked customer");
  const link = node(host, n => n.props["data-testid"] === "customer-manual-link");
  assert.match(textOf(link), /Manually linked/);
  assert.equal(textOf(link).includes("·"), false, "a gone customer must not render a fallback name");
  assert.doesNotMatch(textOf(host.output), /unavailable in the current store/);
  assert.equal(selected(host).props.disabled, false, "the merchant can relink or unlink");
});

test("A14 404 tears down the buyer, choice and customer list, including late focus", async t => {
  const env = environment(t), wire = server({ gone: true });
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "buyer loaded"); pick(host); click(host, "customer-link-save");
  await host.waitFor(() => wire.writes().length === 1 && !host.output.props["aria-busy"] && !textOf(host.output).includes("MOCK_INBOX_BUYER"), "404 teardown");
  env.window.dispatchEvent(new Event("focus")); await host.settle();
  assert.equal(textOf(host.output).includes(customer.display_name), false);
  assert.equal(nodes(host.output).some(n => n.type === "select"), false);
  assert.match(textOf(host.output), /unavailable in the current store/);
});

test("A14 uncertain transport preserves the original key/body and permits only explicit retry", async t => {
  const env = environment(t), wire = server({ unknown: true });
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "buyer loaded"); pick(host); click(host, "customer-link-save");
  await host.waitFor(() => !host.output.props["aria-busy"] && wire.writes().length === 1, "uncertain result");
  assert.equal(selected(host).props.disabled, true);
  await host.settle(); assert.equal(wire.writes().length, 1);
  click(host, "customer-link-retry");
  await host.waitFor(() => ready(host) && textOf(host.output).includes("Manually linked"), "same command completed");
  assert.deepEqual(wire.writes()[1].body, wire.writes()[0].body);
  assert.equal(wire.writes()[1].key, wire.writes()[0].key);
});

test("A14 lost acknowledgement after commit: explicit retry reconciles to success without a misleading conflict", async t => {
  const env = environment(t), wire = server({ lostAck: true });
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "buyer loaded"); pick(host); click(host, "customer-link-save");
  await host.waitFor(() => !host.output.props["aria-busy"] && wire.writes().length === 1, "uncertain result");
  click(host, "customer-link-retry");
  await host.waitFor(() => ready(host) && textOf(host.output).includes("Manually linked"), "reconciled committed link");
  assert.equal(wire.writes().length, 1, "a committed command is reconciled by reading A13/A9, not resent");
  assert.doesNotMatch(textOf(host.output), /changed/i, "no version_conflict notice for our own committed write");
  assert.ok(textOf(host.output).includes(customer.display_name));
});

for (const permissions of [["inbox:read", "inbox:reply"], ["inbox:read", "customers:read"]]) {
  test(`A14 missing required permission ${permissions.join(",")} leaves a explained disabled control`, async t => {
    const env = environment(t), wire = server();
    const restricted = { ...store, role: "staff", permissions };
    const host = env.mount(() => BuyerPanel({ store: restricted, conversationId: conversation.conversation_id }));
    await host.waitFor(() => ready(host), "restricted buyer loaded");
    assert.equal(button(host, "customer-link-save").props.disabled, true);
    assert.match(textOf(host.output), /reply.*customer read|customer read.*reply/i);
    assert.equal(wire.calls.some(c => c.path.includes("/customers?")), false);
  });
}

test("full A13 renders recorded claim session/total/ordinal/auto-reply and distinguishes withheld orders", async t => {
  const env = environment(t); server({ omittedOrders: true });
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "buyer loaded"); const text = textOf(host.output);
  for (const expected of [sessionID, "MOCK_KEYWORD", "TWD 24000", "Purchase #2", "Sent", "The server has not shared order details."]) assert.ok(text.includes(expected), expected);
  assert.equal(text.includes("No orders supplied."), false);
});

for (const [binding, matching, other, enabled] of [[bindingA, "missing_permission", "ok", false], [bindingA, "ok", "missing_permission", true], [null, "ok", "ok", false]] as const) {
  test(`A9 binding ${binding ?? "null"} scopes DM capability to ${matching}, ignoring unrelated ${other}`, async t => {
    const env = environment(t);
    setHealth({ pages: [{ capabilities: [
      { binding_id: bindingA, provider: "facebook", capability: "dm_session", state: matching },
      { binding_id: bindingB, provider: "facebook", capability: "dm_session", state: other },
    ] }] });
    transport(() => response({ ...thread, binding_id: binding, link_version: 0 }));
    const host = mountThread(env); await host.waitFor(() => !host.output.props["aria-busy"] && textOf(host.output).includes("MOCK_DM"), "thread loaded");
    change(host, "reply-text", "MOCK_REPLY");
    assert.equal(node(host, n => n.props["data-testid"] === "reply-send").props.disabled, !enabled);
    assert.equal(nodes(host.output).some(n => n.props["data-testid"] === "inbox-capability-advisory"), false);
  });
}


test("customer-page post-parse session change clears previously visible buyer and chosen customer", async t => {
  const env = environment(t); server(); const fetch = globalThis.fetch; let pageReads = 0, expired = 0;
  globalThis.fetch = async (input, init) => {
    if (String(input).includes("/customers?")) {
      pageReads++;
      const res = response({ items: [customer], next_cursor: "bmV4dA" });
      if (pageReads > 1) {
        const json = res.json.bind(res);
        res.json = async () => { const data = await json(); env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`; return data; };
      }
      return res;
    }
    return fetch(input, init);
  };
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id, onUnauthorized() { expired++; } }));
  await host.waitFor(() => ready(host), "private buyer initially loaded"); pick(host);
  const more = node(host, n => n.type === "button" && textOf(n) === "Next customers");
  more.props.onClick(); host.flush();
  await host.waitFor(() => pageReads === 2 && !host.output.props["aria-busy"], "customer read completed after cookie change");
  assert.equal(expired, 1, "customer session fence must expire the containing private view");
  assert.equal(textOf(host.output).includes("MOCK_INBOX_BUYER"), false);
  assert.equal(textOf(host.output).includes(customer.display_name), false);
  assert.equal(nodes(host.output).some(n => n.type === "select"), false);
});

// PR #41 review (P1): each read's 401/403 revokes at once. A pending sibling read (customer page) must not delay
// it, and a sibling failure that arrives first must not swallow it.
test("A13 403 revokes immediately while the customer page read is still pending", async t => {
  const env = environment(t); server(); const fetch = globalThis.fetch; let expired = 0, release: () => void = () => {};
  globalThis.fetch = async (input, init) => {
    const path = String(input);
    if (path.includes("/customers?")) { await new Promise<void>(r => { release = r; }); return response({ items: [customer], next_cursor: "" }); }
    if (path.includes("/buyer-panel?")) return response({ code: "forbidden" }, 403);
    return fetch(input, init);
  };
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id, onUnauthorized() { expired++; } }));
  await host.waitFor(() => expired === 1, "A13 403 revoked without waiting for the customer page");
  release(); await host.settle();
  assert.equal(expired, 1); assert.equal(textOf(host.output).includes("MOCK_INBOX_BUYER"), false);
});

// PR #41 review (P2): the CAS version must never be newer than the link identity shown, so A9 is read BEFORE A13.
// A concurrent relink between the two reads can then only produce a safe 409, never unlink someone else's link.
test("A9 link_version is read before A13 so the CAS version is never newer than the displayed link", async t => {
  const env = environment(t); server(); const fetch = globalThis.fetch; const order: string[] = [];
  globalThis.fetch = async (input, init) => {
    const path = String(input);
    if (path.includes("/messages?")) { order.push("a9:start"); await new Promise(r => setTimeout(r, 20)); order.push("a9:end"); }
    if (path.includes("/buyer-panel?")) order.push("a13:start");
    return fetch(input, init);
  };
  const host = env.mount(() => BuyerPanel({ store, conversationId: conversation.conversation_id }));
  await host.waitFor(() => ready(host), "buyer loaded");
  assert.ok(order.indexOf("a9:end") >= 0 && order.indexOf("a9:end") < order.indexOf("a13:start"), `A13 started before A9 settled: ${order.join(",")}`);
});
