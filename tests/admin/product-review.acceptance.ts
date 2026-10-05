// R5 review regressions: all merchant writes use real clicks on Next + Go + PG.
// Routes/cookies below inject lost receipts/auth failures, never fake a success.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";
const out = path.resolve("output/product-ui-v2-fix");
const store = () => process.env.LC_BROWSER_STORE!;
const url = (suffix: string) =>
  `${process.env.LC_BROWSER_PUBLIC_ORIGIN}/en/${suffix}?store=${store()}`;
async function openNew(page: Page, name: string) {
  await page.goto(url(""));
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  await expect(page.getByTestId("nav-orders")).toBeAttached();
  await page.goto(url("products"));
  await page.getByTestId("product-new").click();
  await page.getByTestId("product-name").fill(name);
  await page.getByTestId("product-price").fill("60");
  await page.getByTestId("product-untracked").check();
  await page.getByTestId("product-max").fill("3");
}
async function openSaved(page: Page) {
  await expect(page.getByTestId("product-save-result")).toBeVisible();
  await page
    .getByTestId("product-save-result")
    .locator('a[href*="/products/"]')
    .click();
  await expect(page.getByTestId("product-save")).toBeEnabled();
  return /products\/([0-9a-f-]{36})/.exec(page.url())![1];
}
async function evidence(page: Page, name: string, row: object) {
  await mkdir(out, { recursive: true });
  await page.screenshot({
    path: path.join(out, `${name}.png`),
    animations: "disabled",
  });
  await writeFile(
    path.join(out, `${name}.json`),
    JSON.stringify({ ...row, status: "PASS" }, null, 2),
  );
}
export function registerProductReviewAcceptance() {
  test("review: bulk price 80 plus archive sends exclusive archive entry and persists", async ({
    page,
  }) => {
    await openNew(page, `review-${process.env.LC_BROWSER_TAG}-archive`);
    await page.getByTestId("axis-add").click();
    await page.getByTestId("axis-name-0").fill("Size");
    await page.getByTestId("axis-values-0").fill("S, M");
    await page.getByTestId("axis-values-0").press("Enter");
    await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
    await page.getByTestId("bulk-open").click();
    await page.getByTestId("bulk-value").fill("60");
    await page.getByTestId("bulk-apply").click();
    await page.getByTestId("bulk-open").click();
    await page.getByTestId("bulk-field").selectOption("quantity");
    await page.getByTestId("bulk-value").fill("0");
    await page.getByTestId("bulk-apply").click();
    await page.getByTestId("product-create").click();
    const id = await openSaved(page);
    const archivedCode = await page.getByTestId("matrix-code-1").inputValue();
    await page.getByTestId("bulk-open").click();
    await page.getByTestId("bulk-value").fill("80");
    await page.getByTestId("bulk-apply").click();
    await expect(page.getByTestId("new-price-0")).toHaveValue("80");
    await expect(page.getByTestId("new-price-1")).toHaveValue("80");
    await page.getByTestId("matrix-active-1").uncheck();
    const writes: string[] = [];
    page.on("request", (r) => {
      if (r.method() === "PUT" && r.url().endsWith(`/${id}/document`))
        writes.push(r.postData()!);
    });
    const answer = page.waitForResponse(
      (r) =>
        r.request().method() === "PUT" && r.url().endsWith(`/${id}/document`),
    );
    const confirmation = page.waitForEvent("dialog");
    const saveClick = page.getByTestId("product-save").click();
    const dialog = await confirmation;
    expect(dialog.type()).toBe("confirm");
    expect(dialog.message()).toBe(productEditorCopy.en.archiveRows);
    await dialog.accept();
    await saveClick;
    expect((await answer).status()).toBe(200);
    await expect(page.getByTestId("product-save")).toBeEnabled();
    expect(writes).toHaveLength(1);
    const patch = JSON.parse(writes[0]);
    expect(Object.keys(patch).sort()).toEqual(["expected_version", "skus"]);
    expect(patch.skus).toHaveLength(2);
    expect(patch.skus[0]).toEqual({
      id: expect.any(String),
      price_minor: 8000,
    });
    expect(patch.skus[1]).toEqual({ id: expect.any(String), active: false });
    await page.reload();
    await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
    await expect(page.getByTestId("new-price-0")).toHaveValue("80");
    await expect(page.getByTestId("matrix-active-0")).toBeChecked();
    expect(await page.getByTestId("matrix-code-0").inputValue()).not.toBe(
      archivedCode,
    );
    await evidence(page, "archive-price", {
      page: "product edit",
      control: "bulk price / Active / save",
      action: "real clicks then reload",
      expected: "200 once; archive-only row; remaining price80",
      tier: "REAL_PG+BROWSER",
    });
  });

  const cases = (["create", "copy"] as const).flatMap((action) =>
    (["client", "401", "403"] as const).map((denial) => ({ action, denial })),
  );
  const stagedCases = ["order", "publish"].map((action) => ({
    action,
    denial: "403",
  }));
  for (const { action, denial } of [...cases, ...stagedCases]) {
    test(`review: ${action} UNKNOWN then ${denial} preserves receipt fence after reload`, async ({
      page,
      context,
    }) => {
      const name = `review-${process.env.LC_BROWSER_TAG}-${action}-${denial}`;
      await openNew(page, name);
      if (action === "order" || action === "publish") {
        const png = Buffer.from(
          "iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB" +
            "CAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
          "base64",
        );
        await page.getByTestId("photo-input").setInputFiles({
          name: "review.png",
          mimeType: "image/png",
          buffer: png,
        });
      }
      let id = "";
      if (action === "copy") {
        await page.getByTestId("product-create").click();
        id = await openSaved(page);
        await page.getByTestId("product-back").click();
      }
      const endpoint =
        action === "copy"
          ? `products/${id}/copy`
          : action === "order"
            ? "products/*/images/order"
            : action === "publish"
              ? "products/bulk-status"
              : "products/document";
      const pattern = `**/api/stores/${store()}/${endpoint}`;
      const requests: { key: string; body: string | null }[] = [];
      let committed = 0;
      await page.route(pattern, async (route) => {
        requests.push({
          key: route.request().headers()["idempotency-key"],
          body: route.request().postData(),
        });
        if (requests.length === 1) {
          const response = await route.fetch();
          expect(response.status()).toBe(200);
          committed++;
          await route.abort("failed");
        } else {
          await route.fulfill({
            status: Number(denial),
            contentType: "text/plain",
            body: "Synthetic authorization refusal",
          });
        }
      });
      const trigger = () =>
        action === "copy"
          ? page
              .getByTestId(`product-row-${id}`)
              .getByRole("button", { name: "Copy", exact: true })
          : page.getByTestId(
              action === "publish" ? "product-publish" : "product-create",
            );
      await trigger().click();
      const retry = page.getByRole("button", {
        name: productEditorCopy.en.retry,
        exact: true,
      });
      await expect(retry).toBeVisible();
      if (action !== "copy") await expect(retry).toBeInViewport();
      const fenceKey =
        action === "copy"
          ? `catalog-command-pending:${store()}`
          : `product-document-pending:${store()}`;
      const originalKey = await page.evaluate(
        (key) => sessionStorage.getItem(key),
        fenceKey,
      ); // read-only receipt inspection
      expect(originalKey).toMatch(/^[0-9a-f-]{36}$/);
      if (action === "create" || action === "copy")
        expect(originalKey).toBe(requests[0].key);
      if (denial === "client") {
        // Fault injection simulates auth changing in another tab; no DOM or app state is overwritten.
        const cookie = (await context.cookies()).find(
          (c) => c.name === "__Host-commerce_csrf",
        )!;
        await context.addCookies([{ ...cookie, value: "z".repeat(43) }]);
      }
      await retry.click();
      const recovery =
        action === "copy"
          ? productEditorCopy.en.listRecoveryRequired
          : productEditorCopy.en.recoveryRequired;
      await expect(page.getByText(recovery, { exact: true })).toBeVisible();
      if (action !== "copy") await expect(page.getByText(recovery, { exact: true })).toBeInViewport();
      await expect(retry).toHaveCount(0);
      expect(
        await page.evaluate((key) => sessionStorage.getItem(key), fenceKey),
      ).toBe(originalKey);
      await page.reload();
      await expect(page.getByText(recovery, { exact: true })).toBeVisible();
      if (action !== "copy") await expect(page.getByText(recovery, { exact: true })).toBeInViewport();
      await expect(trigger()).toBeDisabled();
      expect(
        await page.evaluate((key) => sessionStorage.getItem(key), fenceKey),
      ).toBe(originalKey);
      expect(committed).toBe(1);
      expect(requests).toHaveLength(denial === "client" ? 1 : 2);
      for (const request of requests) expect(request).toEqual(requests[0]);
      await evidence(page, `receipt-${action}-${denial}`, {
        page: action,
        control: "save/copy → retry → reload",
        denial,
        expected: "one commit, original key retained, fresh write disabled",
        requests: requests.length,
        committed,
        tier: "FAULT_INJECTED+REAL_PG",
      });
    });
  }
}
