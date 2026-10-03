import { test } from "node:test";
import assert from "node:assert/strict";
import {
  draftFromDetail,
  editDocument,
  newRow,
} from "../../apps/admin/lib/product-document.ts";
import type { ProductDetail } from "../../apps/admin/lib/catalog-v2-model.ts";
const detail: ProductDetail = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "Product",
  description: "Keep",
  slug: "product",
  status: "active",
  version: 4,
  seo_title: "Keep SEO",
  seo_description: "Keep description",
  options: [],
  collection_ids: ["22222222-2222-4222-8222-222222222222"],
  warehouse_id: "33333333-3333-4333-8333-333333333333",
  skus: [
    {
      id: "44444444-4444-4444-8444-444444444444",
      code: "SKU1",
      title: "Default",
      price_minor: 6000,
      compare_at_minor: 9000,
      option_values: [],
      version: 1,
      currency: "TWD",
      available: 5,
      on_hand: 7,
      committed: 2,
      keyword: "K12",
      inventory_tracked: true,
      max_per_order: null,
      weight_grams: 30,
      length_mm: 10,
      width_mm: 20,
      height_mm: 30,
      origin_country: "TW",
      customs_name: "Keep",
      hs_candidate: "1234",
    },
  ],
};
test("PE14 new edit SKU carries its own shipping and clears no existing fields", () => {
  const draft = draftFromDetail(detail);
  draft.rows.push({
    ...newRow([]),
    code: "NEW",
    price: "80",
    quantity: "0",
    weight: "42",
    length: "2.5",
    width: "3",
    height: "4",
  });
  const patch = editDocument(draft, detail, "TWD");
  assert.equal(patch.weight_grams, undefined);
  assert.deepEqual((patch.skus as Record<string, unknown>[])[0], {
    option_values: [],
    code: "NEW",
    keyword: "",
    active: true,
    price_minor: 8000,
    compare_at_minor: null,
    stock: { mode: "tracked", opening_qty: 0 },
    weight_grams: 42,
    length_mm: 25,
    width_mm: 30,
    height_mm: 40,
  });
});
test("PE14 patch changes only price and preserves unmentioned fields", () => {
  const draft = draftFromDetail(detail);
  draft.rows[0].price = "80";
  assert.deepEqual(editDocument(draft, detail, "TWD"), {
    expected_version: 4,
    skus: [{ id: detail.skus[0].id, price_minor: 8000 }],
  });
});
test("PE14 on_hand is not available, zero target is transmitted, ambiguous warehouse refuses", () => {
  const draft = draftFromDetail(detail);
  assert.equal(draft.rows[0].quantity, "7");
  draft.rows[0].quantity = "0";
  assert.deepEqual(editDocument(draft, detail, "TWD"), {
    expected_version: 4,
    skus: [{ id: detail.skus[0].id, stock: { target_qty: 0 } }],
  });
  assert.throws(
    () => editDocument(draft, { ...detail, warehouse_id: null }, "TWD"),
    /warehouse/,
  );
});
test("PE14 explicit clears and archive are present, shipping is per SKU only", () => {
  const draft = draftFromDetail(detail);
  draft.collections = [];
  draft.rows[0].keyword = "";
  draft.rows[0].compare = "";
  draft.rows[0].active = false;
  draft.rows[0].weight = "50";
  assert.deepEqual(editDocument(draft, detail, "TWD"), {
    expected_version: 4,
    collection_ids: [],
    skus: [
      {
        id: detail.skus[0].id,
        compare_at_minor: null,
        keyword: "",
        active: false,
        weight_grams: 50,
      },
    ],
  });
});
test("PE14 no-op has no command fields and changed axes explicitly archive omitted variants", () => {
  const draft = draftFromDetail(detail);
  assert.deepEqual(editDocument(draft, detail, "TWD"), { expected_version: 4 });
  draft.rows = [];
  assert.deepEqual(editDocument(draft, detail, "TWD"), {
    expected_version: 4,
    skus: [{ id: detail.skus[0].id, active: false }],
  });
});
