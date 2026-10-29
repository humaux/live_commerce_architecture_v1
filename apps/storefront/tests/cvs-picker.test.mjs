// Purpose: check directory transport and immutable snapshot drift.
// Depends on: Node24, cvs-directory-client, frozen snapshot and static JSON generator.
// Used by: scripts/dev/test-node.sh via the existing storefront test glob.
// Invariants: generation is byte-identical; browser payload contains only public UI fields.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { loadCvsDirectory } from "../lib/cvs-directory-client.ts";

test("CVS directory transport validates, retries and caches without buyer credentials", async (t) => {
  const row = { code: "000001", name: "Synthetic store", address: "Synthetic address", county: "Synthetic county", district: "Synthetic district" };
  const calls = [];
  let reply = async () => ({ ok: true, json: async () => [row] });
  t.mock.method(globalThis, "fetch", async (url, options) => {
    calls.push({ url, options });
    return reply();
  });
  const signal = new AbortController().signal;
  assert.deepEqual(await loadCvsDirectory("UNIMART", signal), [row]);
  assert.equal(calls[0].url, "/cvs/unimart-20261028.json");
  assert.deepEqual(calls[0].options, { signal, credentials: "omit" });
  assert.deepEqual(await loadCvsDirectory("UNIMART", signal), [row]);
  assert.equal(calls.length, 1, "reopening/searching a loaded brand needs no request");

  reply = async () => ({ ok: false });
  await assert.rejects(loadCvsDirectory("FAMI", signal), /directory_unavailable/);
  reply = async () => ({ ok: true, json: async () => [{ ...row, code: 1 }] });
  await assert.rejects(loadCvsDirectory("FAMI", signal), /invalid_directory/);
  reply = async () => ({ ok: true, json: async () => [row] });
  assert.deepEqual(await loadCvsDirectory("FAMI", signal), [row], "failed loads do not poison retry");

  const aborted = new AbortController();
  reply = async () => ({ ok: true, json: async () => { aborted.abort(); return [row]; } });
  await assert.rejects(loadCvsDirectory("HILIFE", aborted.signal), { name: "AbortError" });
  const beforeRetry = calls.length;
  reply = async () => ({ ok: true, json: async () => [row] });
  assert.deepEqual(await loadCvsDirectory("HILIFE", signal), [row]);
  assert.equal(calls.length, beforeRetry + 1, "an aborted response never populates the cache");
  await assert.rejects(loadCvsDirectory("UNIMART", aborted.signal), { name: "AbortError" });
  await assert.rejects(loadCvsDirectory("../buyer", signal), /invalid_directory_brand/);
  assert.equal(calls.length, beforeRetry + 1, "aborted/invalid input starts no transport");
});

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
