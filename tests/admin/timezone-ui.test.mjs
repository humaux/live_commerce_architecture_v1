// Purpose: Checks Taipei merchant timestamps rendered by the real admin TSX components.
// Depends on: node:test, Next SWC, apps/admin/components, packages/format/src/index.ts.
// Used by: timezone audit focused Node gate; presentation and hook shells only are simulated.
// Status: component-level MOCK evidence; loaded timestamp assertions do not certify browser clicks or HTTP/auth seams.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import vm from "node:vm";

const root = resolve(import.meta.dirname, "../..");
const require = createRequire(resolve(root, "apps/admin/package.json"));
const { transformSync } = require("next/dist/build/swc");
const instant = "2026-12-31T16:30:00Z";
const expected = /2027\D+0?1\D+0?1\D+00:30/;
const jsx = (type, props) => ({ type, props: props ?? {} });
const react = { useEffect() {}, useMemo: (fn) => fn(), useRef: (value) => ({ current: value }),
  useCallback: (fn) => fn, useState: (value) => [value, () => {}] };
const passthrough = ({ children }) => children;
const format = compile("packages/format/src/index.ts", {});

function compile(path, imports, suffix = "") {
  const source = readFileSync(resolve(root, path), "utf8") + suffix;
  const result = transformSync(source, {
    filename: path,
    jsc: { parser: { syntax: "typescript", tsx: path.endsWith(".tsx") },
      target: "es2022", transform: { react: { runtime: "automatic" } } },
    module: { type: "commonjs" },
  });
  const module = { exports: {} };
  const map = {
    react,
    "react/jsx-runtime": { jsx, jsxs: jsx, Fragment: passthrough },
    // Lazy: the real format module is itself compiled by this loader during initialization.
    get ["@live-commerce/format"]() { return format; },
    "@live-commerce/ui": { Badge: passthrough, Field: passthrough, FormRow: passthrough, TableFrame: passthrough, TabStrip: passthrough },
    "./WorkspaceFrame": { WorkspaceFrame: passthrough },
    "./AdminPageHeader": { AdminPageHeader: passthrough },
    "@/lib/settings-client": {},
    "./orders.css": {},
    ...imports,
  };
  const importModule = (name) => {
    assert.ok(Object.hasOwn(map, name), `${path}: unlisted dependency ${name}`);
    return map[name];
  };
  vm.runInNewContext("(function(require,module,exports){" + result.code + "\n})", {}, { filename: path })(
    importModule, module, module.exports);
  return module.exports;
}

function textOf(node) {
  if (node === null || node === undefined || typeof node === "boolean") return "";
  if (Array.isArray(node)) return node.map(textOf).join(" ");
  if (typeof node !== "object") return String(node);
  if (typeof node.type === "function") return textOf(node.type(node.props));
  return textOf(node.props.children);
}

test("TSX seam refuses an unlisted import", () => {
  const path = "packages/format/src/index.ts";
  for (const name of ["@/lib/unlisted-timezone-probe", "./unlisted-timezone-probe.css", "toString"]) {
    assert.throws(() => compile(path, {}, `\nimport "${name}";\n`),
      { code: "ERR_ASSERTION", message: `${path}: unlisted dependency ${name}` });
  }
});

test("Ads merchant timestamp uses Taipei at the year boundary", () => {
  const { __testWhen } = compile("apps/admin/components/Ads.tsx", {
    "next/navigation": {}, "@/lib/ads-client": {}, "@/lib/ads-model": {}, "@/lib/ads-copy": {},
    "./Icon": {}, "./AdsConnection": {}, "./AdsDraft": {}, "./AdsResults": {}, "@/lib/attribution-copy": {}, "./ads.css": {},
  }, "\nexport { when as __testWhen };\n");
  assert.match(__testWhen("zh-TW", instant, "empty"), expected);
  assert.equal(__testWhen("zh-TW", null, "empty"), "empty");
});

test("Design version published_at cell uses Taipei at the year boundary", () => {
  const { __testVersions } = compile("apps/admin/components/Design.tsx", {
    "@/lib/catalog-v2-copy": { catalogPresentationCopy: { "zh-TW": { tableScroll: "scroll" } } },
    "@/lib/design-copy": { fill: () => "source" },
    "@/lib/customers-client": {}, "@/lib/storefront-client": {}, "@/lib/design-client": {}, "@/lib/design-model": {},
    "./DesignProfile": {}, "./DesignNav": {}, "./DesignSections": {}, "./DesignPages": {}, "./design.css": {},
  }, "\nexport { Versions as __testVersions };\n");
  const c = { versions: { help: "help", none: "none", version: "version", at: "at",
    live: "live", rollback: "rollback", kind: { publish: "published" } }, tabs: { versions: "versions" } };
  const row = __testVersions({ list: { items: [{ version: 1, kind: "publish", published_at: instant }] },
    live: 1, busy: false, c, locale: "zh-TW", onRollback() {} });
  assert.match(textOf(row), expected);
});

test("ManualOrder bank-transfer expiry uses Taipei at the year boundary", () => {
  let state = 0;
  const placed = { order_id: "order-123456", payment_mode: "bank_transfer", commercial_state: "AWAITING_TRANSFER",
    currency: "TWD", total_minor: 100, expires_at: instant, buyer_link: null };
  // ManualOrder's 14th direct useState owns `placed`; preceding hook additions must update this fixture.
  // The literal expiry assertion below fails if this injection stops reaching the loaded result view.
  const hooks = { ...react, useState(value) { return [state++ === 13 ? placed : value, () => {}]; } };
  const { ManualOrder } = compile("apps/admin/components/ManualOrder.tsx", {
    react: hooks,
    "@/lib/customers-client": { useGuardedRead: () => ({ status: "ready", data: [] }) },
    "@/lib/merchant-tools-model": { draftProblem: () => null },
    "@/lib/merchant-tools-copy": { toolsCopy: { "zh-TW": { manual: { created: "created", expires: "expires" } } } },
    "@/lib/client": { money: () => "NT$100" },
    "./OperationalForms.module.css": { page: "page" },
    "next/link": { default: passthrough }, "@/lib/catalog-v2-client": {}, "@/lib/merchant-tools-client": {}, "@/lib/cod-copy": {},
    "./customers.css": {}, "./merchant-tools.css": {},
    "@/lib/manual-order-form": { emptyManualForm: () => ({ home: {}, cvs: {} }) },
    "./ManualOrderFormFields": {},
  });
  // Compile uses the actual component; only hook state and surrounding presentation are fixtures.
  const tree = ManualOrder({ locale: "zh-TW", stores: [], store: { id: "store", name: "store" },
    initialError: null, renderKey: "test" });
  assert.match(textOf(tree), expected);
});


// These loaded-source checks cover shared form state and callbacks; browser click acceptance remains separate.
function manualFormFixture(locale = "zh-TW", extra = {}) {
  const { emptyManualForm } = compile("apps/admin/lib/manual-order-form.ts", {});
  const copy = compile("apps/admin/lib/merchant-tools-copy.ts", {});
  const cod = compile("apps/admin/lib/cod-copy.ts", {});
  const shared = { "@/lib/merchant-tools-copy": copy, "@/lib/cod-copy": cod,
    "@/lib/client": { money: () => "NT$100" }, "./OperationalForms.module.css": {},
    "@/lib/catalog-v2-client": {} };
  const picker = compile("apps/admin/components/ManualOrderItemPicker.tsx", shared);
  const { ManualOrderFormFields } = compile("apps/admin/components/ManualOrderFormFields.tsx", {
    ...shared, "./ManualOrderItemPicker": picker,
  });
  const patches = [];
  const value = { ...emptyManualForm(locale), lines: [{ sku_id: "sku-1", quantity: 1,
    label: "Catalog product", code: "CAT", price: "NT$100", note: "Claimed quantity: 2" }] };
  const tree = ManualOrderFormFields({ locale, store: { id: "store", name: "Store" },
    value, onChange: (patch) => patches.push(patch), available: [], optionsReady: true, ...extra });
  function nodes(node) {
    if (!node || typeof node !== "object") return [];
    if (Array.isArray(node)) return node.flatMap(nodes);
    if (typeof node.type === "function") return nodes(node.type(node.props));
    return [node, ...nodes(node.props.children)];
  }
  return { value, patches, nodes: nodes(tree), emptyManualForm, copy };
}

test("shared ManualOrder form keeps fresh defaults and legacy input IDs", () => {
  const f = manualFormFixture();
  const a = f.emptyManualForm(); const b = f.emptyManualForm();
  a.lines.push({ sku_id: "a", quantity: 1 }); a.home.city = "changed"; a.cvs.store_code = "changed";
  assert.equal(b.lines.length, 0); assert.equal(b.home.city, ""); assert.equal(b.cvs.store_code, "");
  assert.equal(b.buyerLocale, "zh-TW");
  assert.equal(f.emptyManualForm("en").buyerLocale, "en");
  assert.ok(f.nodes.some((n) => n.props["data-testid"] === "mo-name"));
  assert.ok(f.nodes.some((n) => n.props["data-testid"] === "mo-search"));
  const quantity = f.nodes.find((n) => n.type === "input" && n.props.type === "number");
  assert.equal(quantity.props.min, 1);
  quantity.props.onChange({ target: { value: "0" } });
  assert.equal(f.patches.at(-1).lines[0].quantity, 1);
  const name = f.nodes.find((n) => n.props["data-testid"] === "mo-name");
  name.props.onChange({ target: { value: "Merchant entered" } });
  assert.equal(f.patches.at(-1).name, "Merchant entered");
});

test("shared drawer quantity controls namespace IDs, bound quantity and remove zero in all locales", () => {
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    const f = manualFormFixture(locale, { idPrefix: "drawer", quantityControls: "stepper" });
    const byId = (id) => f.nodes.find((n) => n.props["data-testid"] === id);
    assert.ok(byId("drawer-search")); assert.ok(byId("drawer-lines")); assert.ok(byId("drawer-name"));
    assert.equal(f.nodes.some((n) => n.props["data-testid"]?.startsWith("mo-")), false);
    assert.match(byId("drawer-minus-sku-1").props["aria-label"], new RegExp(f.copy.toolsCopy[locale].manual.quantity));
    assert.match(byId("drawer-plus-sku-1").props["aria-label"], /Catalog product/);
    byId("drawer-plus-sku-1").props.onClick();
    assert.equal(f.patches.at(-1).lines[0].quantity, 2);
    assert.equal(f.patches.at(-1).lines[0].price, "NT$100");
    byId("drawer-minus-sku-1").props.onClick();
    assert.equal(f.patches.at(-1).lines.length, 0);
    byId("drawer-quantity-sku-1").props.onChange({ target: { value: "1001" } });
    assert.equal(f.patches.at(-1).lines[0].quantity, 1000);
    byId("drawer-quantity-sku-1").props.onChange({ target: { value: "0" } });
    assert.equal(f.patches.at(-1).lines.length, 0);
  }
});

test("shared delivery choice emits one patch with the only allowed payment mode", () => {
  const option = { option_key: "home-bank", delivery_kind: "home", payment_modes: ["bank_transfer"],
    name_hant: "宅配", name_hans: "宅配", name_en: "Home" };
  const f = manualFormFixture("en", { available: [option], optionsReady: false });
  const select = f.nodes.find((n) => n.props["data-testid"] === "mo-option");
  assert.equal(select.props.disabled, true);
  select.props.onChange({ target: { value: "home-bank" } });
  assert.equal(f.patches.at(-1).optionKey, "home-bank");
  assert.equal(f.patches.at(-1).mode, "bank_transfer");
  select.props.onChange({ target: { value: "" } });
  assert.equal(f.patches.at(-1).mode, "");
});
