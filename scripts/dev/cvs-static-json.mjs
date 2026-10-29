// Purpose: project the frozen public ECPay directory into immutable per-brand browser files.
// Depends on: Node fs/path/url and internal/fulfillment/cvsdata/stores-20261028.json.
// Used by: CVS-PICK-IMPL generation and storefront's byte-for-byte drift gate.
// Invariants: keep every code/name/address verbatim; never export phone fields.
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const args = process.argv.slice(2);
if (args.length && (args.length !== 2 || args[0] !== "--out")) throw new Error("Usage: cvs-static-json.mjs [--out directory]");
const out = args.length ? resolve(args[1]) : resolve(root, "apps/storefront/public/cvs");
const snapshot = JSON.parse(readFileSync(resolve(root, "internal/fulfillment/cvsdata/stores-20261028.json"), "utf8"));
mkdirSync(out, { recursive: true });
for (const [brand, stores] of Object.entries(snapshot)) {
  const rows = stores.map(({ store_id, store_name, address }) => {
    const prefix = address.normalize("NFKC").replaceAll("台", "臺").trim();
    const region = /^(.{2,3}?[縣市])(.{1,4}?[區鄉鎮市])/.exec(prefix);
    return { code: store_id, name: store_name, address, county: region?.[1] ?? "其他", district: region?.[2] ?? "其他" };
  });
  writeFileSync(resolve(out, `${brand.toLowerCase()}-20261028.json`), `${JSON.stringify(rows)}\n`);
}
