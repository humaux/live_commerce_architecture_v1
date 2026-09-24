import test from "node:test";
import assert from "node:assert/strict";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const bff = Buffer.alloc(32, 1).toString("base64url");
const signing = Buffer.alloc(32, 2).toString("base64url");
const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";

function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = bff;
  process.env.COMMERCE_BUYER_COOKIE_KEY = signing;
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}

function req(method, suffix, { cookie, context, body, extra = {} } = {}) {
  const headers = { Host: "shop.example", ...extra };
  if (method !== "GET") headers.Origin = origin;
  if (cookie) headers.Cookie = cookie;
  if (context) headers["X-Buyer-Context"] = context;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  return new Request(`${origin}/api/buyer/${suffix}`, {
    method,
    headers,
    body,
  });
}

function cookie(response) {
  const set = response.headers.get("set-cookie");
  assert.match(set ?? "", /^__Host-commerce_buyer=/);
  assert.match(set, /Path=\/; Max-Age=3600; Secure; HttpOnly; SameSite=Lax/);
  return set.split(";", 1)[0];
}

test("disabled and invalid configuration fail closed before private fetch", async () => {
  const old = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("must not fetch");
  };
  try {
    process.env.COMMERCE_BUYER_WEB_ENABLED = "0";
    let response = await handleBuyerRequest(req("GET", "session"));
    assert.equal(response.status, 404);
    process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
    process.env.COMMERCE_BUYER_COOKIE_KEY = bff;
    response = await handleBuyerRequest(
      req("POST", "session/prepare", { body: "{}" }),
    );
    assert.equal(response.status, 503);
    assert.equal((await response.json()).retryable, false);
    assert.equal(response.headers.get("set-cookie"), null);
  } finally {
    globalThis.fetch = old;
    enabled();
  }
});

test("private authority, cookie lifecycle and strict local denial", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    assert.ok(String(url).startsWith(`${api}/v1/buyer/`));
    assert.equal(options.headers.get("X-Commerce-Storefront-Origin"), origin);
    assert.equal(options.headers.get("X-Commerce-Buyer-BFF-Key"), bff);
    assert.equal(options.redirect, "error");
    if (String(url).endsWith("/session/retire"))
      return new Response(null, { status: 204 });
    if (String(url).endsWith("/session/bootstrap"))
      return Response.json({
        authenticated: true,
        expires_at: new Date(Date.now() + 3500_000).toISOString(),
      });
    if (String(url).endsWith("/session"))
      return Response.json({ authenticated: true });
    return Response.json({
      id: "cart",
      currency: "TWD",
      version: 1,
      items: [],
    });
  };
  try {
    let response = await handleBuyerRequest(req("GET", "session"));
    assert.deepEqual(await response.json(), {
      state: "absent",
      context: null,
      expires_at: null,
    });
    assert.equal(calls.length, 0);

    response = await handleBuyerRequest(
      req("POST", "session/prepare", { body: "{}" }),
    );
    assert.equal(response.status, 200);
    const firstCookie = cookie(response);
    const prepared = await response.json();
    assert.equal(prepared.state, "inactive");
    assert.match(prepared.context, /^[A-Za-z0-9_-]{43}$/);
    assert.equal(JSON.stringify(prepared).includes("token"), false);
    assert.equal(calls.length, 0);

    const stale = await handleBuyerRequest(
      req("PUT", "cart", {
        cookie: firstCookie,
        context: bff,
        body: '{"items":[]}',
        extra: { "Idempotency-Key": "validkey1" },
      }),
    );
    assert.equal(stale.status, 409);
    assert.equal((await stale.json()).code, "context_changed");
    assert.equal(calls.length, 0);

    for (const body of [
      '{"items":null}',
      '{"items":[],"items":[]}',
      '{"items":[{"sku_id":"x","quantity":1,"extra":1}]}',
    ]) {
      const bad = await handleBuyerRequest(
        req("PUT", "cart", {
          cookie: firstCookie,
          context: prepared.context,
          body,
          extra: { "Idempotency-Key": "validkey1" },
        }),
      );
      assert.equal(bad.status, 400);
      assert.equal(calls.length, 0);
    }
    response = await handleBuyerRequest(
      req("POST", "session/activate", {
        cookie: firstCookie,
        context: prepared.context,
        body: "{}",
      }),
    );
    assert.equal(response.status, 200);
    assert.equal((await response.json()).state, "active");
    assert.equal(response.headers.get("set-cookie"), null);

    response = await handleBuyerRequest(
      req("GET", "cart", { cookie: firstCookie, context: prepared.context }),
    );
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), {
      id: "cart",
      currency: "TWD",
      version: 1,
      items: [],
    });
    assert.equal(response.headers.get("set-cookie"), null);

    response = await handleBuyerRequest(
      req("POST", "session/reset", {
        cookie: firstCookie,
        context: prepared.context,
        body: "{}",
      }),
    );
    assert.equal(response.status, 200);
    const secondCookie = cookie(response);
    assert.notEqual(secondCookie, firstCookie);
    assert.equal((await response.json()).state, "inactive");
    assert.equal(
      calls.filter((call) => String(call.url).endsWith("/session/retire"))
        .length,
      1,
    );
  } finally {
    globalThis.fetch = old;
  }
});

test("bad host, cookie, query and method never reach private API", async () => {
  enabled();
  const old = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("must not fetch");
  };
  try {
    for (const host of [
      "SHOP.example",
      "shop.example:443",
      "127.0.0.1",
      "shop.example,other.example",
      "shop.example.",
    ]) {
      const response = await handleBuyerRequest(
        req("POST", "session/prepare", { body: "{}", extra: { Host: host } }),
      );
      assert.equal(response.status, 403, host);
      assert.equal(response.headers.get("set-cookie"), null);
    }
    let response = await handleBuyerRequest(
      req("POST", "session/prepare", {
        body: "{}",
        extra: { Authorization: `Bearer ${bff}` },
      }),
    );
    assert.equal(response.status, 403);
    response = await handleBuyerRequest(
      req("GET", "session", { cookie: "__Host-commerce_buyer=bad" }),
    );
    assert.equal(response.status, 401);
    response = await handleBuyerRequest(
      req("GET", "session", {
        cookie: "__Host-commerce_buyer=bad; __Host-commerce_buyer=bad",
      }),
    );
    assert.equal(response.status, 401);
    response = await handleBuyerRequest(req("GET", "session?"));
    assert.equal(response.status, 422);
    response = await handleBuyerRequest(req("GET", "catalog?limit=1&limit=2"));
    assert.equal(response.status, 422);
    response = await handleBuyerRequest(req("HEAD", "session"));
    assert.equal(response.status, 405);
    response = await handleBuyerRequest(req("GET", "quotes/not-a-uuid"));
    assert.equal(response.status, 422);
  } finally {
    globalThis.fetch = old;
  }
});

test("stalled inbound POST body is canceled at its deadline without a cookie or private call", async (t) => {
  enabled();
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const old = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("must not fetch");
  };
  let canceled = false;
  const stalled = new ReadableStream({
    pull() {
      return new Promise(() => {});
    },
    cancel() {
      canceled = true;
    },
  });
  try {
    const request = new Request(`${origin}/api/buyer/session/prepare`, {
      method: "POST",
      duplex: "half",
      body: stalled,
      headers: {
        Host: "shop.example",
        Origin: origin,
        "Content-Type": "application/json",
      },
    });
    const pending = handleBuyerRequest(request);
    t.mock.timers.tick(12_000);
    const response = await pending;
    assert.equal(response.status, 503);
    assert.equal((await response.json()).retryable, false);
    assert.equal(response.headers.get("set-cookie"), null);
    assert.equal(canceled, true);
  } finally {
    globalThis.fetch = old;
    t.mock.timers.reset();
  }
});
