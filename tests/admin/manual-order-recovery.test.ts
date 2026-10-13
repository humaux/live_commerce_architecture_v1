// Purpose: reproduce the manual-page UNKNOWN/dynamic-refusal dead end through actual component and recovery client seams.
// Depends on: inbox review hook host, production ManualOrder and fetch-boundary fixtures; no business-client mocks.
// Used by: test-node; MOCK component evidence complements the real browser/PG manual-order gate.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { environment, store, response, nodes, node, textOf } from "./inbox-review-host.test.ts";
// Presentational Next boundaries only; form, attempts, parsers and network clients remain production code.
registerHooks({ resolve(specifier, context, next) {
  const presentation: Record<string, string> = {
    "next/link": 'export default function Link(props){return {type:"a",props}}',
    "./AdminPageHeader": 'export function AdminPageHeader(){return null}',
  };
  return Object.hasOwn(presentation, specifier) ? { url: "data:text/javascript," + encodeURIComponent(presentation[specifier]), shortCircuit: true } : next(specifier, context);
} });
const { ManualOrder } = await import("../../apps/admin/components/ManualOrder.tsx");
const { createOrderCopy } = await import("../../apps/admin/lib/create-order-copy.ts");

const sku = "77777777-7777-4777-8777-777777777777";
const optionKey = `${sku}|TW|home`;
const order = "44444444-4444-4444-8444-444444444444";
const option = { option_key: optionKey, market_id: sku, country: "TW", delivery_code: "home", delivery_kind: "home",
  mode: "offline", name_en: "Synthetic home", name_hans: "测试宅配", name_hant: "測試宅配", currency: "TWD",
  service_version: 1, allocation_version: 1, payment_modes: ["bank_transfer"], pickup_selection: null };

async function form(t: any, locale: "en" | "zh-TW" | "zh-CN", resolution: "not_created" | "created" | "unknown" | "unauthorized" | "initial_created" | Promise<Response>) {
  const env = environment(t);
  const saved = new Map<string, string>();
  Object.assign(env.window, { sessionStorage: { getItem: (key: string) => saved.get(key) ?? null,
    setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key) } });
  const writes: { path: string; key: string | null; body: string }[] = [];
  globalThis.fetch = async (input, init) => {
    const path = String(input);
    if (path.endsWith("/orders/manual/options")) return response({ options: [option] });
    if (path.endsWith("/orders/manual") || path.endsWith("/orders/manual/recover")) {
      writes.push({ path, key: new Headers(init?.headers).get("Idempotency-Key"), body: String(init?.body) });
      if (path.endsWith("/recover")) return typeof resolution !== "string" ? resolution : resolution === "unauthorized" ? response({ code: "unauthorized" }, 401) : resolution === "unknown" ? response({ code: "retry_later" }, 503)
        : response(resolution === "created" ? { state: "created", order_id: order } : { state: "not_created" });
      if (resolution === "initial_created") return response({ order_id: order, commercial_state: "AWAITING_TRANSFER", payment_mode: "bank_transfer",
        total_minor: 100, currency: "TWD", expires_at: "2030-01-01T00:00:00Z", buyer_link: `https://synthetic.invalid/en/pay#o=${order}&t=${"a".repeat(43)}`,
        link_state: "configured", source: "merchant_manual" }, 201);
      return writes.length === 1 ? response({ code: "retry_later" }, 503) : response({ code: "bank_transfer_unavailable" }, 422);
    }
    throw new Error(`Unexpected manual recovery network fixture: ${path}`);
  };
  const render = () => ManualOrder({ locale, stores: [store], store, initialError: null, renderKey: "manual-review" });
  const h = env.mount(render); await h.settle();
  assert.ok(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-form"), "positive loaded form");
  // Exercise the controlled form's public change port rather than a private useState ordinal.
  const fields = nodes(h.raw).find(n => (n.type as any)?.name === "ManualOrderFormFields");
  assert.ok(fields, "real shared form interface");
  fields.props.onChange({ lines: [{ sku_id: sku, quantity: 1, label: "Synthetic", code: "SKU", price: "TWD 1" }],
    name: "SYNTHETIC BUYER", phone: "0912345678", optionKey, mode: "bank_transfer",
    home: { region: "Taiwan", city: "Taipei", postal_code: "100", line1: "Synthetic address", line2: "" } });
  await h.settle();
  const submit = async () => {
    assert.equal(node(h, n => n.props["data-testid"] === "manual-order-submit").props.disabled, false);
    node(h, n => n.props["data-testid"] === "manual-order-form").props.onSubmit({ preventDefault() {} }); await h.settle();
  };
  await submit();
  if (resolution === "initial_created") return { h, writes, submit, env, render };
  node(h, n => n.props["data-testid"] === "mo-name").props.onChange({ target: { value: "SYNTHETIC EDITED DRAFT" } });
  await h.settle(); await submit();
  assert.ok(textOf(node(h, n => n.props["data-testid"] === "manual-order-error")), "UNKNOWN then dynamic refusal loaded");
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-new-attempt"), false);
  assert.equal(writes.length, 2); assert.equal(writes[0].key, writes[1].key); assert.equal(writes[0].body, writes[1].body);
  return { h, writes, submit, env, render };
}
for (const event of ["hide", "logout", "forbidden", "not-found"] as const) {
  test(`ManualOrder late link regeneration after ${event} cannot restore a discarded order or buyer link`, async t => {
    const { h, env, render, writes } = await form(t, "en", "initial_created");
    assert.ok(node(h, n => n.props["data-testid"] === "manual-order-result"), "positive completed order before regenerate");
    let resolve!: (value: Response) => void;
    const delayed = new Promise<Response>(r => { resolve = r; });
    const previous = globalThis.fetch;
    let regenerations = 0;
    globalThis.fetch = async (input, init) => {
      if (String(input).endsWith("/orders/manual/regenerate-link")) { regenerations++; return delayed; }
      return previous(input, init);
    };
    node(h, n => n.props["data-testid"] === "manual-order-regenerate").props.onClick(); await h.settle();
    assert.equal(regenerations, 1, "actual regeneration client reached fetch");
    if (event === "hide") {
      env.document.visibilityState = "hidden"; env.document.dispatchEvent(new Event("visibilitychange")); await h.settle();
      env.document.visibilityState = "visible"; env.document.dispatchEvent(new Event("visibilitychange"));
    } else if (event === "logout") env.window.dispatchEvent(new Event("commerce-session-logout"));
    else { h.render = () => ManualOrder({ locale: "en", stores: [store], store, initialError: event, renderKey: "manual-review" }); h.dirty = true; }
    await h.settle();
    resolve(response({ order_id: order, source: "merchant_manual", buyer_link: `https://synthetic.invalid/en/pay#o=${order}&t=${"b".repeat(43)}` }, 201));
    await h.settle();
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-result"), false);
    assert.equal(textOf(h.output).includes(order), false);
    assert.equal(textOf(h.output).includes("synthetic.invalid"), false);
    assert.equal(writes.length, 1); assert.equal(regenerations, 1);
    h.dispose(); const remounted = env.mount(render); await remounted.settle();
    assert.equal(nodes(remounted.output).some(n => n.props["data-testid"] === "manual-order-result"), false);
  });
}

for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`ManualOrder ${locale} UNKNOWN/refusal only unlocks a new intent after authoritative absence seal`, async t => {
    const { h, writes } = await form(t, locale, "not_created");
    const recover = node(h, n => n.props["data-testid"] === "manual-order-recover-attempt");
    assert.equal(recover.props.disabled, false); recover.props.onClick(); await h.settle();
    assert.equal(writes.length, 3); assert.ok(writes[2].path.endsWith("/orders/manual/recover"));
    assert.ok(writes.every(w => w.key === writes[0].key && w.body === writes[0].body));
    assert.equal(node(h, n => n.props["data-testid"] === "manual-order-submit").props.disabled, true);
    assert.ok(textOf(h.output).includes(createOrderCopy[locale].recovered));
    node(h, n => n.props["data-testid"] === "manual-order-new-attempt").props.onClick(); await h.settle();
    assert.equal(writes.length, 3, "new intent is explicit and sends no request");
    assert.equal(node(h, n => n.props["data-testid"] === "mo-name").props.value, "SYNTHETIC EDITED DRAFT");
    assert.equal(node(h, n => n.props["data-testid"] === "manual-order-submit").props.disabled, false);
  });
}
test("ManualOrder partial recovery retains the original key/body and prevents a new intent", async t => {
  const { h, writes, submit } = await form(t, "en", "unknown");
  node(h, n => n.props["data-testid"] === "manual-order-recover-attempt").props.onClick(); await h.settle();
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-new-attempt"), false);
  await submit(); assert.equal(writes.length, 4);
  assert.ok(writes.every(w => w.key === writes[0].key && w.body === writes[0].body));
});
test("ManualOrder created recovery shows the existing order without offering a replacement key", async t => {
  const { h, writes } = await form(t, "en", "created");
  node(h, n => n.props["data-testid"] === "manual-order-recover-attempt").props.onClick(); await h.settle();
  const link = node(h, n => n.props["data-testid"] === "manual-order-recovered-order");
  assert.ok(link.props.href.includes(order));
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-submit"), false);
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-new-attempt"), false);
  assert.equal(writes.length, 3);
});
test("ManualOrder reload retains only an opaque block and cannot dispatch a replacement key", async t => {
  const { h, env, render, writes } = await form(t, "en", "unknown");
  h.dispose(); const reloaded = env.mount(render); await reloaded.settle();
  assert.ok(textOf(node(reloaded, n => n.props["data-testid"] === "manual-order-guard")).includes(createOrderCopy.en.guard));
  assert.equal(nodes(reloaded.output).some(n => n.props["data-testid"] === "manual-order-form"), false);
  assert.equal(writes.length, 2);
  assert.equal(textOf(reloaded.output).includes("SYNTHETIC EDITED DRAFT"), false);
});
test("ManualOrder recovery authorization loss hides private draft without releasing its unresolved guard", async t => {
  const { h, env, render, writes } = await form(t, "en", "unauthorized");
  node(h, n => n.props["data-testid"] === "manual-order-recover-attempt").props.onClick(); await h.settle();
  assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-form"), false);
  assert.equal(textOf(h.output).includes("SYNTHETIC EDITED DRAFT"), false);
  h.dispose(); const reloaded = env.mount(render); await reloaded.settle();
  assert.ok(nodes(reloaded.output).some(n => n.props["data-testid"] === "manual-order-guard"));
  assert.equal(writes.length, 3);
});
for (const event of ["hide", "pagehide", "logout"] as const) for (const state of ["not_created", "created"] as const) {
  test(`ManualOrder late ${state} recovery after ${event} preserves the guard and discards private state`, async t => {
    let resolve!: (value: Response) => void;
    const delayed = new Promise<Response>(r => { resolve = r; });
    const { h, env, render, writes } = await form(t, "en", delayed);
    node(h, n => n.props["data-testid"] === "manual-order-recover-attempt").props.onClick(); await h.settle();
    assert.equal(writes.length, 3, "recovery actually reached the real client");
    if (event === "hide") { env.document.visibilityState = "hidden"; env.document.dispatchEvent(new Event("visibilitychange")); }
    else env.window.dispatchEvent(new Event(event === "logout" ? "commerce-session-logout" : "pagehide"));
    await h.settle();
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-form"), false, "conceal clears the private form synchronously");
    resolve(response(state === "created" ? { state, order_id: order } : { state })); await h.settle();
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-recovered-order"), false);
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-new-attempt"), false);
    assert.equal(textOf(h.output).includes("SYNTHETIC EDITED DRAFT"), false);
    // Native hide does not change the CSRF cookie: the privacy fence, not the session hash, must reject the late response.
    h.dispose(); env.document.visibilityState = "visible";
    const remounted = env.mount(render); await remounted.settle();
    assert.ok(nodes(remounted.output).some(n => n.props["data-testid"] === "manual-order-guard"));
    assert.equal(writes.length, 3, "conceal cannot dispatch a replacement order");
  });
}
for (const code of ["forbidden", "not-found"] as const) for (const state of ["not_created", "created"] as const) {
  test(`ManualOrder late ${state} recovery after ${code} read cannot restore private state or release the guard`, async t => {
    let resolve!: (value: Response) => void;
    const delayed = new Promise<Response>(r => { resolve = r; });
    const { h, env, render, writes } = await form(t, "en", delayed);
    node(h, n => n.props["data-testid"] === "manual-order-recover-attempt").props.onClick(); await h.settle();
    assert.equal(writes.length, 3, "positive pending recovery");
    h.render = () => ManualOrder({ locale: "en", stores: [store], store, initialError: code, renderKey: "manual-review" });
    h.dirty = true; await h.settle();
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-form"), false);
    resolve(response(state === "created" ? { state, order_id: order } : { state })); await h.settle();
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-recovered-order"), false);
    assert.equal(nodes(h.output).some(n => n.props["data-testid"] === "manual-order-new-attempt"), false);
    assert.equal(textOf(h.output).includes("SYNTHETIC EDITED DRAFT"), false);
    h.dispose(); const remounted = env.mount(render); await remounted.settle();
    assert.ok(nodes(remounted.output).some(n => n.props["data-testid"] === "manual-order-guard"));
    assert.equal(writes.length, 3);
  });
}
