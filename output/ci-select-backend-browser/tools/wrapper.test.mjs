// Purpose: sandbox wrapper — runs derive-covers.mjs under `node --test` (the permitted invocation form).
// Depends on: derive-covers.mjs (same directory).
// Used by: unit ci-select-backend-browser derivation step only.
import test from "node:test";
test("derive covers", async () => { await import("./derive-covers.mjs"); });
