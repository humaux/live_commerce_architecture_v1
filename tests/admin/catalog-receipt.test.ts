import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { test, type TestContext } from "node:test";

// Run the real client with native TS, resolving only extensionless app imports.
const lib = new URL("../../apps/admin/lib/", import.meta.url).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (
      context.parentURL?.startsWith(lib) &&
      specifier.startsWith("./") &&
      !specifier.endsWith(".ts")
    )
      return nextResolve(`${specifier}.ts`, context);
    return nextResolve(specifier, context);
  },
});
const { command, send } =
  await import("../../apps/admin/lib/catalog-v2-client.ts");
const { sessionBoundary } =
  await import("../../apps/admin/lib/settings-client.ts");
const store = "11111111-1111-4111-8111-111111111111";
const csrf = "c".repeat(43);
async function fixture(t: TestContext) {
  const original = Object.getOwnPropertyDescriptor(globalThis, "document");
  const document = { cookie: `__Host-commerce_csrf=${csrf}` };
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: document,
  });
  t.after(() => {
    if (original) Object.defineProperty(globalThis, "document", original);
    else Reflect.deleteProperty(globalThis, "document");
  });
  return { document, boundary: await sessionBoundary() };
}

for (const mode of ["client", "401", "403"] as const) {
  test(`UNKNOWN retry ${mode} authorization loss requires reconciliation, not a fresh command`, async (t) => {
    const { document, boundary } = await fixture(t);
    const cmd = command("POST", "products/document", {
      name: "Synthetic fixture",
    });
    const sent: RequestInit[] = [];
    let lost = true;
    t.mock.method(
      globalThis,
      "fetch",
      async (_input: unknown, init?: RequestInit) => {
        sent.push(init!);
        if (lost) throw new TypeError("Synthetic lost response");
        // Non-JSON 401/403 must also be classified by status, not error-copy text.
        return new Response("Denied", { status: Number(mode) });
      },
    );
    assert.deepEqual(await send(store, cmd, boundary, (v) => v), {
      ok: false,
      code: "retry_later",
      uncertain: true,
    });
    lost = false;
    if (mode === "client") document.cookie = "";
    const result = await send(store, cmd, boundary, (v) => v);
    assert.equal(result.ok, false);
    assert.equal("reconcile" in result && result.reconcile, true);
    assert.equal(sent.length, mode === "client" ? 1 : 2);
    for (const request of sent) {
      assert.equal(request.body, cmd.body);
      assert.equal(
        (request.headers as Record<string, string>)["Idempotency-Key"],
        cmd.key,
      );
    }
  });
}

test("a conclusive command rejection stays distinct from authorization loss", async (t) => {
  const { boundary } = await fixture(t);
  const cmd = command("PUT", "products/fixture/document", {
    expected_version: 4,
  });
  for (const status of [404, 409, 422]) {
    t.mock.method(
      globalThis,
      "fetch",
      async () =>
        new Response(JSON.stringify({ code: "invalid_request" }), { status }),
    );
    assert.deepEqual(await send(store, cmd, boundary, (v) => v), {
      ok: false,
      code: "invalid_request",
      uncertain: false,
    });
  }
});
