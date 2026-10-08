// Purpose: assert the requested old-to-new selection monotonicity using captured real CLI outputs.
// Depends on: compare.py's synthetic Git commits and actual base/candidate pr-modes executions.
// Used by: independent read-only cross-review; never loaded by production or repository CI.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
const rows = JSON.parse(readFileSync(new URL("./cli-comparison.json", import.meta.url), "utf8"));
for (const row of rows) {
  test("CLI monotonicity: " + row.name, () => {
    assert.deepEqual(row.dropped, [], "legacy required modes dropped for " + JSON.stringify(row.paths));
    assert.equal(row.deployDropped, false, "legacy deploy-smoke was dropped for " + JSON.stringify(row.paths));
  });
}

