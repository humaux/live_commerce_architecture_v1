// Purpose: G-UI8 classification and media journey protocol regressions without starting a browser.
// Depends on: Node assert/test, click-sweep-lib and the real J1 cover driver with controlled page/check doubles.
// Used by: scripts/dev/test-node.sh and the click-sweep mode preflight.
// Run by scripts/dev/test-node.sh and by `test-local.sh --browser-click-sweep` before any build.
import assert from "node:assert/strict";
import test from "node:test";
import { classKey, isCartLineRemoval, isDestructive, isIrreversible, isSignOut, matchKnown, renderMarkdown, CANCEL_RE, protocolHrefOK } from "./click-sweep-lib.mjs";

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
