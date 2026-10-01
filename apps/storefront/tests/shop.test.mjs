// Pure-logic tests of the storefront shell (unit storefront-shell): design document coercion + contrast maths, catalog-v2
// parsers and variant resolution, list query hygiene, SEO builders (JSON-LD/sitemap/robots), money display and the cart-line
// write helper. No browser, no network: the MOCK browser gate (tests/storefront/shop-gate.mjs) covers the rendered pages.
import test from "node:test";
import assert from "node:assert/strict";
import { accentText, contrast, defaultDesign, internalHref, normalizeDesign, onAccent, resolveNav, safeExternal, withPreview } from "../lib/design.ts";
import { freeShippingProgress, initialChoice, parseCollections, parseProductDetail, parseProductList, priceBounds, resolveVariant, valueAvailable } from "../lib/shop-contract.ts";
import { parseListQuery } from "../lib/shop-query.ts";
import { productJsonLd, robotsTxt, sitemapXml } from "../lib/seo.ts";
import { formatMoney, majorToMinor, minorToMajor } from "../lib/money.ts";
import { cartWithQuantity, validOption } from "../lib/purchase.ts";
import { cartCount, cartProblem } from "../lib/cart-state.ts";
import { BuyerClientError } from "../lib/buyer-client.ts";

const nb = (s) => s.replace(/\u00a0/g, " ");
const U = (n) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;

test("design: garbage and hostile documents collapse to the safe default shape", () => {
  assert.deepEqual(normalizeDesign(null, "Shop"), defaultDesign("Shop"));
  const d = normalizeDesign({
    profile: { name: "  Maison  ", accent_color: "#ABCDEF", announcement: "", contact: { line_url: "javascript:alert(1)", facebook_url: "http://insecure", instagram_url: "https://instagram.com/x", email: "a@b.c" } },
    nav: { header: [{ label: "x", kind: "url", target: "javascript:alert(1)" }, { label: "Shop", kind: "all_products", target: "ignored" }, { label: "Bad", kind: "collection", target: "Not A Slug" }, { label: "Sale", kind: "collection", target: "sale" }] },
    home: { sections: [{ type: "evil" }, { type: "hero", image_id: "nope", heading: "H", cta_label: "Go", cta_kind: "collection", cta_target: "BAD" }, { type: "product_grid", limit: 999, sort: "weird" }] },
    pages: [{ slug: "about", title: "About", body: "x" }, { slug: "Bad Slug", title: "t", body: "" }],
  });
  assert.equal(d.profile.name, "Maison");
  assert.equal(d.profile.accent_color, "#abcdef");
  assert.equal(d.profile.announcement, null);
  assert.equal(d.profile.contact.line_url, null);
  assert.equal(d.profile.contact.facebook_url, null);
  assert.equal(d.profile.contact.instagram_url, "https://instagram.com/x");
  assert.deepEqual(d.nav.header.map((n) => n.label), ["Shop", "Sale"]);
  assert.equal(d.home.sections.length, 2);
  assert.equal(d.home.sections[0].image_id, null);
  assert.equal(d.home.sections[0].cta_label, null, "a CTA without a valid destination is dropped, not half rendered");
  assert.equal(d.home.sections[1].limit, 48);
  assert.equal(d.home.sections[1].sort, "newest");
  assert.deepEqual(d.pages.map((p) => p.slug), ["about"]);
});

test("design: links, preview token propagation, nav resolution", () => {
  assert.equal(safeExternal("tel:+886212345678"), "tel:+886212345678");
  assert.equal(safeExternal("https://a.example/x y"), null);
  assert.equal(safeExternal("data:text/html,x"), null);
  assert.equal(internalHref("zh-TW", "collection", "tea"), "/zh-TW/collections/tea");
  assert.equal(internalHref("en", "page", "about"), "/en/pages/about");
  assert.equal(internalHref("en", "url", "https://x"), null);
  assert.equal(withPreview("/en/products", "tok"), "/en/products?preview=tok");
  assert.equal(withPreview("/en/products?sort=title", "tok"), "/en/products?sort=title&preview=tok");
  assert.equal(withPreview("https://x", "tok"), "https://x");
  assert.equal(withPreview("/en", null), "/en");
  const links = resolveNav("en", [{ label: "A", kind: "all_products", target: null }, { label: "L", kind: "url", target: "https://l.example" }, { label: "T", kind: "url", target: "tel:1" }], "tok");
  assert.deepEqual(links, [{ label: "A", href: "/en/products?preview=tok", external: false }, { label: "L", href: "https://l.example", external: true }, { label: "T", href: "tel:1", external: false }]);
});

test("design: any merchant accent yields readable button text and link colour (WCAG AA)", () => {
  for (const hex of ["#ffffff", "#fff200", "#ffe4e1", "#000000", "#247965", "#2f6b5a", "#ff3b30", "#7a5cff", "#f5f5f5"]) {
    assert(contrast(hex, onAccent(hex)) >= 4.5, `button text on ${hex}`);
    assert(contrast(accentText(hex), "#ffffff") >= 4.5, `link colour from ${hex}`);
  }
  assert.equal(onAccent("#000000"), "#ffffff");
  assert.equal(accentText("#247965"), "#247965", "an already-dark accent is kept");
});

const variants = [
  { sku_id: U(1), title: "M / Red", option_values: ["M", "Red"], price_minor: 100, compare_at_minor: 150, stock: "in" },
  { sku_id: U(2), title: "M / Blue", option_values: ["M", "Blue"], price_minor: 100, compare_at_minor: null, stock: "out" },
  { sku_id: U(3), title: "L / Red", option_values: ["L", "Red"], price_minor: 120, compare_at_minor: null, stock: "low" },
];
const options = [{ name: "Size", values: ["M", "L"] }, { name: "Color", values: ["Red", "Blue"] }];

test("variants: resolution, disabled out-of-stock combinations, initial choice, price bounds", () => {
  assert.equal(resolveVariant(options, variants, ["M", "Red"]).sku_id, U(1));
  assert.equal(resolveVariant(options, variants, ["L", "Blue"]), null, "a combination that does not exist resolves to nothing");
  assert.equal(resolveVariant(options, variants, ["M", null]), null);
  assert.equal(resolveVariant([], [variants[0]], []).sku_id, U(1));
  assert.equal(valueAvailable(variants, 1, "Blue", ["M", "Red"]), false, "M / Blue is out");
  assert.equal(valueAvailable(variants, 1, "Blue", [null, null]), false);
  assert.equal(valueAvailable(variants, 1, "Red", ["L", null]), true);
  assert.deepEqual(initialChoice(variants), ["M", "Red"], "cheapest in-stock variant");
  assert.deepEqual(priceBounds(variants), { min: 100, max: 120, compareAtMin: 150 });
  assert.deepEqual(priceBounds([variants[1]]), { min: 100, max: 100, compareAtMin: null }, "all out: still shows a price");
});

test("catalog-v2 parsers accept the frozen shapes and refuse drift", () => {
  const card = { id: U(5), slug: "a-b", title: "T", price_min_minor: 1, price_max_minor: 2, compare_at_min_minor: null, cover_image_id: null, in_stock: true };
  const list = { store: { name: "S", currency: "TWD" }, products: [card], next: null };
  assert.equal(parseProductList(list).products[0].slug, "a-b");
  assert.equal(parseProductList({ ...list, store: { name: "S", currency: "twd" } }), null);
  assert.equal(parseProductList({ ...list, products: [{ ...card, price_min_minor: -1 }] }), null);
  assert.equal(parseProductList({ ...list, products: [{ ...card, id: "nope" }] }), null);
  const detail = { id: U(6), slug: "p", title: "P", description: "d", seo: { title: "", description: "" }, images: [{ id: U(7), width: null, height: 10 }], options, variants, collections: [{ slug: "c", title: "C" }] };
  assert.equal(parseProductDetail(detail).variants.length, 3);
  assert.equal(parseProductDetail({ ...detail, variants: [{ ...variants[0], stock: "plenty" }] }), null);
  assert.equal(parseProductDetail({ ...detail, variants: [{ ...variants[0], option_values: ["M"] }] }), null, "values must align with the axes");
  assert.equal(parseProductDetail({ ...detail, options: [{}, {}, {}, {}] }), null);
  assert.deepEqual(parseCollections({ collections: [{ slug: "c", title: "C", image_id: null, product_count: 2 }] }), [{ id: null, slug: "c", title: "C", image_id: null, product_count: 2 }]);
  assert.equal(parseCollections({ collections: [{ slug: "c", title: "C", image_id: null, product_count: -1 }] }), null);
});

test("free-delivery progress words the hint only when a threshold exists", () => {
  assert.equal(freeShippingProgress(500, null), null);
  assert.equal(freeShippingProgress(500, 0), null);
  assert.deepEqual(freeShippingProgress(500, 2000), { reached: false, remaining: 1500, ratio: 0.25 });
  assert.deepEqual(freeShippingProgress(2000, 2000), { reached: true, remaining: 0, ratio: 1 });
});

test("list query: only whitelisted, canonical values survive", () => {
  const q = (o) => parseListQuery((k) => o[k]);
  assert.equal(q({ sort: "price_asc" }).sort, "price_asc");
  assert.equal(q({ sort: "drop table" }).sort, "newest");
  assert.equal(q({ collection: "Tea Time" }).collection, "");
  assert.equal(q({ collection: "tea-time" }).collection, "tea-time");
  assert.equal(q({ q: "  a\u0000b  " }).q, "a b");
  assert.equal([...q({ q: "x".repeat(200) }).q].length, 60);
  assert.deepEqual([q({ min: "900", max: "100" }).min, q({ min: "900", max: "100" }).max], [100, 900], "inverted range is swapped");
  assert.equal(q({ min: "-1" }).min, null);
  assert.equal(q({ min: "1.5" }).min, null, "the API route takes minor units: integers only");
  assert.equal(q({ limit: "99" }).limit, 24);
  assert.equal(q({ limit: "12" }).limit, 12);
  assert.equal(q({ after: "../x" }).after, null);
});

test("seo: JSON-LD cannot break out of its script tag; offers reflect stock and price range", () => {
  const detail = { id: U(6), slug: "p", title: "</script><b>x", description: "d", seo: { title: "", description: "" }, images: [{ id: U(7), width: 1, height: 1 }], options, variants, collections: [] };
  const json = productJsonLd({ origin: "https://s.example", locale: "zh-TW", product: detail, currency: "TWD", name: "Shop" });
  assert(!json.includes("</script>") && !json.includes("<"));
  const data = JSON.parse(json.replace(/\\u003c/g, "<"));
  assert.equal(data.offers["@type"], "AggregateOffer");
  assert.equal(data.offers.lowPrice, "1.00");
  assert.equal(data.offers.highPrice, "1.20");
  assert.equal(data.image[0], `https://s.example/media/p/${U(6)}/${U(7)}`);
  const out = productJsonLd({ origin: "https://s.example", locale: "en", product: { ...detail, variants: [{ ...variants[1] }] }, currency: "TWD", name: "Shop" });
  assert.equal(JSON.parse(out).offers.availability, "https://schema.org/OutOfStock");
});

test("seo: sitemap escapes and carries hreflang; robots refuses everything for a closed host", () => {
  const xml = sitemapXml("https://s.example", [{ path: "" }, { path: "/products/a&b" }], ["zh-CN", "zh-TW", "en"], "zh-TW");
  assert(xml.includes("<loc>https://s.example/zh-TW</loc>") && xml.includes("/products/a&amp;b") && xml.includes('hreflang="en"'));
  assert.equal(robotsTxt(null, false), "User-agent: *\nDisallow: /\n");
  const open = robotsTxt("https://s.example", true);
  assert(open.includes("Sitemap: https://s.example/sitemap.xml") && open.includes("Disallow: /*/products/Checkout") && open.includes("Disallow: /api/"));
});

test("money: whole amounts drop cents, minor digits come from the currency, filters round-trip", () => {
  assert.equal(nb(formatMoney("zh-TW", 98000, "TWD")), "TWD 980");
  assert.equal(nb(formatMoney("zh-TW", 98050, "TWD")), "TWD 980.50");
  assert.equal(nb(formatMoney("en", 98000, "TWD")), "NT$980");
  assert.equal(majorToMinor("zh-TW", "TWD", "980"), 98000);
  assert.equal(majorToMinor("zh-TW", "TWD", "12.5"), 1250);
  assert.equal(majorToMinor("zh-TW", "TWD", "abc"), null);
  assert.equal(majorToMinor("zh-TW", "TWD", ""), null);
  assert.equal(majorToMinor("en", "JPY", "500"), 500);
  assert.equal(minorToMajor("zh-TW", "TWD", 98000), "980");
});

test("cart line write: replaces one line, keeps the others, 0 removes, CAS version is the one shown", () => {
  const cart = { id: U(9), currency: "TWD", version: 4, items: [{ sku_id: U(2), quantity: 1 }, { sku_id: U(1), quantity: 2 }] };
  assert.deepEqual(cartWithQuantity(cart, U(1), 5), { expected_version: 4, items: [{ sku_id: U(1), quantity: 5 }, { sku_id: U(2), quantity: 1 }] });
  assert.deepEqual(cartWithQuantity(cart, U(1), 0).items, [{ sku_id: U(2), quantity: 1 }]);
  assert.deepEqual(cartWithQuantity(cart, U(3), 1).items.map((i) => i.sku_id), [U(1), U(2), U(3)]);
  assert.throws(() => cartWithQuantity(cart, "nope", 1), BuyerClientError);
  assert.throws(() => cartWithQuantity(cart, U(1), -1), BuyerClientError);
  assert.equal(cartCount(cart), 3);
  assert.equal(cartCount(null), 0);
});

test("cart problems map to buyer-facing classes without leaking transport detail", () => {
  assert.equal(cartProblem(new BuyerClientError("uncertain")), "uncertain");
  assert.equal(cartProblem(new BuyerClientError("unavailable")), "storage");
  assert.equal(cartProblem(new BuyerClientError("context_changed", 409)), "session");
  assert.equal(cartProblem(new BuyerClientError("request_failed", 409)), "conflict");
  assert.equal(cartProblem(new BuyerClientError("request_failed", 500)), "failed");
  assert.equal(cartProblem(new Error("x")), "failed");
});

test("checkout options accept the optional free-shipping threshold only as a positive integer or null", () => {
  const row = { market_id: U(1), country: "TW", currency: "TWD", method: "delivery:home_tw", delivery_kind: "home", service_version: 1, allocation_version: 1, mode: "MANUAL", name_hans: "a", name_hant: "b", name_en: "c", sort_order: 1 };
  assert.equal(validOption(row), true);
  assert.equal(validOption({ ...row, free_shipping_threshold_minor: 150000 }), true);
  assert.equal(validOption({ ...row, free_shipping_threshold_minor: null }), true);
  assert.equal(validOption({ ...row, free_shipping_threshold_minor: 0 }), false);
  assert.equal(validOption({ ...row, free_shipping_threshold_minor: "150000" }), false);
});
