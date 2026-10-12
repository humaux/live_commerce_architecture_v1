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

// PR #32 review P1 (thread 4232390268): a drawer-only 401/403 must revoke the parent Inbox/Console too, not just
// this BuyerPanel, or the parent keeps showing buyer names, messages and drafts after authority loss.
test("PR32 drawer 401 propagates to the parent onUnauthorized", async t => {
  const env = browserEnvironment(t), calls = network();
  const fetch = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    if (!String(input).includes("/tools/inbox/order-prefill?")) return fetch(input, init);
    calls.push(String(input)); return response({ code: "unauthorized" }, 401);
  };
  let parent = 0;
  const h = env.mount(() => BuyerPanel({ store: allowedStore, conversationId: conversation.conversation_id, onUnauthorized() { parent++; } } as any));
  await h.settle(); assert.ok(textOf(h.output).includes("SYNTHETIC_REVIEW_BUYER"), "A13 positively loaded");
  node(h, n => n.props["data-testid"] === "buyer-create-order").props.onClick(); await h.settle();
  assert.ok(calls.some(path => path.includes("/tools/inbox/order-prefill?")), "drawer issued A15");
  assert.equal(parent, 1, "parent console/inbox scope is revoked");
  assert.equal(textOf(h.output).includes("SYNTHETIC_REVIEW_BUYER"), false, "buyer facts cleared");
});

for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`PR32 ${locale} queued payment replay never rotates its sealed DM link through plain copy`, async t => {
    const env = browserEnvironment(t);
    const calls = network();
    const upstream = globalThis.fetch;
    const optionKey = `${session}|TW|home1`;
    globalThis.fetch = async (input, init) => {
      const path = String(input);
      if (path.includes("/tools/inbox/order-prefill?")) return response({ ...prefill,
        customer: { name: "SYNTHETIC_REVIEW_BUYER", phone: "0912345678", email: "" },
        last_delivery: { option_key: optionKey, home_address: { region: "", city: "Taipei", postal_code: "", line1: "Synthetic road", line2: "" } },
        suggested_option_key: optionKey });
      if (path.endsWith("/tools/orders/manual/options")) return response({ options: [{ option_key: optionKey,
        market_id: session, country: "TW", delivery_code: "home1", delivery_kind: "home", mode: "MANUAL",
        name_hans: "宅配", name_hant: "宅配", name_en: "Home", currency: "TWD", service_version: 1, allocation_version: 1,
        payment_modes: ["bank_transfer"], pickup_selection: null }] });
      if (path.endsWith("/tools/orders/for-buyer")) {
        calls.push(path);
        return response({ order_id: offer, commercial_state: "AWAITING_TRANSFER", payment_mode: "bank_transfer",
          total_minor: 200, currency: "TWD", expires_at: "2030-01-01T00:00:00Z", buyer_link: null,
          link_state: "configured", source: "merchant_manual", live_price: "not_applied", live_price_reason: "no_conversation",
          send: { state: "queued", operation_id: otherOffer } });
      }
      if (path.endsWith("/tools/orders/manual/regenerate-link")) {
        calls.push(path);
        return response({ order_id: offer, source: "merchant_manual", buyer_link: "https://store.example.invalid/en/order-link#o=" + offer + "&t=synthetic" });
      }
      return upstream(input, init);
    };
    const h = env.mount(() => CreateOrderDrawer({ locale, store: allowedStore, bundleId: bundle, sessionId: session, onClose() {} } as any));
    await h.settle();
    const mode = node(h, n => n.props["data-testid"] === "drawer-mode-bank_transfer");
    mode.props.onChange({ target: { checked: true } }); await h.settle();
    assert.equal(node(h, n => n.props["data-testid"] === "drawer-submit").props.disabled, false, "real loaded valid form");
    node(h, n => n.props["data-testid"] === "drawer-form").props.onSubmit({ preventDefault() {} });
    await h.settle();
    assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "drawer-result"), "positive success sentinel before negative checks");
    assert.ok(textOf(node(h, n => n.props["data-testid"] === "drawer-send-result")).includes(createOrderCopy[locale].queued));
    const copy = nodes(h.output).find(n => n.props["data-testid"] === "drawer-copy-link");
    if (copy && !copy.props.disabled) { copy.props.onClick(); await h.settle(); }
    assert.deepEqual(calls.filter(path => path.endsWith("/regenerate-link")), [], "plain copy cannot silently supersede the queued DM credential");
    assert.equal(copy, undefined, "queued state offers no plain copy control");
  });
}

/** Supply valid read DTOs at the fetch edge; all form, client, session and receipt code stays real. */
async function recoveryForm(t: any, locale: "en" | "zh-TW" | "zh-CN", outcome: (path: string, init?: RequestInit) => Response | Promise<Response>) {
  const env = browserEnvironment(t); network(); const upstream = globalThis.fetch;
  const optionKey = `${session}|TW|home1`;
  const writes: { path: string; body: string; key: string | null }[] = [];
  globalThis.fetch = async (input, init) => {
    const path = String(input);
    if (path.includes("/tools/inbox/order-prefill?")) return response({ ...prefill,
      customer: { name: "SYNTHETIC_REVIEW_BUYER", phone: "0912345678", email: "" },
      last_delivery: { option_key: optionKey, home_address: { region: "", city: "Taipei", postal_code: "", line1: "Synthetic road", line2: "" } }, suggested_option_key: optionKey });
    if (path.endsWith("/tools/orders/manual/options")) return response({ options: [{ option_key: optionKey,
      market_id: session, country: "TW", delivery_code: "home1", delivery_kind: "home", mode: "MANUAL",
      name_hans: "宅配", name_hant: "宅配", name_en: "Home", currency: "TWD", service_version: 1, allocation_version: 1,
      payment_modes: ["bank_transfer"], pickup_selection: null }] });
    if (path.includes("/tools/orders/for-buyer")) {
      writes.push({ path, body: init!.body as string, key: new Headers(init!.headers).get("Idempotency-Key") });
      return outcome(path, init);
    }
    return upstream(input, init);
  };
  let revoked = 0;
  const h = env.mount(() => CreateOrderDrawer({ locale, store: allowedStore, bundleId: bundle, sessionId: session,
    onClose() {}, onUnauthorized() { revoked++; } } as any));
  await h.settle(); node(h, n => n.props["data-testid"] === "drawer-mode-bank_transfer").props.onChange({ target: { checked: true } });
  await h.settle();
  const submit = async () => {
    assert.equal(node(h, n => n.props["data-testid"] === "drawer-submit").props.disabled, false);
    node(h, n => n.props["data-testid"] === "drawer-form").props.onSubmit({ preventDefault() {} }); await h.settle();
  };
  await submit(); await submit();
  assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "drawer-error"), "UNKNOWN then business refusal positively loaded");
  assert.equal(node(h, n => n.props["data-testid"] === "drawer-close").props.disabled, true);
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "drawer-new-attempt"), false, "no blind new key after 422");
  return { h, env, writes, revoked: () => revoked };
}
for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`PR32 ${locale} UNKNOWN then business422 permits a new intent only after durable not-created recovery`, async t => {
    let placed = 0;
    const { h, writes } = await recoveryForm(t, locale, path => path.endsWith("/recover")
      ? response({ state: "not_created" }) : ++placed === 1 ? response({ code: "retry_later" }, 503) : response({ code: "bank_transfer_unavailable" }, 422));
    const recover = node(h, n => n.props["data-testid"] === "drawer-recover-attempt");
    assert.equal(recover.props.disabled, false); recover.props.onClick(); await h.settle();
    assert.equal(writes.length, 3);
    assert.ok(writes[2].path.endsWith("/orders/for-buyer/recover"));
    assert.ok(writes.every(w => w.key === writes[0].key && w.body === writes[0].body), "original frozen request/key throughout recovery");
    assert.equal(node(h, n => n.props["data-testid"] === "drawer-submit").props.disabled, true, "seal cannot automatically create a fresh attempt");
    assert.equal(node(h, n => n.props["data-testid"] === "drawer-close").props.disabled, false);
    node(h, n => n.props["data-testid"] === "drawer-new-attempt").props.onClick(); await h.settle();
    assert.equal(writes.length, 3, "new-intent action never POSTs");
    assert.equal(node(h, n => n.props["data-testid"] === "drawer-submit").props.disabled, false);
  });
}
for (const resolution of ["created", "unknown", "unauthorized"] as const) {
  test(`PR32 UNKNOWN business422 recovery ${resolution} never enables a replacement order`, async t => {
    let placed = 0;
    const { h, writes, revoked } = await recoveryForm(t, "en", path => !path.endsWith("/recover")
      ? ++placed === 1 ? response({ code: "retry_later" }, 503) : response({ code: "invalid_request" }, 422)
      : resolution === "created" ? response({ state: "created", order_id: offer })
      : resolution === "unauthorized" ? response({ code: "unauthorized" }, 401) : response({ code: "retry_later" }, 503));
    node(h, n => n.props["data-testid"] === "drawer-recover-attempt").props.onClick(); await h.settle();
    assert.equal(writes.length, 3);
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "drawer-new-attempt"), false);
    if (resolution === "created") {
      assert.ok(node(h, n => n.props["data-testid"] === "drawer-view-order").props.href.includes(offer));
      assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "drawer-submit"), false);
    } else if (resolution === "unauthorized") {
      assert.equal(revoked(), 1); assert.equal(h.output, null);
    } else {
      assert.equal(node(h, n => n.props["data-testid"] === "drawer-close").props.disabled, true);
      node(h, n => n.props["data-testid"] === "drawer-recover-attempt").props.onClick(); await h.settle();
      assert.equal(writes.length, 4);
      assert.ok(writes.every(w => w.key === writes[0].key && w.body === writes[0].body), "lost resolution reply only retries its original intent");
    }
  });
}
test("PR32 lost seal reply then attempt_closed can recover the same durable receipt without a new key", async t => {
  let placed = 0;
  const { h, writes } = await recoveryForm(t, "en", path => path.endsWith("/recover")
    ? response({ state: "not_created" }) : ++placed === 1 ? response({ code: "retry_later" }, 503) : response({ code: "attempt_closed" }, 409));
  const recover = node(h, n => n.props["data-testid"] === "drawer-recover-attempt");
  assert.equal(recover.props.disabled, false); recover.props.onClick(); await h.settle();
  assert.equal(writes.length, 3);
  assert.ok(writes.every(w => w.key === writes[0].key && w.body === writes[0].body));
  assert.equal(node(h, n => n.props["data-testid"] === "drawer-close").props.disabled, false);
  assert.ok(node(h, n => n.props["data-testid"] === "drawer-recovery-result"));
});
for (const status of [401, 403]) test(`PR32 malformed recovery ${status} revokes the parent and discards the drawer`, async t => {
  let placed = 0;
  const { h, revoked } = await recoveryForm(t, "en", path => path.endsWith("/recover")
    ? new Response("PRIVATE_DIAGNOSTIC", { status }) : ++placed === 1 ? response({ code: "retry_later" }, 503) : response({ code: "invalid_request" }, 422));
  node(h, n => n.props["data-testid"] === "drawer-recover-attempt").props.onClick(); await h.settle();
  assert.equal(revoked(), 1); assert.equal(h.output, null);
});
test("PR32 a committed seal with lost recovery reply keeps the key until the same receipt is read", async t => {
  let placed = 0, recovered = 0;
  const { h, writes } = await recoveryForm(t, "en", path => path.endsWith("/recover")
    ? ++recovered === 1 ? (() => { throw new Error("lost seal response"); })() : response({ state: "not_created" })
    : ++placed === 1 ? response({ code: "retry_later" }, 503) : placed === 2 ? response({ code: "invalid_request" }, 422) : response({ code: "attempt_closed" }, 409));
  node(h, n => n.props["data-testid"] === "drawer-recover-attempt").props.onClick(); await h.settle();
  assert.equal(writes.length, 3); assert.equal(node(h, n => n.props["data-testid"] === "drawer-close").props.disabled, true);
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "drawer-new-attempt"), false);
  node(h, n => n.props["data-testid"] === "drawer-form").props.onSubmit({ preventDefault() {} }); await h.settle();
  node(h, n => n.props["data-testid"] === "drawer-recover-attempt").props.onClick(); await h.settle();
  assert.equal(writes.length, 5); assert.ok(writes.every(w => w.key === writes[0].key && w.body === writes[0].body));
  assert.equal(node(h, n => n.props["data-testid"] === "drawer-close").props.disabled, false);
});
for (const revoke of ["hide", "logout"] as const) test(`PR32 delayed recovery after ${revoke} cannot display absence or unlock another order`, async t => {
  let placed = 0, resolve: (r: Response) => void = () => { throw new Error("recovery not requested"); };
  const delayed = new Promise<Response>(r => { resolve = r; });
  const { h, env, writes } = await recoveryForm(t, "en", path => path.endsWith("/recover")
    ? delayed : ++placed === 1 ? response({ code: "retry_later" }, 503) : response({ code: "invalid_request" }, 422));
  node(h, n => n.props["data-testid"] === "drawer-recover-attempt").props.onClick(); await h.settle();
  assert.equal(writes.length, 3, "recovery reached the real client before revocation");
  if (revoke === "hide") { env.document.visibilityState = "hidden"; env.document.dispatchEvent(new Event("visibilitychange")); }
  else { env.document.cookie = ""; env.window.dispatchEvent(new Event("commerce-session-logout")); }
  await h.settle(); assert.equal(h.output, null, "private drawer is discarded before the late response");
  resolve(response({ state: "not_created" })); await h.settle();
  assert.equal(h.output, null);
  assert.equal(writes.length, 3, "a late recovery never submits a new intent");
});
