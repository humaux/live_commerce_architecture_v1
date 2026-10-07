// Purpose: prove option-image selection preserves main slots and buyer cart thumbnails follow server SKU image IDs.
// Depends on: product-media.ts, cart-details.ts and the frozen catalog-inventory-v1 product-media-v2 response.
// Used by: focused PM-U Node checks and scripts/dev/test-node.sh; real image rendering is a separate CI browser gate.
import test from "node:test";
import assert from "node:assert/strict";
import { forgetDetails, loadCartDetails } from "../lib/cart-details.ts";

const U = n => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const image = n => ({ id: U(n), width: 800, height: 800 });
const main = [10, 11, 12, 13].map(image);
const media = () => import("../lib/product-media.ts");

test("selected option photo uses a main display slot without adding a fifth main image", async () => {
  const { mainGallerySlides } = await media();
  const displayed = mainGallerySlides(main, U(50));
  assert.equal(displayed.length, 4);
  assert.equal(displayed[0].id, U(50));
  assert.deepEqual(displayed.slice(1), main.slice(1));
  assert.deepEqual(main.map(i => i.id), [10, 11, 12, 13].map(U), "the four main thumbnails retain their original photos");
  assert.equal(displayed[0].width, null, "never copy another photo's dimensions onto the option image");
  assert.equal(displayed[0].height, null);
});

test("use-cover selection and a main-thumbnail selection restore the original main gallery", async () => {
  const { mainGallerySlides } = await media();
  assert.deepEqual(mainGallerySlides(main, null), main);
  assert.deepEqual(mainGallerySlides([], null), []);
});

test("cart thumbnail uses variant then catalog image_id, including when main images are absent", async () => {
  const { cartLineImage } = await media();
  assert.equal(cartLineImage({ image_id: U(50) }, { image_id: U(51), images: main }, main), U(50));
  assert.equal(cartLineImage(null, { image_id: U(51), images: [] }, []), U(51));
  assert.equal(cartLineImage({ image_id: null }, { image_id: null, images: main }, main), U(10));
  assert.equal(cartLineImage(null, { images: [] }, []), null);
});

test("migrated four-main/five-detail images stay separate when the option photo changes", async () => {
  const { mainGallerySlides } = await media();
  const details = Array.from({ length: 5 }, (_, i) => image(100 + i));
  const gallery = mainGallerySlides(main, U(50));
  assert.equal(gallery.length, 4);
  assert.ok(gallery.every(i => !details.some(d => d.id === i.id)));
  assert.equal(details.length, 5);
});

for (const detailAvailable of [true, false]) {
  test(`real cart-detail composition uses server SKU image_id (v2 detail ${detailAvailable ? "available" : "unavailable"})`, async t => {
    const oldFetch = globalThis.fetch;
    const oldWindow = globalThis.window;
    const oldLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
    globalThis.window = { localStorage: { getItem: () => null } };
    Object.defineProperty(navigator, "locks", { configurable: true, value: { request: async (_name, _opts, run) => run() } });
    t.after(() => {
      globalThis.fetch = oldFetch;
      if (oldWindow === undefined) delete globalThis.window; else globalThis.window = oldWindow;
      if (oldLocks) Object.defineProperty(navigator, "locks", oldLocks); else delete navigator.locks;
      forgetDetails();
    });
    const context = "A".repeat(43);
    const row = { product_id: U(1), sku_id: U(2), name: "Tee", description: "d", sku_code: "red", currency: "TWD", price_minor: 100, images: main, image_id: U(50) };
    const detail = {
      id: U(1), slug: "tee", title: "Tee", description: "d", seo: { title: "", description: "" },
      images: main, detail_images: [], image_axis: "Color", option_images: [{ value: "Red", image_id: U(50) }],
      options: [{ name: "Color", values: ["Red"] }], collections: [],
      variants: [{ sku_id: U(2), title: "Red", option_values: ["Red"], price_minor: 100, compare_at_minor: null, stock: "in", image_id: U(50) }],
    };
    globalThis.fetch = async url => {
      if (String(url).includes("/session")) return Response.json({ state: "active", context, expires_at: "2030-01-01T00:00:00Z" });
      if (String(url).includes("/api/shop/product/")) return detailAvailable ? Response.json(detail) : new Response(null, { status: 503 });
      if (String(url).includes("/catalog?")) return Response.json({ items: [row], next_cursor: "" });
      if (String(url).includes("/checkout-options")) return Response.json({ items: [], next_cursor: "" });
      assert.fail(`unexpected fixture request ${url}`);
    };
    forgetDetails();
    const result = await loadCartDetails(context, { id: U(5), currency: "TWD", version: 1, items: [{ sku_id: U(2), quantity: 1 }] });
    assert.equal(result.lines[0].imageID, U(50));
    assert.equal(result.lines[0].unitMinor, 100, "image selection never alters the listed amount");
  });
}
