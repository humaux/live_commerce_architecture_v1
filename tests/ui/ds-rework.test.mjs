// Purpose: DS-2 render contracts and regression negatives; authored for CI, never substitutes for real click/visual review.
// Depends on: node:test/assert, React SSR, TypeScript transpilation; real DS modules loaded with CSS names only stubbed.
// Used by: ds-tokens.test.mjs in test-node.sh and --browser-ui-gallery. Browser/red-green execution remains CI NOT_RUN.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import { test } from "node:test";
import ts from "typescript-api";
import * as logic from "../../packages/ui/src/ds/logic.ts";
const app = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = app("react");
const { renderToStaticMarkup } = app("react-dom/server");
const cache = new Map();
const css = { __esModule: true, default: new Proxy({}, { get: (_, key) => String(key) }) };
function load(name) {
  if (name === "./logic") return logic;
  if (!name.startsWith("./")) return app(name);
  if (name.endsWith(".css")) return css;
  if (cache.has(name)) return cache.get(name);
  const exports = {};
  cache.set(name, exports);
  const source = readFileSync(`packages/ui/src/ds/${name.slice(2)}.tsx`, "utf8");
  runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText, { exports, require: load, process });
  return exports;
}
const { Icon } = load("./Icon");
const { Field, TextInput, FormGrid } = load("./Fields");
const { Badge, Alert, Button } = load("./Controls");
const { FormActionBar, PageHeader, Timeline } = load("./Layout");
const { MenuButton, DataTable } = load("./Data");
const { UploadGrid, EmptyState } = load("./States");
const el = React.createElement;
const html = node => renderToStaticMarkup(node);

test("icons: fixed optical box, stroke, sizes and decorative semantics; labelled variant has a title", () => {
  for (const name of ["dashboard", "orders", "live", "messages", "products", "customers", "marketing", "store", "analytics", "settings", "chevronRight", "chevronDown", "search", "close", "check", "alert", "info", "warning", "danger", "upload", "image", "more", "plus", "externalLink", "help", "bell", "user", "spinner"]) {
    const out = html(el(Icon, { name, size: 16 }));
    assert.match(out, /viewBox="0 0 24 24"/);
    assert.match(out, /stroke-width="1.75"/);
    assert.match(out, /stroke="currentColor"/);
    assert.match(out, /aria-hidden="true"/);
    assert.match(out, /<path d="[^"]+"/);
  }
  for (const size of [16, 20, 24]) assert.match(html(el(Icon, { name: "check", size })), new RegExp(`width="${size}" height="${size}"`));
  const titled = html(el(Icon, { name: "help", title: "Help" }));
  assert.match(titled, /role="img"/);
  assert.match(titled, /<title id="[^"]+">Help<\/title>/);
  assert.doesNotMatch(titled, /aria-hidden/);
});

test("badges: a single text-only rule; loading/alert controls use SVG, never text glyphs", () => {
  assert.equal(html(el(Badge, { tone: "success" }, "Paid")), '<span class="badge" data-tone="success">Paid</span>');
  assert.match(html(el(Button, { loading: true }, "Saving")), /<svg/);
  const alert = html(el(Alert, { title: "Problem", onDismiss() {}, dismissLabel: "Close" }, "Description"));
  assert.equal((alert.match(/<svg/g) ?? []).length, 2);
  assert.doesNotMatch(alert, /[×✓✕●▸]/);
});

test("Field: the actual input owns label, error/help/counter associations and maxLength", () => {
  const out = html(el(Field, { label: "Name", required: true, help: "Shown in store", error: "Required", width: "lg", counter: { value: "abc", max: 80, format: (n, max) => `${n} / ${max}` }, children: props => el(TextInput, { ...props, defaultValue: "abc" }) }));
  const id = /<input[^>]*?\sid="([^"]+)"/.exec(out)?.[1]; // \sid= so aria-invalid="true" (…id="true") is never read as the id
  assert.ok(id, `no input id in: ${out}`);
  assert.ok(out.includes(`for="${id}"`), `label not tied to input id ${id} in: ${out}`);
  // getByLabel exact uses the label's text, including aria-hidden descendants. Required marks must be siblings.
  assert.ok(out.includes(`<label for="${id}">Name</label>`), "required field has one exact, unadorned label");
  assert.match(out, /<\/label><span class="required" aria-hidden="true">\*<\/span>/);
  assert.match(out, /<input[^>]* required=""/);
  assert.ok(out.includes(`aria-describedby="${id}-help ${id}-error ${id}-counter"`));
  assert.match(out, /aria-invalid="true"/);
  assert.match(out, /maxLength="80"/);
  assert.match(out, /3 \/ 80/);
  assert.match(out, /role="alert">Required/);
  assert.doesNotMatch(html(el(Field, { label: "Name", children: props => el(TextInput, props) })), /aria-invalid|maxLength|aria-describedby/);
  assert.match(html(el(FormGrid, null, "content")), /class="grid"/);
});

test("FormActionBar: one connected frame, two responsive bars, explicit state, one live announcement", () => {
  const out = html(el(FormActionBar, { label: "Save", state: "saving", savingNote: "Saving", actions: el(Button, { loading: true }, "Save") }, "Form"));
  assert.match(out, /class="formFrame"/);
  assert.equal((out.match(/data-state="saving"/g) ?? []).length, 2);
  assert.equal((out.match(/role="status"/g) ?? []).length, 1);
  assert.match(out, /class="formContent" data-form-content="true">Form/);
  assert.match(out, /class="formSurface" data-form-surface="true"/);
});

test("upload grid: progress, error, retry, remove and completion keep explicit names", () => {
  const out = html(el(UploadGrid, { label: "Queue", items: [
    { id: "a", name: "a.png", status: "uploading", progress: 35 },
    { id: "b", name: "b.png", status: "error", progress: 0, error: "Failed" },
    { id: "c", name: "c.png", status: "done", progress: 100 },
  ], removeLabel: n => `Remove ${n}`, retryLabel: n => `Retry ${n}`, progressLabel: n => `${n}%`, doneLabel: "Done", onRemove() {}, onRetry() {} }));
  assert.match(out, /<progress[^>]*value="35"/);
  assert.match(out, /role="alert">Failed/);
  assert.match(out, /Retry b.png/);
  assert.match(out, /aria-label="Remove a.png"/);
  assert.match(out, /role="status">Done/);
  assert.doesNotMatch(out, /Retry a.png|Retry c.png/);
});

test("state density is explicit and timeline facts carry ISO time", () => {
  assert.match(html(el(EmptyState, { title: "Empty" })), /data-mode="compact"/);
  assert.match(html(el(EmptyState, { title: "Empty", mode: "full" })), /data-mode="full"/);
  assert.match(html(el(Timeline, { label: "History", items: [{ id: "a", title: "Received", dateTime: "2026-10-15T10:00:00+08:00", time: "10:00 UTC+8" }] })), /<time dateTime="2026-10-15T10:00:00\+08:00">/i);
});

test("round 2: icon menu has one SVG and an explicit name, header status shares the title row", () => {
  const menu = html(el(MenuButton, { label: "More", icon: "more", size: "sm", items: [] }));
  assert.equal((menu.match(/<svg/g) ?? []).length, 1);
  assert.match(menu, /aria-label="More"/);
  assert.match(menu, /data-icon-only="true"/);
  const header = html(el(PageHeader, { breadcrumbLabel: "Trail", title: "Order", status: el(Badge, null, "Paid") }));
  assert.match(header, /class="pageTitleRow"><h1[^>]*>Order<\/h1><div class="pageStatus">/);
});

test("round 2: tables expose a localized scroll cue and focusable native scroll group", () => {
  const out = html(el(DataTable, { caption: "Orders", columns: [], rows: [], rowId: row => row.id, empty: "None", error: "Error", loadingLabel: "Loading", scrollHint: "Scroll horizontally" }));
  const hint = /id="([^"]+)" data-table-scroll-hint/.exec(out)?.[1];
  assert.ok(hint);
  assert.match(out, /Scroll horizontally/);
  assert.ok(out.includes(`aria-describedby="${hint}"`));
  assert.match(out, /tabindex="0" role="group" aria-label="Orders"/i);
});
