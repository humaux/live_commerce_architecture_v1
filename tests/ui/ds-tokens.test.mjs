// Purpose: DS-1 token gate: WCAG contrast of every text/surface and focus pair, the 4-based spacing and 12..28 type scales, nav width, z-index order, legacy --ui-* values unchanged, and no literal colour outside tokens.css.
// Depends on: node:test/assert/fs and packages/ui/src/tokens.css + packages/ui/src/ds sources (read only); no browser.
// Used by: scripts/dev/test-node.sh and the --browser-ui-gallery mode.
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { test } from "node:test";

const css = readFileSync("packages/ui/src/tokens.css", "utf8");
const decls = new Map();
for (const m of css.matchAll(/(--[a-z0-9-]+):\s*([^;]+);/g)) if (!decls.has(m[1])) decls.set(m[1], m[2].trim());
const resolve = (name, depth = 0) => {
  const v = decls.get(name);
  assert.ok(v !== undefined, `${name} is not defined in tokens.css`);
  const ref = /^var\((--[a-z0-9-]+)\)$/.exec(v);
  return ref && depth < 5 ? resolve(ref[1], depth + 1) : v;
};
const ds = (token) => resolve(`--ds-${token}`);

const channel = (v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
function luminance(hex) {
  const h = /^#([0-9a-f]{6})$/i.exec(hex)?.[1] ?? (/^#([0-9a-f]{3})$/i.exec(hex)?.[1] ?? "").replace(/./g, "$&$&");
  assert.equal(h.length, 6, `not a hex colour: ${hex}`);
  const [r, g, b] = [0, 2, 4].map((i) => channel(parseInt(h.slice(i, i + 2), 16) / 255));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
const ratio = (a, b) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};

// [foreground token, background token]: text and icons that carry meaning, WCAG 2.2 AA 4.5:1.
const TEXT_PAIRS = [
  ["text-1", "surface"], ["text-1", "page"], ["text-2", "surface"], ["text-2", "page"], ["text-3", "surface"], ["text-3", "page"],
  ["text-2", "surface-muted"], ["text-3", "surface-muted"], ["text-1", "surface-muted"], ["text-1", "primary-soft"],
  ["on-primary", "primary"], ["on-primary", "primary-hover"], ["on-primary", "danger"], ["on-primary", "danger-hover"], ["on-primary", "toggle-on"], ["on-primary", "text-2"],
  ["link", "surface"], ["link", "page"], ["link-hover", "surface"], ["primary", "primary-soft"], ["primary", "surface"],
  ["success-text", "surface"], ["success-text", "success-bg"], ["warn-text", "warn-bg"], ["warn-text", "warn-soft"], ["warn-text", "surface"],
  ["danger-text", "surface"], ["danger-text", "danger-bg"], ["info-text", "surface"], ["info-text", "info-bg"], ["neutral-text", "neutral-bg"],
  ["nav-text", "nav-bg"], ["nav-text", "nav-active"], ["nav-text", "nav-logo-bg"], ["nav-text-muted", "nav-bg"], ["nav-text-muted", "nav-active"], ["nav-accent-text", "nav-bg"], ["nav-accent-text", "nav-active"],
];
// Non-text: focus ring on both surfaces (and the dark ring on the dark nav), control boundaries, the active-nav bar: WCAG 1.4.11 / 2.4.11, 3:1.
const UI_PAIRS = [
  ["focus", "surface"], ["focus", "page"], ["focus", "surface-muted"], ["focus", "primary-soft"],
  ["focus-on-dark", "nav-bg"], ["focus-on-dark", "nav-active"], ["focus-on-dark", "nav-logo-bg"],
  ["border-input", "surface"], ["border-input", "page"], ["nav-accent", "nav-bg"], ["primary", "page"],
];

test("contrast: every text pair holds 4.5:1", () => {
  for (const [fg, bg] of TEXT_PAIRS) assert.ok(ratio(ds(fg), ds(bg)) >= 4.5, `${fg} on ${bg}: ${ratio(ds(fg), ds(bg)).toFixed(2)} < 4.5`);
});

test("contrast: focus ring and control boundaries hold 3:1 on every surface they sit on", () => {
  for (const [fg, bg] of UI_PAIRS) assert.ok(ratio(ds(fg), ds(bg)) >= 3, `${fg} on ${bg}: ${ratio(ds(fg), ds(bg)).toFixed(2)} < 3`);
});

test("contrast: the calculator rejects the reference values that fail (so the gate can fail)", () => {
  assert.ok(ratio("#ffffff", "#4492d1") < 4.5, "SHOPLINE primary #4492D1 with white text is below AA");
  assert.ok(ratio("#ffffff", "#777777") < 4.5, "SHOPLINE text-3 #777777 on white is below AA");
  assert.ok(ratio("#ffffff", "#74b666") < 4.5, "SHOPLINE toggle-on #74B666 with white text is below AA");
});

test("scales: spacing is 4-based, type scale is 12/13/14/16/18/22/28, z-index is strictly ordered", () => {
  for (const [name, value] of decls)
    if (/^--ds-space-\d+$/.test(name)) assert.equal(parseInt(value, 10) % 4, 0, `${name}: ${value} is not a multiple of 4`);
  const sizes = ["xs", "sm", "md", "lg", "xl", "2xl", "3xl"].map((k) => parseInt(ds(`text-${k}`), 10));
  assert.deepEqual(sizes, [12, 13, 14, 16, 18, 22, 28]);
  const z = ["base", "sticky", "topbar", "nav", "drawer", "dialog", "toast"].map((k) => Number(ds(`z-${k}`)));
  assert.deepEqual([...z].sort((a, b) => a - b), z);
  assert.equal(new Set(z).size, z.length);
  const nav = parseInt(ds("nav-width"), 10);
  assert.ok(nav >= 248 && nav <= 270, `nav width ${nav}`);
  assert.ok(parseInt(ds("target"), 10) >= 44);
});

test("aliases: every legacy --ui-* token keeps its old value, and data-ds=2 only remaps names that exist", () => {
  const legacy = {
    "--ui-primary": "#247965", "--ui-primary-hover": "#1c6554", "--ui-primary-soft": "#e5f2ed", "--ui-brand": "var(--ui-primary)", "--ui-brand-text": "var(--ui-primary)",
    "--ui-link": "var(--ui-primary-hover)", "--ui-rail-active": "#b4e8d6", "--ui-rail-selected": "#263e39", "--ui-page": "#f5f6f8", "--ui-surface": "#fff", "--ui-border": "#e5e7eb",
    "--ui-text": "#1f2937", "--ui-muted": "#566171", "--ui-rail": "#1f2937", "--ui-danger": "#b42318", "--ui-success": "#166534", "--ui-warning": "#854d0e", "--ui-info": "#1d4ed8",
    "--ui-neutral-soft": "#edf0f3", "--ui-success-soft": "#e4f3e9", "--ui-warning-soft": "#fff4d9", "--ui-danger-soft": "#fdebe9", "--ui-info-soft": "#eaf0fc", "--ui-focus": "#1d4ed8",
    "--ui-radius": "8px", "--ui-space-1": "4px", "--ui-space-2": "8px", "--ui-space-3": "12px", "--ui-space-4": "16px", "--ui-space-6": "24px", "--ui-target": "44px",
    "--ui-control-compact": "40px", "--ui-text-body": "14px", "--ui-text-helper": "12px", "--ui-heading-page": "29px", "--ui-heading-section": "18px", "--ui-heading-subsection": "14px",
    "--ui-field-short": "220px", "--ui-field-medium": "400px", "--ui-field-long": "640px", "--ui-rail-width": "220px",
  };
  const root = css.slice(css.indexOf(":root {"), css.indexOf("}", css.indexOf(":root {")));
  const actual = Object.fromEntries([...root.matchAll(/(--ui-[a-z0-9-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
  assert.deepEqual(actual, legacy, "the legacy :root block changed: current pages would change");
  const remap = css.slice(css.indexOf('[data-ds="2"]'));
  const keys = [...remap.matchAll(/(--ui-[a-z0-9-]+):\s*var\((--ds-[a-z0-9-]+)\)/g)];
  assert.ok(keys.length >= 25);
  for (const [, ui, target] of keys) {
    assert.ok(ui in legacy, `${ui} is remapped but is not a legacy token`);
    assert.ok(decls.has(target), `${ui} -> ${target} is undefined`);
  }
  assert.match(css.slice(0, css.indexOf(":root {")), /ALIAS LIST/, "the alias list is documented in the file header");
});

test("tokens only: no literal colour in the DS CSS modules or sources; tokens.css is the one exemption", () => {
  const dir = "packages/ui/src/ds";
  const files = readdirSync(dir).filter((f) => /\.(css|tsx?)$/.test(f));
  assert.ok(files.length >= 12);
  for (const f of files) {
    const source = readFileSync(`${dir}/${f}`, "utf8");
    assert.doesNotMatch(source, /#[0-9a-fA-F]{3,8}\b/, `${f}: literal hex colour (use a --ds-* token)`);
    assert.doesNotMatch(source, /\b(?:rgb|hsl)a?\((?!\s*0\s+0\s+0\s*\/)/i, `${f}: literal colour function`);
  }
});

test("no copy inside the ui package: components render strings that arrive as props", () => {
  // A literal word between JSX tags would be copy. Glyphs and punctuation (✓ ‹ › … ▾ ×) are symbols, not language.
  for (const f of readdirSync("packages/ui/src/ds").filter((n) => n.endsWith(".tsx"))) {
    const source = readFileSync(`packages/ui/src/ds/${f}`, "utf8");
    const stripped = source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");
    assert.doesNotMatch(stripped, />\s*[A-Za-z一-鿿][A-Za-z一-鿿 ]{2,}\s*</, `${f}: literal text between JSX tags`);
  }
});
