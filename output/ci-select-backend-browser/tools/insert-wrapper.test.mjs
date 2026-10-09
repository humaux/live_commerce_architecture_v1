// Purpose: sandbox wrapper — runs insert-covers.mjs under `node --test` (the permitted invocation form).
// Depends on: insert-covers.mjs (same directory).
// Used by: unit ci-select-backend-browser registry-write step only.
import test from "node:test";
test("insert covers", async () => { await import("./insert-covers.mjs"); });
