// Purpose: Render the actual LC-U1 component for degraded secondary reads, permission locks and platform notice boundaries.
// Depends on: existing React SSR/typescript-api VM, real LiveConsole/workspace copy/format; read hook values are test inputs, not network acceptance.
// Used by: test-node and LC-U1 review; no browser, database, provider or production-state write.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { workspaceCopy } from "../../apps/admin/src/features/live/workspace-copy.ts";
import * as model from "../../apps/admin/src/features/live/workspace-model.ts";
import * as format from "../../packages/format/src/index.ts";

const app = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = app("react"), { renderToStaticMarkup } = app("react-dom/server");
const store = { id: "11111111-1111-4111-8111-111111111111", role: "owner", name: "Synthetic store" }, scene = "22222222-2222-4222-8222-222222222222";
function render(locale: "en" | "zh-TW" | "zh-CN", platform: string, embeddable: boolean, denied = false) {
  const snapshots: any[] = [
    { session: { title: "A1 PRIVATE TITLE", lifecycle: "draft", version: 1 }, stats: { comments: { total: null, source: "unavailable" }, keyword_comments: 0, buyers: 0, orders: { count: 0, amount_minor: 0 }, paid: { amount_minor: 0 }, currency: "TWD", as_of: "2026-10-07T00:00:00Z" }, stream: { state: "not_started", source_platform: platform, video_embeddable: embeddable, last_ok_at: null }, offers: [], recommended: null },
    null, null, null,
  ];
  let at = 0, listRefreshes = 0; const polls: boolean[] = [], refreshes: number[] = [];
  let refresh: (() => void) | undefined;
  const exports: any = {};
  let source = readFileSync("apps/admin/components/LiveConsole.tsx", "utf8");
  // Fault calibration is in-memory only; neither API fixtures nor shipped code are changed.
  if (process.env.LC_LIVE_RENDER_FAULT === "a1_poll") source = source.replace("readConsole(store.id, sessionID, signal), true)", "readConsole(store.id, sessionID, signal), false)");
  if (process.env.LC_LIVE_RENDER_FAULT === "notice") source = source.replace('data.stream.source_platform === "instagram" || !data.stream.video_embeddable', "true");
  runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText, { exports, require: (name: string) => {
    if (name === "react") return { ...React, useEffect: () => {} };
    // LC-U1 render host isolates offers/lifecycle; LC-U2a executes its real child in separate hook/browser gates.
    if (name === "./CommentStream") return { CommentStream: () => null };
    if (name === "next/navigation") return { useRouter: () => ({ push: () => {} }) };
    if (name === "react/jsx-runtime") {
      const runtime = app(name);
      const capture = (fn: any) => (tag: any, props: any, ...rest: any[]) => { if (props?.["data-testid"] === "live-console-refresh") refresh = props.onClick; return fn(tag, props, ...rest); };
      return { ...runtime, jsx: capture(runtime.jsx), jsxs: capture(runtime.jsxs) };
    }
    if (name === "@live-commerce/format") return format;
    if (name.endsWith("workspace-copy")) return { workspaceCopy };
    if (name.endsWith("workspace-model")) return model;
    if (name.endsWith("use-live-workspace")) return {
      useLiveRead: (_s: string, _e: boolean, _read: any, poll = false) => { const i = at++; polls.push(poll); return { data: denied ? null : snapshots[i], error: denied ? "forbidden" : i ? "unavailable" : "", boundary: denied ? "" : "synthetic", refresh: () => { refreshes.push(i); } }; },
      useLiveCommand: () => ({ blocked: true, busy: false, canRetry: false, error: "", reason: "", invalidate: () => {}, run: () => {}, retry: () => {} }),
    };
    if (name.endsWith("-client") || name.endsWith("command-journal")) return {};
    return app(name);
  } });
  const html = renderToStaticMarkup(React.createElement(exports.LiveConsole, { locale, store, sessionID: scene, navigationGuard: { current: () => true }, onRefreshList: () => { listRefreshes++; } }));
  return { html, polls, refresh, refreshes, listRefreshes: () => listRefreshes };
}
test("secondary failures preserve A1; only A1 is periodic; IG notice follows the platform/embeddability observation", () => {
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const ready = render(locale, "facebook", true);
    assert.match(ready.html, /A1 PRIVATE TITLE/); assert.match(ready.html, /live-status-bar/);
    assert.deepEqual(ready.polls, [true, false, false, false]); assert.doesNotMatch(ready.html, /live-instagram-notice/);
    for (const [platform, embeddable] of [["instagram", true], ["facebook", false]] as const)
      assert.match(render(locale, platform, embeddable).html, /live-instagram-notice/);
    const denied = render(locale, "facebook", true, true).html;
    assert.doesNotMatch(denied, /A1 PRIVATE TITLE|live-primary-action|live-status-bar/);
    assert.match(denied, /live-console-unavailable/); assert.match(denied, /disabled=""/);
  }
});

test("the existing console refresh also explicitly refreshes its parent list; no automatic list read", () => {
  const view = render("en", "facebook", true);
  assert.equal(view.listRefreshes(), 0); assert.deepEqual(view.refreshes, []);
  assert.equal(typeof view.refresh, "function"); view.refresh!();
  assert.deepEqual(view.refreshes, [0, 1, 2, 3]); assert.equal(view.listRefreshes(), 1);
});

test("parent forwards only an explicit refresh callback and retains the scene when list is unavailable", () => {
  for (const unavailable of [false, true]) {
    const exports: any = {}; let props: any, refreshes = 0, periodic: boolean | undefined;
    runInNewContext(ts.transpileModule(readFileSync("apps/admin/components/LiveWorkspace.tsx", "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText, { exports, require: (name: string) => {
      if (name === "next/navigation") return { useRouter: () => ({ push: () => {} }) };
      if (name.endsWith("workspace-copy")) return { workspaceCopy };
      if (name.endsWith("use-live-workspace")) return { useLiveRead: (_scope: string, _enabled: boolean, _read: any, poll = false) => { periodic = poll; return { error: unavailable ? "unavailable" : "", data: unavailable ? null : { items: [{ session_id: scene, title: "Synthetic" }] }, refresh: () => { refreshes++; } }; } };
      if (name.endsWith("LiveConsole")) return { LiveConsole: (p: any) => { props = p; return React.createElement("section", { "data-scene": p.sessionID }); } };
      if (name.endsWith("WorkspaceFrame")) return { WorkspaceFrame: (p: any) => p.children };
      if (name.endsWith("AdminPageHeader")) return { AdminPageHeader: () => null };
      if (name.endsWith(".css") || name.endsWith("studio-client")) return {};
      return app(name);
    } });
    const html = renderToStaticMarkup(React.createElement(exports.LiveWorkspace, { locale: "en", store, scene, initialError: null }));
    assert.equal(periodic, false); assert.equal(props.sessionID, scene); assert.match(html, /data-scene=/);
    assert.equal(refreshes, 0); assert.equal(typeof props.onRefreshList, "function"); props.onRefreshList(); assert.equal(refreshes, 1);
  }
});

test("every explicit browser response/event/navigation waiter has a finite sub-test budget", () => {
  const source = ts.createSourceFile("console.spec.ts", readFileSync("tests/admin/live-console.spec.ts", "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  let count = 0;
  const walk = (node: any) => {
    if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && ["waitForResponse", "waitForEvent", "waitForURL"].includes(node.expression.name.text)) {
      count++; const options = node.arguments[1];
      assert.ok(options && ts.isObjectLiteralExpression(options), `${node.expression.name.text} needs explicit options`);
      const field = options.properties.find((p: any) => ts.isPropertyAssignment(p) && p.name.getText(source) === "timeout");
      assert.ok(field && ts.isNumericLiteral(field.initializer), "timeout must be an explicit finite budget");
      const value = Number(field.initializer.text); assert.ok(value > 0 && value <= 15000, "wait budget must fit well inside 110s case timeout");
    }
    ts.forEachChild(node, walk);
  };
  walk(source); assert.ok(count >= 3, "response, popup and logout guards must all be covered");
});
