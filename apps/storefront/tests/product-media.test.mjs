// Purpose: DB-free gate of the product-media-v2 buyer wire shapes (contracts/catalog-inventory-v1.md "Amendment — product-media-v2"):
// the detail parser accepts 4 main + 20 detail + option-value images and rejects 5 main / 21 detail, the v1 catalog row carries image_id,
// and every frontend cap equals the Go constant it mirrors (lesson of the pilot bug: a frontend limit drifted from the backend).
// Depends on: lib/shop-contract.ts (parseProductDetail, MAX_*), lib/purchase.ts (validProduct), internal/catalog/images.go (read as text).
// Used by: scripts/dev/test-node.sh (apps/storefront/tests/*.test.mjs).
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { MAX_DETAIL_IMAGES, MAX_MAIN_IMAGES, MAX_OPTION_IMAGES, parseProductDetail } from "../lib/shop-contract.ts";
import { validProduct } from "../lib/purchase.ts";

const U = (n) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const img = (n) => ({ id: U(n), width: 800, height: 800 });
const goConst = (name) => Number(new RegExp(`${name}\\s*=\\s*(\\d+)`).exec(readFileSync(new URL("../../../internal/catalog/images.go", import.meta.url), "utf8"))?.[1]);

const base = (over = {}) => ({
  id: U(1), slug: "tee", title: "Tee", description: "d", seo: { title: "", description: "" },
  images: [img(10)], detail_images: [], image_axis: "Color", option_images: [{ value: "Red", image_id: U(50) }],
  options: [{ name: "Color", values: ["Red", "Blue"] }],
  variants: [
    { sku_id: U(2), title: "Red", option_values: ["Red"], price_minor: 100, compare_at_minor: null, stock: "in", image_id: U(50) },
    { sku_id: U(3), title: "Blue", option_values: ["Blue"], price_minor: 100, compare_at_minor: null, stock: "in", image_id: null },
  ],
  collections: [], ...over,
});

test("parity: frontend caps equal the Go constants", () => {
  assert.equal(MAX_MAIN_IMAGES, goConst("MaxMainImages"));
  assert.equal(MAX_DETAIL_IMAGES, goConst("MaxDetailImages"));
  assert.equal(MAX_OPTION_IMAGES, 50); // Go: MaxOptionImages = maxAxisValues (internal/catalog/options.go)
  const axisValues = Number(/maxAxisValues\s*=\s*(\d+)/.exec(readFileSync(new URL("../../../internal/catalog/options.go", import.meta.url), "utf8"))?.[1]);
  assert.equal(MAX_OPTION_IMAGES, axisValues);
});

test("detail parser: 4 main + 20 detail + option-value images are accepted and mapped", () => {
  const d = parseProductDetail(base({ images: [10, 11, 12, 13].map(img), detail_images: Array.from({ length: 20 }, (_, i) => img(100 + i)) }));
  assert.ok(d);
  assert.equal(d.images.length, 4);
  assert.equal(d.detail_images.length, 20);
  assert.equal(d.image_axis, "Color");
  assert.deepEqual(d.option_images, [{ value: "Red", image_id: U(50) }]);
  assert.equal(d.variants[0].image_id, U(50));
  assert.equal(d.variants[1].image_id, null);
});

test("detail parser: 5 main, 21 detail, bad ids and over-cap option images are refused", () => {
  assert.equal(parseProductDetail(base({ images: [10, 11, 12, 13, 14].map(img) })), null);
  assert.equal(parseProductDetail(base({ detail_images: Array.from({ length: 21 }, (_, i) => img(100 + i)) })), null);
  assert.equal(parseProductDetail(base({ detail_images: [{ id: "nope", width: 1, height: 1 }] })), null);
  assert.equal(parseProductDetail(base({ option_images: Array.from({ length: 51 }, (_, i) => ({ value: `v${i}`, image_id: U(200 + i) })) })), null);
  assert.equal(parseProductDetail(base({ option_images: [{ value: "Red", image_id: "x" }] })), null);
  assert.equal(parseProductDetail(base({ image_axis: 7 })), null);
  const bad = base();
  bad.variants[0].image_id = "not-a-uuid";
  assert.equal(parseProductDetail(bad), null);
});

test("detail parser: a backend before product-media-v2 (no new keys) still parses with empty defaults", () => {
  const old = base();
  delete old.detail_images;
  delete old.image_axis;
  delete old.option_images;
  for (const v of old.variants) delete v.image_id;
  const d = parseProductDetail(old);
  assert.ok(d);
  assert.deepEqual([d.detail_images, d.image_axis, d.option_images, d.variants[0].image_id], [[], null, [], null]);
});

test("v1 catalog row: images are main-only (<= 4) and image_id is a uuid or null", () => {
  const row = (over) => ({ product_id: U(1), sku_id: U(2), name: "n", description: "d", sku_code: "c", currency: "TWD", price_minor: 100, images: [img(10)], image_id: U(10), ...over });
  assert.equal(validProduct(row()), true);
  assert.equal(validProduct(row({ image_id: null })), true);
  assert.equal(validProduct(row({ image_id: undefined })), true);
  assert.equal(validProduct(row({ image_id: "x" })), false);
  assert.equal(validProduct(row({ images: [1, 2, 3, 4, 5].map((n) => img(10 + n)) })), false);
  assert.equal(validProduct(row({ images: [1, 2, 3, 4].map((n) => img(10 + n)) })), true);
});
