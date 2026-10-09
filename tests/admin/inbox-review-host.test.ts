// Purpose: independent PR8 counterexamples execute actual inbox hook/component handlers and render state.
// Depends on: Node registerHooks/test/assert, installed typescript-api, real InboxFence/ReplyReceipt/client.
// Used by: LC-U2b focused Node gate; MOCK hook host is not React/browser or backend acceptance.
// Invariants: I02, I06, I11, I18; no production privacy rule is copied or replaced here.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks, createRequire } from "node:module";
import { readFileSync, existsSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import { resolve, dirname } from "node:path";
const require = createRequire(import.meta.url);
const ts = require("typescript-api");
const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
type VNode = { type: unknown; props: Record<string, any> };
type Slot = { value?: any; deps?: unknown[]; cleanup?: () => void };
const CONTEXT = Symbol.for("react.context"), PORTAL = Symbol.for("react.portal");
type ContextValue = { $$typeof: symbol; defaultValue: unknown; Provider: ContextValue };
type Contexts = Map<ContextValue, unknown>;
const sameContexts = (a: Contexts, b: Contexts) => a.size === b.size && [...a].every(([key,value]) => b.has(key) && Object.is(value,b.get(key)));
let active: Host;
const same = (a?: unknown[], b?: unknown[]) =>
  !!a && !!b && a.length === b.length && a.every((v, i) => Object.is(v, b[i]));
// A generic synchronous hook scheduler: it only implements hook storage/dependency ordering.
// Privacy, fencing, submission receipts and all component state transitions remain real source.
class Host {
  raw: any;
  children = new Map<string, { host: Host; props: any }>();
  slots: Slot[] = [];
  cursor = 0;
  dirty = true;
  effects: (() => void)[] = [];
  output: any;
  render: () => any;
  contexts: Contexts = new Map();
  private portalIds = new WeakMap<object, number>();
  private nextPortalId = 0;
  private observers = new Set<() => void>();
  private commitQueued = false;
  private disposed = false;
  constructor(render: () => any) {
    this.render = render;
  }
  flush() {
    let renders = 0;
    while (this.dirty) {
      assert.ok(++renders < 50, "hook host render loop");
      this.dirty = false;
      this.cursor = 0;
      const previous = active;
      active = this;
      try { this.raw = this.render(); } finally { active = previous; }
      const effects = this.effects.splice(0);
      for (const effect of effects) effect();
    }
    const seen = new Set<string>();
    const expand = (value: any, path: string, contexts: Contexts): any => {
      if (Array.isArray(value)) return value.map((item, i) => expand(item, `${path}.${i}`, contexts));
      if (value?.$$typeof === PORTAL) {
        let target = this.portalIds.get(value.containerInfo);
        if (target === undefined) { target = ++this.nextPortalId; this.portalIds.set(value.containerInfo,target); }
        // Portals change physical ownership, not logical context. Target/key changes remount
        // children; the browser gate, not this VNode host, owns DOM layout/event bubbling.
        const children = expand(value.children, `${path}.portal:${JSON.stringify([target,value.key])}`, contexts);
        return { ...value, type: PORTAL, props: { children } };
      }
      if (!value || typeof value !== "object" || !value.props) return value;
      if (value.type?.$$typeof === CONTEXT) {
        // Per-subtree maps preserve nested shadowing, explicit undefined and sibling/root isolation.
        const nested = new Map(contexts); nested.set(value.type,value.props.value);
        return expand(value.props.children, `${path}.provider`, nested);
      }
      if (typeof value.type === "function") {
        const key = `${path}:${value.type.name}:${value.props.key ?? ""}`;
        seen.add(key);
        let child = this.children.get(key);
        if (!child) {
          child = { host: new Host(() => value.type(value.props)), props: value.props };
          this.children.set(key, child);
        } else if (child.props !== value.props) {
          child.props = value.props;
          child.host.render = () => value.type(value.props);
          child.host.dirty = true;
        }
        if (!sameContexts(child.host.contexts, contexts)) {
          child.host.contexts = new Map(contexts);
          child.host.dirty = true;
        }
        return child.host.flush();
      }
      return { ...value, props: { ...value.props, children: expand(value.props.children, `${path}.c`, contexts) } };
    };
    this.output = expand(this.raw, "root", this.contexts);
    for (const [key, child] of this.children)
      if (!seen.has(key)) {
        child.host.dispose();
        this.children.delete(key);
      }
    return this.output;
  }
  async settle() {
    // WebCrypto session fences and Promise-finally reads yield to the event loop.
    for (let i = 0; i < 12; i++) {
      this.flush();
      await new Promise<void>((done) => setImmediate(done));
    }
    this.flush();
  }
  // Batch the same JS turn: send.finally briefly clears busy before starting its authority reload.
  changed() {
    if (!this.observers.size || this.commitQueued) return;
    this.commitQueued = true;
    queueMicrotask(() => {
      this.commitQueued = false;
      for (const observer of [...this.observers]) observer();
    });
  }
  /** Wait for this root host's actual state commits, with a bounded failure; no guessed event-loop turns. */
  waitFor(ready: () => boolean, description: string, timeoutMs = 5000): Promise<void> {
    return new Promise((resolve, reject) => {
      const finish = (error?: unknown) => {
        clearTimeout(timer);
        this.observers.delete(inspect);
        if (error) reject(error); else resolve();
      };
      const inspect = () => {
        try {
          if (this.disposed) throw new Error(`Host disposed before ${description}`);
          this.flush();
          if (ready()) finish();
        } catch (error) { finish(error); }
      };
      const timer = setTimeout(() => finish(new Error(`Timed out waiting for ${description}`)), timeoutMs);
      this.observers.add(inspect);
      inspect();
    });
  }
  ref<T>(klass: new (...args: any[]) => T): T {
    const found = this.slots.find((slot) => slot.value?.current instanceof klass);
    assert.ok(found, "real production ref exists");
    return found.value.current;
  }
  dispose() {
    this.disposed = true;
    for (const observer of [...this.observers]) observer();
    for (const child of this.children.values()) child.host.dispose();
    for (const slot of this.slots) slot.cleanup?.();
  }
}
const react = {
  createContext(defaultValue: unknown): ContextValue {
    const context = { $$typeof: CONTEXT, defaultValue } as ContextValue;
    context.Provider = context; // React19's context provider and legacy .Provider are equivalent.
    return context;
  },
  useContext(context: ContextValue) {
    if (!active) throw new Error("useContext requires an active render");
    if (context?.$$typeof !== CONTEXT) throw new Error("useContext requires a context");
    return active.contexts.has(context) ? active.contexts.get(context) : context.defaultValue;
  },
  useState(initial: any) {
    const host = active,
      index = host.cursor++;
    host.slots[index] ??= { value: typeof initial === "function" ? initial() : initial };
    return [
      host.slots[index].value,
      (next: any) => {
        const slot = host.slots[index];
        const value = typeof next === "function" ? next(slot.value) : next;
        if (!Object.is(value, slot.value)) {
          slot.value = value;
          host.dirty = true;
          host.changed();
        }
      },
    ];
  },
  useRef(initial: any) {
    const host = active,
      index = host.cursor++;
    host.slots[index] ??= { value: { current: initial } };
    return host.slots[index].value;
  },
  useCallback(callback: any, deps: unknown[]) {
    const host = active,
      index = host.cursor++;
    if (!same(host.slots[index]?.deps, deps)) host.slots[index] = { value: callback, deps };
    return host.slots[index].value;
  },
  useEffect(effect: () => any, deps: unknown[]) {
    const host = active,
      index = host.cursor++;
    const prior = host.slots[index];
    if (same(prior?.deps, deps)) return;
    host.slots[index] = { deps, cleanup: prior?.cleanup };
    host.effects.push(() => {
      prior?.cleanup?.();
      host.slots[index].cleanup = effect();
    });
  },
};
/** Model React's portal descriptor and target validation; rendering stays in its logical Host tree. */
function createPortal(children: any, containerInfo: any, key: unknown = null) {
  if (!containerInfo || ![1,9,11].includes(containerInfo.nodeType)) throw new Error("Target container is not a DOM element");
  return { $$typeof: PORTAL, children, containerInfo, key: key == null ? null : "" + (key as string) };
}
const defaultHealth = {
  pages: [
    { capabilities: [{ binding_id: "MOCK_BINDING", provider: "facebook", capability: "dm_session", state: "ok" }] },
  ],
};
const runtime = {
  ...react,
  createPortal,
  health: defaultHealth,
  jsx: (type: unknown, props: any, key?: string): VNode => ({ type, props: { ...props, key } }),
  Fragment: Symbol("fragment"),
};
/** Supply explicit MOCK B1 facts; actual Thread capability decisions remain production code. */
export function setHealth(health: any) {
  runtime.health = health;
}
(globalThis as any).__inboxReviewRuntime = runtime;
const mock = (code: string) => `data:text/javascript,${encodeURIComponent(code)}`;
const runtimeURL = mock(`const r=globalThis.__inboxReviewRuntime;
export const {createContext,useContext,useState,useRef,useCallback,useEffect,jsx,Fragment}=r;export const jsxs=jsx;`);
const adapters: Record<string, string> = {
  react: runtimeURL,
  "react/jsx-runtime": runtimeURL,
  "react-dom": mock("export const flushSync=callback=>callback(); export const createPortal=(...args)=>globalThis.__inboxReviewRuntime.createPortal(...args);"),
  "next/navigation": mock('export const useParams=()=>({locale:"en"});'),
  "@live-commerce/format": mock(
    "export const displayTime=(_,value)=>value; export const money=(_,currency,value)=>`${currency} ${value}`;",
  ),
  "@/lib/meta-health-hook": mock("export const useMetaHealth=()=>({data:globalThis.__inboxReviewRuntime.health});"),
  "./WorkspaceFrame": mock("export const WorkspaceFrame=({children})=>children;"),
};
registerHooks({
  resolve(specifier, context, next) {
    if (adapters[specifier]) return { url: adapters[specifier], shortCircuit: true };
    if (specifier.endsWith(".css")) return { url: mock("export default {};"), shortCircuit: true };
    let file: string | undefined;
    if (specifier.startsWith("@/")) file = resolve(root, "apps/admin", specifier.slice(2));
    else if (specifier.startsWith(".") && context.parentURL?.startsWith("file:")) {
      file = resolve(dirname(fileURLToPath(context.parentURL)), specifier);
    }
    if (file) {
      for (const suffix of ["", ".ts", ".tsx"]) {
        if (existsSync(file + suffix)) return { url: pathToFileURL(file + suffix).href, shortCircuit: true };
      }
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.startsWith("file:") && /\.tsx?$/.test(url)) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(fileURLToPath(url), "utf8"), {
          compilerOptions: {
            target: ts.ScriptTarget.ES2023,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
          fileName: fileURLToPath(url),
        }).outputText,
      };
    }
    return next(url, context);
  },
});
const { useInboxPrivacy } = await import("../../apps/admin/src/features/messages/use-privacy.ts");
const { InboxFence, ReplyReceipt } = await import("../../apps/admin/src/features/messages/privacy.ts");
const { InboxThread } = await import("../../apps/admin/components/InboxThread.tsx");
const { BuyerPanel } = await import("../../apps/admin/components/BuyerPanel.tsx");
const { InboxError } = await import("../../apps/admin/lib/inbox-client.ts");
class MockDocument extends EventTarget {
  visibilityState = "visible";
  cookie = `__Host-commerce_csrf=${"a".repeat(43)}`;
}
function environment(t: any) {
  runtime.health = defaultHealth;
  const previous = {
    document: (globalThis as any).document,
    window: (globalThis as any).window,
    fetch: globalThis.fetch,
    BroadcastChannel: globalThis.BroadcastChannel,
  };
  const document = new MockDocument(),
    window = new EventTarget();
  Object.assign(globalThis, {
    document,
    window,
    BroadcastChannel: class extends EventTarget {
      close() {}
    },
  });
  const hosts: Host[] = [];
  t.after(() => {
    for (const host of hosts) host.dispose();
    Object.assign(globalThis, previous);
  });
  return {
    document,
    window,
    mount(render: () => any) {
      const host = new Host(render);
      hosts.push(host);
      host.flush();
      return host;
    },
  };
}
function nodes(tree: any): VNode[] {
  if (Array.isArray(tree)) return tree.flatMap(nodes);
  if (!tree || typeof tree !== "object" || !tree.props) return [];
  return [tree, ...nodes(tree.props.children)];
}
function textOf(tree: any): string {
  if (Array.isArray(tree)) return tree.map(textOf).join(" ");
  if (tree && typeof tree === "object") return textOf(tree.props?.children);
  return tree === null || tree === undefined || typeof tree === "boolean" ? "" : String(tree);
}
function node(host: Host, predicate: (node: VNode) => boolean) {
  const found = nodes(host.flush()).find(predicate);
  assert.ok(found, "actual component VNode found");
  return found;
}
function change(host: Host, id: string, value: string) {
  const input = node(host, (n) => n.props["data-testid"] === id);
  assert.equal(input.props.disabled, false, "actual input is enabled");
  input.props.onChange({ target: { value } });
  host.flush();
}
function submit(host: Host) {
  assert.equal(node(host, (n) => n.props["data-testid"] === "reply-send").props.disabled, false);
  node(host, (n) => n.type === "form").props.onSubmit({ preventDefault() {} });
  host.flush();
}
function clickText(host: Host, label: string) {
  const button = node(host, (n) => n.type === "button" && textOf(n) === label);
  assert.equal(button.props.disabled, false, "actual button is enabled");
  button.props.onClick();
  host.flush();
}
const store = {
  id: "10000000-0000-4000-8000-000000000001",
  name: "MOCK_STORE",
  currency: "TWD",
  role: "owner",
};
const conversation = {
  conversation_id: "20000000-0000-4000-8000-000000000002",
  platform: "messenger",
  display_name: "MOCK_NAME",
  last_at: "2030-01-01T00:00:00Z",
  unread: true,
  unreplied: true,
  mode: "auto",
  assignee: null,
  window_open_until: "2099-01-01T00:00:00Z",
  linked_customer_id: null,
};
const thread = {
  has_unknown_outbound: false,
  items: [
    {
      direction: "in",
      seq: 1,
      at: "2030-01-01T00:00:00Z",
      text: "MOCK_DM",
      attachments: [],
    },
  ],
  window_open_until: "2099-01-01T00:00:00Z",
  mode: "auto",
  takeover_generation: 7,
  human_until: null,
};
const response = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
    },
  });
function transport(read: () => any, send?: () => any) {
  const calls: {
    path: string;
    method: string;
    body?: string;
    key: string | null;
  }[] = [];
  globalThis.fetch = async (input, init) => {
    const path = String(input),
      method = init?.method ?? "GET";
    calls.push({
      path,
      method,
      body: init?.body as string | undefined,
      key: new Headers(init?.headers).get("Idempotency-Key"),
    });
    if (path.endsWith("message-templates")) return response({ items: [] });
    if (method === "POST" && path.endsWith("/read")) return response({ read_seq: 1 });
    if (method === "GET" && path.includes("/messages?")) return read();
    if (method === "POST" && path.endsWith("/messages")) return send?.() ?? response({ send_state: "queued" });
    throw new Error(`unexpected MOCK route ${method} ${path}`);
  };
  return calls;
}
function mountThread(env: ReturnType<typeof environment>) {
  return env.mount(() =>
    InboxThread({
      store,
      conversation,
      locale: "en",
      onUnauthorized() {},
      onRead() {},
    } as any),
  );
}

export {
  Host,
  environment,
  nodes,
  textOf,
  node,
  change,
  submit,
  clickText,
  store,
  conversation,
  thread,
  response,
  transport,
  mountThread,
  InboxFence,
  ReplyReceipt,
  InboxError,
  useInboxPrivacy,
  BuyerPanel,
  react,
};
export type { VNode };

// Isolation gates live with the imported host so every existing test-node caller exercises
// its semantics too; no registry change or stand-in product component is needed.
test("Host subset: nested Provider and React19 context shadow defaults without sibling leakage", () => {
  const context = react.createContext("default");
  const Read = () => runtime.jsx("span", { children: String(react.useContext(context)) });
  const h = new Host(() => runtime.jsx("div", { children: [
    runtime.jsx(context.Provider, { value: "outer", children: [runtime.jsx(Read, {}), runtime.jsx(context, { value: "inner", children: runtime.jsx(Read, {}) }), runtime.jsx(Read, {})] }),
    runtime.jsx(Read, {}), runtime.jsx(context.Provider, { value: undefined, children: runtime.jsx(Read, {}) }),
    runtime.jsx(context.Provider, { value: null, children: runtime.jsx(Read, {}) }),
  ] }));
  try { assert.deepEqual(nodes(h.flush()).filter(n=>n.type==="span").map(textOf), ["outer","inner","outer","default","undefined","null"]); }
  finally { h.dispose(); }
});
test("Host subset: provider updates reach stable children and separate roots keep their own values", () => {
  const context = react.createContext("default"); let value = "first", reads = 0;
  const Read = () => { reads++; const [left] = react.useState("L"), v = react.useContext(context), [right] = react.useState("R"); return runtime.jsx("span", { children: `${left}:${v}:${right}` }); };
  const stable = runtime.jsx(Read, {}), a = new Host(() => runtime.jsx(context.Provider, { value, children: stable })), b = new Host(() => runtime.jsx(context, { value:"other", children: stable }));
  try {
    assert.equal(textOf(a.flush()), "L:first:R"); assert.equal(textOf(b.flush()), "L:other:R");
    const before = reads; value = "updated"; a.dirty = true;
    assert.equal(textOf(a.flush()), "L:updated:R"); assert.equal(reads,before+1);
    assert.equal(textOf(b.flush()), "L:other:R");
    a.dirty = true; a.flush(); assert.equal(reads,before+1,"same Object.is value and stable child need no rerender");
  } finally { a.dispose(); b.dispose(); }
});
test("Host subset: render errors restore context and hook ownership", () => {
  const context = react.createContext("default");
  const Bad = () => { assert.equal(react.useContext(context),"private"); throw new Error("synthetic render failure"); };
  const bad = new Host(() => runtime.jsx(context.Provider,{value:"private",children:runtime.jsx(Bad,{})}));
  assert.throws(()=>bad.flush(),/synthetic render failure/);
  assert.throws(()=>react.useContext(context),/render/);
  const good = new Host(()=>runtime.jsx("span",{children:react.useContext(context)}));
  try { assert.equal(textOf(good.flush()),"default"); } finally { bad.dispose();good.dispose(); }
});
test("Host subset: portal children inherit logical context and retain handlers/state", () => {
  const context = react.createContext("default"), target = {nodeType:1}; let value = "outer";
  const Child = () => { const v=react.useContext(context), [count,setCount]=react.useState(0); return runtime.jsx("button",{onClick:()=>setCount((n:number)=>n+1),children:`${v}:${count}`}); };
  const stable = runtime.jsx(Child,{}), h = new Host(()=>runtime.jsx(context.Provider,{value,children:createPortal(stable,target,"preview")}));
  try {
    assert.equal(textOf(h.flush()),"outer:0"); assert.equal(h.output.containerInfo,target); assert.equal(h.output.key,"preview");
    node(h,n=>n.type==="button").props.onClick();assert.equal(textOf(h.flush()),"outer:1");
    value="updated";h.dirty=true;assert.equal(textOf(h.flush()),"updated:1");
  } finally { h.dispose(); }
});
test("Host subset: portal target/key changes remount children and removal runs cleanup", () => {
  let target={nodeType:1}, key="one", visible=true, mounts=0, cleanups=0;
  const Child=()=>{const [id]=react.useState(()=>++mounts);react.useEffect(()=>()=>{cleanups++;},[]);return runtime.jsx("span",{children:id});};
  const stable=runtime.jsx(Child,{}), h=new Host(()=>visible?createPortal(stable,target,key):null);
  try {
    assert.equal(textOf(h.flush()),"1");h.dirty=true;assert.equal(textOf(h.flush()),"1");assert.equal(cleanups,0);
    key="two";h.dirty=true;assert.equal(textOf(h.flush()),"2");assert.equal(cleanups,1);
    target={nodeType:1};h.dirty=true;assert.equal(textOf(h.flush()),"3");assert.equal(cleanups,2);
    visible=false;h.dirty=true;assert.equal(h.flush(),null);assert.equal(cleanups,3);
  } finally { h.dispose(); }
});
test("Host subset: createPortal rejects invalid DOM targets rather than swallowing children", () => {
  for(const target of [null,undefined,{},"body",{nodeType:3}])assert.throws(()=>createPortal("text",target),/Target container/);
  for(const nodeType of [1,9,11])assert.equal(createPortal("text",{nodeType}).children,"text");
  assert.equal(createPortal("text",{nodeType:1},0).key,"0");
  assert.throws(()=>createPortal("text",{nodeType:1},Symbol("unsupported")),TypeError);
});
