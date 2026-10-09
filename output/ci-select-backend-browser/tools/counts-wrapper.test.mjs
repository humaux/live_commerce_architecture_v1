// Purpose: sandbox wrapper — runs selection-counts.mjs under `node --test` (the permitted invocation form).
// Depends on: selection-counts.mjs (same directory).
// Used by: unit ci-select-backend-browser evidence step only.
import test from "node:test";
test("selection counts", async () => { await import("./selection-counts.mjs"); });
