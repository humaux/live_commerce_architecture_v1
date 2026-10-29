// Purpose: CVS-PICK-IMPL matcher acceptance against the frozen public-store snapshot.
// Depends on: node:test/assert, cvs-match.ts and the owner-sanitized R1–R8 fixture.
// Used by: explicit packages/format invocation in Foundation's scripts/dev/test-node.sh.
// Invariants: string codes, selected-brand isolation, no automatic selection or query mutation.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";

const snapshot = JSON.parse(readFileSync(new URL("../../../internal/fulfillment/cvsdata/stores-20261028.json", import.meta.url), "utf8"));
const fixture = JSON.parse(readFileSync(new URL("./fixtures/cvs-match-real-cases.json", import.meta.url), "utf8"));
const moduleURL = new URL("../src/cvs-match.ts", import.meta.url);
const stores = Object.fromEntries(Object.entries(snapshot).map(([brand, rows]) => [brand, rows.map((row) => ({
  code: row.store_id, name: row.store_name, address: row.address, county: "", district: "",
}))]));
async function matcher() {
  assert.ok(existsSync(moduleURL), "CVS-PICK-IMPL: exported pure matcher must exist");
  const module = await import(moduleURL.href);
  assert.equal(typeof module.matchCvsStores, "function", "public matcher export");
  return module.matchCvsStores;
}
const codes = (result) => result.candidates.map((store) => store.code);

for (const row of fixture.cases) test(`CVS real case ${row.id}: ${row.expect.result}`, async () => {
  const match = await matcher();
  const before = JSON.stringify(stores);
  const result = match(row.brand, row.input, stores[row.brand]);
  assert.equal(result.query, row.input, "query survives every result, including misses");
  assert.equal(result.selected, undefined, "matcher never chooses a store");
  assert.equal(JSON.stringify(stores), before, "pure matcher cannot mutate snapshot or order");
  if (row.expect.exact_code) {
    assert.deepEqual(codes(result), [row.expect.exact_code]);
    assert.equal(result.candidates[0].name, row.expect.name);
    assert.equal(result.codeMiss, null);
  } else if (row.expect.code_miss) {
    assert.equal(result.code, row.expect.code_miss);
    assert.equal(result.codeMiss, row.expect.code_miss);
    assert.equal(result.candidates[0]?.code, row.expect.candidates_first);
    assert.equal(result.candidates[0]?.name, row.expect.name);
    assert.ok(!codes(result).includes(row.expect.code_miss), "never substitute missing code into a candidate");
  } else if (row.expect.first) {
    assert.equal(result.candidates[0]?.code, row.expect.first);
  } else {
    assert.equal(row.id, "R8");
    assert.deepEqual(codes(result), []);
    assert.equal(result.brandMismatch, true, "FamilyMart-format code asks to switch brand");
  }
  for (const candidate of result.candidates) assert.deepEqual(candidate, stores[row.brand].find((store) => store.code === candidate.code), "canonical snapshot strings stay verbatim");
});

test("code-only miss is exact-only; pasted miss may use labeled store facts", async () => {
  const match = await matcher();
  for (const [brand, code] of [["UNIMART", "000000"], ["FAMI", "F022305"], ["OKMART", "999999"], ["HILIFE", "Z999"]]) {
    const result = match(brand, code, stores[brand]);
    assert.deepEqual(codes(result), []);
    assert.equal(result.codeMiss, code.replace(/^F(?=\d{6}$)/, ""));
    assert.equal(result.query, code);
  }
});

test("all chain code formats retain strings and normalize full-width Latin/digits", async () => {
  const match = await matcher();
  for (const [brand, code] of [["UNIMART", "231196"], ["FAMI", "024793"], ["OKMART", "000005"], ["HILIFE", "2656"], ["HILIFE", "D010"]]) {
    assert.ok(stores[brand].some((store) => store.code === code), "fixture code must exist in real snapshot");
    const wide = code.replace(/[A-Z0-9]/g, (c) => String.fromCharCode(c.charCodeAt(0) + 0xfee0));
    assert.deepEqual(codes(match(brand, wide, stores[brand])), [code]);
  }
});

test("name prefixes/suffixes and reordered traditional/simplified labels", async () => {
  const match = await matcher();
  for (const query of ["7-ELEVEN橋鈺門市", "統一超商橋鈺门市", "7-11橋鈺門市"]) assert.equal(match("UNIMART", query, stores.UNIMART).candidates[0]?.code, "216843");
  for (const query of [
    "門市地址 新北市泰山區貴子路69號B1\n門市名稱 全家泰山新輔醫店\n門市代號 F022305",
    "门市名称 全家泰山新輔醫店\n门市代号 F022305\n门市地址 新北市泰山區貴子路69號B1",
  ]) {
    const result = match("FAMI", query, stores.FAMI);
    assert.equal(result.codeMiss, "022305");
    assert.equal(result.candidates[0]?.code, "024967");
  }
  // Unrecognized lines are deliberately public-store text, never recipient/phone fixtures.
  const result = match("FAMI", "備註 橋鈺\n門市代號 F022305\n門市名稱 全家泰山新輔醫店\n門市地址 新北市泰山區貴子路69號B1", stores.FAMI);
  assert.equal(result.candidates[0]?.code, "024967", "unrecognized lines cannot influence ranking");
});

test("address normalization, deterministic ranking, bounded list and no-result retention", async () => {
  const match = await matcher();
  for (const query of ["臺北市大安區敦化南路二段３３１巷１４号", "台北市大安區敦化南路二段331巷14號", "台北市大安區敦化南路2段331巷14號"]) assert.equal(match("UNIMART", query, stores.UNIMART).candidates[0]?.code, "121275");
  for (const query of ["新竹市東區科園里新安路2之1號1樓", "新竹市東區科園里新安路２－１号１樓"]) assert.equal(match("UNIMART", query, stores.UNIMART).candidates[0]?.code, "264624");
  const result = match("UNIMART", "敦化南路", stores.UNIMART);
  assert.ok(codes(result).includes("121275") && codes(result).includes("127620"), "two real choices must survive fuzzy matching");
  assert.ok(result.candidates.length <= 8);
  assert.deepEqual(result, match("UNIMART", "敦化南路", [...stores.UNIMART].reverse()), "tie-breaking independent of input order");
  assert.deepEqual(codes(match("UNIMART", "不存在的火星門市XYZ", stores.UNIMART)), []);
  assert.equal(match("UNIMART", "不存在的火星門市XYZ", stores.UNIMART).query, "不存在的火星門市XYZ");
  assert.deepEqual(codes(match("UNIMART", "", stores.UNIMART)), []);
});
