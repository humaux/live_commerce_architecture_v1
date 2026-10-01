// catalog-media: storefront home model (lib/home.ts), product image validation (lib/purchase.ts) and the public
// upstream helpers of the media route (lib/public-upstream.ts). Pure; the route/page are exercised by the browser gates.
import test from "node:test";
import assert from "node:assert/strict";
import { groupProducts, mediaPath, validHomePage } from "../lib/home.ts";
import { validProduct, validProductImages } from "../lib/purchase.ts";
import { candidateOrigin, upstreamConfig } from "../lib/public-upstream.ts";

const P1 = "00000000-0000-4000-8000-000000000001";
const P2 = "00000000-0000-4000-8000-000000000002";
const S = (n) => `00000000-0000-4000-8000-0000000001${String(n).padStart(2, "0")}`;
const I1 = "00000000-0000-4000-8000-0000000000a1";
const row = (product, sku, price, images) => ({
  product_id: product, sku_id: sku, name: `n-${product.slice(-1)}`, description: "", sku_code: `c${sku.slice(-2)}`,
  currency: "TWD", price_minor: price, images,
});

test("CM6 groupProducts: one card per product, lowest price, cover = first image, first-seen order", () => {
  const img = { id: I1, width: null, height: 4 };
  const cards = groupProducts([row(P1, S(1), 900, [img]), row(P2, S(2), 500, []), row(P1, S(3), 300, [img])]);
  assert.deepEqual(cards.map((c) => [c.product_id, c.price_minor, c.cover?.id ?? null]), [[P1, 300, I1], [P2, 500, null]]);
  assert.deepEqual(groupProducts([]), []);
  assert.equal(groupProducts([row(P1, S(1), 1, undefined)])[0].cover, null);
});

test("CM6 validHomePage: exact keys, store_name string, rows validated", () => {
  const ok = { items: [row(P1, S(1), 100, [])], next_cursor: "", store_name: "Shop" };
  assert.ok(validHomePage(ok));
  assert.ok(!validHomePage({ items: ok.items, next_cursor: "" }), "store_name is required");
  assert.ok(!validHomePage({ ...ok, extra: 1 }));
  assert.ok(!validHomePage({ ...ok, store_name: 5 }));
  assert.ok(!validHomePage({ ...ok, items: [{ ...ok.items[0], price_minor: -1 }] }));
});

test("CM4 product images: exact {id,width,height}, at most 8, ids canonical, dims positive or null", () => {
  assert.ok(validProductImages([]));
  assert.ok(validProductImages([{ id: I1, width: 10, height: null }]));
  assert.ok(!validProductImages(Array.from({ length: 9 }, () => ({ id: I1, width: 1, height: 1 }))));
  assert.ok(!validProductImages([{ id: "x", width: 1, height: 1 }]));
  assert.ok(!validProductImages([{ id: I1, width: 0, height: 1 }]));
  assert.ok(!validProductImages([{ id: I1, width: 1, height: 1, url: "/x" }]), "no extra keys");
  assert.ok(validProduct(row(P1, S(1), 1, undefined)), "images stay optional for older rows");
  assert.ok(!validProduct(row(P1, S(1), 1, "nope")));
});

test("CM4 mediaPath is the public same-origin path", () => {
  assert.equal(mediaPath(P1, I1), `/media/p/${P1}/${I1}`);
});

test("CM4 candidateOrigin accepts only a lowercase DNS host (no port, IP, userinfo, list)", () => {
  assert.equal(candidateOrigin("shop.example.com"), "https://shop.example.com");
  for (const bad of [null, "", "Shop.Example.com", "shop.example.com:443", "127.0.0.1", "a@b.example.com", "a.com,b.com", "shop.example.com.", "localhost", "a_b.example.com", "x%2e.example.com"])
    assert.equal(candidateOrigin(bad), null, String(bad));
});

test("CM4 upstreamConfig mirrors buyer-server config: enabled flag, https or loopback+port, 43-char key", () => {
  const key = "A".repeat(43);
  const good = { COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: "https://api.example.com", COMMERCE_BUYER_BFF_KEY: key };
  assert.deepEqual(upstreamConfig(good), { api: "https://api.example.com", bff: key });
  assert.deepEqual(upstreamConfig({ ...good, COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:8080" })?.api, "http://127.0.0.1:8080");
  for (const bad of [
    { ...good, COMMERCE_BUYER_WEB_ENABLED: "0" },
    { ...good, COMMERCE_BUYER_API_ORIGIN: "http://api.example.com" },
    { ...good, COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1" },
    { ...good, COMMERCE_BUYER_API_ORIGIN: "https://api.example.com/path" },
    { ...good, COMMERCE_BUYER_API_ORIGIN: "https://u:p@api.example.com" },
    { ...good, COMMERCE_BUYER_BFF_KEY: "short" },
  ])
    assert.equal(upstreamConfig(bad), null);
});
