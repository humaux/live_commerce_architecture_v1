// Purpose: pin the storefront's mirrored limits to the Go/SQL source of truth (drift audit output/storefront-image-cap/DRIFT.md).
// Depends on: lib/shop-query.ts, lib/shop-contract.ts, lib/design.ts, lib/cvs-contract.ts, lib/payment-contract.ts; reads Go/SQL source text.
// Used by: scripts/dev/test-node.sh (apps/storefront/tests/*.test.mjs). A limit changed on one side without the other fails here, not in a pilot.
import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { parseListQuery } from "../lib/shop-query.ts";
import { parseCollections } from "../lib/shop-contract.ts";
import { normalizeDesign } from "../lib/design.ts";
import { CODE_RULES, validStoreCode, RETURN_PATH } from "../lib/cvs-contract.ts";

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

test("CVS snapshot store codes match Go and return paths match the active SQL CHECK", async () => {
  const go = await src("internal/fulfillment/cvs_rules.go");
  for (const [kinds, rule] of [
    [["cvs_711", "cvs_familymart", "cvs_okmart"], "storeCode6"],
    [["cvs_hilife"], "storeCodeHiLife"],
  ]) {
    const format = go.match(new RegExp(`${rule}\\s*=\\s*regexp\\.MustCompile\\(\x60([^\x60]+)\x60\\)`));
    assert.ok(format, `${rule} Go regex not found`);
    assert.ok(go.includes(`case ${kinds.map((kind) => JSON.stringify(kind)).join(", ")}:\n\t\treturn ${rule}.MatchString(code)`), `${rule} Go kind dispatch`);
    for (const kind of kinds) assert.equal(CODE_RULES[kind].source, format[1], `${kind} regex source parity`);
  }
  assert.equal(validStoreCode("cvs_711", "123456"), true);
  assert.equal(validStoreCode("cvs_okmart", "123456"), true);
  assert.equal(validStoreCode("cvs_hilife", "1234"), true);
  assert.equal(validStoreCode("cvs_hilife", "D123"), true);
  assert.equal(validStoreCode("cvs_okmart", "1234"), false);
  assert.equal(validStoreCode("cvs_711", "12345"), false);
  assert.equal(validStoreCode("cvs_hilife", "12345"), false);
  assert.equal(validStoreCode("cvs_hilife", "d123"), false);

  const returnPathCheck = async (file) => {
    const sql = await src(`migrations/${file}`);
    const check = sql.match(/ADD CONSTRAINT cvs_selections_return_path_check\s+CHECK\s*\(\s*return_path ~ '([^']+)'\s*\)/);
    assert.ok(check, `${file}: cvs_selections_return_path_check not found`);
    return new RegExp(check[1]);
  };
  const historical = await returnPathCheck("0093_storefront_integration.sql");
  const current = await returnPathCheck("0230_storefront_buy_now.sql");
  const norm = (re) => re.source.replaceAll("\\/", "/").replaceAll("?:", "");
  assert.equal(norm(RETURN_PATH), norm(current));

  const id = "a12bcdef-0000-4000-8000-000000000001";
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    for (const path of [`/${locale}/checkout`, `/${locale}/products/item_01-A`]) {
      for (const rule of [historical, current, RETURN_PATH]) assert.ok(rule.test(path), path);
    }
    const buyNow = `/${locale}/checkout/buy-now`;
    assert.equal(historical.test(`${buyNow}/${id}`), false);
    assert.equal(current.test(`${buyNow}/${id}`), true);
    assert.equal(RETURN_PATH.test(`${buyNow}/${id}`), true);
    for (const path of [
      buyNow, `${buyNow}/`, `${buyNow}/${id.slice(1)}`, `${buyNow}/${id.toUpperCase()}`,
      `${buyNow}/${id}/extra`, `${buyNow}/${id}?cart=1`,
    ]) {
      for (const rule of [historical, current, RETURN_PATH]) assert.equal(rule.test(path), false, path);
    }
  }
  for (const path of [
    "/fr/checkout", "//example.com/en/checkout", "/en/checkout?cart=1",
    "/en/products/", `/en/products/${"P".repeat(65)}`,
  ]) {
    for (const rule of [historical, current, RETURN_PATH]) assert.equal(rule.test(path), false, path);
  }
});
