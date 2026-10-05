import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { createHash } from "node:crypto";

const directory = new URL("browser/", import.meta.url);
const result = JSON.parse(readFileSync(new URL("ps-browser-result.json", directory), "utf8"));
const { rows } = JSON.parse(readFileSync(new URL("click-ledger.json", directory), "utf8"));
assert.equal(rows.every(row => row.result === "PASS"), true);
assert.equal(result.pass, 30);
assert.equal(result.fail, 0);
assert.equal(result.clicks, 390);
assert.equal(result.messagingCopyCases, 12);
const checked = rows.filter(row => row.action === "messaging-copy");
assert.equal(checked.length, 12);
assert.equal(new Set(checked.map(row => `${row.locale}/${row.width}/${row.page}`)).size, 12);
const screenshots = readdirSync(directory).filter(file => file.endsWith(".png")).sort().map(file => {
  const data = readFileSync(new URL(file, directory));
  const width = data.readUInt32BE(16), height = data.readUInt32BE(20);
  assert.equal(width, Number(file.match(/-(390|1586)\.png$/)[1]));
  assert.ok(height >= (width === 390 ? 844 : 992));
  return { file, width, height, sha256: createHash("sha256").update(data).digest("hex") };
});
assert.equal(screenshots.length, 30);
console.log(JSON.stringify({ result, allLedgerRowsPass: true, screenshots }, null, 2));
