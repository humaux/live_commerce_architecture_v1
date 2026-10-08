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
  });
  // Compile uses the actual component; only hook state and surrounding presentation are fixtures.
  const tree = ManualOrder({ locale: "zh-TW", stores: [], store: { id: "store", name: "store" },
    initialError: null, renderKey: "test" });
  assert.match(textOf(tree), expected);
});
