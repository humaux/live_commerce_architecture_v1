// Purpose: check immutable static-directory drift in existing storefront CI.
// Depends on: Node24, the frozen ECPay snapshot and the committed static JSON generator.
// Used by: scripts/dev/test-node.sh via the existing storefront test glob.
// Invariants: generation is byte-identical; browser payload contains only public UI fields.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

const root = fileURLToPath(new URL("../../../", import.meta.url));
test("CVS static JSON regenerates byte-identically from all snapshot brands", () => {
  const generator = path.join(root, "scripts/dev/cvs-static-json.mjs");
  assert.ok(existsSync(generator), "CVS-PICK-IMPL: deterministic static directory generator must exist");
  const temp = mkdtempSync(path.join(tmpdir(), "cvs-pick-drift-"));
  try {
    execFileSync(process.execPath, [generator, "--out", temp], { cwd: root, stdio: "pipe" });
    const snapshot = JSON.parse(readFileSync(path.join(root, "internal/fulfillment/cvsdata/stores-20261028.json"), "utf8"));
    const names = Object.keys(snapshot).map((brand) => `${brand.toLowerCase()}-20261028.json`).sort();
    assert.deepEqual(readdirSync(temp).sort(), names, "immutable per-brand names; no mixed directory");
    for (const name of names) {
      const generated = readFileSync(path.join(temp, name));
      assert.deepEqual(readFileSync(path.join(root, "apps/storefront/public/cvs", name)), generated, `${name} drift`);
      const rows = JSON.parse(generated);
      const source = snapshot[name.split("-")[0].toUpperCase()];
      assert.equal(rows.length, source.length, "never silently drop a public store");
      const originals = new Map(source.map((store) => [store.store_id, store]));
      for (const row of rows) {
        assert.deepEqual(Object.keys(row).sort(), ["address", "code", "county", "district", "name"]);
        assert.equal(typeof row.code, "string");
        assert.ok(row.county && row.district, "unparseable region uses 其他");
        assert.ok(!row.county.includes("台"), "county key uses 臺");
        const original = originals.get(row.code);
        assert.ok(original);
        assert.equal(row.name, original.store_name);
        assert.equal(row.address, original.address, "canonical full-width source address must survive");
      }
    }
    const fami = JSON.parse(readFileSync(path.join(temp, "fami-20261028.json"), "utf8"));
    const taipei = JSON.parse(readFileSync(path.join(temp, "unimart-20261028.json"), "utf8")).find((row) => row.code === "121275");
    assert.equal(taipei.county, "臺北市");
    assert.equal(taipei.district, "大安區");
    assert.ok(fami.some((row) => row.county === "其他"), "unparseable snapshot records are retained");
  } finally { rmSync(temp, { recursive: true, force: true }); }
});
