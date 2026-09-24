import test from "node:test";
import assert from "node:assert/strict";
import {
  BuyerClientError,
  buyerRequest,
  initializeBuyerSession,
  readBuyerSession,
  resetBuyerSession,
  logoutBuyerSession,
} from "../lib/buyer-client.ts";

const first = Buffer.alloc(32, 3).toString("base64url");
const second = Buffer.alloc(32, 4).toString("base64url");
const expiry = new Date(Date.now() + 3600_000).toISOString();
const absent = { state: "absent", context: null, expires_at: null };
const inactive = (context) => ({
  state: "inactive",
  context,
  expires_at: expiry,
});
const active = (context) => ({ state: "active", context, expires_at: expiry });

function browser() {
  const data = new Map();
  const localStorage = {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => {
      data.set(key, value);
    },
    removeItem: (key) => {
      data.delete(key);
    },
  };
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: { localStorage },
  });
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: {
      locks: { request: async (_name, _options, callback) => callback() },
    },
  });
  return { data, localStorage };
}

function safeError(status, code) {
  return Response.json(
    {
      code,
      message: "Request failed.",
      request_id: "a".repeat(32),
      retryable: status === 503 || status === 429,
      details: {},
    },
    {
      status,
      headers: {
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
      },
    },
  );
}

async function rejectsCode(promise, code) {
  await assert.rejects(
    promise,
    (error) => error instanceof BuyerClientError && error.code === code,
  );
}

test("uncertain prepare blocks all mutations; changed cookie context resolves journal", async () => {
  const { data } = browser();
  const old = globalThis.fetch;
  let current = absent;
  let prepares = 0;
  globalThis.fetch = async (url, options) => {
    if (url === "/api/buyer/session" && options.method === "GET")
      return Response.json(current);
    if (url === "/api/buyer/session/prepare") {
      prepares++;
      throw new Error("response lost");
    }
    if (url === "/api/buyer/session/activate") {
      current = active(first);
      return Response.json(current);
    }
    throw new Error(`unexpected ${url}`);
  };
  try {
    await rejectsCode(initializeBuyerSession(), "uncertain");
    assert.equal(prepares, 1);
    assert.ok(data.has("commerce-buyer-pending-v1"));
    assert.equal(data.get("commerce-buyer-pending-v1").includes(first), false);
    await rejectsCode(initializeBuyerSession(), "uncertain");
    await rejectsCode(
      buyerRequest("PUT", "cart", first, { items: [] }, "validkey1"),
      "uncertain",
    );
    assert.equal(prepares, 1);
    current = inactive(first);
    assert.deepEqual(await readBuyerSession(), current);
    assert.equal(data.has("commerce-buyer-pending-v1"), false);
    assert.deepEqual(await initializeBuyerSession(), active(first));
  } finally {
    globalThis.fetch = old;
  }
});

test("fully parsed local denial clears only its own pending operation", async () => {
  const { data } = browser();
  const old = globalThis.fetch;
  let posts = 0;
  globalThis.fetch = async (url) => {
    if (url === "/api/buyer/session") return Response.json(absent);
    if (url === "/api/buyer/session/prepare") {
      posts++;
      return safeError(429, "rate_limited");
    }
    throw new Error(`unexpected ${url}`);
  };
  try {
    await rejectsCode(initializeBuyerSession(), "request_failed");
    assert.equal(data.has("commerce-buyer-pending-v1"), false);
    await rejectsCode(initializeBuyerSession(), "request_failed");
    assert.equal(posts, 2);
  } finally {
    globalThis.fetch = old;
  }
});

test("malformed journal, unavailable lock and stale context fail before writes", async () => {
  const { data } = browser();
  const old = globalThis.fetch;
  let writes = 0;
  globalThis.fetch = async (url) => {
    if (url === "/api/buyer/session") return Response.json(inactive(first));
    writes++;
    throw new Error("must not write");
  };
  try {
    data.set("commerce-buyer-pending-v1", "not JSON");
    await rejectsCode(resetBuyerSession(first), "uncertain");
    await rejectsCode(logoutBuyerSession(first), "uncertain");
    await rejectsCode(
      buyerRequest("POST", "quotes", first, {}, "validkey1"),
      "uncertain",
    );
    assert.equal(writes, 0);
    data.delete("commerce-buyer-pending-v1");
    await rejectsCode(resetBuyerSession(second), "context_changed");
    assert.equal(writes, 0);
    delete globalThis.navigator.locks;
    await rejectsCode(initializeBuyerSession(), "unavailable");
    assert.equal(writes, 0);
  } finally {
    globalThis.fetch = old;
  }
});
