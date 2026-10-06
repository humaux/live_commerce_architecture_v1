// Purpose: independent negative properties for tracking DTOs, route grammar and lossless spreadsheet cells.
// Depends on: frozen fa25f019 API shape and pure tracking-import-model; no browser/PG.
// Used by: scripts/dev/test-node.sh and tracking-backfill acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { parseTrackingPreview, parseTrackingCommit, trackingRows, pasteTrackingCSV, trackingRoute, validTrackingQuery, trackingOrderNumber, TRACKING_MAX_BYTES } from "../../apps/admin/lib/tracking-import-model.ts";
const id = "11111111-1111-4111-8111-111111111111";
const a = { row: 1, order_number: id, carrier_code: "black_cat", tracking_number: "00123", outcome: "apply" };
const f = { row: 2, order_number: "bad-order", carrier_code: "other", tracking_number: "00045", outcome: "failed", code: "invalid_order_ref" };
const preview = { file_sha256: "a".repeat(64), rows_total: 2, apply_rows: 1, unchanged_rows: 0, failed_rows: 1, mail_eta_hours: 1, rows: [a, f] };
test("closed preview preserves raw failed references and rejects inconsistent or private projections", () => {
  assert.deepEqual(parseTrackingPreview(preview), preview);
  for (const change of [{ rows_total: 3 }, { apply_rows: -1 }, { failed_rows: 1.5 }, { apply_rows: Number.MAX_SAFE_INTEGER + 1 }, { mail_eta_hours: 2 }, { file_sha256: "x" }, { buyer_name: "private" }]) assert.throws(() => parseTrackingPreview({ ...preview, ...change }));
  for (const change of [{ outcome: "success" }, { row: 2 }, { tracking_number: 123 }, { carrier_code: "cvs_711" }, { code: "ok" }]) assert.throws(() => parseTrackingPreview({ ...preview, rows: [{ ...a, ...change }, f] }));
  const { code, ...noCode } = f; assert.throws(() => parseTrackingPreview({ ...preview, rows: [a, noCode] }));
});
test("replayed receipt keeps original applied count and accepts no internal fields", () => {
  const result = { batch_id: id, applied: 1, unchanged: 0, failed: 1, replayed: true };
  assert.deepEqual(parseTrackingCommit(result), result);
  for (const change of [{ batch_id: "../other" }, { replayed: 1 }, { applied: 0 }, { failed: -1 }, { applied: 501 }, { principal_id: id }]) assert.throws(() => parseTrackingCommit({ ...result, ...change }));
});
test("a valid receipt for a different confirmed row count is not this request's receipt", () => {
  const result = { batch_id: id, applied: 1, unchanged: 0, failed: 0, replayed: true };
  assert.throws(() => parseTrackingCommit(result, 2), /invalid_response/);
  assert.deepEqual(parseTrackingCommit(result, 1), result);
});
test("failure-first display is stable and leaves the source preview unchanged", () => {
  const valid = parseTrackingPreview(preview), before = JSON.stringify(valid);
  assert.deepEqual(trackingRows(valid).map(r => r.row), [2, 1]);
  assert.equal(JSON.stringify(valid), before);
  assert.equal(trackingOrderNumber(id), "LC-11111111111141118111111111111111");
  assert.equal(trackingOrderNumber("bad-order"), "bad-order");
});
test("only canonical tracking resource/method/query combinations can reach the BFF", () => {
  assert.equal(trackingRoute("POST", "shipments/tracking-import/preview"), "tracking-preview");
  assert.equal(trackingRoute("POST", "shipments/tracking-import/commit"), "tracking-commit");
  assert.equal(trackingRoute("GET", `shipments/tracking-import/${id}/result.csv`), "tracking-result");
  for (const [method, path] of [["GET", "shipments/tracking-import/commit"], ["PUT", "shipments/tracking-import/preview"], ["GET", "shipments/tracking-import/../../result.csv"]]) assert.equal(trackingRoute(method, path), null);
  for (const n of [0, 1, 500]) assert.equal(validTrackingQuery("tracking-commit", `?expected_apply_rows=${n}`), true);
  for (const q of ["", "?", "?expected_apply_rows=-1", "?expected_apply_rows=1.5", "?expected_apply_rows=501", "?expected_apply_rows=01", "?expected_apply_rows=1&expected_apply_rows=1", "?expected_apply_rows=1&tenant_id=x"]) assert.equal(validTrackingQuery("tracking-commit", q), false);
  assert.equal(validTrackingQuery("tracking-result", "?only=failed"), true);
  assert.equal(validTrackingQuery("tracking-result", "?only=failed&x=1"), false);
  assert.equal(validTrackingQuery("tracking-preview", "?"), false);
});
test("TSV cells retain leading zeroes, Unicode, embedded commas/quotes/newlines and business whitespace", () => {
  const raw = `${id}\t其他\t00012\t"合成,物流\n第二行"\n${id}\t黑貓\t00123 \t"A""B"`;
  const encoded = new TextDecoder().decode(pasteTrackingCSV(raw));
  assert.equal(encoded, `"order_number","carrier","tracking_number","carrier_name","tracking_url"\r\n"${id}","其他","00012","合成,物流\n第二行",""\r\n"${id}","黑貓","00123 ","A""B",""\r\n`);
});
test("header/BOM CSV is not given a second header and its data are not business-normalized", () => {
  const raw = `\uFEFF訂單編號,物流商,運單號\r\n${id},黑貓,00001\r\n\r\n`;
  assert.equal(new TextDecoder().decode(pasteTrackingCSV(raw)), `"order_number","carrier","tracking_number"\r\n"${id}","黑貓","00001"\r\n`);
  const quoted = 'order_number,carrier,tracking_number,carrier_name\n"bad,ref",其他,001,"A""B"';
  assert.match(new TextDecoder().decode(pasteTrackingCSV(quoted)), /"bad,ref","其他","001","A""B"/);
});
test("paste refuses malformed quotes, empty data, 501 rows and UTF8 bytes over the cap", () => {
  for (const raw of ["", "  \n\n", `${id},黑貓,"unterminated`, `${id},黑貓,"closed"garbage`, "one\ttwo"]) assert.throws(() => pasteTrackingCSV(raw));
  assert.doesNotThrow(() => pasteTrackingCSV(Array.from({ length: 500 }, () => `${id}\t黑貓\t00001`).join("\n")));
  assert.throws(() => pasteTrackingCSV(Array.from({ length: 501 }, () => `${id}\t黑貓\t00001`).join("\n")), /too_many_rows/);
  assert.throws(() => pasteTrackingCSV("中".repeat(Math.ceil(TRACKING_MAX_BYTES / 3) + 1)), /too_large/);
});
