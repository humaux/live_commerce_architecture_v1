// Purpose: G-UI8 classification and media journey protocol regressions without starting a browser.
// Depends on: Node assert/test, click-sweep-lib and the real J1 cover driver with controlled page/check doubles.
// Used by: scripts/dev/test-node.sh and the click-sweep mode preflight.
// Invariants: busy placeholders cannot earn readiness; recovery shares bounded waits; classification assertions remain unchanged.
// Run by scripts/dev/test-node.sh and by `test-local.sh --browser-click-sweep` before any build.
import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { runInNewContext } from "node:vm";
import { INIT_SCRIPT, listControls, monitor, sweepControl, settle, restore } from "./click-sweep-lib.mjs";
import { classKey, isCartLineRemoval, isDestructive, isIrreversible, isSignOut, matchKnown, renderMarkdown, CANCEL_RE, protocolHrefOK } from "./click-sweep-lib.mjs";

test("native file picker is an observed effect on existing and newly opened pages", () => {
  const handlers = new Map();
  const page = { on: (event, handler) => handlers.set(event, handler), url: () => "https://admin.example.invalid/en/customers/import" };
  let attach;
  const context = { pages: () => [page], on: (event, handler) => { assert.equal(event, "page"); attach = handler; }, route: () => {} };
  const mon = monitor(context, { allowedHosts: new Set(["admin.example.invalid"]) });
  const start = mon.mark();
  assert.deepEqual(mon.since(start), [], "a file-picker label or click alone earns no effect");
  handlers.get("filechooser")();
  assert.deepEqual(mon.since(start).map(({ type, text }) => ({ type, text })), [{ type: "filechooser", text: "native file picker opened" }]);
  const nextHandlers = new Map();
  attach({ on: (event, handler) => nextHandlers.set(event, handler), opener: () => null });
  const next = mon.mark();
  nextHandlers.get("filechooser")();
  assert.equal(mon.since(next)[0].type, "filechooser");
  handlers.get("pageerror")(new Error("synthetic failure"));
  assert.equal(mon.since(next)[1].type, "pageerror", "native effects never suppress browser errors");
});

// A deterministic read-only DOM/clock double: no browser, timer sleeps or server.
function readinessFixture(t, { readyAt = 1200, marker = "busy" } = {}) {
  let now = 0;
  t.mock.method(Date, "now", () => now);
  const pending = () => now < readyAt;
  const loading = {
    textContent: marker === "status" ? "載入中…" : "",
    getClientRects: () => [1],
    matches: (selector) => marker === "skeleton" && selector.includes(".sf-skel"),
  };
  const main = {
    matches: () => marker === "busy" && pending(),
    querySelector: () => null,
    querySelectorAll: () => pending() && marker !== "busy" ? [loading] : [],
  };
  const page = {
    url: () => "https://shop.example.invalid/zh-TW/orders/order",
    evaluate: async (fn) => runInNewContext(`(${fn.toString()})()`, {
      window: { __sweepHelpers: { state: () => ({ mut: 0, layers: 0 }) } },
      document: { querySelectorAll: () => [main] },
    }),
    waitForTimeout: async (ms) => { now += ms; },
    goto: async () => { now = 0; },
  };
  return { page, mon: { idle: () => true }, elapsed: () => now };
}

for (const marker of ["busy", "status", "skeleton"]) {
  test(`readiness: briefly quiet but still loading (${marker}) waits before assertions`, async (t) => {
    const f = readinessFixture(t, { marker });
    assert.equal(await settle(f.page, f.mon, { ready: true, max: 2000 }), true);
    assert.ok(f.elapsed() >= 1200, "quiet DOM/network must not credit a still-loading page");
    assert.ok(f.elapsed() <= 2000, "readiness must share the existing settle budget");
  });
}

test("readiness: a page that stays busy fails explicitly within the bound", async (t) => {
  const f = readinessFixture(t, { readyAt: Infinity });
  await assert.rejects(settle(f.page, f.mon, { ready: true, max: 800 }), /page stayed busy/i);
  assert.ok(f.elapsed() <= 800, "the busy page must not extend the bound");
});

test("readiness: recovery reload waits for the page, not just DOM/network quiet", async (t) => {
  const f = readinessFixture(t);
  assert.equal(await restore(f.page, f.mon, f.page.url(), { layer: false }), "reloaded");
  assert.ok(f.elapsed() >= 1200, "recovery must not enumerate the loading placeholder");
});

test("readiness: initial and fresh-context navigations opt in before unchanged page assertions", async () => {
  const source = await readFile(new URL("./click-sweep.mjs", import.meta.url), "utf8");
  assert.match(source, /const open = async \(\) => \{[^]*?await settle\(cur.page, cur.mon, \{ ready: true \}\);/);
  assert.match(source, /const quiet = await settle\(cur.page, cur.mon, \{ max: 8000, ready: true \}\);[^]*?const headings = await cur.page.locator\("h1, h2"\).count\(\);/);
});

test("storefront cart-line removal is reversible: clicked for real, not guarded", () => {
  for (const name of ["Remove Sweep Wool Scarf from the cart", "將 Sweep Wool Scarf 移出購物車", "将 Scarf 移出购物车"]) {
    assert.equal(isCartLineRemoval(name), true, name);
    assert.equal(isDestructive(name), false, name);
  }
  for (const name of ["Remove staff", "將成員移出店鋪"]) assert.equal(isCartLineRemoval(name), false, name);
});

test("destructive / irreversible / sign-out classification (owner list, en + zh)", () => {
  for (const name of ["Delete product", "刪除商品", "删除", "Archive", "封存", "Void shipment", "作廢", "Disconnect Page", "斷開連線", "Refund", "退款", "Cancel order", "取消訂單", "Publish", "上架", "Unpublish", "revoke invite", "Remove staff", "取消發佈", "解除綁定", "中斷連接", "Suspend", "Detach", "將成員移出店鋪"])
    assert.equal(isDestructive(name), true, name);
  for (const name of ["Save", "儲存", "Cancel", "取消", "Next page", "Add to cart", "Choose delivery", "Search", "Pause", "暫停"]) assert.equal(isDestructive(name), false, name);
  assert.equal(isSignOut("Sign out"), true); assert.equal(isSignOut("登出"), true); assert.equal(isSignOut("Signed in as"), false);
  assert.equal(isIrreversible("Place order (cash on delivery)"), true); assert.equal(isIrreversible("送出訂單（貨到付款）"), true); assert.equal(isIrreversible("Save draft"), false);
  assert.equal(CANCEL_RE.test("Cancel"), true); assert.equal(CANCEL_RE.test("取消"), true); assert.equal(CANCEL_RE.test("Delete"), false);
});

test("alike controls (rows with ids / numbers) share one class key", () => {
  const d = (name, testid, href) => ({ scope: "main", tag: "button", role: "", type: "", testid, name, hrefPath: href });
  assert.equal(classKey(d("Order 12", "order-expand-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "")), classKey(d("Order 13", "order-expand-11111111-2222-3333-4444-555555555555", "")));
  assert.notEqual(classKey(d("Save", "save", "")), classKey(d("Delete", "delete", "")));
});

test("known defects classify, never silence: stale entries are reported", () => {
  const failures = [{ page: "/ads", failure: "no-effect", control: "Connect", testid: "ads-connect", viewport: "desktop", locale: "en" }, { page: "/orders", failure: "http5xx", control: "Next", testid: "", viewport: "mobile", locale: "zh-TW" }];
  const known = [
    { id: "D1", route: "/ads", kind: "no-effect", control: "ads-connect", owner_unit: "ads-ui", severity: "P2", summary: "x" },
    { id: "D2", route: "/gone", kind: "no-effect", owner_unit: "u", severity: "P2", summary: "x" },
    { id: "D3", route: ["/gone", "/also-gone"], kind: ["no-effect", "http5xx"], owner_unit: "u", severity: "P2", summary: "x", flaky: true },
  ];
  const { classified, stale } = matchKnown(failures, known);
  assert.deepEqual(classified.map((c) => c.known), ["D1", null]);
  assert.deepEqual(stale.map((s) => s.id), ["D2"], "D2 matches nothing and fails the run; the flaky D3 may go unmatched");
});

test("the markdown ledger has one row per interaction and names the verdict", () => {
  const rows = [{ id: "r00001", app: "admin", page: "/orders", viewport: "desktop", locale: "en", scope: "main", control: "Next | page", testid: "orders-next", action: "click", expected: "visible change", actual: "DOM changed", result: "pass", classSize: 1 },
    { id: "r00002", app: "admin", page: "/orders", viewport: "mobile", locale: "en", scope: "main", control: "X", testid: "", action: "click", expected: "visible change", actual: "no visible change within 3 s", result: "fail", failure: "no-effect", classSize: 4 }];
  const md = renderMarkdown(rows, { generated: "now", line: "2 rows" });
  assert.match(md, /\| r00001 \| admin \| \/orders \| desktop \| en \| main \| Next \\\| page/);
  assert.match(md, /FAIL no-effect/); assert.match(md, /\[1 of 4 alike\]/);
});

test("protocolHrefOK accepts well-formed tel:/mailto: and rejects malformed ones", () => {
  for (const ok of ["tel:+886223456789", "tel:+886 2 2345 6789", "tel:02-2345-6789", "tel:0912345678", "mailto:support@example.com"]) assert.equal(protocolHrefOK(ok), true, ok);
  for (const bad of ["tel:", "tel:12345", "tel:123456", "tel:+88622345678901234567", "tel:+886abc", "mailto:", "mailto:a@b", "mailto:a@b.com?subject=x", "javascript:alert(1)"]) assert.equal(protocolHrefOK(bad), false, bad);
});

function mediaDriverFixture({ status = 200, role = "main" } = {}) {
  const calls = [],
    id = "11111111-1111-4111-8111-111111111111";
  let ready = false,
    uploaded = false,
    receive;
  const response = { status: () => status, json: async () => ({ id, role }) };
  const tile = { kind: "tile", locator: () => tile };
  const main = {
    kind: "main",
    locator: (selector) =>
      selector.startsWith("[data-image-id=") ? tile : { kind: "main-images" },
  };
  const picker = {
    kind: "picker",
    isVisible: async () => true,
    click: async () => {
      assert.ok(ready, "the picker must be ready before clicking");
      calls.push("click-picker");
    },
    setInputFiles: async () => {
      calls.push("programmatic-setInputFiles");
      uploaded = ready;
    },
  };
  const check = (value) => ({
    toBeVisible: async () => {
      if (value.kind === "tile") assert.ok(uploaded);
      calls.push("visible:" + value.kind);
    },
    toBeEnabled: async () => {
      assert.equal(value, picker);
      ready = true;
      calls.push("picker-ready");
    },
    toHaveCount: async (n) => {
      const actual =
        ["main-images", "legacy"].includes(value.kind) && uploaded ? 1 : 0;
      assert.equal(actual, n);
      calls.push("count:" + value.kind);
    },
    toBe: (expected) => assert.equal(value, expected),
  });
  const page = {
    url: () =>
      "https://admin.example.invalid/zh-TW/products/product?store=store",
    getByTestId: (name) =>
      name === "media-main-list"
        ? main
        : name === "photo-input"
          ? picker
          : name === "photo-row"
            ? { kind: "legacy" }
            : { kind: name, locator: () => ({ kind: name + "-images" }) },
    waitForEvent: async (event) => {
      assert.equal(event, "filechooser");
      calls.push("listen-chooser");
      return {
        setFiles: async (file) => {
          assert.ok(calls.includes("click-picker"));
          assert.ok(ready);
          assert.ok(file.buffer.length);
          calls.push("chooser-files");
          uploaded = status === 200;
          receive?.(response);
        },
      };
    },
    waitForResponse: (predicate) => {
      calls.push("listen-upload");
      assert.ok(
        predicate({
          request: () => ({ method: () => "POST" }),
          url: () =>
            "https://admin.example.invalid/api/stores/store/products/product/images?role=main",
        }),
      );
      assert.equal(
        predicate({
          request: () => ({ method: () => "GET" }),
          url: () =>
            "https://admin.example.invalid/api/stores/store/products/product/images?role=main",
        }),
        false,
      );
      assert.equal(
        predicate({
          request: () => ({ method: () => "POST" }),
          url: () =>
            "https://admin.example.invalid/api/stores/store/products/product/images?role=detail",
        }),
        false,
      );
      return new Promise((resolve) => {
        receive = resolve;
      });
    },
  };
  return { page, check, calls, id };
}
test("J1 waits for enabled media and clicks a real chooser; the uploaded cover is bound to the main section", async () => {
  const { uploadJourneyMainCover } =
    await import("./product-media-journey.mjs");
  const f = mediaDriverFixture();
  const id = await uploadJourneyMainCover(
    f.page,
    {
      name: "cover.png",
      mimeType: "image/png",
      buffer: Buffer.from([137, 80, 78, 71]),
    },
    f.check,
  );
  assert.equal(id, f.id);
  assert.ok(f.calls.indexOf("picker-ready") < f.calls.indexOf("click-picker"));
  assert.ok(
    f.calls.indexOf("listen-chooser") < f.calls.indexOf("click-picker"),
  );
  assert.ok(
    f.calls.indexOf("listen-upload") < f.calls.indexOf("chooser-files"),
  );
  assert.ok(f.calls.includes("count:main-images"));
  assert.equal(f.calls.includes("programmatic-setInputFiles"), false);
});
test("J1 cannot credit a detail-image receipt as the main cover", async () => {
  const { uploadJourneyMainCover } =
    await import("./product-media-journey.mjs");
  const f = mediaDriverFixture({ role: "detail" });
  await assert.rejects(
    uploadJourneyMainCover(f.page, { buffer: Buffer.from([1]) }, f.check),
    /main/,
  );
});
test("J1 refuses a failed cover upload rather than continuing to activate", async () => {
  const { uploadJourneyMainCover } =
    await import("./product-media-journey.mjs");
  const f = mediaDriverFixture({ status: 500 });
  await assert.rejects(
    uploadJourneyMainCover(f.page, { buffer: Buffer.from([1]) }, f.check),
    /200/,
  );
});

// Registered only by the existing browser gate: the ordinary Node suite stays browser-free.
// All pages are synthetic loopback responses; the production helper performs the real clicks.
if (process.env.LC_SWEEP_NAVIGATION_CANARY === "1") {
  test("navigation canary: same-URL document replacement is an effect; inert clicks are not", { timeout: 45000 }, async (t) => {
    const { chromium } = await import("@playwright/test");
    const browser = await chromium.launch({ headless: true });
    t.after(() => browser.close());
    const cases = [];
    for (const kind of ["noop", "preventDefault", "missing-origin", "invalid-origin", "same-url"]) {
      const context = await browser.newContext();
      try {
        await context.addInitScript(INIT_SCRIPT);
        let documents = 0;
        const url = `http://127.0.0.1/sweep-navigation/${kind}?q=Sweep`;
        await context.route("**/*", async (route) => {
          if (!route.request().isNavigationRequest()) return route.fulfill({ status: 204, body: "" });
          documents++;
          const type = ["same-url", "preventDefault"].includes(kind) ? "submit" : "button";
          const script = kind === "preventDefault" ? '<script>document.querySelector("form").addEventListener("submit", e => e.preventDefault())</script>' : "";
          await route.fulfill({ contentType: "text/html", body: `<!doctype html><title>Navigation canary</title><main><form method="get"><input type="hidden" name="q" value="Sweep"><button type="${type}">Search</button></form></main>${script}` });
        });
        const page = await context.newPage();
        const mon = monitor(context, { allowedHosts: new Set(["127.0.0.1"]) });
        await page.goto(url, { waitUntil: "load" });
        await settle(page, mon, { ready: true });
        const before = await page.evaluate(() => ({ url: location.href, documentTimeOrigin: performance.timeOrigin }));
        // Fault injection at the read boundary, without changing the page or click:
        // missing/non-finite document evidence must never earn a navigation PASS.
        if (["missing-origin", "invalid-origin"].includes(kind)) {
          const evaluate = page.evaluate.bind(page);
          page.evaluate = async (fn, arg) => {
            const value = await evaluate(fn, arg);
            if (String(fn).includes("__sweepHelpers.state")) {
              value.documentTimeOrigin = kind === "missing-origin" ? undefined : Number.NaN;
            }
            return value;
          };
        }
        const desc = (await listControls(page, "main", "main"))[0];
        assert.equal(desc?.name, "Search");
        const row = await sweepControl({ page, mon, rootSelector: "main", unit: { route: "/canary", viewport: "desktop", locale: "en" } }, desc, 1);
        const after = await page.evaluate(() => ({ url: location.href, documentTimeOrigin: performance.timeOrigin }));
        cases.push({ kind, documents, before, after, result: row.result, failure: row.failure, actual: row.actual, documentNavigation: row.documentNavigation });
      } finally {
        await context.close();
      }
    }
    // Keep raw causal observations in the existing browser log even when the assertion is RED.
    console.log("SWEEP_NAVIGATION_CANARY " + JSON.stringify(cases));
    for (const c of cases.filter(c => c.kind !== "same-url")) {
      assert.equal(c.documents, 1, `${c.kind}: no extra document request`);
      assert.equal(c.after.documentTimeOrigin, c.before.documentTimeOrigin, `${c.kind}: same document`);
      assert.equal(c.result, "fail", `${c.kind}: must still fail`);
      assert.equal(c.failure, "no-effect", `${c.kind}: must still be no-effect`);
    }
    const navigated = cases.find(c => c.kind === "same-url");
    assert.equal(navigated.documents, 2, "native GET really requested a second document");
    assert.equal(navigated.after.url, navigated.before.url, "native GET retained the exact URL");
    assert.notEqual(navigated.after.documentTimeOrigin, navigated.before.documentTimeOrigin, "browser replaced the document");
    assert.equal(navigated.result, "pass", `real same-URL navigation was missed: ${navigated.actual}`);
    assert.match(navigated.actual, /reload|navigat|document/i, "must observe navigation itself, not incidental DOM churn");
  });
}
