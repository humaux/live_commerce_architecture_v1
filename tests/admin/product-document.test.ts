// PE12/13/14 model negatives. UI writes still require browser click + persisted readback.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  newRow,
  syncMatrix,
  applyBulk,
  createDocument,
  safeEditProblem,
  parseBulk,
} from "../../apps/admin/lib/product-document.ts";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy.ts";

test("PE13 Cartesian matrix is local, preserves edits and deduplicates values", () => {
  const axes = [
    { name: "Color", values: ["White", "Black", "Blue"] },
    { name: "Size", values: ["S", "M", "L", "XL"] },
  ];
  const rows = syncMatrix(axes, [newRow([])]);
  assert.equal(rows.length, 12);
  rows[0].price = "60";
  assert.equal(syncMatrix(axes, rows)[0].price, "60");
  const filled = applyBulk(rows, "price", "80", "empty", "set");
  assert.equal(filled.rows[0].price, "60");
  assert.equal(filled.rows[11].price, "80");
});
test("PE13 bulk quantity skips negative rows without modifying them", () => {
  const rows = [newRow(["A"]), newRow(["B"])];
  rows[0].quantity = "2";
  rows[1].quantity = "8";
  const result = applyBulk(rows, "quantity", "5", "all", "subtract");
  assert.equal(result.skipped, 1);
  assert.equal(result.rows[0].quantity, "2");
  assert.equal(result.rows[1].quantity, "3");
});
test("PE12 untracked document uses whole currency and per-order cap, never quantity", () => {
  const row = newRow([]);
  row.price = "60";
  row.tracked = false;
  row.max = "3";
  row.keyword = "a12";
  const doc = createDocument(
    {
      name: "Test",
      description: "",
      slug: "test",
      seo_title: "",
      seo_description: "",
      axes: [],
      rows: [row],
      collections: [],
      weight: "",
      length: "",
      width: "",
      height: "",
      warehouse: "",
    },
    "TWD",
  );
  assert.equal(doc.skus[0].price_minor, 6000);
  assert.deepEqual(doc.skus[0].stock, { mode: "untracked", max_per_order: 3 });
  assert.equal(doc.skus[0].keyword, "A12");
  row.price = "60.5";
  assert.throws(() =>
    createDocument(
      {
        name: "Test",
        description: "",
        slug: "test",
        seo_title: "",
        seo_description: "",
        axes: [],
        rows: [row],
        collections: [],
        weight: "",
        length: "",
        width: "",
        height: "",
        warehouse: "",
      },
      "TWD",
    ),
  );
});
test("PE14 missing readback is not guessed or silently cleared by full replacement", () => {
  assert.equal(safeEditProblem(), "document_readback_incomplete");
});
test("bulk status accepts explicit per-item results, never empty or duplicated confirmations", () => {
  const id = "11111111-1111-4111-8111-111111111111";
  assert.deepEqual(parseBulk([{ id, status: "draft" }]), [
    { id, status: "draft" },
  ]);
  assert.deepEqual(parseBulk([{ id, error: "live_window_open" }]), [
    { id, error: "live_window_open" },
  ]);
  for (const value of [
    [],
    [{ id }],
    [{ id, status: "active", error: "not_found" }],
    [
      { id, status: "draft" },
      { id, status: "draft" },
    ],
  ])
    assert.throws(() => parseBulk(value));
});
test("matrix refuses over 100 SKUs before expansion", () => {
  assert.throws(() =>
    syncMatrix(
      [
        { name: "A", values: Array.from({ length: 11 }, (_, i) => String(i)) },
        { name: "B", values: Array.from({ length: 10 }, (_, i) => String(i)) },
      ],
      [],
    ),
  );
});
test("PR #1 4212540344: per-row matrix labels reappear where the matrix header is not visible (≤900px), desktop untouched", () => {
  const css = readFileSync(new URL("../../apps/admin/components/ProductDocument.css", import.meta.url), "utf8");
  const variants = readFileSync(new URL("../../apps/admin/components/ProductDocumentVariants.tsx", import.meta.url), "utf8");
  // Desktop keeps the header-only layout…
  assert.match(css, /\.pe-matrix-row label > span \{\n\s+display: none;\n\s*\}/);
  // …but the same selector is overridden to visible inside the ≤900px block, after it.
  const hide = css.indexOf(".pe-matrix-row label > span");
  const media = css.indexOf("@media (max-width: 900px)");
  const show = css.indexOf(".pe-matrix-row label > span", media);
  assert.notEqual(media, -1);
  assert.ok(show > media && show > hide, "label visibility override must live in the ≤900px block");
  assert.match(css.slice(show), /\.pe-matrix-row label > span \{\n\s+display: block;/);
  // Accessible names survive at every width: inputs keep aria-label, labels wrap input + span.
  assert.equal((variants.match(/aria-label=\{/g) ?? []).length >= 7, true);
  assert.match(variants, /<label>\n\s+<span>\{c\.price\}<\/span>/);
});
test("PR #1 4212540352: pre-publish missing-items list collapses behind a real toggle at ≤900px instead of being removed", () => {
  const css = readFileSync(new URL("../../apps/admin/components/ProductDocument.css", import.meta.url), "utf8");
  const form = readFileSync(new URL("../../apps/admin/components/ProductDocumentForm.tsx", import.meta.url), "utf8");
  // The old unconditional removal is gone and no CSS rule hides the section anymore — visibility
  // is owned by the component so the control and its label are always rendered together.
  assert.doesNotMatch(css, /\.pe-index section \{\n\s+display: none;\n\s*\}/);
  assert.doesNotMatch(css, /pe-readiness-toggle[^\n]*~ section/);
  // The toggle is a REAL button with a visible text label, aria-expanded/aria-controls, and it is
  // only rendered at ≤900px — at >900px no toggle exists in the DOM at all (axe
  // hidden-explicit-label fired on the previous visually-hidden checkbox).
  assert.doesNotMatch(form, /pe-readiness-toggle[^\n]*type="checkbox"/);
  assert.doesNotMatch(form, /pe-readiness-toggle-label/);
  // The ≤900px breakpoint is owned by the presentation layout hook (axe hidden-explicit-label
  // fix: no labelled control may exist without its visible label, so >900px renders nothing).
  const layout = readFileSync(new URL("../../apps/admin/components/useProductEditorLayout.ts", import.meta.url), "utf8");
  assert.match(layout, /window\.matchMedia\("\(max-width: 900px\)"\)/);
  assert.match(form, /noteSaveAttempt, narrowViewport \} =/);
  assert.match(form, /narrowViewport && \(/);
  assert.match(
    form,
    /<button type="button" className="pe-readiness-toggle" aria-expanded=\{readinessOpen\}\n\s+aria-controls="pe-readiness-section" onClick=\{\(\) => setReadinessOpen\(\(open\) => !open\)\}>/,
  );
  assert.match(form, /\{c\.progress\}\n\s+<\/button>/);
  // The controlled section carries the id and is hidden only while collapsed on a small screen.
  assert.match(form, /<section id="pe-readiness-section" hidden=\{narrowViewport && !readinessOpen\}>/);
  // The visible label exists in all three locales via the existing copy file.
  for (const locale of ["zh-TW", "zh-CN", "en"] as const)
    assert.ok(productEditorCopy[locale].progress.length > 0, `${locale}: toggle label`);
  // The button styling lives in the ≤900px block; desktop has no toggle chrome at all.
  const media = css.indexOf("@media (max-width: 900px)");
  const rule = css.indexOf(".pe-readiness-toggle {", media);
  assert.ok(rule > media, "toggle styles must live in the ≤900px block");
});
