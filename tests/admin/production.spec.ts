import { test, expect } from "@playwright/test";

test("production build has no env-bearer merchant session", async ({
  page,
  request,
}) => {
  await page.goto("http://127.0.0.1:3101/en");
  await expect(page.locator('.message[role="alert"]')).toContainText(
    "Sign-in is not connected",
  );
  await expect(
    page.getByRole("button", { name: "Add product", exact: true }),
  ).toBeDisabled();
  await expect(page.locator("tbody tr")).toHaveCount(0);
  const response = await request.get(
    "http://127.0.0.1:3101/api/stores/00000000-0000-0000-0000-000000000001/products",
  );
  expect(response.status()).toBe(401);
  expect(response.headers()["cache-control"]).toContain("no-store");
  expect((await response.json()).code).toBe("unauthorized");
});
