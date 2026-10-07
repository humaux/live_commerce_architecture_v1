// Purpose: DB-free gate of the admin side of product-media-v2 (contracts/catalog-inventory-v1.md): the strict ImageList validator accepts
// 4 main + 20 detail + sku images in role order, rejects 5 main / 21 detail / bad order / unknown keys, and the caps equal the Go constants.
// Depends on: apps/admin/lib/image-list-contract.ts (validImage, validImageList, MAX_*), internal/catalog/images.go + options.go (read as text).
// Used by: scripts/dev/test-node.sh.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  MAX_DETAIL_PHOTOS,
  MAX_MAIN_PHOTOS,
  MAX_OPTION_PHOTOS,
  MAX_PHOTOS,
  validImage,
  validImageList,
} from "../../apps/admin/lib/image-list-contract.ts";

const U = (n: number) =>
  `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const go = (file: string) =>
  readFileSync(
    new URL(`../../internal/catalog/${file}`, import.meta.url),
    "utf8",
  );
const goConst = (name: string) =>
  Number(new RegExp(`${name}\\s*=\\s*(\\d+)`).exec(go("images.go"))?.[1]);
const image = (
  n: number,
  role: "main" | "detail" | "sku",
  position: number,
) => ({
  id: U(n),
  product_id: U(1),
  role,
  position,
  content_type: "image/jpeg",
  size_bytes: 1000,
  width: 800,
  height: 800,
  version: 1,
});
const list = (
  items: unknown[],
  image_axis: string | null = "Color",
  option_images: unknown[] = [],
) => ({ items, image_axis, option_images });
const mains = (k: number) =>
  Array.from({ length: k }, (_, i) => image(10 + i, "main", i));
const details = (k: number) =>
  Array.from({ length: k }, (_, i) => image(100 + i, "detail", i));

test("parity: admin caps equal the Go constants", () => {
  assert.equal(MAX_MAIN_PHOTOS, goConst("MaxMainImages"));
  assert.equal(MAX_DETAIL_PHOTOS, goConst("MaxDetailImages"));
  assert.equal(
    MAX_OPTION_PHOTOS,
    Number(/maxAxisValues\s*=\s*(\d+)/.exec(go("options.go"))?.[1]),
  );
  assert.equal(MAX_PHOTOS, MAX_MAIN_PHOTOS); // the current gallery UI cap is the main cap
});

test("image list: 4 main + 20 detail + 2 sku in role order is accepted", () => {
  const sku = [image(300, "sku", 0), image(301, "sku", 1)];
  const links = [
    { option_name: "Color", option_value: "Red", image_id: U(300) },
    { option_name: "Color", option_value: "Blue", image_id: U(301) },
  ];
  assert.equal(
    validImageList(list([...mains(4), ...details(20), ...sku], "Color", links)),
    true,
  );
  assert.equal(validImageList(list([], null)), true);
});

test("image list: 5 main, 21 detail, wrong role order, gaps and extra keys are refused", () => {
  assert.equal(validImageList(list(mains(5))), false);
  assert.equal(validImageList(list([...mains(1), ...details(21)])), false);
  assert.equal(validImageList(list([...details(1), ...mains(1)])), false); // detail before main
  assert.equal(validImageList(list([image(10, "main", 1)])), false); // position gap
  assert.equal(validImageList(list([image(10, "gallery" as never, 0)])), false);
  assert.equal(validImageList({ items: mains(1) }), false); // the old bare shape is no longer the contract
  assert.equal(validImageList({ ...list(mains(1)), extra: 1 }), false);
  assert.equal(validImageList(list(mains(1), "x".repeat(31))), false);
  assert.equal(
    validImageList(
      list(mains(1), "Color", [{ option_name: "Color", option_value: "Red" }]),
    ),
    false,
  );
});

test("single image: role is required and its position is bounded by the role cap", () => {
  assert.equal(validImage(image(1, "main", 3)), true);
  assert.equal(validImage(image(1, "main", 4)), false);
  assert.equal(validImage(image(1, "detail", 19)), true);
  assert.equal(validImage(image(1, "detail", 20)), false);
  assert.equal(validImage(image(1, "sku", 49)), true);
  const { role: _role, ...noRole } = image(1, "main", 0);
  assert.equal(validImage(noRole), false);
});

test("option text limits count Go runes rather than UTF-16 units", () => {
  const options = go("options.go");
  const nameCap = Number(/maxAxisName\s*=\s*(\d+)/.exec(options)?.[1]);
  const valueCap = Number(/maxAxisValue\s*=\s*(\d+)/.exec(options)?.[1]);
  const axis = "🎨".repeat(nameCap),
    value = "🔵".repeat(valueCap),
    sku = image(300, "sku", 0);
  const link = { option_name: axis, option_value: value, image_id: sku.id };
  assert.equal(validImageList(list([sku], axis, [link])), true);
  assert.equal(validImageList(list([sku], axis + "🎨", [link])), false);
  assert.equal(
    validImageList(
      list([sku], axis, [{ ...link, option_value: value + "🔵" }]),
    ),
    false,
  );
});
