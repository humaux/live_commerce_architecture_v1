// Purpose: product document invariants and source ratchets for mobile editor presentation.
// Depends on: node:test/assert/vm/module, TypeScript API, React SSR, product-document/editor copy and actual form/CSS sources.
// Used by: test-node.sh; real visual geometry remains owned by product-editor browser acceptance.
// PE12/13/14 model negatives. UI writes still require browser click + persisted readback.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
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
  const filled = Reflect.apply(applyBulk, undefined, [rows, "price", "80", "empty", "set", "TWD"]);
  assert.equal(filled.rows[0].price, "60");
  assert.equal(filled.rows[11].price, "80");
});
test("PE13 bulk quantity skips negative rows without modifying them", () => {
  const rows = [newRow(["A"]), newRow(["B"])];
  rows[0].quantity = "2";
  rows[1].quantity = "8";
  const result = Reflect.apply(applyBulk, undefined, [rows, "quantity", "5", "all", "subtract", "TWD"]);
  assert.equal(result.skipped, 1);
  assert.equal(result.rows[0].quantity, "2");
  assert.equal(result.rows[1].quantity, "3");
});
// c5.1: invoke the required sixth currency argument through the public function.
// Reflect.apply keeps this RED card executable against the old five-argument API;
// failure must be a wrong draft result, never a compile error. Oracles are literals.
test("PE13 bulk price set/add/subtract uses the actual currency", () => {
  for (const [currency, base, operation, value, expected] of [
    ["TWD", "60", "set", "70", "70"],
    ["TWD", "60", "add", "10", "70"],
    ["TWD", "60", "subtract", "10", "50"],
    ["USD", "60.15", "set", "70.25", "70.25"],
    ["USD", "60.15", "add", "0.10", "60.25"],
    ["USD", "60.15", "subtract", "0.10", "60.05"],
    ["JPY", "60", "set", "70", "70"],
    ["JPY", "60", "add", "10", "70"],
    ["JPY", "60", "subtract", "10", "50"],
  ] as const) {
    const row = { ...newRow([currency]), price: base, quantity: "8", code: "KEEP", keyword: "KEEP" };
    const result: ReturnType<typeof applyBulk> = Reflect.apply(applyBulk, undefined, [[row], "price", value, "all", operation, currency]);
    assert.deepEqual(result, { rows: [{ ...row, price: expected }], skipped: 0 }, `${currency} ${operation}`);
    assert.equal(row.price, base, "bulk never mutates its input");
  }
});
test("PE13 bulk price refuses negative, overflow and currency precision without clamping", () => {
  for (const [currency, base, operation, value] of [
    ["TWD", "5", "subtract", "10"],
    ["TWD", "10000000000", "add", "1"],
    ["USD", "10000000000.00", "add", "0.01"],
    ["JPY", "999999999999", "add", "2"],
    ["JPY", "999999999999", "add", "1"], // Existing parser refuses a 13-digit whole part on Save.
    ["TWD", "60", "set", "60.5"],
    ["USD", "60.00", "add", "0.001"],
    ["JPY", "60", "subtract", "0.1"],
    ["TWD", "60", "add", "-10"],
    ["TWD", "60", "set", "10000000001"],
  ] as const) {
    const row = { ...newRow([]), price: base };
    const result = Reflect.apply(applyBulk, undefined, [[row], "price", value, "all", operation, currency]);
    assert.deepEqual(result, { rows: [row], skipped: 1 }, `${currency} ${base} ${operation} ${value}`);
  }
});
test("PE13 bulk price permits zero and the largest round-trippable currency amounts", () => {
  for (const [currency, base, operation, value, expected] of [
    ["TWD", "10", "subtract", "10", "0"],
    ["TWD", "60", "add", "0", "60"],
    ["USD", "60.15", "subtract", "0", "60.15"],
    ["TWD", "9999999999", "add", "1", "10000000000"],
    ["USD", "9999999999.99", "add", "0.01", "10000000000.00"],
    ["JPY", "999999999998", "add", "1", "999999999999"],
  ] as const) {
    const row = { ...newRow([]), price: base };
    const result = Reflect.apply(applyBulk, undefined, [[row], "price", value, "all", operation, currency]);
    assert.deepEqual(result, { rows: [{ ...row, price: expected }], skipped: 0 });
  }
});
test("PE13 bulk set repairs blank or malformed old prices but arithmetic requires a valid base", () => {
  for (const base of ["", "   ", "oops", "-1", "60.5"]) {
    const row = { ...newRow([]), price: base, compare: "100" };
    assert.deepEqual(Reflect.apply(applyBulk, undefined, [[row], "price", "70", "all", "set", "TWD"]),
      { rows: [{ ...row, price: "70" }], skipped: 0 });
    for (const operation of ["add", "subtract"] as const)
      assert.deepEqual(Reflect.apply(applyBulk, undefined, [[row], "price", "10", "all", operation, "TWD"]),
        { rows: [row], skipped: 1 }, `${operation} requires base ${JSON.stringify(base)}`);
  }
});
test("PE13 bulk validates the candidate against unchanged compare-at and skips only invalid rows", () => {
  for (const [base, compare, operation, value, expected, skipped] of [
    ["60", "70", "add", "10", "60", 1],
    ["60", "65", "add", "10", "60", 1],
    ["60", "oops", "set", "50", "60", 1],
    ["60", "60.5", "set", "50", "60", 1],
    ["60", "50", "subtract", "20", "40", 0],
    ["60", "100", "add", "10", "70", 0],
  ] as const) {
    const row = { ...newRow([]), price: base, compare };
    assert.deepEqual(Reflect.apply(applyBulk, undefined, [[row], "price", value, "all", operation, "TWD"]),
      { rows: [{ ...row, price: expected }], skipped });
  }
  const rows = [{ ...newRow(["valid"]), price: "60" }, { ...newRow(["conflict"]), price: "60", compare: "70" }];
  assert.deepEqual(Reflect.apply(applyBulk, undefined, [rows, "price", "10", "all", "add", "TWD"]),
    { rows: [{ ...rows[0], price: "70" }, rows[1]], skipped: 1 });
});
test("PE13 bulk empty scope trims blanks, preserves zero and excludes inactive rows from skips", () => {
  const rows = [
    { ...newRow(["blank"]), price: "  " },
    { ...newRow(["zero"]), price: "0" },
    { ...newRow(["filled"]), price: "60" },
    { ...newRow(["inactive"]), price: "", active: false },
  ];
  assert.deepEqual(Reflect.apply(applyBulk, undefined, [rows, "price", "70", "empty", "set", "TWD"]),
    { rows: [{ ...rows[0], price: "70" }, rows[1], rows[2], rows[3]], skipped: 0 });
  for (const operation of ["add", "subtract"] as const)
    assert.deepEqual(Reflect.apply(applyBulk, undefined, [rows, "price", "10", "empty", operation, "TWD"]),
      { rows, skipped: 1 });
});
test("PE13 bulk all scope changes active rows only, preserving inactive fields without counting them", () => {
  for (const field of ["price", "quantity", "compare", "code", "keyword"] as const) {
    const row = { ...newRow([]), price: "60", quantity: "8", compare: "100", code: "KEEP", keyword: "KEEP", active: false };
    assert.deepEqual(Reflect.apply(applyBulk, undefined, [[row], field, "10", "all", "set", "TWD"]),
      { rows: [row], skipped: 0 }, field);
  }
});
test("PE13 bulk invalid delta does not overwrite a valid price", () => {
  for (const value of ["", "  ", "oops", "NaN", "Infinity", "1e2"]) {
    const row = { ...newRow([]), price: "60" };
    for (const operation of ["set", "add", "subtract"] as const)
      assert.deepEqual(Reflect.apply(applyBulk, undefined, [[row], "price", value, "all", operation, "TWD"]),
        { rows: [row], skipped: 1 }, `${operation} ${JSON.stringify(value)}`);
  }
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
  // The controlled section carries the id and is hidden only while collapsed on a small screen.
  assert.match(form, /<section id="pe-readiness-section" hidden=\{narrowViewport && !readinessOpen\}>/);
  // The button styling lives in the ≤900px block; desktop has no toggle chrome at all.
  const media = css.indexOf("@media (max-width: 900px)");
  const rule = css.indexOf(".pe-readiness-toggle {", media);
  assert.ok(rule > media, "toggle styles must live in the ≤900px block");
});

test("readiness toggle renders the distinct visible editor copy in every supported locale", () => {
  const source = readFileSync(new URL("../../apps/admin/components/ProductDocumentForm.tsx", import.meta.url), "utf8");
  const file = ts.createSourceFile("ProductDocumentForm.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  let toggle = "";
  const visit = (node: ts.Node) => {
    if (ts.isJsxElement(node) && node.openingElement.attributes.properties.some(attr =>
      ts.isJsxAttribute(attr) && attr.name.getText(file) === "className" && attr.initializer &&
      ts.isStringLiteral(attr.initializer) && attr.initializer.text === "pe-readiness-toggle")) toggle = node.getText(file);
    ts.forEachChild(node, visit);
  };
  visit(file);
  assert.ok(toggle, "execute the production toggle JSX, not a copied label");
  const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
  const React = requireApp("react"), { renderToStaticMarkup } = requireApp("react-dom/server");
  const module = { exports: {} as Record<string, any> };
  const compiled = ts.transpileModule(`export function Toggle({c, locale}: any) {
    const readinessOpen = false, setReadinessOpen = () => {}; return (${toggle});
  }`, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;
  runInNewContext(compiled, { module, exports: module.exports, require: requireApp });
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const c = productEditorCopy[locale], names = [c.navigation, c.sections, c.readinessToggle, c.progress];
    assert.ok(names.every(name => name.trim().length > 0), `${locale}: visible copy`);
    assert.equal(new Set(names).size, 4, `${locale}: distinct accessible names`);
    const html = renderToStaticMarkup(React.createElement(module.exports.Toggle, { c, locale }));
    assert.equal(html.replace(/<[^>]*>/g, "").trim(), c.readinessToggle, `${locale}: actual visible toggle text`);
    assert.doesNotMatch(html, /\bhidden(?:=|\s|>)/, `${locale}: toggle is visible`);
  }
  assert.equal(Reflect.get(productEditorCopy, "unsupported"), undefined, "unknown locales have no Chinese fallback");
});

// Structural regressions complement the real browser geometry assertions; they do not prove pixels.
test("mobile product sections reuse the existing hinted TabStrip", () => {
  const form = readFileSync(new URL("../../apps/admin/components/ProductDocumentForm.tsx", import.meta.url), "utf8");
  assert.match(form, /<TabStrip[^>]*previousLabel=\{presentationCopy\[locale\]\.previous\}/);
  assert.match(form, /nextLabel=\{presentationCopy\[locale\]\.next\}/);
});
test("mobile save status has its own row instead of a fixed narrow text cap", () => {
  const css = readFileSync(new URL("../../apps/admin/components/ProductDocument.css", import.meta.url), "utf8");
  const mobile = css.slice(css.indexOf("@media (max-width: 680px)"));
  const status = mobile.match(/\.pe-savebar > span \{([^}]+)\}/)?.[1] ?? "";
  assert.doesNotMatch(status, /max-width:\s*90px/);
  assert.match(status, /flex-basis:\s*100%/);
});

test("empty mobile save status reserves the same line as the first dirty edit", () => {
  const css = readFileSync(new URL("../../apps/admin/components/ProductDocument.css", import.meta.url), "utf8");
  const mobile = css.slice(css.indexOf("@media (max-width: 680px)"));
  const status = mobile.match(/\.pe-savebar > span \{([^}]+)\}/)?.[1] ?? "";
  assert.match(status, /min-height:\s*1\.5em/);
  assert.match(status, /line-height:\s*1\.5;/);
  assert.doesNotMatch(mobile, /\.pe-savebar > span:empty\s*\{[^}]*display:\s*none/);
});

test("readiness cannot promise field focus while the actual destination is locked", () => {
  // Execute the production component; only compilation/SSR replace the browser, not its props or markup.
  const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
  const React = requireApp("react"), { renderToStaticMarkup } = requireApp("react-dom/server");
  const module = { exports: {} as Record<string, any> };
  const source = ts.transpileModule(readFileSync(new URL("../../apps/admin/components/ProductReadiness.tsx", import.meta.url), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  runInNewContext(source, { module, exports: module.exports, require: requireApp });
  for (const disabled of [true, false]) {
    const html = renderToStaticMarkup(React.createElement(module.exports.ProductReadiness, {
      c: productEditorCopy.en, items: [{ key: "media", label: "Images", ok: false }],
      activeSection: "basics", focus: () => {}, disabled,
    }));
    assert.equal(html.includes('disabled=""'), disabled);
    assert.match(html, /aria-controls="media"/);
  }
  const form = readFileSync(new URL("../../apps/admin/components/ProductDocumentForm.tsx", import.meta.url), "utf8");
  assert.equal((form.match(/<ProductReadiness\b[\s\S]*?disabled=\{disabled\}/g) ?? []).length, 2);
  assert.match(form, /const sectionButtons = sections\.map\([\s\S]*?disabled=\{disabled\}/);
});

test("pane focus selects an enabled rendered control and rounds fractional reveal inside the clip", () => {
  // Execute the real focus callback with browser-style scroll rounding, not a copied scroll formula.
  for (const first of ["enabled", "disabled", "hidden"]) {
  let scroll = 0, focused = false;
  const root = {
    get scrollTop() { return scroll; }, set scrollTop(n: number) { scroll = Math.round(n); },
    scrollTo({ top }: { top: number }) { this.scrollTop = top; }, contains: () => true,
    getBoundingClientRect: () => ({ top: 100, bottom: 260.5 }),
  };
  const control = {
    focus: () => { focused = true; },
    matches: () => false, getClientRects: () => [{}],
    getBoundingClientRect: () => ({ top: 370 - scroll, bottom: 463.91 - scroll }),
  };
  const unavailable = { ...control, focus: () => {}, matches: () => first === "disabled",
    getClientRects: () => first === "hidden" ? [] : [{}] };
  const controls = first === "enabled" ? [control] : [unavailable, control];
  const target = { getBoundingClientRect: () => ({ top: 300 - scroll }),
    querySelector: () => controls[0], querySelectorAll: () => controls };
  const refs = [null, root, null];
  const react = { useRef: () => ({ current: refs.shift() }), useState: () => [false, () => {}],
    useCallback: (fn: unknown) => fn, useEffect: () => {}, useLayoutEffect: () => {} };
  const module = { exports: {} as Record<string, any> };
  const source = ts.transpileModule(readFileSync(new URL("../../apps/admin/components/useProductEditorLayout.ts", import.meta.url), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS },
  }).outputText;
  runInNewContext(source, { module, exports: module.exports, require: () => react,
    document: { getElementById: () => target }, HTMLDetailsElement: class {} });
  module.exports.useProductEditorLayout(["media"], () => {}, "", false).focus("media");
  assert.equal(focused, true, `the first ${first} control must not steal the destination`);
  assert.ok(control.getBoundingClientRect().top >= root.getBoundingClientRect().top);
  assert.ok(control.getBoundingClientRect().bottom <= root.getBoundingClientRect().bottom);
  }
});
