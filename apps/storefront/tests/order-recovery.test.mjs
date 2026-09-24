import test from "node:test";
import assert from "node:assert/strict";
import {
  validQuote,
  validOption,
  validDestinationWrite,
  validDestination,
  validOrder,
  checkoutInput,
  parsePending,
  pendingPurchase,
  knownOrderID,
  orderRecoveryRequired,
  writePurchase,
  writeDestination,
  writeCheckout,
  currentDestination,
  forgetAddressAttempt,
} from "../lib/purchase.ts";

// Deterministic browser-client tests only. Real Next/Go/PG gates are separate;
// all names/addresses here are synthetic, and no browser storage receives them.
const uid = (n) => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
const context = "a".repeat(43),
  foreignContext = "b".repeat(43);
const until = new Date(Date.now() + 3_600_000).toISOString();
const cart = {
  id: uid(1),
  version: 2,
  currency: "TWD",
  items: [{ sku_id: uid(2), quantity: 1 }],
};
const option = {
  market_id: uid(3),
  country: "TW",
  currency: "TWD",
  method: "delivery:home",
  delivery_kind: "home",
  mode: "MANUAL",
  service_version: 4,
  allocation_version: 5,
  name_hans: "测试",
  name_hant: "測試",
  name_en: "Synthetic",
  sort_order: 1,
};
const quote = {
  id: uid(4),
  cart_id: cart.id,
  cart_version: cart.version,
  market_id: option.market_id,
  country: "TW",
  method: option.method,
  currency: "TWD",
  expires_at: until,
  lines: [
    {
      sku_id: uid(2),
      name: "Synthetic item",
      code: "SYNTH",
      quantity: 1,
      unit_price_minor: 500,
    },
  ],
  amount: {
    subtotal_minor: 500,
    discount_minor: 0,
    shipping_minor: 0,
    shipping_tax_minor: 0,
    tax_minor: 0,
    total_minor: 500,
  },
};
const address = {
  expected_version: 0,
  cart_version: cart.version,
  kind: "home",
  country: "TW",
  recipient_name: "Synthetic Recipient",
  phone: "+886900000001",
  home_address: {
    region: "",
    city: "Synthetic City",
    postal_code: "",
    line1: "Synthetic Street",
    line2: "",
  },
};
function destination(body = address, version = body.expected_version + 1) {
  const { expected_version, ...details } = body;
  return {
    ...details,
    id: uid(100 + version),
    version,
    cart_id: cart.id,
    selected_at: new Date().toISOString(),
    expires_at: until,
  };
}
const head = destination();
const input = {
  quote_id: quote.id,
  destination_id: head.id,
  cart_version: 2,
  service_version: 4,
  allocation_version: 5,
};
function order(state = "DRAFT") {
  return {
    order_id: uid(6),
    commercial_state: state,
    fulfillment_state:
      state === "CANCELLED" ? "CANCELLED" : "MANUAL_UNASSIGNED",
    ...(state === "DRAFT" ? { hold_expires_at: until } : {}),
    snapshot: {
      quote: {
        currency: quote.currency,
        lines: quote.lines,
        amount: quote.amount,
      },
      destination: {
        kind: "home",
        country: "TW",
        recipient_name: address.recipient_name,
        phone: address.phone,
        home_address: address.home_address,
      },
      service: {
        code: "home",
        name_hans: "测试",
        name_hant: "測試",
        name_en: "Synthetic",
        delivery_kind: "home",
        mode: "MANUAL",
      },
    },
  };
}
function denial(status) {
  return Response.json(
    {
      code: "not_found",
      message: "Synthetic denial",
      request_id: "0".repeat(32),
      retryable: false,
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

async function browserFixture(run) {
  const previous = {
    fetch: globalThis.fetch,
    localStorage: globalThis.localStorage,
    sessionStorage: globalThis.sessionStorage,
    window: globalThis.window,
    locks: Object.getOwnPropertyDescriptor(navigator, "locks"),
  };
  const data = new Map(),
    session = new Map();
  const store = (map) => ({
    getItem: (k) => map.get(k) ?? null,
    setItem: (k, v) => {
      // Check every write, not just the final contents after journal removal.
      for (const pii of [
        address.recipient_name,
        address.phone,
        address.home_address.city,
        address.home_address.line1,
      ])
        assert(
          !String(v).includes(pii),
          "a storage write attempted synthetic PII",
        );
      map.set(k, String(v));
    },
    removeItem: (k) => map.delete(k),
  });
  globalThis.localStorage = store(data);
  globalThis.sessionStorage = store(session);
  globalThis.window = { localStorage };
  const tails = new Map(); // Web Locks serialize per name, not across unrelated locks.
  const locks = [];
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: {
      request(name, ...args) {
        locks.push(name);
        const next = (tails.get(name) ?? Promise.resolve()).then(args.at(-1));
        tails.set(
          name,
          next.catch(() => {}),
        );
        return next;
      },
    },
  });
  const requests = [];
  let route = async () => {
    throw new Error("unexpected synthetic request");
  };
  let activeContext = context;
  globalThis.fetch = async (url, init) => {
    if (url.endsWith("/session"))
      return Response.json({
        state: "active",
        context: activeContext,
        expires_at: until,
      });
    const request = {
      path: url.replace("/api/buyer/", ""),
      method: init.method,
      key: new Headers(init.headers).get("Idempotency-Key"),
      body: init.body ? JSON.parse(init.body) : undefined,
    };
    requests.push(request);
    return route(request);
  };
  forgetAddressAttempt();
  try {
    await run({
      data,
      requests,
      locks,
      route(fn) {
        route = fn;
      },
      changeContext(value) {
        activeContext = value;
      },
    });
    for (const raw of [...data.values(), ...session.values()]) {
      for (const pii of [
        address.recipient_name,
        address.phone,
        address.home_address.city,
        address.home_address.line1,
      ])
        assert(
          !raw.includes(pii),
          "browser journal must not persist synthetic PII",
        );
    }
  } finally {
    forgetAddressAttempt();
    for (const key of ["fetch", "localStorage", "sessionStorage", "window"]) {
      if (previous[key] === undefined) delete globalThis[key];
      else globalThis[key] = previous[key];
    }
    if (previous.locks)
      Object.defineProperty(navigator, "locks", previous.locks);
    else delete navigator.locks;
  }
}

test("private quote/option authority fields are required; only exact MANUAL home can checkout", () => {
  assert(validQuote(quote));
  assert(validOption(option));
  assert(validDestination(head));
  assert(validOrder(order()));
  assert(validOption({ ...option, mode: "API" }));
  assert.deepEqual(checkoutInput(quote, option, cart, head), input);
  for (const k of ["cart_id", "market_id", "country", "method"]) {
    const bad = { ...quote };
    delete bad[k];
    assert.equal(validQuote(bad), false);
  }
  for (const k of ["service_version", "allocation_version", "mode"]) {
    const bad = { ...option };
    delete bad[k];
    assert.equal(validOption(bad), false);
  }
  for (const changed of [
    { country: "HK" },
    { market_id: uid(9) },
    { method: "delivery:other" },
    { currency: "HKD" },
    { mode: "API" },
    { delivery_kind: "cvs_711" },
    { allocation_version: 0 },
    { service_version: 1.5 },
  ])
    assert.throws(() =>
      checkoutInput(quote, { ...option, ...changed }, cart, head),
    );
  assert.throws(() =>
    checkoutInput(quote, option, cart, { ...head, cart_version: 3 }),
  );
  assert.throws(() =>
    checkoutInput(quote, option, cart, head, Date.parse(until)),
  );
  assert.equal(validOrder({ ...order(), hold_expires_at: undefined }), false);
  assert(validOrder(order("CANCELLED")));
});

test("native address validation mirrors printable Unicode, limits and phone; journal forbids PII", () => {
  assert(validDestinationWrite(address));
  assert(validDestinationWrite({ ...address, recipient_name: "陳測試😀" }));
  for (const bad of [
    { ...address, recipient_name: "\nBad" },
    { ...address, recipient_name: "x".repeat(121) },
    { ...address, phone: "abc123456" },
    { ...address, kind: "cvs_711" },
    { ...address, home_address: { ...address.home_address, city: "" } },
    { ...address, token: "forbidden" },
  ])
    assert.equal(validDestinationWrite(bad), false);
  const marker = {
    v: 1,
    context,
    key: uid(9),
    kind: "destination",
    body: { expected_version: 0, cart_version: 2, kind: "home", country: "TW" },
  };
  assert.deepEqual(parsePending(JSON.stringify(marker), context), marker);
  assert.throws(() =>
    parsePending(JSON.stringify({ ...marker, body: address }), context),
  );
  assert.throws(() =>
    parsePending(
      JSON.stringify({ ...marker, context: foreignContext }),
      context,
    ),
  );
});

test("address lost reply replays exact in-memory intent, then confirms current head", async () =>
  browserFixture(async (f) => {
    let count = 0;
    f.route((req) => {
      if (req.method === "GET") return Response.json({ destination: head });
      if (++count === 1) throw new Error("synthetic lost destination response");
      return Response.json(head);
    });
    await assert.rejects(writeDestination(context, address));
    const pending = pendingPurchase(context);
    assert.equal(pending.kind, "destination");
    await assert.rejects(
      writePurchase(context, {
        kind: "cart",
        body: { expected_version: 2, items: [] },
      }),
    );
    await assert.rejects(
      writeDestination(context, { ...address, phone: "+886900000002" }),
    );
    const result = await writeDestination(context, address);
    assert.equal(result.id, head.id);
    assert.deepEqual(
      f.requests.filter((x) => x.method === "PUT").map((x) => [x.key, x.body]),
      [
        [pending.key, address],
        [pending.key, address],
      ],
    );
    assert.equal(pendingPurchase(context), null);
  }));

test("address reload requires explicit new intent and exact observed CAS; stale head never auto-bumps", async () =>
  browserFixture(async (f) => {
    f.route(() => {
      throw new Error("synthetic lost reply");
    });
    await assert.rejects(writeDestination(context, address));
    const old = pendingPurchase(context);
    forgetAddressAttempt(); // Another tab or document reload has no address body.
    await assert.rejects(writeDestination(context, address));
    let current = head;
    f.route((req) =>
      req.method === "GET"
        ? Response.json({ destination: current })
        : Response.json(destination(req.body)),
    );
    assert.equal((await currentDestination(context)).version, 1);
    await assert.rejects(writeDestination(context, address, old.key), {
      code: "request_failed",
      status: 409,
    });
    assert.equal(pendingPurchase(context).key, old.key);
    const confirmed = { ...address, expected_version: 1 };
    // Before the explicit replacement PUT, GET still reports the observed head.
    f.route((req) => {
      if (req.method === "PUT") current = destination(req.body);
      return Response.json(
        req.method === "PUT" ? current : { destination: current },
      );
    });
    assert.equal(
      (await writeDestination(context, confirmed, old.key)).version,
      2,
    );
    assert.notEqual(f.requests.at(-2).key, old.key);
    assert.equal(pendingPurchase(context), null);
  }));

test("old address receipt cannot confirm another tab's newer current head", async () =>
  browserFixture(async (f) => {
    f.route((req) =>
      Response.json(
        req.method === "PUT"
          ? head
          : { destination: { ...head, id: uid(888), version: 2 } },
      ),
    );
    await assert.rejects(writeDestination(context, address), {
      code: "request_failed",
      status: 409,
    });
    assert.equal(pendingPurchase(context), null);
  }));

test("a late address CAS winner requires another explicit confirmation, not an automatic version bump", async () =>
  browserFixture(async (f) => {
    f.route(() => {
      throw new Error("synthetic initial uncertainty");
    });
    await assert.rejects(writeDestination(context, address));
    const old = pendingPurchase(context);
    forgetAddressAttempt();
    f.route((req) =>
      req.method === "GET" ? Response.json({ destination: null }) : denial(409),
    );
    await assert.rejects(writeDestination(context, address, old.key), {
      code: "request_failed",
      status: 409,
    });
    assert.equal(pendingPurchase(context), null);
    const writes = f.requests.filter((x) => x.method === "PUT");
    assert.equal(writes.length, 2);
    assert.deepEqual(
      writes.map((x) => x.body.expected_version),
      [0, 0],
    );
    assert.notEqual(writes[0].key, writes[1].key);
  }));

test("checkout lost reply, queued duplicate and current GET reuse one key and one order", async () =>
  browserFixture(async (f) => {
    let posts = 0;
    f.route((req) => {
      if (req.method === "GET") return Response.json(order("CANCELLED"));
      if (++posts === 1) throw new Error("synthetic commit with lost reply");
      return Response.json({ order_id: uid(6), hold_expires_at: until });
    });
    await assert.rejects(writeCheckout(context, input));
    const pending = pendingPurchase(context);
    assert.equal(orderRecoveryRequired(context), true);
    await assert.rejects(
      writePurchase(context, {
        kind: "cart",
        body: { expected_version: 2, items: [] },
      }),
    );
    await assert.rejects(writeDestination(context, address));
    await assert.rejects(writeCheckout(context, { ...input, cart_version: 3 }));
    const [a, b] = await Promise.all([
      writeCheckout(context),
      writeCheckout(context, input),
    ]);
    assert.equal(a.commercial_state, "CANCELLED");
    assert.deepEqual(a, b);
    assert.equal(posts, 2); // First lost reply, then original-key replay, never a third POST.
    assert.deepEqual(
      f.requests.filter((x) => x.method === "POST").map((x) => [x.key, x.body]),
      [
        [pending.key, input],
        [pending.key, input],
      ],
    );
    assert.equal(knownOrderID(context), uid(6));
    assert.equal(pendingPurchase(context), null);
    assert.equal(orderRecoveryRequired(context), true);
    assert(
      f.locks.every((name) =>
        ["commerce-purchase-write-v1", "commerce-buyer-session-v1"].includes(
          name,
        ),
      ),
    );
    await assert.rejects(
      writePurchase(context, {
        kind: "cart",
        body: { expected_version: 2, items: [] },
      }),
    );
  }));

test("uncertain checkout retains original journal across later 401/403/404/5xx and malformed success", async () =>
  browserFixture(async (f) => {
    f.route(() => {
      throw new Error("synthetic uncertainty");
    });
    await assert.rejects(writeCheckout(context, input));
    const original = pendingPurchase(context);
    for (const status of [401, 403, 404, 500, 503]) {
      f.route(() => denial(status));
      await assert.rejects(writeCheckout(context), {
        code: "uncertain",
        status,
      });
      assert.deepEqual(pendingPurchase(context), original);
    }
    f.route(() => Response.json({ order_id: "bad" }));
    await assert.rejects(writeCheckout(context), { code: "uncertain" });
    assert.deepEqual(pendingPurchase(context), original);
  }));

test("only a fresh definitive denial clears checkout; no order locator is invented", async () =>
  browserFixture(async (f) => {
    f.route(() => denial(403));
    await assert.rejects(writeCheckout(context, input), {
      code: "request_failed",
      status: 403,
    });
    assert.equal(pendingPurchase(context), null);
    assert.equal(knownOrderID(context), null);
  }));

test("locator storage failure after commit retains same key; reload GET failure keeps locator", async () =>
  browserFixture(async (f) => {
    f.route((req) =>
      req.method === "POST"
        ? Response.json({ order_id: uid(6), hold_expires_at: until })
        : Response.json(order()),
    );
    const originalSet = localStorage.setItem;
    localStorage.setItem = (key, value) => {
      if (key.includes("purchase-order")) throw new Error("synthetic quota");
      originalSet(key, value);
    };
    await assert.rejects(writeCheckout(context, input), {
      code: "unavailable",
    });
    const pending = pendingPurchase(context);
    localStorage.setItem = originalSet;
    const result = await writeCheckout(context);
    assert.equal(result.order_id, uid(6));
    assert.equal(knownOrderID(context), uid(6));
    assert.deepEqual(
      f.requests.filter((x) => x.method === "POST").map((x) => x.key),
      [pending.key, pending.key],
    );
    f.route(() => denial(401));
    await assert.rejects(writeCheckout(context), {
      code: "request_failed",
      status: 401,
    });
    assert.equal(knownOrderID(context), uid(6));
    assert.equal(orderRecoveryRequired(context), true);
  }));

test("locator persisted but journal removal failed resumes pending receipt, not unrelated pointer", async () =>
  browserFixture(async (f) => {
    f.route((req) =>
      Response.json(
        req.method === "POST"
          ? { order_id: uid(6), hold_expires_at: until }
          : order(),
      ),
    );
    const remove = localStorage.removeItem;
    localStorage.removeItem = () => {
      throw new Error("synthetic storage failure");
    };
    await assert.rejects(writeCheckout(context, input), {
      code: "unavailable",
    });
    const pending = pendingPurchase(context);
    assert.equal(knownOrderID(context), uid(6));
    localStorage.removeItem = remove;
    f.route(() => Response.json({ order_id: uid(7), hold_expires_at: until }));
    await assert.rejects(writeCheckout(context), { code: "uncertain" });
    assert.deepEqual(pendingPurchase(context), pending);
    assert.equal(knownOrderID(context), uid(6));
    f.route((req) =>
      Response.json(
        req.method === "POST"
          ? { order_id: uid(6), hold_expires_at: until }
          : order(),
      ),
    );
    assert.equal((await writeCheckout(context)).order_id, uid(6));
    assert.equal(pendingPurchase(context), null);
  }));

test("unavailable storage, invalid locator and late changed context fail closed", async () =>
  browserFixture(async (f) => {
    const set = localStorage.setItem;
    localStorage.setItem = () => {
      throw new Error("synthetic disabled storage");
    };
    await assert.rejects(writeCheckout(context, input), {
      code: "unavailable",
    });
    assert.equal(f.requests.length, 0);
    localStorage.setItem = set;
    set(
      `commerce-purchase-order-v1:${context}`,
      JSON.stringify({ v: 1, context: foreignContext, order_id: uid(6) }),
    );
    await assert.rejects(writeCheckout(context, input), { code: "uncertain" });
    assert.equal(f.requests.length, 0);
    f.data.clear();
    f.route(() => {
      f.changeContext(foreignContext);
      return Response.json({ order_id: uid(6), hold_expires_at: until });
    });
    await assert.rejects(writeCheckout(context, input), {
      code: "context_changed",
    });
    assert.equal(knownOrderID(context), null);
    assert.equal(pendingPurchase(context).kind, "checkout");
    assert.equal(knownOrderID(foreignContext), null);
  }));

test("order locator readback mismatch and no-op journal removal both retain checkout recovery", async () =>
  browserFixture(async (f) => {
    f.route((req) =>
      Response.json(
        req.method === "POST"
          ? { order_id: uid(6), hold_expires_at: until }
          : order(),
      ),
    );
    const get = localStorage.getItem;
    localStorage.getItem = (key) =>
      key.includes("purchase-order") ? null : get(key);
    await assert.rejects(writeCheckout(context, input), {
      code: "unavailable",
    });
    const original = pendingPurchase(context);
    localStorage.getItem = get;
    const remove = localStorage.removeItem;
    localStorage.removeItem = () => {};
    await assert.rejects(writeCheckout(context), { code: "unavailable" });
    assert.deepEqual(pendingPurchase(context), original);
    assert.equal(knownOrderID(context), uid(6));
    localStorage.removeItem = remove;
    assert.equal((await writeCheckout(context)).order_id, uid(6));
    assert.equal(pendingPurchase(context), null);
    assert.deepEqual(
      f.requests.filter((x) => x.method === "POST").map((x) => x.key),
      [original.key, original.key, original.key],
    );
  }));
