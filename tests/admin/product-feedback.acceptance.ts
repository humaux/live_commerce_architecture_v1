// R6 repeated validation: actual signed merchant UI, no DOM/style injection.
// Both Publish clicks must stop before any product-document mutation request.
import { expect, test } from "@playwright/test";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";

export function registerProductFeedbackAcceptance() {
  for (const viewport of [
    { width: 1586, height: 992 },
    { width: 390, height: 844 },
  ]) {
    test(`feedback: identical image-required validation reveals again at ${viewport.width}px`, async ({
      page,
    }) => {
      const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!,
        store = process.env.LC_BROWSER_STORE!,
        c = productEditorCopy.en;
      const documentMutations: { method: string; path: string }[] = [];
      page.on("request", (request) => {
        const path = new URL(request.url()).pathname;
        if (
          request.method() !== "GET" &&
          /^\/api\/stores\/[^/]+\/products(?:\/[^/]+)?\/document$/.test(path)
        )
          documentMutations.push({ method: request.method(), path });
      });
      await page.setViewportSize(viewport);
      await page.goto(`${origin}/en/`);
      await page
        .getByRole("button", { name: "Sign in with identity service" })
        .click();
      await expect(page.getByTestId("nav-orders")).toBeAttached();
      await page.goto(`${origin}/en/products?store=${store}`);
      await page.getByTestId("product-new").click();
      const form = page.getByTestId("product-create-form");
      await expect(form).toBeVisible();
      await form
        .getByTestId("product-name")
        .fill(`feedback-${process.env.LC_BROWSER_TAG}-${viewport.width}`);
      await form.getByTestId("product-price").fill("60");
      await form.getByTestId("product-untracked").check();
      await form.getByTestId("product-max").fill("3");
      const publish = form.getByTestId("product-publish"),
        message = form.getByTestId("product-message");
      await expect(publish).toBeEnabled();

      await publish.click();
      await expect(message).toHaveText(c.imageRequired);
      await expect(message).toBeVisible();
      await expect(message).toBeInViewport({ ratio: 1 });
      expect(
        documentMutations,
        "first validation makes no document write",
      ).toEqual([]);

      await form
        .getByRole("navigation", { name: c.progress })
        .getByRole("button", { name: c.basics, exact: true })
        .click();
      await expect
        .poll(() =>
          form
            .locator("#basics")
            .evaluate((section) => section.contains(document.activeElement)),
        )
        .toBe(true);
      // Prove the second click needs a new reveal rather than inheriting the
      // first outcome's scroll position. Do not edit or dismiss its message.
      await expect(message).not.toBeInViewport();
      await publish.click();
      await expect(message).toHaveText(c.imageRequired);
      await expect(message).toBeVisible();
      await expect(message).toBeInViewport({ ratio: 1 });
      expect(
        documentMutations,
        "both repeated validations make no document write",
      ).toEqual([]);
    });
  }
}
