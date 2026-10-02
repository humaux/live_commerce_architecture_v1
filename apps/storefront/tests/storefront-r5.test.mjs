import test from "node:test";
import assert from "node:assert/strict";
import * as catalog from "../lib/shop-contract.ts";
import { antiFraudCopy } from "../lib/anti-fraud-copy.ts";
import { legalSlugs, legalFooterLinks } from "../lib/legal-copy.ts";

test("G3: same-series cards exclude the current product and never invent absent draft/foreign products", () => {
  // Input is the current Host's PUBLIC collection response, not the merchant catalog.
  // Publication and tenant filtering remain the Go API's responsibility (also covered by SFR04).
  const current = { id: "current" }, sibling = { id: "sibling" };
  assert.deepEqual(catalog.relatedCards?.([current, sibling, sibling], current.id), [sibling]);
  assert.deepEqual(catalog.relatedCards?.([current], current.id), []);
});

test("G5: safety guidance exists in three languages without changing the five merchant policy approval boundaries", () => {
  assert.deepEqual(legalSlugs, ["privacy", "terms", "refunds", "shipping", "contact"]);
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    const guide = antiFraudCopy[locale];
    assert(guide.intro && guide.source);
    assert.equal(guide.sections.length, 3);
    assert(guide.sections.every(s => s.heading && s.text));
    assert.equal(legalFooterLinks(locale).filter(link => link.href === `/${locale}/legal/anti-fraud`).length, 1);
  }
});

test("G4: category chips omit empty collections; hidden collections absent from the public API stay absent", () => {
  const visible = { slug: "tea", product_count: 3 };
  const empty = { slug: "empty", product_count: 0 };
  assert.deepEqual(catalog.nonEmptyCollections?.([visible, empty]), [visible]);
  assert.deepEqual(catalog.nonEmptyCollections?.([]), []);
});
