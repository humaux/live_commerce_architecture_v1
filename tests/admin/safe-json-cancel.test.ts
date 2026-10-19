// Purpose: issue #84 regression — the merchant BFF's safeJSON must cancel an upstream body it refuses to parse (wrong
// content type) instead of leaving it to GC (Node 24 bundled Undici guidance), and still return null.
// Depends on: apps/admin/lib/auth.ts (safeJSON); "server-only" is resolved to an empty module by registerHooks, as
// tests/admin/team-bff.test.ts does. No env, no network.
// Used by: scripts/dev/test-node.sh (Foundation "Node unit suites").
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { test } from "node:test";

registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    return next(specifier, context);
  },
});
const { safeJSON } = await import("../../apps/admin/lib/auth.ts");

test("safeJSON cancels a wrong-content-type body and returns null", async () => {
  for (const type of ["text/html", "application/problem+json", ""]) {
    let cancelled = 0;
    const response = new Response(new ReadableStream({ start() {}, cancel() { cancelled++; } }), {
      status: 502, headers: type ? { "content-type": type } : {},
    });
    assert.equal(await safeJSON(response), null, type);
    assert.equal(cancelled, 1, `${type || "no type"}: rejected body not cancelled`);
  }
});
