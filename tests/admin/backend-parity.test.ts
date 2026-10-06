// Purpose: pin the admin's mirrored catalog/photo limits to the Go/SQL source of truth (drift audit output/storefront-image-cap/DRIFT.md).
// Depends on: apps/admin/lib/catalog-v2-model.ts (limits); reads apps/admin/lib/images-client.ts (not importable by node: extensionless imports),
//   internal/catalog/*.go and migrations/*.sql as text.
// Used by: scripts/dev/test-node.sh. A limit raised in Go/SQL (like the 8 -> 12 photo widening of migration 0109) without the admin fails here.
import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { limits } from "../../apps/admin/lib/catalog-v2-model.ts";

const src = (rel: string) => readFile(new URL(`../../${rel}`, import.meta.url), "utf8");
const num = (text: string, name: string) => {
  const m = text.match(new RegExp(`\\b${name}\\s*=\\s*([0-9_]+(?: *(?:<<|\\*) *[0-9]+)*)`));
  assert.ok(m, `${name} not found`);
  return Function(`"use strict"; return (${m[1].replaceAll("_", "")})`)() as number;
};

test("option/slug/SKU caps equal internal/catalog (options.go, catalog.go)", async () => {
  const options = await src("internal/catalog/options.go");
  assert.equal(limits.axes, num(options, "maxAxes"));
  assert.equal(limits.axisName, num(options, "maxAxisName"));
  assert.equal(limits.axisValues, num(options, "maxAxisValues"));
  assert.equal(limits.axisValue, num(options, "maxAxisValue"));
  assert.equal(limits.slug, num(options, "maxSlug"));
  assert.equal(limits.variants, num(await src("internal/catalog/catalog.go"), "maxActiveSKUsPerProduct"));
  assert.equal(limits.collectionProducts, num(await src("internal/catalog/collections.go"), "maxCollectionProducts"));
});

test("text caps equal the Go validators (name 120, description 8000, SEO 70/160, collection title 80)", async () => {
  const go = await src("internal/catalog/catalog.go");
  assert.match(go, new RegExp(`RuneCountInString\\(n\\) <= ${limits.name}\\b`));
  assert.match(go, new RegExp(`RuneCountInString\\(d\\) <= ${limits.description}\\b`));
  assert.match(go, new RegExp(`RuneCountInString\\(\\*in\\.SEOTitle\\) <= ${limits.seoTitle}\\b`));
  assert.match(go, new RegExp(`RuneCountInString\\(\\*in\\.SEODescription\\) <= ${limits.seoDescription}\\b`));
  assert.match(await src("internal/catalog/collections.go"), new RegExp(`utf8Count\\(t\\) <= ${limits.collectionTitle}\\b`));
  assert.match(await src("migrations/0086_catalog_v2.sql"), new RegExp(`char_length\\(description\\) <= ${limits.collectionDescription}\\b`));
});

test("photo caps equal Go (MaxImagesPerProduct, MaxImageBytes) and the SKU code pattern", async () => {
  const images = await src("internal/catalog/images.go");
  const client = await src("apps/admin/lib/images-client.ts");
  assert.equal(num(client, "MAX_PHOTOS"), num(images, "MaxImagesPerProduct"));
  assert.equal(num(client, "MAX_PHOTO_BYTES"), num(images, "MaxImageBytes"));
  assert.match(await src("internal/catalog/catalog.go"), new RegExp(`skuCodePattern = regexp\\.MustCompile\\(\`\\^\\[A-Za-z0-9_\\.-\\]\\{1,${limits.code}\\}\\$\``));
});
