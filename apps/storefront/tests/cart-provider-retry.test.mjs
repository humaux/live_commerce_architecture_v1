// Purpose: dead-Retry regression for CartProvider (PR #1 review comment 4212512400). When the
// first cart write's response is lost and there was no prior session context, the obtained
// purchase context must already be published so Retry resumes the SAME journalled request
// (same context + Idempotency-Key), never a second purchase.
// Also pins refresh observation ordering: a failed later session read cannot erase an earlier valid cart read.
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

for (const failure of ["transport", "503"]) {
  test(`failed later ${failure} session refresh keeps an earlier observed cart eligible`, async () => {
    browser();
    const originalFetch = globalThis.fetch;
    const initial = Promise.withResolvers(), entered = Promise.withResolvers(), release = Promise.withResolvers();
    let sessionReads = 0, failNext = false, h;
    const sessionA = { state: "active", context: ctx, expires_at: expiry };
    globalThis.fetch = async (url, init = {}) => {
      if (url === "/api/buyer/session") {
        if (++sessionReads === 1) return initial.promise; // Hold only the mount read; exercise refresh through its exact promise.
        if (failNext) {
          failNext = false;
          if (failure === "transport") throw new Error("synthetic transient session failure");
          return Response.json({ code: "unavailable" }, { status: 503 });
        }
        return Response.json(sessionA);
      }
      if (url === "/api/buyer/cart" && init.method !== "PUT") {
        assert.equal(new Headers(init.headers).get("X-Buyer-Context"), ctx);
        entered.resolve();
        await release.promise;
        return Response.json(cart);
      }
      throw new Error("unexpected MOCK route");
    };
    try {
      h = harness(); h.render();
      const earlier = h.api().refresh();
      await entered.promise; h.render();
      assert.equal(h.api().context, ctx);
      assert.equal(h.api().cart, null); assert.equal(h.api().ready, false);
      failNext = true;
      await h.api().refresh(); // The same refresh invoked by focus/storage; await its actual completion.
      release.resolve();
      await earlier; h.render();
      assert.equal(h.api().context, ctx);
      assert.equal(h.api().cart?.id, cart.id, "failed later session read must not discard the valid earlier cart");
      assert.equal(h.api().cart.version, 8); assert.equal(h.api().count, 5);
      assert.equal(h.api().ready, true);
    } finally {
      h?.unmount(); release.resolve(); initial.resolve(Response.json(sessionA));
      globalThis.fetch = originalFetch;
      for (const name of ["window", "localStorage", "sessionStorage", "navigator"]) delete globalThis[name];
    }
  });
}

for (const result of ["cart", "cart-fails", "absent"]) {
  test(`newer successful session supersedes a delayed valid A cart on ${result}`, async () => {
    browser();
    const originalFetch = globalThis.fetch, contextB = "b".repeat(43);
    const initial = Promise.withResolvers(), entered = Promise.withResolvers(), validationEntered = Promise.withResolvers();
    const cartRelease = Promise.withResolvers(), validationRelease = Promise.withResolvers();
    const sessionA = { state: "active", context: ctx, expires_at: expiry };
    const sessionB = { ...sessionA, context: contextB };
    const cartB = { ...cart, id: "00000000-0000-0000-0000-000000000004", version: 20, items: [{ sku_id: sku, quantity: 2 }] };
    let sessionReads = 0, session = sessionA, holdValidation = false, h;
    globalThis.fetch = async (url, init = {}) => {
      if (url === "/api/buyer/session") {
        if (++sessionReads === 1) return initial.promise;
        if (holdValidation) {
          holdValidation = false;
          validationEntered.resolve();
          await validationRelease.promise;
          return Response.json(sessionA); // An earlier valid A validation response, delivered after B's newer observation.
        }
        return Response.json(session);
      }
      if (url === "/api/buyer/cart" && init.method !== "PUT") {
        const owner = new Headers(init.headers).get("X-Buyer-Context");
        if (owner === ctx) {
          entered.resolve(); await cartRelease.promise;
          return Response.json(cart);
        }
        assert.equal(owner, contextB);
        return result === "cart" ? Response.json(cartB) : Response.json({ code: "unavailable" }, { status: 503 });
      }
      throw new Error("unexpected MOCK route");
    };
    try {
      h = harness(); h.render();
      const earlier = h.api().refresh();
      await entered.promise;
      holdValidation = true; cartRelease.resolve();
      await validationEntered.promise; // Hold the real readPurchase context validation, so A truly completes successfully later.
      session = result === "absent" ? { state: "absent", context: null, expires_at: null } : sessionB;
      await h.api().refresh(); h.render();
      assert.equal(h.api().context, result === "absent" ? "" : contextB);
      assert.equal(h.api().cart?.id ?? null, result === "cart" ? cartB.id : null);
      validationRelease.resolve();
      await earlier; h.render();
      assert.equal(h.api().context, result === "absent" ? "" : contextB, "old A cannot restore its ownership");
      assert.equal(h.api().cart?.id ?? null, result === "cart" ? cartB.id : null, "A's items cannot appear under B even if B cart read fails");
      assert.equal(h.api().count, result === "cart" ? 2 : 0);
      assert.equal(h.api().ready, true);
    } finally {
      h?.unmount(); cartRelease.resolve(); validationRelease.resolve(); initial.resolve(Response.json(sessionA));
      globalThis.fetch = originalFetch;
      for (const name of ["window", "localStorage", "sessionStorage", "navigator"]) delete globalThis[name];
    }
  });
}

test("a completed same-context command fences an earlier valid cart read", async () => {
  browser();
  const originalFetch = globalThis.fetch;
  const initial = Promise.withResolvers(), entered = Promise.withResolvers(), release = Promise.withResolvers();
  const sessionA = { state: "active", context: ctx, expires_at: expiry };
  let sessionReads = 0, cartReads = 0, current = cart, puts = 0, h;
  globalThis.fetch = async (url, init = {}) => {
    if (url === "/api/buyer/session") return ++sessionReads === 1 ? initial.promise : Response.json(sessionA);
    if (url === "/api/buyer/cart") {
      if (init.method === "PUT") {
        puts++;
        const body = JSON.parse(init.body);
        assert.equal(body.expected_version, 8);
        current = { ...cart, version: 9, items: body.items };
        return Response.json(current);
      }
      if (++cartReads === 1) {
        entered.resolve(); await release.promise;
        return Response.json(cart);
      }
      return Response.json(current);
    }
    throw new Error("unexpected MOCK route");
  };
  try {
    h = harness(); h.render();
    const earlier = h.api().refresh(); await entered.promise;
    assert.equal(await h.api().add(sku, 1), true); h.render();
    assert.equal(puts, 1); assert.equal(h.api().cart.version, 9); assert.equal(h.api().count, 6);
    release.resolve(); await earlier; h.render();
    assert.equal(h.api().cart.version, 9, "pre-command cart cannot overwrite the acknowledged write even with the same context");
    assert.equal(h.api().context, ctx); assert.equal(h.api().count, 6); assert.equal(h.api().ready, true);
  } finally {
    h?.unmount(); release.resolve(); initial.resolve(Response.json(sessionA));
    globalThis.fetch = originalFetch;
    for (const name of ["window", "localStorage", "sessionStorage", "navigator"]) delete globalThis[name];
  }
});

for (const phase of ["session", "cart"]) {
  test(`unmount still fences a delayed ${phase} refresh after separating issued and observed generations`, async () => {
    browser();
    const originalFetch = globalThis.fetch;
    const initial = Promise.withResolvers(), entered = Promise.withResolvers(), release = Promise.withResolvers();
    const sessionA = { state: "active", context: ctx, expires_at: expiry };
    let sessionReads = 0, h;
    globalThis.fetch = async (url) => {
      if (url === "/api/buyer/session") {
        if (++sessionReads === 1) return initial.promise;
        if (phase === "session") { entered.resolve(); await release.promise; }
        return Response.json(sessionA);
      }
      if (url === "/api/buyer/cart") { entered.resolve(); await release.promise; return Response.json(cart); }
      throw new Error("unexpected MOCK route");
    };
    try {
      h = harness(); h.render();
      const earlier = h.api().refresh(); await entered.promise;
      h.unmount(); const writesAtUnmount = h.stateWrites.length;
      release.resolve(); await earlier;
      assert.equal(h.stateWrites.length, writesAtUnmount, "late read must never publish or set ready after actual effect cleanup");
    } finally {
      h?.unmount(); release.resolve(); initial.resolve(Response.json(sessionA));
      globalThis.fetch = originalFetch;
      for (const name of ["window", "localStorage", "sessionStorage", "navigator"]) delete globalThis[name];
    }
  });
}

for (const newer of ["refresh", "mutation"]) {
  test(`round2 stale A session response cannot supersede newer ${newer} B or its lost-write Retry`, async () => {
    browser();
    const originalFetch = globalThis.fetch, contextB = "b".repeat(43);
    const cartB = { ...cart, id: "00000000-0000-0000-0000-000000000004", version: 20, items: [{ sku_id: sku, quantity: 2 }] };
    let session = { state: "active", context: ctx, expires_at: expiry };
    let holdNext = false, releaseOld;
    const puts = [];
    globalThis.fetch = async (url, init = {}) => {
      if (url === "/api/buyer/session") {
        if (holdNext) { holdNext = false; return new Promise(resolve => { releaseOld = resolve; }); }
        return Response.json(session);
      }
      if (String(url).startsWith("/api/buyer/session/")) throw new Error("must not create a replacement session");
      if (url === "/api/buyer/cart" && init.method === "PUT") {
        puts.push({ body: init.body, key: new Headers(init.headers).get("Idempotency-Key"), context: new Headers(init.headers).get("X-Buyer-Context") });
        if (puts.length === 1) throw new Error("synthetic lost B acknowledgement");
        return Response.json({ ...cartB, version: 21 });
      }
      if (url === "/api/buyer/cart") return Response.json(session.context === ctx ? cart : { ...cartB, version: puts.length > 1 ? 22 : 20 });
      throw new Error("unexpected MOCK route");
    };
    try {
      const { render, api } = harness();
      const settle = async () => { for (let i=0;i<4;i++) await tick(); render(); };
      render();await settle();assert.equal(api().cart.id,cart.id);
      holdNext = true;
      const olderRefresh = api().refresh();
      assert.equal(typeof releaseOld,"function","older session read is actually in flight");
      session = { state: "active", context: contextB, expires_at: expiry };
      if (newer === "refresh") {
        await api().refresh();await settle();
        assert.equal(api().context,contextB);assert.equal(api().cart.id,cartB.id);
      } else {
        assert.equal(await api().add(sku,1),false);await settle();
        assert.equal(api().problem,"uncertain");assert.equal(api().context,contextB);
      }
      releaseOld(Response.json({ state:"active", context:ctx, expires_at:expiry }));
      await olderRefresh;await settle();
      assert.equal(api().context,contextB,"late A must not roll back the published context");
      if (newer === "refresh") {
        assert.equal(api().cart.id,cartB.id,"late A must not clear the newer cart");
        assert.equal(await api().add(sku,1),false);await settle();
      }
      assert.equal(api().problem,"uncertain");
      const pending=pendingPurchase(contextB);assert.equal(pending?.kind,"cart");assert.equal(pendingPurchase(ctx),null);
      assert.equal(puts.length,1);assert.equal(puts[0].key,pending.key);assert.equal(puts[0].context,contextB);
      await api().retry();await settle();
      assert.equal(puts.length,2);assert.deepEqual(puts[1],puts[0],"Retry must replay B's same context/key/body");
      assert.equal(api().context,contextB);assert.equal(api().cart.id,cartB.id);assert.equal(api().cart.version,22);
      assert.equal(pendingPurchase(contextB),null);assert.equal(api().problem,null);
    } finally {
      globalThis.fetch=originalFetch;
      for(const name of ["window","localStorage","sessionStorage","navigator"]) delete globalThis[name];
    }
  });
}

for (const failure of ["read", "lost-write", "refresh-read"]) {
  test(`rotated buyer context clears the prior cart on ${failure} and preserves the B receipt`, async () => {
    browser();
    const oldFetch = globalThis.fetch, contextB = "b".repeat(43);
    let session = { state: "active", context: ctx, expires_at: expiry };
    const cartB = { ...cart, id: "00000000-0000-0000-0000-000000000004", version: 20, items: [{ sku_id: sku, quantity: 2 }] };
    const puts = []; let sessionWrites = 0;
    globalThis.fetch = async (url, init = {}) => {
      if (url === "/api/buyer/session") return Response.json(session);
      if (String(url).startsWith("/api/buyer/session/")) { sessionWrites++; throw new Error("must reuse active B"); }
      if (url === "/api/buyer/cart" && init.method === "PUT") {
        puts.push({ body: init.body, key: new Headers(init.headers).get("Idempotency-Key"), context: new Headers(init.headers).get("X-Buyer-Context") });
        if (puts.length === 1) throw new Error("synthetic lost B acknowledgement");
        return Response.json({ ...cartB, version: 21 });
      }
      if (url === "/api/buyer/cart") {
        if (session.context === ctx) return Response.json(cart);
        if (failure !== "lost-write") return Response.json({ code: "unavailable" }, { status: 503 });
        return Response.json({ ...cartB, version: puts.length > 1 ? 22 : 20 });
      }
      throw new Error("unexpected MOCK route");
    };
    try {
      const { render, api } = harness();
      const settle = async () => { for (let n = 0; n < 4; n++) await tick(); render(); };
      render(); await settle();
      assert.equal(api().context, ctx); assert.equal(api().cart.id, cart.id); assert.equal(api().count, 5);
      session = { state: "active", context: contextB, expires_at: expiry };
      if (failure === "refresh-read") await api().refresh(); else assert.equal(await api().add(sku, 1), false);
      await settle();
      assert.equal(api().context, contextB);
      assert.equal(api().cart, null, "cart A must not remain visible after adopting context B");
      assert.equal(api().count, 0);
      assert.equal(pendingPurchase(ctx), null);
      if (failure === "lost-write") {
        assert.equal(api().problem, "uncertain");
        const pending = pendingPurchase(contextB); assert.equal(pending?.kind, "cart");
        assert.equal(puts.length, 1); assert.equal(puts[0].key, pending.key); assert.equal(puts[0].context,contextB);
        await api().retry(); await settle();
        assert.equal(puts.length, 2); assert.deepEqual(puts[1], puts[0], "Retry must keep B's exact key and body");
        assert.equal(pendingPurchase(contextB), null); assert.equal(api().cart.id, cartB.id); assert.equal(api().cart.version, 22);
      } else { assert.equal(puts.length, 0); assert.equal(pendingPurchase(contextB), null); }
      assert.equal(sessionWrites, 0, "never create another buyer session to recover B");
    } finally {
      globalThis.fetch = oldFetch;
      for (const name of ["window", "localStorage", "sessionStorage", "navigator"]) delete globalThis[name];
    }
  });
}

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
  const cleanups = [];
  const stateWrites = [];
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
          stateWrites.push(index);
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
          const cleanup = effect();
          if (typeof cleanup === "function") cleanups.push(cleanup);
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
  const unmount = () => { for (const cleanup of cleanups.splice(0)) cleanup(); };
  return { render, api, unmount, stateWrites };
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
