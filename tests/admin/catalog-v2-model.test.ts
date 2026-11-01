// Purpose: catalog v2 collection parser acceptance for manual and automatic collections (issue #406).
// Depends on: apps/admin/lib/catalog-v2-model.ts (parseCollection, parseCollectionDetail, parseCollectionPage, limits).
// Used by: scripts/dev/test-node.sh.
import test from "node:test";
import assert from "node:assert/strict";
import {
  limits,
  parseCollection,
  parseCollectionDetail,
  parseCollectionPage,
} from "../../apps/admin/lib/catalog-v2-model.ts";

const U = (n: number) =>
  `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;

const makeProduct = (i: number) => ({
  product_id: U(1000 + i),
  name: `Product ${i}`,
  slug: `product-${i}`,
  status: "active" as const,
  position: i,
  cover_image_id: null,
});

test("automatic collection with 501 products is accepted without truncation (#406)", () => {
  const auto501 = {
    id: U(1),
    slug: "automatic-over-cap",
    title: "Automatic Collection",
    description: "Automatic collection with over 500 products",
    sort_mode: "newest",
    status: "active",
    image_id: null,
    membership_mode: "automatic",
    product_count: 501,
    version: 1,
  };

  const parsed = parseCollection(auto501);
  assert.equal(parsed.id, U(1));
  assert.equal(parsed.product_count, 501);

  const autoDetail501 = {
    ...auto501,
    products: Array.from({ length: 501 }, (_, i) => makeProduct(i)),
  };

  const detail = parseCollectionDetail(autoDetail501);
  assert.equal(detail.id, U(1));
  assert.equal(detail.product_count, 501);
  assert.equal(detail.products.length, 501);
  assert.equal(detail.products[0].position, 0);
  assert.equal(detail.products[500].position, 500);
});

test("manual collection with 501 products or position > 499 is rejected", () => {
  const manual501 = {
    id: U(2),
    slug: "manual-over-cap",
    title: "Manual Collection",
    description: "Manual collection with 501 products",
    sort_mode: "manual",
    status: "active",
    image_id: null,
    membership_mode: "manual",
    product_count: 501,
    version: 1,
  };

  assert.throws(
    () => parseCollection(manual501),
    /invalid/,
    "manual collection product_count > 500 must be rejected"
  );

  const manualDetail501 = {
    ...manual501,
    product_count: 500,
    products: Array.from({ length: 501 }, (_, i) => makeProduct(i)),
  };

  assert.throws(
    () => parseCollectionDetail(manualDetail501),
    /invalid/,
    "manual collection products array > 500 must be rejected"
  );

  const manualDetailPos500 = {
    ...manual501,
    product_count: 1,
    products: [
      {
        product_id: U(300),
        name: "Product 500",
        slug: "product-500",
        status: "active" as const,
        position: 500,
        cover_image_id: null,
      },
    ],
  };

  assert.throws(
    () => parseCollectionDetail(manualDetailPos500),
    /invalid/,
    "manual collection product position > 499 must be rejected"
  );
});

test("collection switched automatic -> manual with valid manual shape is accepted", () => {
  const switched = {
    id: U(3),
    slug: "switched-to-manual",
    title: "Switched Collection",
    description: "Switched from automatic to manual",
    sort_mode: "manual",
    status: "active",
    image_id: null,
    membership_mode: "manual",
    rules: null,
    product_count: 3,
    version: 4,
  };

  const parsed = parseCollection(switched);
  assert.equal(parsed.id, U(3));
  assert.equal(parsed.product_count, 3);

  const switchedDetail = {
    ...switched,
    products: Array.from({ length: 3 }, (_, i) => makeProduct(i)),
  };

  const detail = parseCollectionDetail(switchedDetail);
  assert.equal(detail.id, U(3));
  assert.equal(detail.product_count, 3);
  assert.equal(detail.products.length, 3);
  assert.equal(detail.products[0].position, 0);
  assert.equal(detail.products[2].position, 2);
});

test("empty automatic and empty manual collections are accepted", () => {
  const emptyAuto = {
    id: U(4),
    slug: "empty-automatic",
    title: "Empty Auto",
    description: "Zero products matched",
    sort_mode: "newest",
    status: "active",
    image_id: null,
    membership_mode: "automatic",
    rules: { match: "all", conditions: [{ field: "created_at", operator: "within_days", days: 7 }] },
    product_count: 0,
    version: 1,
  };

  const parsedAuto = parseCollection(emptyAuto);
  assert.equal(parsedAuto.product_count, 0);

  const emptyAutoDetail = {
    ...emptyAuto,
    products: [],
  };
  const parsedAutoDetail = parseCollectionDetail(emptyAutoDetail);
  assert.equal(parsedAutoDetail.product_count, 0);
  assert.equal(parsedAutoDetail.products.length, 0);

  const emptyManual = {
    id: U(5),
    slug: "empty-manual",
    title: "Empty Manual",
    description: "No products assigned",
    sort_mode: "manual",
    status: "active",
    image_id: null,
    membership_mode: "manual",
    rules: null,
    product_count: 0,
    version: 1,
  };

  const parsedManual = parseCollection(emptyManual);
  assert.equal(parsedManual.product_count, 0);

  const emptyManualDetail = {
    ...emptyManual,
    products: [],
  };
  const parsedManualDetail = parseCollectionDetail(emptyManualDetail);
  assert.equal(parsedManualDetail.product_count, 0);
  assert.equal(parsedManualDetail.products.length, 0);
});

test("collection page mixing automatic 501 collection with manual collections is accepted", () => {
  const mixedPage = {
    items: [
      {
        id: U(10),
        slug: "manual-one",
        title: "Manual One",
        description: "First manual collection",
        sort_mode: "manual",
        status: "active",
        image_id: null,
        membership_mode: "manual",
        product_count: 5,
        version: 1,
      },
      {
        id: U(11),
        slug: "automatic-501",
        title: "Automatic 501",
        description: "Automatic collection with 501 products",
        sort_mode: "newest",
        status: "active",
        image_id: null,
        membership_mode: "automatic",
        product_count: 501,
        version: 2,
      },
      {
        id: U(12),
        slug: "manual-two",
        title: "Manual Two",
        description: "Second manual collection",
        sort_mode: "manual",
        status: "hidden",
        image_id: null,
        membership_mode: "manual",
        product_count: 0,
        version: 1,
      },
    ],
    next_cursor: "cursor-token-xyz",
  };

  const page = parseCollectionPage(mixedPage);
  assert.equal(page.items.length, 3);
  assert.equal(page.items[0].id, U(10));
  assert.equal(page.items[0].product_count, 5);
  assert.equal(page.items[1].id, U(11));
  assert.equal(page.items[1].product_count, 501);
  assert.equal(page.items[2].id, U(12));
  assert.equal(page.items[2].product_count, 0);
  assert.equal(page.next_cursor, "cursor-token-xyz");
});

test("manual collection with exactly 500 products (positions 0..499) is accepted", () => {
  const manual500 = {
    id: U(6),
    slug: "manual-at-cap",
    title: "Manual 500",
    description: "Manual collection at 500 cap",
    sort_mode: "manual",
    status: "active",
    image_id: null,
    membership_mode: "manual",
    product_count: limits.collectionProducts,
    version: 1,
  };

  const parsed = parseCollection(manual500);
  assert.equal(parsed.product_count, 500);

  const manualDetail500 = {
    ...manual500,
    products: Array.from({ length: 500 }, (_, i) => makeProduct(i)),
  };

  const detail = parseCollectionDetail(manualDetail500);
  assert.equal(detail.product_count, 500);
  assert.equal(detail.products.length, 500);
  assert.equal(detail.products[499].position, 499);
});

test("legacy collection without membership_mode defaults to manual bounds", () => {
  const legacy = {
    id: U(7),
    slug: "legacy-collection",
    title: "Legacy Collection",
    description: "Legacy shape before membership_mode was added",
    sort_mode: "manual",
    status: "active",
    image_id: null,
    product_count: 10,
    version: 1,
  };

  const parsed = parseCollection(legacy);
  assert.equal(parsed.product_count, 10);
});
