// Purpose: Execute LC-U1 hook effects with deterministic timers, lifecycle events and durable command replay.
// Depends on: node:test/vm/assert, existing typescript-api and real hook/client source; the hook dispatcher is a test double, not React DOM.
// Used by: test-node; focused red/green protects cadence, concealment and receipt identity without a browser or PG.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const store = "11111111-1111-4111-8111-111111111111", scene = "22222222-2222-4222-8222-222222222222";
const scope = `${store}:${scene}`, path = `/api/stores/${store}/live-sessions/${scene}/lifecycle`;
const request = { method: "POST", path, body: JSON.stringify({ action: "start", expected_version: 1 }) };
const flush = async () => { for (let i = 0; i < 30; i++) await Promise.resolve(); };

function fixture() {
  const document = Object.assign(new EventTarget(), { visibilityState: "visible", cookie: "csrf" });
  const window = new EventTarget(), saved = new Map<string, string>();
  const sessionStorage = { getItem: (k: string) => saved.get(k) ?? null, setItem: (k: string, v: string) => saved.set(k, v), removeItem: (k: string) => saved.delete(k) };
  const settings = { csrfCookie: () => document.cookie, sessionBoundary: async () => "session-one" };
  let execute = async (_request: any, _key: string, _boundary: string): Promise<unknown> => ({ ok: true });
  const calls: any[] = [], cache = new Map<string, any>();
  const react: Record<string, any> = {};
  const load = (file: string): any => {
    file = resolve(file);
    if (cache.has(file)) return cache.get(file);
    const exports: any = {}; cache.set(file, exports);
    runInNewContext(ts.transpileModule(readFileSync(file, "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText, {
      exports, require: (name: string) => {
        if (name === "react") return react;
        if (name === "next/navigation") return { useRouter: () => ({ push: () => {} }) };
        if (name.endsWith("settings-client")) return settings;
        if (name.endsWith("console-client")) return { executeLiveRequest: async (r: any, k: string, b: string) => { calls.push({ ...r, key: k, boundary: b }); return execute(r, k, b); } };
        const candidate = name.startsWith("@/") ? resolve("apps/admin", name.slice(2)) : name.startsWith(".") ? resolve(dirname(file), name) : "";
        if (candidate && existsSync(candidate) && candidate.endsWith(".ts")) return load(candidate);
        if (candidate && existsSync(`${candidate}.ts`)) return load(`${candidate}.ts`);
        return requireApp(name);
      }, document, window, sessionStorage, crypto: globalThis.crypto, AbortController, AbortSignal,
      Response, Date, fetch: (...args: any[]) => (globalThis.fetch as any)(...args), setTimeout, clearTimeout,
      BroadcastChannel: undefined, queueMicrotask,
    });
    return exports;
  };
  const studio = load("apps/admin/lib/studio-client.ts"), hooks = load("apps/admin/src/features/live/use-live-workspace.ts");
  let mounted: any = null;
  function mount(run: () => any) {
    const slots: any[] = [], effects: { deps: any[]; cleanup?: () => void }[] = [];
    let at = 0, queued = false, alive = true, view: any;
    const render = () => {
      if (!alive) return;
      queued = false; at = 0;
      Object.assign(react, {
        useState: (initial: any) => { const i = at++; if (!(i in slots)) slots[i] = typeof initial === "function" ? initial() : initial; return [slots[i], (next: any) => { slots[i] = typeof next === "function" ? next(slots[i]) : next; if (!queued) { queued = true; queueMicrotask(render); } }]; },
        useRef: (initial: any) => { const i = at++; if (!(i in slots)) slots[i] = { current: initial }; return slots[i]; },
        useCallback: (fn: any) => { at++; return fn; },
        useEffect: (fn: () => any, deps: any[]) => { const i = at++, previous = effects[i]; if (!previous || deps.some((d, j) => d !== previous.deps[j])) { queueMicrotask(() => { if (!alive) return; previous?.cleanup?.(); effects[i] = { deps, cleanup: fn() }; }); } },
      });
      view = run();
    };
    render();
    mounted = { get view() { return view; }, render, stop: () => { alive = false; for (const e of effects) e?.cleanup?.(); } };
    return mounted;
  }
  return { hooks, studio, load, document, window, saved, calls, mount, stop: () => mounted?.stop(), setExecute: (fn: typeof execute) => { execute = fn; } };
}

test("A1 failures back off 3/6/12/24/30 seconds and success resets to five seconds", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const f = fixture(); let reads = 0;
  const h = f.mount(() => f.hooks.useLiveRead(scope, true, async () => { reads++; if (reads <= 5) throw new f.studio.StudioError("unavailable"); return { title: "console" }; }, true));
  t.after(f.stop); await flush(); assert.equal(reads, 1);
  for (const seconds of [3, 6, 12, 24, 30]) { const before = reads; t.mock.timers.tick(seconds * 1000 - 1); await flush(); assert.equal(reads, before); t.mock.timers.tick(1); await flush(); assert.equal(reads, before + 1); }
  assert.equal(h.view.data.title, "console");
  t.mock.timers.tick(4999); await flush(); assert.equal(reads, 6); t.mock.timers.tick(1); await flush(); assert.equal(reads, 7);
});

test("Retry-After is a minimum; hidden tabs cancel polling; forbidden locks and conceals", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const f = fixture(); let reads = 0;
  const h = f.mount(() => f.hooks.useLiveRead(scope, true, async () => { reads++; if (reads === 1) throw new f.studio.StudioError("unavailable", "", 40000, 429); return { title: "PRIVATE" }; }, true));
  t.after(f.stop); await flush(); t.mock.timers.tick(39999); await flush(); assert.equal(reads, 1);
  t.mock.timers.tick(1); await flush(); assert.equal(reads, 2);
  f.document.visibilityState = "hidden"; f.document.dispatchEvent(new Event("visibilitychange")); await flush();
  t.mock.timers.tick(60000); await flush(); assert.equal(reads, 2); assert.equal(h.view.data, null);
  f.document.visibilityState = "visible"; f.document.dispatchEvent(new Event("visibilitychange")); await flush(); assert.equal(reads, 3);
  f.stop();
  const denied = f.mount(() => f.hooks.useLiveRead(scope, true, async () => { throw new f.studio.StudioError("forbidden", "forbidden", 0, 403); }, true));
  await flush(); t.mock.timers.tick(120000); await flush(); assert.equal(denied.view.error, "forbidden"); assert.equal(denied.view.data, null); assert.equal(denied.view.boundary, "");
  denied.view.refresh(); f.document.dispatchEvent(new Event("visibilitychange")); await flush(); assert.equal(denied.view.error, "forbidden");
});

test("secondary reads are mount/refresh only, independent from A1 and non-fatal", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const f = fixture(); let reads = 0;
  const h = f.mount(() => f.hooks.useLiveRead(`${scope}:secondary`, true, async () => { reads++; throw new f.studio.StudioError("unavailable"); }));
  t.after(f.stop); await flush(); t.mock.timers.tick(60000); await flush(); assert.equal(reads, 1);
  h.view.refresh(); await flush(); assert.equal(reads, 2);
});

test("UNKNOWN journal restores the exact key/method/path/body and explicit retry after reload", async (t) => {
  const f = fixture(); f.setExecute(async () => { throw new f.studio.StudioError("uncertain"); });
  let h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); t.after(f.stop); await flush();
  await h.view.run(request); await flush(); const record = JSON.parse(f.saved.get(`live-workspace-command:${scope}`)!);
  assert.equal(record.method, "POST"); assert.equal(record.path, path); assert.equal(record.body, request.body); assert.equal(f.calls.length, 1);
  f.stop(); h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); await flush(); assert.equal(h.view.canRetry, true);
  f.setExecute(async () => ({ ok: true })); await h.view.retry(); await flush();
  assert.equal(f.calls[1].key, f.calls[0].key); assert.equal(f.calls[1].body, f.calls[0].body); assert.equal(f.calls[1].path, f.calls[0].path);
  assert.equal(f.saved.size, 0);
});

test("first definite 403 clears journal but locks commands; late success after navigation clears only its own key", async (t) => {
  const f = fixture(); f.setExecute(async () => { throw new f.studio.StudioError("forbidden", "forbidden", 0, 403); });
  let h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); t.after(f.stop); await flush();
  await h.view.run(request); await flush(); assert.equal(f.saved.size, 0); assert.equal(h.view.error, "forbidden"); assert.equal(h.view.blocked, true);
  f.stop(); let done!: (v: unknown) => void, navigations = 0;
  f.setExecute(() => new Promise((resolve) => { done = resolve; }));
  h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); await flush();
  const pending = h.view.run(request, () => { navigations++; }); await flush(); h.view.invalidate(); f.stop(); done({ ok: true }); await pending; await flush();
  assert.equal(f.saved.size, 0); assert.equal(navigations, 0);
});

test("private reads preserve Retry-After seconds and dates, without changing existing error codes", async () => {
  const f = fixture(), original = globalThis.fetch;
  try {
    for (const [status, header] of [[429, "12"], [503, new Date(Date.now() + 15000).toUTCString()]] as const) {
      globalThis.fetch = async () => new Response(null, { status, headers: { "Retry-After": header } });
      await assert.rejects(f.studio.read("/api/test", new AbortController().signal), (e: any) => e.code === "unavailable" && e.retryAfterMs >= 12000 && e.status === status);
    }
  } finally { globalThis.fetch = original; }
});

test("a restored in-flight or UNKNOWN request retains its journal after a replay 403", async (t) => {
  for (const beforeReload of ["in-flight", "unknown-after-departure", "postflight-signout"]) {
    const f = fixture(); let reject!: (e: unknown) => void;
    f.setExecute(() => new Promise((_, no) => { reject = no; }));
    let h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); await flush();
    const pending = h.view.run(request); await flush(); const firstKey = f.calls[0].key;
    h.view.invalidate(); f.stop();
    if (beforeReload !== "in-flight") { reject(new f.studio.StudioError(beforeReload === "postflight-signout" ? "signed-out" : "uncertain")); await pending; }
    h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); await flush();
    f.setExecute(async () => { throw new f.studio.StudioError("forbidden", "forbidden", 0, 403); });
    await h.view.retry(); await flush();
    assert.equal(f.calls[1].key, firstKey); assert.equal(f.saved.size, 1, beforeReload); assert.equal(h.view.blocked, true);
    f.stop(); if (beforeReload === "in-flight") { reject(new f.studio.StudioError("uncertain")); await pending; }
  }
});

test("restored copy success dispatches completion only in the still-current scene", async (t) => {
  for (const departed of [false, true]) {
    const f = fixture(), results: any[] = [];
    f.setExecute(async () => { throw new f.studio.StudioError("uncertain"); });
    let h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {})); await flush();
    const copy = { method: "POST", path: path.replace("/lifecycle", "/copy"), body: JSON.stringify({ title: "Next scene", scheduled_at: null, expected_version: 1 }) };
    await h.view.run(copy); f.stop();
    h = f.mount(() => f.hooks.useLiveCommand(scope, "session-one", () => {}, (r: any, value: any) => results.push({ path: r.path, value }))); await flush();
    let done!: (v: unknown) => void; f.setExecute(() => new Promise((resolve) => { done = resolve; }));
    const pending = h.view.retry(); await flush(); if (departed) { h.view.invalidate(); f.stop(); }
    done({ session: { session_id: "new-scene" }, conflicts: [] }); await pending; await flush();
    assert.equal(results.length, departed ? 0 : 1); if (!departed) assert.equal(results[0].path, copy.path);
    assert.equal(f.saved.size, 0); f.stop();
  }
});

function element(tree: any, predicate: (node: any) => boolean): any {
  if (!tree || typeof tree !== "object") return null;
  if (Array.isArray(tree)) { for (const child of tree) { const found = element(child, predicate); if (found) return found; } return null; }
  return predicate(tree) ? tree : element(tree.props?.children, predicate);
}

test("SessionCopy resyncs closed same-ID titles, preserves open edits and cancels to latest draft", async (t) => {
  const f = fixture(), { SessionCopy } = f.load("apps/admin/src/features/live/SessionCopy.tsx");
  let draft = { session_id: scene, title: "Original session", version: 1 };
  const h = f.mount(() => SessionCopy({ locale: "en", store, draft, boundary: "session-one", disabled: false, refresh: () => {}, navigationGuard: { current: () => true } }));
  t.after(f.stop); await flush();
  draft = { ...draft, title: "Renamed in Studio", version: 2 }; h.render(); await flush();
  element(h.view, (n) => n.props?.["data-testid"] === "live-copy-session").props.onClick(); await flush();
  assert.equal(element(h.view, (n) => n.type === "input").props.value, "Renamed in Studio");
  element(h.view, (n) => n.type === "input").props.onChange({ target: { value: "My edited copy" } }); await flush();
  draft = { ...draft, title: "Refreshed while open", version: 3 }; h.render(); await flush();
  assert.equal(element(h.view, (n) => n.type === "input").props.value, "My edited copy");
  element(h.view, (n) => n.type === "button" && n.props.children === "Cancel").props.onClick(); await flush();
  element(h.view, (n) => n.props?.["data-testid"] === "live-copy-session").props.onClick(); await flush();
  assert.equal(element(h.view, (n) => n.type === "input").props.value, "Refreshed while open");
  assert.equal(f.calls.length, 0);
});

test("SessionCopy submit uses latest version; UNKNOWN request survives later draft/input changes", async (t) => {
  const f = fixture(), { SessionCopy } = f.load("apps/admin/src/features/live/SessionCopy.tsx");
  f.setExecute(async () => { throw new f.studio.StudioError("uncertain"); });
  let draft = { session_id: scene, title: "Source", version: 1 };
  const h = f.mount(() => SessionCopy({ locale: "en", store, draft, boundary: "session-one", disabled: false, refresh: () => {}, navigationGuard: { current: () => true } }));
  t.after(f.stop); await flush();
  element(h.view, (n) => n.props?.["data-testid"] === "live-copy-session").props.onClick(); await flush();
  element(h.view, (n) => n.type === "input").props.onChange({ target: { value: "  Copy edited by merchant  " } }); await flush();
  draft = { ...draft, title: "Newer source", version: 2 }; h.render(); await flush();
  element(h.view, (n) => n.type === "form").props.onSubmit({ preventDefault: () => {} }); await flush();
  assert.equal(f.calls.length, 1); assert.equal(JSON.parse(f.calls[0].body).title, "Copy edited by merchant"); assert.equal(JSON.parse(f.calls[0].body).expected_version, 2);
  const saved = f.saved.get(`live-workspace-command:${scope}`), key = f.calls[0].key;
  draft = { ...draft, title: "Refreshed after UNKNOWN", version: 3 }; h.render(); await flush();
  element(h.view, (n) => n.type === "input").props.onChange({ target: { value: "Another input" } }); await flush();
  assert.equal(f.saved.get(`live-workspace-command:${scope}`), saved);
  element(h.view, (n) => n.type === "button" && n.props.children === "Retry the same request").props.onClick(); await flush();
  assert.equal(f.calls.length, 2); assert.equal(f.calls[1].key, key); assert.equal(f.calls[1].body, f.calls[0].body);
});
