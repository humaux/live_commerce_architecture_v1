// Purpose: pin the storefront's mirrored limits to the Go/SQL source of truth (drift audit output/storefront-image-cap/DRIFT.md).
// Depends on: lib/shop-query.ts, lib/shop-contract.ts, lib/design.ts, lib/cvs-contract.ts, lib/payment-contract.ts; reads Go/SQL source text.
// Used by: scripts/dev/test-node.sh (apps/storefront/tests/*.test.mjs). A limit changed on one side without the other fails here, not in a pilot.
import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { parseListQuery } from "../lib/shop-query.ts";
import { MAX_PRODUCT_IMAGES, parseCollections } from "../lib/shop-contract.ts";
import { normalizeDesign } from "../lib/design.ts";
import { validStoreCode, validStoreName, validStoreAddress, RETURN_PATH } from "../lib/cvs-contract.ts";

const src = (rel) => readFile(new URL(`../../../${rel}`, import.meta.url), "utf8");
const num = (text, name) => {
  const m = text.match(new RegExp(`\\b${name}\\s*=\\s*([0-9_]+)`));
  assert.ok(m, `${name} not found`);
  return Number(m[1].replaceAll("_", ""));
};

test("catalog list query limits equal the Go v2 parser (internal/buyerhttp/catalogv2.go)", async () => {
  const go = await src("internal/buyerhttp/catalogv2.go");
  const maxLimit = num(go, "v2MaxLimit"), maxQuery = num(go, "v2MaxQuery");
  const at = (limit) => parseListQuery((k) => (k === "limit" ? String(limit) : null)).limit;
  assert.equal(at(maxLimit), maxLimit);
  assert.equal(at(maxLimit + 1), 24, "above the Go cap falls back to the default instead of a 422 round trip");
  const q = (n) => parseListQuery((k) => (k === "q" ? "字".repeat(n) : null)).q;
  assert.equal([...q(maxQuery)].length, maxQuery);
  assert.equal([...q(maxQuery + 5)].length, maxQuery);
});

test("collection list cap equals Go maxCollectionsPerStore", async () => {
  const cap = num(await src("internal/catalog/collections.go"), "maxCollectionsPerStore");
  const row = (i) => ({ id: `00000000-0000-4000-8000-${String(i).padStart(12, "0")}`, slug: `c-${i}`, title: "t", image_id: null, product_count: 1 });
  assert.ok(parseCollections({ collections: Array.from({ length: cap }, (_, i) => row(i)) }), `${cap} collections accepted`);
});

test("design caps equal internal/design/schema.go (nav, sections, pages)", async () => {
  const go = await src("internal/design/schema.go");
  const header = num(go, "maxHeaderNav"), footer = num(go, "maxFooterNav"), sections = num(go, "maxSections"), pages = num(go, "maxPages");
  const nav = (n, kind) => Array.from({ length: n + 1 }, (_, i) => ({ label: `L${i}`, kind }));
  const d = normalizeDesign({
    nav: { header: nav(header, "all_products"), footer: Array.from({ length: footer + 1 }, (_, i) => ({ label: `P${i}`, kind: "page", target: "about" })) },
    home: { sections: Array.from({ length: sections + 1 }, () => ({ type: "product_grid", sort: "newest", limit: 12 })) },
    pages: Array.from({ length: pages + 1 }, (_, i) => ({ slug: `p${i}`, title: "T", body: "" })),
  });
  assert.equal(d.nav.header.length, header);
  assert.equal(d.nav.footer.length, footer);
  assert.equal(d.home.sections.length, sections);
  assert.equal(d.pages.length, pages);
});

test("CVS buyer-entered store bounds and return path equal Go (internal/checkout/cvs.go, migration 0093)", async () => {
  const go = await src("internal/checkout/cvs.go");
  assert.match(go, /boundedText\(in\.StoreName, 1, 40\)/);
  assert.match(go, /boundedText\(in\.StoreAddress, 5, 120\)/);
  assert.equal(validStoreName("字".repeat(40)), true);
  assert.equal(validStoreName("字".repeat(41)), false);
  assert.equal(validStoreAddress("字".repeat(120)), true);
  assert.equal(validStoreAddress("字".repeat(121)), false);
  assert.equal(validStoreAddress("字".repeat(4)), false);
  const sql = await src("migrations/0093_storefront_integration.sql");
  const m = sql.match(/return_path ~ '([^']+)'/);
  assert.ok(m, "return_path CHECK not found");
  const norm = (re) => re.replaceAll("\\/", "/").replaceAll("?:", "");
  assert.equal(norm(RETURN_PATH.source), norm(m[1]));
  assert.equal(validStoreCode("cvs_711", "123456"), true);
  assert.equal(validStoreCode("cvs_okmart", "1234"), true);
});
