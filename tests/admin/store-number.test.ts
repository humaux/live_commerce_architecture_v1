import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

test("numeric store IDs have no suggestion BFF or browser client", () => {
  assert.equal(existsSync(new URL("../../apps/admin/app/api/onboarding/handle-suggest/route.ts", import.meta.url)), false);
  assert.equal(existsSync(new URL("../../apps/admin/lib/onboarding-client.ts", import.meta.url)), false);
  const entry = readFileSync(new URL("../../apps/admin/components/Entry.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(entry, /suggestHandle|handleNameHint|parseHandleSuggestion/);
});
