// Purpose: preserve failure-first row correlation and refuse raw cells in a preview-only failure CSV.
// Depends on: Node test/assert and actual import-view-model helpers; fixtures are synthetic markers only.
// Used by: W5-U1 local Node red/green and Node CI; no browser/PG acceptance claim.
import test from "node:test";
import assert from "node:assert/strict";
import { orderedImportRows, safeImportFailureCSV } from "../../apps/admin/lib/import-view-model.ts";

const rows = [
  { row: 2, outcome: "created" as const, external_id: "DO_NOT_SHOW_SUCCESS_CELL" },
  { row: 9, outcome: "failed" as const, code: "erased", external_id: "DO_NOT_SHOW_FAILED_CELL", name: "DO_NOT_SHOW_NAME_CELL" },
  { row: 3, outcome: "updated" as const, warning: "city_dropped" as const },
  { row: 4, outcome: "failed" as const, code: "invalid_phone" },
];
test("failure verdicts precede successful rows, by source row number, without mutating the input", () => {
  const original = JSON.stringify(rows);
  assert.deepEqual(orderedImportRows(rows).map(row => row.row), [4, 9, 2, 3]);
  assert.equal(JSON.stringify(rows), original);
});
test("the failure download contains only failed row metadata, with no source cell or success identifier", () => {
  const csv = safeImportFailureCSV(rows);
  assert.equal(csv, "\uFEFFrow,outcome,code\r\n4,failed,invalid_phone\r\n9,failed,erased\r\n");
  assert.doesNotMatch(csv, /DO_NOT_SHOW|created|updated|city_dropped/);
  assert.equal(safeImportFailureCSV([]), "\uFEFFrow,outcome,code\r\n");
});
