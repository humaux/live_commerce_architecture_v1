import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { isDeepStrictEqual } from "node:util";
import { expect } from "@playwright/test";
import { returnRequestCopy } from "../../apps/storefront/components/order-status/return-request-copy.ts";

export async function r03ReturnLineJourney({ origin, evidence, control, newContext, calls }) {
  const { order, other_order, link, other_link, fresh_link, contact, sku_id, warehouse_id } = await control("r03/fixture");
  const locale = "zh-TW", copy = returnRequestCopy[locale];
  const orderPath = (id) => `/${locale}/orders/${id}`;
  const facts = () => control("r03/facts");
  const json = (name, data) => writeFile(path.join(evidence, `${name}.json`), JSON.stringify(data, null, 2), { mode: 0o600 });
  const returns = () => calls.filter((call) => call.path.includes("/return-requests"));
  const mutations = () => returns().filter((call) => call.method === "POST");
  const evidenceSnapshots = {}, ledger = [];
  const record = (p, control, action, expected, actual) => ledger.push({
    page: new URL(p.url()).pathname, control, action, expected, actual,
    status: isDeepStrictEqual(actual, expected) ? "PASS" : "FAIL",
  });
  const [lookup, linked] = [await newContext(true), await newContext(true)];
  const guest = await lookup.newPage();
  let page;
  const reactErrors = [];
  const watchConsole = (p) => p.on("console", (message) => {
    if (/same key|duplicate key|Minified React error #(?:418|423|425)/i.test(message.text())) reactErrors.push(message.text());
  });
  watchConsole(guest);
  const screenshot = (p, name) => p.screenshot({ path: path.join(evidence, `${name}.png`), fullPage: true, timeout: 8000 });
  const box = (i) => page.getByRole("checkbox").nth(i);
  const row = (i) => box(i).locator("xpath=../..");
  const qty = (i) => row(i).getByRole("spinbutton");
  const readRows = () => page.getByRole("checkbox").evaluateAll((boxes) => boxes.map((checkbox) => ({
    checked: checkbox.checked,
    label: checkbox.labels[0].textContent,
    quantity: checkbox.parentElement.parentElement.querySelector('input[type="number"]')?.value ?? null,
  })));
  const openLink = async (id, token) => {
    try {
      await Promise.all([
        page.waitForURL(origin + orderPath(id) + "?access=link", { timeout: 12000 }),
        page.goto(`${origin}/${locale}/order-link#o=${id}&t=${token}`, { waitUntil: "commit" }),
      ]);
    } catch (error) {
      throw new Error(`Order link did not reach ${orderPath(id)} (${error.name})`);
    }
  };
  const openForm = async (id, count) => {
    await page.getByTestId("request-after-sales").click();
    await expect(page).toHaveURL(origin + orderPath(id) + "/return?access=link");
    await expect(page.getByRole("checkbox")).toHaveCount(count);
  };
  const getRows = async (p) => {
    const lines = p.getByTestId("order-section").locator(".order-lines li");
    await expect(lines).toHaveCount(2);
    await expect(lines.nth(0)).toContainText("× 2");
    await expect(lines.nth(0)).toContainText("NT$25");
    await expect(lines.nth(1)).toContainText("× 1");
    await expect(lines.nth(1)).toContainText("NT$13");
  };

  try {
    const before = evidenceSnapshots.before = await facts();
    assert.deepEqual(before.quote_lines.map((line) => [line.sku_id, line.quantity, line.unit_price_minor]), [[sku_id, 2, 1250], [sku_id, 1, 1300]]);
    assert.deepEqual(before.reservation_lines, [{ sku_id, warehouse_id, quantity: 3 }]);
    assert.equal(before.revision, 2);
    assert.equal(before.shipment_revision, 2);
    assert.equal(before.collection, "PENDING");
    assert.equal(before.total_minor + before.surcharge_minor, 8800);
    assert.deepEqual([before.payment_attempts, before.refunds, before.external_operations, before.rmas], [0, 0, 0, 0]);
    assert.deepEqual(before.requests, []);
    assert.equal(before.receipts, 0);

    await Promise.all([
      guest.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/session" && r.request().method() === "GET" && r.status() === 200),
      guest.goto(`${origin}/${locale}/orders/lookup`),
    ]);
    const lookupRef = guest.getByTestId("lookup-ref"), lookupContact = guest.getByTestId("lookup-contact");
    await lookupRef.fill(order);
    await lookupContact.fill(contact);
    await expect(lookupRef).toHaveValue(order);
    await expect(lookupContact).toHaveValue(contact);
    const [found] = await Promise.all([
      guest.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/orders/lookup" && r.request().method() === "POST"),
      guest.getByTestId("lookup-submit").click(),
    ]);
    assert.equal(found.status(), 200);
    await expect(guest).toHaveURL(origin + orderPath(order) + "?access=lookup");
    await getRows(guest);
    const guestEntry = guest.getByTestId("request-after-sales");
    await expect(guestEntry).toHaveCount(0);
    record(guest, "查詢訂單", "提交查單表單", { order: true, returnEntry: 0 }, {
      order: await guest.getByTestId("order-section").isVisible(),
      returnEntry: await guestEntry.count(),
    });
    await screenshot(guest, "01-lookup-read-only");
    await guest.goto(origin + orderPath(order) + "/return?access=lookup");
    await expect(guest.locator(".sf-state")).toBeVisible();
    await expect(guest.getByRole("checkbox")).toHaveCount(0);
    assert.equal(returns().at(-1).status, 403);
    assert.deepEqual(await facts(), before);

    page = await linked.newPage();
    watchConsole(page);
    await openLink(order, link);
    await getRows(page);
    await screenshot(page, "02-linked-order");
    await openForm(order, 2);
    const rowsBefore = await readRows();
    await box(1).click();
    await expect(box(1)).toBeChecked();
    const rowsAfter = await readRows();
    await json("second-click", { before: rowsBefore, after: rowsAfter });
    await screenshot(page, "03-after-second-click");
    record(page, "第二個價格列的選取框", "點選", rowsBefore[0], rowsAfter[0]);
    assert.deepEqual(rowsAfter[0], rowsBefore[0], "clicking the second price row must leave the entire first selection unchanged");
    assert.equal(rowsAfter[1].checked, true);
    assert.equal(rowsAfter[1].quantity, "1");
    await expect(row(0)).toContainText("NT$12.50");
    await expect(row(1)).toContainText("NT$13");
    await expect(qty(1)).toHaveAttribute("max", "1");
    await box(0).click();
    await expect(qty(0)).toHaveAttribute("max", "2");
    await qty(0).fill("2");
    await expect(qty(1)).toHaveValue("1");
    await qty(1).fill("2");
    assert.equal(await qty(1).evaluate((input) => input.validity.rangeOverflow), true);
    await page.getByRole("button", { name: copy.submit, exact: true }).click();
    record(page, "第二列數量與送出", "填 2 並點選送出", 0, mutations().length);
    assert.equal(mutations().length, 0);
    await qty(1).fill("1");
    await expect(qty(0)).toHaveValue("2");
    await box(1).click();
    await expect(qty(1)).toHaveCount(0);
    await expect(qty(0)).toHaveValue("2");
    await box(1).click();
    await box(0).click();
    await expect(qty(0)).toHaveCount(0);
    await expect(qty(1)).toHaveValue("1");
    record(page, "兩列選取框與數量", "修改第一列，分別取消又選取", [null, "1"],
      (await readRows()).map(({ quantity }) => quantity));
    await page.reload();
    await expect(page.getByRole("checkbox")).toHaveCount(2);
    assert.deepEqual(await readRows(), rowsBefore);
    await box(1).click();

    await page.goto(origin + orderPath(other_order) + "/return?access=link");
    await expect(page.locator(".sf-state")).toBeVisible();
    await expect(page.getByRole("checkbox")).toHaveCount(0);
    await openLink(other_order, other_link);
    await openForm(other_order, 1);
    await expect(box(0)).not.toBeChecked();
    await expect(qty(0)).toHaveCount(0);
    assert.equal(mutations().length, 0);
    assert.deepEqual(await facts(), before, "lookup, edits and order switches leave order and return facts untouched");
    await openLink(order, fresh_link);
    await openForm(order, 2);
    assert.deepEqual(await readRows(), rowsBefore);
    record(page, "專屬連結與申請入口", "登上另一筆訂單，再返回", rowsBefore, await readRows());

    const createPath = `/api/buyer/orders/${order}/return-requests`;
    const submit = () => Promise.all([
      page.waitForResponse((r) => new URL(r.url()).pathname === createPath && r.request().method() === "POST"),
      page.getByRole("button", { name: copy.submit, exact: true }).click(),
    ]);
    const selectBothRows = async () => {
      await box(1).click();
      await box(0).click();
      await qty(0).fill("2");
    };
    const expectedBody = { reason: "damaged", note: "", lines: [{ sku_id, quantity: 3 }] };
    await selectBothRows();
    const [refused] = await submit();
    const refusal = { status: refused.status(), code: (await refused.json()).code };
    record(page, copy.submit, "待收款時點選", { status: 409, code: "not_returnable" }, refusal);
    assert.deepEqual(refusal, { status: 409, code: "not_returnable" });
    await expect(page.getByText(copy.errors.not_returnable, { exact: true })).toBeVisible();
    assert.equal(mutations().length, 1);
    assert.deepEqual(JSON.parse(mutations()[0].body), expectedBody);
    evidenceSnapshots.refused = await facts();
    assert.deepEqual(evidenceSnapshots.refused, before);
    await screenshot(page, "03-pending-refused");

    // Fixture preparation only: COD must be recorded as collected before Go accepts a return.
    await control("r03/record-synthetic-collection", "POST");
    const eligible = evidenceSnapshots.eligible = await facts();
    assert.deepEqual(eligible, { ...before, collection: "COLLECTED" });
    await page.reload();
    await expect(page.getByRole("checkbox")).toHaveCount(2);
    assert.deepEqual(await readRows(), rowsBefore);

    await selectBothRows();
    const exactDetail = /\/api\/buyer\/return-requests\/[a-f0-9-]{36}$/;
    // Lose the first persisted read only. The browser still sends the real keyed POST to Go/PG.
    await page.route(exactDetail, (route) => route.fulfill({ status: 503, contentType: "application/json", body: '{"code":"unavailable"}' }), { times: 1 });
    const [failedRead, { ack, request }] = await Promise.all([
      page.waitForResponse((r) => exactDetail.test(r.url())),
      submit().then(async ([ack]) => {
        const request = await ack.json();
        assert.equal(ack.status(), 201, request.code);
        return { ack, request };
      }),
    ]);
    const create = mutations()[1];
    assert.equal(failedRead.status(), 503);
    record(page, copy.submit, "點選", { write: 201, persistedRead: 503 }, { write: ack.status(), persistedRead: failedRead.status() });
    assert.equal(mutations().length, 2);
    assert.match(create.key, /^[A-Za-z0-9_.:-]{8,128}$/);
    assert.deepEqual(JSON.parse(create.body), expectedBody);
    assert.deepEqual(request.lines, [{ sku_id, warehouse_id, quantity: 3 }]);
    const requested = evidenceSnapshots.requested = await facts();
    assert.equal(requested.receipts, 1);
    assert.deepEqual(requested.requests, [{ id: request.id, status: "requested", version: 1, lines: request.lines }]);

    const retry = page.locator(".sf-state").getByRole("button");
    await expect(retry).toBeVisible();
    await screenshot(page, "04-persisted-read-failed");
    const [recovered] = await Promise.all([
      page.waitForResponse((r) => exactDetail.test(r.url()) && r.request().method() === "GET"),
      retry.click(),
    ]);
    assert.equal(recovered.status(), 200);
    await expect(page.getByTestId("return-status")).toHaveAttribute("data-status", "requested");
    assert.equal(mutations().length, 2);
    assert.deepEqual(await facts(), requested);
    record(page, "重新讀取", "點選", { status: "requested", posts: 2 }, {
      status: await page.getByTestId("return-status").getAttribute("data-status"), posts: mutations().length,
    });
    await screenshot(page, "05-requested-after-read-retry");

    await page.getByRole("button", { name: copy.withdraw, exact: true }).click();
    await page.getByRole("dialog").getByRole("button", { name: copy.confirm, exact: true }).click();
    await expect(page.getByTestId("return-status")).toHaveAttribute("data-status", "withdrawn");
    await page.reload();
    await expect(page.getByTestId("return-status")).toHaveAttribute("data-status", "withdrawn");
    record(page, copy.withdraw, "點選撤回、確認並刷新", { status: "withdrawn", posts: 3 }, {
      status: await page.getByTestId("return-status").getAttribute("data-status"), posts: mutations().length,
    });
    await screenshot(page, "06-withdrawn-reloaded");
    const writes = mutations();
    assert.deepEqual(writes.map(({ path, status }) => [path, status]), [[createPath, 409], [createPath, 201], [`/api/buyer/return-requests/${request.id}/withdraw`, 200]]);
    assert.match(writes[2].key, /^[A-Za-z0-9_.:-]{8,128}$/);
    assert.notEqual(writes[2].key, create.key);
    assert.deepEqual(JSON.parse(writes[2].body), { expected_version: 1 });
    const after = evidenceSnapshots.after = await facts();
    assert.deepEqual(after, { ...eligible, receipts: 2, requests: [{ id: request.id, status: "withdrawn", version: 2, lines: request.lines }] });
    assert.deepEqual(reactErrors, []);
    assert(ledger.every(({ status }) => status === "PASS"));
    console.log("PASS R03 actual lookup/link/order/return: independent price rows, PENDING refused, COLLECTED one SKU write, read retry, withdrawal and PG equality");
  } finally {
    await json("network", {
      // Credentials and lookup contact are excluded. Return bodies are synthetic and contain no private order details.
      calls: calls.map(({ path, method, status, body, key }) => ({
        path, method, status, ...(path.includes("/return-requests") ? { body, key } : {}),
      })),
      reactErrors,
    });
    await json("pg-facts", evidenceSnapshots);
    await json("click-ledger", ledger);
  }
}
