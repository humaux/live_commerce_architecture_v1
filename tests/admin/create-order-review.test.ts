// Purpose: reproduce PR32 session/catalog and UNKNOWN-settlement counterexamples through actual UI/client seams (MOCK).
// Depends on: inbox hook host, production BuyerPanel/CommentStream/Drawer, real clients and Node fetch edge fixtures.
// Used by: test-node; component evidence complements, and does not replace, real browser/PG drawer acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { environment, store, conversation, response, nodes, node, textOf, BuyerPanel, thread } from "./inbox-review-host.test.ts";
const { CreateOrderDrawer } = await import("../../apps/admin/components/CreateOrderDrawer.tsx");
const { CommentStream } = await import("../../apps/admin/components/CommentStream.tsx");
const { createOrderCopy } = await import("../../apps/admin/lib/create-order-copy.ts");
const { retainOrderAttempt } = await import("../../apps/admin/lib/create-order-attempt.ts");

const session = "22222222-2222-4222-8222-222222222222";
const otherSession = "33333333-3333-4333-8333-333333333333";
const offer = "44444444-4444-4444-8444-444444444444";
const otherOffer = "55555555-5555-4555-8555-555555555555";
const bundle = "66666666-6666-4666-8666-666666666666";
const sku = "77777777-7777-4777-8777-777777777777";
const permissions = ["inbox:read", "live:read", "orders:read", "inventory:reserve"];
const limitedStore = { ...store, role: "staff", permissions };
const allowedStore = { ...limitedStore, permissions: [...permissions, "catalog:read"] };
const claim = (offer_id: string, session_id: string) => ({ offer_id, session_id, keyword: "A1", quantity: 1, status: "ACCEPTED", total_minor: 100 });
const panel = (claims: unknown[]) => ({ display_name: "SYNTHETIC_REVIEW_BUYER", platform: "facebook", purchase_ordinal: 1, claims, claim_total_minor: 100, orders: [], link_pending_manual: false });
const prefill = {
  items: [{ sku_id: sku, offer_id: offer, keyword: "A1", name: "Synthetic item", variant: "SKU", quantity: 1, live_price_minor: 100, catalog_price_minor: 200, sellable: true, live_quantity_remaining: 1 }],
  bundles: [bundle], customer: null, last_delivery: null, suggested_option_key: null,
  live_price_eligible: false, live_price_reason: "no_conversation",
};
const row = {
  ref: "123_456", seq: 1, parent_ref: null, created_at: "2026-10-10T01:00:00Z", author_name: null,
  text: "SYNTHETIC_REVIEW_COMMENT", is_page: false, has_attachment: false,
  marks: { intake: null, claim: { status: "ACCEPTED", reason: null, offer_id: offer, keyword: "A1", quantity: 1, bundle_id: bundle }, private_reply: null, printed: null, public_replies: 0, private_reply_available: true },
};
const commentPage = { epoch: 1, items: [row], reset: false, scan_exhausted: false, next: { epoch: 1, seq: 1 }, older_cursor: null, stream: { state: "live", source_platform: "facebook", video_embeddable: true } };

/** Adapt browser primitives only; authorization, session hashing, DTO parsing and requests stay production source. */
function browserEnvironment(t: any) {
  const env = environment(t);
  const previousHTMLElement = (globalThis as any).HTMLElement;
  (globalThis as any).HTMLElement = class {};
  const saved = new Map<string, string>();
  Object.assign(env.window, { sessionStorage: { getItem: (key: string) => saved.get(key) ?? null, setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key) } });
  t.after(() => { (globalThis as any).HTMLElement = previousHTMLElement; });
  return env;
}
function network(claims = [claim(otherOffer, otherSession), claim(offer, session)]) {
  const calls: string[] = [];
  globalThis.fetch = async (input) => {
    const path = String(input);
    calls.push(path);
    if (path.includes("/inbox/buyer-panel?")) return response(panel(claims));
    // Trunk BuyerPanel (LC-U2b2) reads the A9 CAS header before A13 and pages existing customers when it can link.
    if (/\/inbox\/conversations\/[^/]+\/messages\?limit=1$/.test(path)) return response(thread);
    if (path.includes("/customers?")) return response({ items: [], next_cursor: "" });
    if (path.includes("/tools/inbox/order-prefill?")) return response(prefill);
    if (path.endsWith("/tools/orders/manual/options")) return response({ options: [] });
    if (path.includes("/claims/blocklist/check?")) return response({ restricted: true });
    if (path.includes("/comments?")) return response(commentPage);
    throw new Error(`Unexpected network edge fixture: ${path}`);
  };
  return calls;
}
const orderReads = (calls: string[]) => calls.filter(path => /\/tools\/(?:inbox\/order-prefill|orders\/(?:manual\/options|for-buyer))/.test(path));

for (const outcome of [{ status: 400, code: "invalid_json" }, { status: 415, code: "json_required" }]) {
  test(`PR32 parser ${outcome.status}/${outcome.code} cannot settle a previously UNKNOWN order`, () => {
    assert.equal(retainOrderAttempt({ ...outcome, uncertain: false }, true), true,
      "parser refusal happens before receipt lookup and cannot prove the prior order was not created");
  });
}
for (const target of ["conversation", "bundle"] as const) {
  test(`PR32 ${target} blocklist check uses the ordered offer session, not the first A13 claim`, async t => {
    // A13 by bundle_id reads only that bundle's claims (0165 v_bundles := ARRAY[p_bundle]) and a bundle has one session,
    // so mixed sessions are only real for a conversation (bundle_peers). The bundle variant keeps one session with the
    // ordered offer second; W3-U2 BuyerRestriction checks the same bundle session, so assert the set of checked URLs.
    const env = browserEnvironment(t), calls = network(target === "conversation" ? undefined : [claim(otherOffer, session), claim(offer, session)]);
    const h = env.mount(() => BuyerPanel({ store: allowedStore, ...(target === "conversation" ? { conversationId: conversation.conversation_id } : { bundleId: bundle }) } as any));
    await h.settle();
    assert.ok(textOf(h.output).includes("SYNTHETIC_REVIEW_BUYER"), "A13 positively loaded");
    const button = node(h, n => n.props["data-testid"] === "buyer-create-order");
    assert.equal(button.props.disabled, false);
    button.props.onClick(); await h.settle();
    assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "drawer-form"), "actual nested A15/options form loaded");
    assert.deepEqual([...new Set(calls.filter(path => path.includes("/claims/blocklist/check?")))],
      [`/api/stores/${store.id}/live-sessions/${session}/claims/blocklist/check?bundle_id=${bundle}`]);
    assert.equal(textOf(node(h, n => n.props["data-testid"] === "drawer-blocklist-warning")), createOrderCopy.en.restricted);
  });
}
test("PR32 zero claims shows an explicit unchecked warning after the actual drawer loads", async t => {
  const env = browserEnvironment(t), calls = network([]);
  const h = env.mount(() => BuyerPanel({ store: allowedStore, conversationId: conversation.conversation_id } as any));
  await h.settle(); assert.ok(textOf(h.output).includes("SYNTHETIC_REVIEW_BUYER"));
  node(h, n => n.props["data-testid"] === "buyer-create-order").props.onClick(); await h.settle();
  assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "drawer-form"), "positive loaded sentinel prevents vacuous absence");
  assert.equal(textOf(node(h, n => n.props["data-testid"] === "drawer-blocklist-warning")), createOrderCopy.en.restrictionUnavailable);
  assert.deepEqual(calls.filter(path => path.includes("/claims/blocklist/check?")), [], "never guess a session");
});
test("PR32 BuyerPanel missing catalog permission disables the real entry and sends no drawer request", async t => {
  const env = browserEnvironment(t), calls = network();
  const h = env.mount(() => BuyerPanel({ store: limitedStore, conversationId: conversation.conversation_id } as any));
  await h.settle(); assert.ok(textOf(h.output).includes("SYNTHETIC_REVIEW_BUYER"));
  const button = node(h, n => n.props["data-testid"] === "buyer-create-order");
  if (!button.props.disabled) { button.props.onClick(); await h.settle(); }
  assert.deepEqual({ disabled: button.props.disabled, orderRequests: orderReads(calls) }, { disabled: true, orderRequests: [] });
  assert.match(textOf(h.output), /catalog/i);
});
for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`PR32 ${locale} direct drawer missing catalog permission has no A15/options/A16 requests`, async t => {
    const env = browserEnvironment(t), calls = network(); let expired = 0;
    const h = env.mount(() => CreateOrderDrawer({ locale, store: limitedStore, bundleId: bundle, sessionId: session, onClose() {}, onUnauthorized() { expired++; } } as any));
    await h.settle();
    assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "create-order-drawer"), "actual drawer rendered");
    assert.deepEqual(orderReads(calls), [], "defence in depth prevents unauthorized reads and writes even through direct mounting");
    assert.equal(expired, 0, "missing local permission cannot expire the whole buyer panel");
    assert.ok(textOf(h.output).includes(createOrderCopy[locale].permission));
  });
  test(`PR32 ${locale} CommentStream missing catalog permission disables the actual row entry`, async t => {
    const env = browserEnvironment(t), calls = network();
    const h = env.mount(() => CommentStream({ store: limitedStore, session, locale, platform: "facebook", capabilities: {} } as any));
    await h.settle(); assert.ok(textOf(h.output).includes("SYNTHETIC_REVIEW_COMMENT"), "real A2 row loaded");
    const button = node(h, n => n.props["data-testid"] === `comment-create-order-${row.ref}`);
    if (!button.props.disabled) { button.props.onClick(); await h.settle(); }
    assert.deepEqual({ disabled: button.props.disabled, orderRequests: orderReads(calls) }, { disabled: true, orderRequests: [] });
    assert.ok(textOf(h.output).includes(createOrderCopy[locale].permission));
  });
}
test("PR32 granted catalog permission positively reaches real A15 and options seams", async t => {
  const env = browserEnvironment(t), calls = network();
  const h = env.mount(() => CreateOrderDrawer({ locale: "en", store: allowedStore, bundleId: bundle, sessionId: session, onClose() {} } as any));
  await h.settle();
  assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "drawer-form"));
  assert.deepEqual(orderReads(calls).sort(), [`/api/stores/${store.id}/tools/inbox/order-prefill?bundle_id=${bundle}`, `/api/stores/${store.id}/tools/orders/manual/options`].sort());
});
