// Purpose: dead-Retry regression for CartProvider (PR #1 review comment 4212512400). When the
// first cart write's response is lost and there was no prior session context, the obtained
// purchase context must already be published so Retry resumes the SAME journalled request
// (same context + Idempotency-Key), never a second purchase.
// Depends on: the REAL CartProvider.tsx executed in a synthetic-hooks VM (real hooks order, jsx
// stub), the REAL lib/purchase journal + lib/buyer-client session handshake with only
// globalThis.fetch faked, in-memory Storage and navigator.locks stubs (same seams as
// buyer-client.test.mjs / purchase.test.mjs).
// Used by: scripts/dev/test-node.sh (apps/storefront/tests/*.test.mjs). MOCK evidence only; the
// Playwright storefront gates remain the UI authority.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { pendingPurchase } from "../lib/purchase.ts";
import * as realBuyerClient from "../lib/buyer-client.ts";
import * as realPurchase from "../lib/purchase.ts";
import * as realCartState from "../lib/cart-state.ts";

const ctx = "a".repeat(43);
const sku = "00000000-0000-0000-0000-000000000001";
const other = "00000000-0000-0000-0000-000000000002";
const cart = {
  id: "00000000-0000-0000-0000-000000000003",
  currency: "TWD",
  version: 8,
  items: [
    { sku_id: other, quantity: 4 },
    { sku_id: sku, quantity: 1 },
  ],
};
const expiry = new Date(Date.now() + 3600_000).toISOString();
const tick = () => new Promise((resolve) => setImmediate(resolve));

function browser() {
  const data = new Map();
  const storage = {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => {
      data.set(key, String(value));
    },
    removeItem: (key) => {
      data.delete(key);
    },
  };
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage: storage,
      addEventListener() {},
      removeEventListener() {},
    },
  });
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: storage });
  Object.defineProperty(globalThis, "sessionStorage", { configurable: true, value: storage });
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: { locks: { request: (...args) => args.at(-1)() } },
  });
  return { data, storage };
}

/** Execute the REAL CartProvider.tsx with synthetic hooks; render() re-runs it and replays effects once. */
function harness() {
  const source = readFileSync(new URL("../components/CartProvider.tsx", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2023,
      module: ts.ModuleKind.CommonJS,
      jsx: ts.JsxEmit.ReactJSX,
      fileName: "CartProvider.tsx",
    },
  }).outputText;
  const slots = [];
  const effects = [];
  let cursor = 0;
  const react = {
    createContext: () => ({ Provider: "CartProvider.Ctx.Provider" }),
    useContext: () => null,
    useState(initial) {
      const index = cursor++;
      slots[index] ??= { value: initial };
      return [
        slots[index].value,
        (value) => {
          slots[index].value = typeof value === "function" ? value(slots[index].value) : value;
        },
      ];
    },
    useRef(initial) {
      const index = cursor++;
      return (slots[index] ??= { current: initial });
    },
    useEffect(effect) {
      const index = cursor++;
      if (!slots[index]) {
        slots[index] = {};
        effects.push(() => {
          effect();
        });
      }
    },
    useCallback: (fn) => fn,
    useMemo: (fn) => fn(),
  };
  const jsx = (type, props) => ({ type, props });
  const module = { exports: {} };
  runInNewContext(compiled, {
    module,
    exports: module.exports,
    window: globalThis.window,
    require: (name) => {
      if (name === "react") return react;
      if (name === "react/jsx-runtime") return { jsx, jsxs: jsx, Fragment: "Fragment" };
      if (name === "../lib/buyer-client") return realBuyerClient;
      if (name === "../lib/purchase") return realPurchase;
      if (name === "../lib/cart-state") return realCartState;
      throw new Error(`unexpected import ${name}`);
    },
  });
  let tree;
  function render() {
    cursor = 0;
    effects.length = 0;
    tree = module.exports.default({ children: null });
    for (const effect of effects.splice(0)) effect();
    return tree;
  }
  /** Latest rendered CartApi: state reads and callbacks both come from the real component. */
  const api = () => tree.props.value;
  return { render, api };
}

test("lost first write keeps Retry alive: same context, same idempotency key, no second purchase", async () => {
  browser();
  const oldFetch = globalThis.fetch;
  let session = { state: "absent", context: null, expires_at: null };
  const puts = [];
  globalThis.fetch = async (url, init = {}) => {
    if (url === "/api/buyer/session" && init.method === "GET")
      return Response.json(session);
    if (url === "/api/buyer/session/prepare") {
      session = { state: "inactive", context: ctx, expires_at: expiry };
      return Response.json(session);
    }
    if (url === "/api/buyer/session/activate") {
      session = { state: "active", context: ctx, expires_at: expiry };
      return Response.json(session);
    }
    if (url === "/api/buyer/cart" && init.method === "PUT") {
      puts.push({
        body: init.body,
        key: new Headers(init.headers).get("Idempotency-Key"),
      });
      // The first write's response is lost: the journal keeps the request, the UI goes "uncertain".
      if (puts.length === 1) throw new Error("synthetic connection loss");
      return Response.json({ ...cart, version: 9 });
    }
    if (url === "/api/buyer/cart") return Response.json({ ...cart, version: 10 });
    throw new Error(`unexpected ${url}`);
  };
  try {
    const { render, api } = harness();
    // State lives in hook slots; re-render after each awaited step so the memoized api value
    // reflects the latest setState (same reason a mounted tree re-renders in the browser).
    const settle = async (rounds = 4) => {
      for (let i = 0; i < rounds; i++) await tick();
      render();
    };
    render();
    await settle();
    assert.equal(api().ready, true);
    assert.equal(api().context, "", "no prior session context before the first add");

    const added = await api().add(sku, 2);
    await settle();
    assert.equal(added, false);
    assert.equal(api().problem, "uncertain");
    assert.equal(puts.length, 1, "the first write was attempted once");
    assert.equal(pendingPurchase(ctx)?.kind, "cart", "the journal holds the lost write");

    // The fix (comment 4212512400): the obtained context is published before the write, so the
    // Retry button is live and resumes the SAME purchase instead of being dead.
    assert.equal(api().context, ctx, "obtained context must be available to the retry");

    await api().retry();
    await settle();

    assert.equal(puts.length, 2, "retry must re-send the journalled write");
    assert.deepEqual(puts[1], puts[0], "retry replays the exact same key and body (no second purchase)");
    assert.equal(api().problem, null);
    assert.equal(pendingPurchase(ctx), null, "a parsed receipt clears the journal");
    assert.equal(api().cart.version, 10, "buyer sees the current cart, not the lost response");
  } finally {
    globalThis.fetch = oldFetch;
    delete globalThis.window;
    delete globalThis.localStorage;
    delete globalThis.sessionStorage;
    delete globalThis.navigator;
  }
});
