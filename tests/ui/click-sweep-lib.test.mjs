// Unit tests of the pure parts of the G-UI8 click-sweep library (classification, class keys, known-defect matching, ledger rendering).
// Run by scripts/dev/test-node.sh and by `test-local.sh --browser-click-sweep` before any build.
import assert from "node:assert/strict";
import test from "node:test";
import { classKey, isDestructive, isIrreversible, isSignOut, matchKnown, renderMarkdown, CANCEL_RE, protocolHrefOK } from "./click-sweep-lib.mjs";

test("destructive / irreversible / sign-out classification (owner list, en + zh)", () => {
  for (const name of ["Delete product", "刪除商品", "删除", "Archive", "封存", "Void shipment", "作廢", "Disconnect Page", "斷開連線", "Refund", "退款", "Cancel order", "取消訂單", "Publish", "上架", "Unpublish", "revoke invite", "Remove staff", "取消發佈", "解除綁定", "中斷連接", "Suspend", "Detach", "將 Sweep Wool Scarf 移出購物車", "将 Scarf 移出购物车"])
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
