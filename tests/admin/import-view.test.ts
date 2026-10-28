// Purpose: preserve failure-first row correlation and refuse raw cells in a preview-only failure CSV.
// Depends on: Node test/assert and actual import-view-model helpers; fixtures are synthetic markers only.
// Used by: W5-U1 local Node red/green and Node CI; no browser/PG acceptance claim.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { orderedImportRows, safeImportFailureCSV } from "../../apps/admin/lib/import-view-model.ts";
import { importCopy } from "../../apps/admin/lib/import-copy.ts";

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
test("CU-01 explains source IDs and safe failure-download metadata in every locale", () => {
  const expected = {
    en: {
      sourceIdHint: "Unique customer ID in your old system (source_id).",
      failedDownload: "Download rows not imported (CSV)",
      downloadStarted: "Download started. The CSV contains only source row numbers, outcomes and reason codes; no customer names, phone numbers or source IDs.",
    },
    "zh-TW": {
      sourceIdHint: "\u4F60\u820A\u7CFB\u7D71\u88E1\u6BCF\u4F4D\u9867\u5BA2\u7684\u552F\u4E00\u7DE8\u865F\uFF08source_id\uFF09\u3002",
      failedDownload: "\u4E0B\u8F09\u672A\u532F\u5165\u7684\u5217\uFF08CSV\uFF09",
      downloadStarted: "\u5DF2\u958B\u59CB\u4E0B\u8F09\u3002CSV \u53EA\u542B\u4F86\u6E90\u5217\u865F\u3001\u7D50\u679C\u8207\u539F\u56E0\u4EE3\u78BC\uFF0C\u4E0D\u542B\u59D3\u540D\u3001\u96FB\u8A71\u6216\u4F86\u6E90\u7DE8\u865F\u3002",
    },
    "zh-CN": {
      sourceIdHint: "\u4F60\u65E7\u7CFB\u7EDF\u91CC\u6BCF\u4F4D\u987E\u5BA2\u7684\u552F\u4E00\u7F16\u53F7\uFF08source_id\uFF09\u3002",
      failedDownload: "\u4E0B\u8F7D\u672A\u5BFC\u5165\u7684\u884C\uFF08CSV\uFF09",
      downloadStarted: "\u5DF2\u5F00\u59CB\u4E0B\u8F7D\u3002CSV \u53EA\u542B\u6765\u6E90\u884C\u53F7\u3001\u7ED3\u679C\u4E0E\u539F\u56E0\u4EE3\u7801\uFF0C\u4E0D\u542B\u59D3\u540D\u3001\u7535\u8BDD\u6216\u6765\u6E90\u7F16\u53F7\u3002",
    },
  } as const;
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    assert.equal(importCopy[locale].sourceIdHint, expected[locale].sourceIdHint);
    assert.equal(importCopy[locale].failedDownload, expected[locale].failedDownload);
    assert.equal(importCopy[locale].downloadStarted, expected[locale].downloadStarted);
  }
  const steps = readFileSync(new URL("../../apps/admin/components/ImportWizardSteps.tsx", import.meta.url), "utf8");
  assert.match(steps, /c\.sourceIdHint/);
});
