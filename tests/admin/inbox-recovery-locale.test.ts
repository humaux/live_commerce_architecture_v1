// Purpose: reproduce locale-sensitive premature completion in the real recovery wait helper.
// Depends on: actual BundleRecovery/claims client, generic Host and existing recoveryDone/bounded helpers.
// Used by: K3 item3 Node regression; a held M7 response supplies exact arrival/completion events, no browser/Go/PG.
// Invariants: waits never accept a busy Chinese control; request/draft/privacy logic remain real production code.
import test from "node:test";
import assert from "node:assert/strict";
import { environment, node, textOf, store, conversation, response } from "./inbox-review-host.test.ts";
import { recoveryDone, bounded } from "./inbox-recovery-env.test.ts";
const { BundleRecovery } = await import("../../apps/admin/src/features/messages/bundle-recovery.tsx");
const sid = "30000000-0000-4000-8000-000000000003",
  bid = "40000000-0000-4000-8000-000000000004",
  product = "60000000-0000-4000-8000-000000000006";
for (const [locale, loading] of [
  ["zh-TW", "載入中…"],
  ["zh-CN", "加载中…"],
  ["en", "Loading…"],
]) {
  test(`recoveryDone waits for actual held M7 completion in ${locale}`, async (t) => {
    const env = environment(t),
      arrived = Promise.withResolvers<void>(),
      held = Promise.withResolvers<Response>();
    let copies = 0,
      posts = 0;
    const previousNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");
    Object.defineProperty(globalThis, "navigator", {
      configurable: true,
      value: {
        clipboard: {
          async writeText() {
            copies++;
          },
        },
      },
    });
    t.after(() => {
      held.resolve(response({}, 503));
      if (previousNavigator) Object.defineProperty(globalThis, "navigator", previousNavigator);
      else delete (globalThis as any).navigator;
    });
    globalThis.fetch = async (input, init) => {
      const path = String(input);
      const privateJSON = (value: unknown) =>
        new Response(JSON.stringify(value), {
          headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store" },
        });
      if (path.includes("claims/bundles?"))
        return privateJSON({
          items: [
            {
              bundle_id: bid,
              ref: "ABCDEF12",
              platform: "facebook",
              label: "",
              bound: false,
              version: 1,
              link: { state: "NONE", generation: 0, expires_at: null },
              lines: [],
              created_at: "2030-01-01T00:00:00Z",
              updated_at: "2030-01-01T00:00:00Z",
            },
          ],
          next_cursor: "",
        });
      if (path.includes("/products?"))
        return response({ items: [{ id: product, name: "MOCK_PRODUCT", status: "active" }], next_cursor: "" });
      if (path.includes("purchase-entry?"))
        return response({
          product_id: product,
          locale: "en",
          state: "configured",
          url: `https://mock-shop.invalid/en/products/${product}`,
        });
      if (init?.method === "POST" && path.endsWith(`/claims/bundles/${bid}/link`)) {
        posts++;
        arrived.resolve();
        return held.promise;
      }
      throw new Error("Unexpected MOCK recovery locale route");
    };
    const host = env.mount(() => ({
      type: BundleRecovery,
      props: {
        store,
        locale,
        conversation: {
          ...conversation,
          conversation_id: null,
          bundle_id: bid,
          session_id: sid,
          link_pending_manual: true,
        },
        onUnauthorized() {},
      },
    }));
    const action = node(host, (n) => n.props["data-testid"] === "bundle-copy-link").props.onClick();
    await bounded(arrived.promise, "actual M7 request arrival");
    host.flush();
    const busy = node(host, (n) => n.props["data-testid"] === "bundle-link-retry");
    assert.equal(busy.props.disabled, true);
    assert.equal(textOf(busy), loading);
    let completed = false;
    const waiting = recoveryDone(host).then(() => {
      completed = true;
    });
    // This event probes one complete JS turn while the actual response remains held; it is not a readiness timer.
    await new Promise<void>((done) => setImmediate(done));
    assert.equal(completed, false, "helper must stay pending while locale-specific control is busy");
    assert.equal(posts, 1);
    assert.equal(copies, 0);
    held.resolve(
      new Response(
        JSON.stringify({
          token: "M".repeat(42) + "A",
          generation: 1,
          expires_at: "2099-01-01T00:00:00Z",
          released: false,
          replayed: false,
        }),
        { headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store" } },
      ),
    );
    await bounded(action, "actual component copy completion");
    await bounded(waiting, "locale-aware idle commit");
    assert.equal(completed, true);
    assert.equal(posts, 1);
    assert.equal(copies, 1);
    assert.equal(node(host, (n) => n.props["data-testid"] === "bundle-copy-link").props.disabled, false);
  });
}
