// Purpose: PM-U real-upload browser acceptance spec (unit docs/delivery/units/product-media-v2.md UI + Acceptance;
//   contracts/catalog-inventory-v1.md "Amendment — product-media-v2"). Real clicks/file choosers only: the merchant
//   uploads 4 main + 2 option (Color Red/Blue) + 6 detail images — including a 750x4000 tall detail and a > 2 MiB
//   EXIF-orientation-6 JPEG the UI must downsize, not refuse — publishes, and buyers (zh-TW/en at1440/390) see exactly
//   4 main thumbnails, the variant image swap, the lazy detail stack without horizontal overflow, the option image in
//   the cart line, and reach the checkout delivery/payment step without placing an order. A seeded 9-image product in
//   POST-migration layout (4 main + 5 detail) is only rendered; the upgrade itself is REAL_PG
//   (TestPMv2MigrationNineImages).
// Depends on: production admin/storefront Next, signed MOCK IdP, real BFF -> Go -> PG; harness env
//   LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_EVIDENCE, LC_PM_PROXY, LC_PM_BUYER_ORIGIN, LC_PM_FILES, LC_PM_MANIFEST;
//   storefront copy modules for button names (purchase-copy, bank-transfer-copy).
// Used by: tests/foundation/browser_product_media_v2_test.go (TestBrowserProductMediaV2RealUpload), which owns every
//   process, the buyer.example TLS edge + CONNECT proxy, and the PG readback. Suite "product-media-v2" in
//   playwright.config.ts and --browser-product-media-v2 in scripts/dev/test-local.sh are integrator wiring.
// Status: BROWSER + REAL_PG evidence; MOCK IdP, synthetic TLS edge; no provider, no order placement, no payment.
import { expect, test, type Locator, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { purchaseCopy } from "../../apps/storefront/lib/purchase-copy";
import { bankTransferCopy } from "../../apps/storefront/lib/bank-transfer-copy";
import { productMediaCopy } from "../../apps/admin/lib/product-media-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value)
    throw new Error(`${name} is required (the Go gate harness sets it)`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const buyerOrigin = required("LC_PM_BUYER_ORIGIN");
const filesDir = required("LC_PM_FILES");

type ManifestFile = {
  name: string;
  width: number;
  height: number;
  bytes: number;
  sha256: string;
  fit_width: number;
  fit_height: number;
  exif: boolean;
  missing: boolean;
};
type Upload = {
  role: "main" | "detail" | "sku";
  option_value: string;
  name: string;
  id: string;
  content_type: string;
  size_bytes: number;
  width: number;
  height: number;
};
const manifest = JSON.parse(required("LC_PM_MANIFEST")) as {
  store: string;
  product: {
    id: string;
    slug: string;
    name: string;
    skus: Record<string, string>;
  };
  migrated: {
    id: string;
    slug: string;
    name: string;
    main: string[];
    detail: string[];
  };
  files: {
    main: ManifestFile[];
    detail: ManifestFile[];
    sku: Record<string, ManifestFile>;
  };
};
const store = manifest.store,
  product = manifest.product,
  migrated = manifest.migrated;

test.use({
  baseURL: origin,
  ignoreHTTPSErrors: true, // the buyer.example edge certificate is self-signed (synthetic edge)
  // Only buyer.example goes through the proxy; the loopback admin and IdP stay direct.
  launchOptions: {
    proxy: { server: required("LC_PM_PROXY"), bypass: "127.0.0.1" },
  },
});
test.setTimeout(480_000);

// The merchant phase fills these; the buyer phases read them (serial, workers: 1).
const uploaded: Upload[] = [];
const byRole = (role: Upload["role"]) =>
  uploaded.filter((u) => u.role === role);
const pageErrors: string[] = [];
const ledger: {
  page: string;
  control: string;
  action: string;
  expect: string;
  actual: string;
  pass: boolean;
}[] = [];
let cases = 0;
function pass(
  pageName: string,
  control: string,
  action: string,
  expected: string,
  actual: string,
) {
  cases++;
  ledger.push({
    page: pageName,
    control,
    action,
    expect: expected,
    actual,
    pass: true,
  });
  console.log(`PASS [${pageName}] ${control} ${action}: ${actual}`);
}
const shots: { file: string; sha256: string; page: string }[] = [];
async function shot(page: Page, name: string) {
  const file = path.join(evidence, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true, animations: "disabled" });
  shots.push({
    file: path.basename(file),
    sha256: createHash("sha256")
      .update(await readFile(file))
      .digest("hex"),
    page: name,
  });
}

// Real file-chooser upload: click the visible trigger (the input when rendered, otherwise its wrapping label) and
// answer the chooser with files the Go fixture generated on disk. setInputFiles on a hidden input is NOT used: the
// owner requires the click/file-chooser path.
async function pickFiles(
  page: Page,
  scope: Page | Locator,
  testid: string,
  names: string[],
) {
  const input = scope.getByTestId(testid);
  await expect(input, `${testid} exists and is enabled`).toBeEnabled();
  const chooser = page.waitForEvent("filechooser");
  const target = (await input.isVisible()) ? input : input.locator("xpath=..");
  await target.click();
  await (await chooser).setFiles(names.map((n) => path.join(filesDir, n)));
}

// Upload `files` through one picker and observe every upload response (owned id/type/bytes/dimensions).
async function upload(
  page: Page,
  scope: Page | Locator,
  testid: string,
  role: Upload["role"],
  optionValue: string,
  files: ManifestFile[],
) {
  const imagesPath = `/api/stores/${store}/products/${product.id}/images`;
  // One chooser per file keeps response identity unambiguous; equal wait predicates would all observe the first response.
  for (const f of files) {
    const waiting = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        new URL(r.url()).pathname === imagesPath,
    );
    await pickFiles(page, scope, testid, [f.name]);
    const response = await waiting;
    expect(response.status(), `upload ${f.name} accepted`).toBe(200);
    const body = (await response.json()) as {
      id: string;
      role: string;
      content_type: string;
      size_bytes: number;
      width: number | null;
      height: number | null;
    };
    expect(body.role, `upload ${f.name} role`).toBe(role);
    expect(
      body.size_bytes,
      `upload ${f.name} fits the 2 MiB cap`,
    ).toBeLessThanOrEqual(2 * 1024 * 1024);
    if (f.bytes <= 2 * 1024 * 1024 && Math.max(f.width, f.height) <= 2000) {
      expect(body.content_type, `${f.name} keeps its original format`).toBe(
        f.name.endsWith(".png") ? "image/png" : "image/jpeg",
      );
      expect(body.size_bytes, `${f.name} keeps its original bytes`).toBe(
        f.bytes,
      );
    }
    if (f.exif) {
      // The > 2 MiB EXIF-orientation-6 photo is downsized before upload, never refused: JPEG, portrait (rotation
      // applied), longest side <= 2000. First-fit stops at exactly 1500x2000 for this fixture's entropy.
      expect(body.content_type, "EXIF photo normalized to JPEG").toBe(
        "image/jpeg",
      );
      expect(
        [body.width, body.height],
        "EXIF photo rotated to portrait and fitted to 1500x2000",
      ).toEqual([1500, 2000]);
      expect(
        body.size_bytes,
        "EXIF photo was downsized, not refused",
      ).toBeLessThan(f.bytes);
    } else {
      expect(
        [body.width, body.height],
        `upload ${f.name} stored dimensions`,
      ).toEqual([f.fit_width, f.fit_height]);
    }
    uploaded.push({
      role,
      option_value: optionValue,
      name: f.name,
      id: body.id,
      content_type: body.content_type,
      size_bytes: body.size_bytes,
      width: body.width ?? 0,
      height: body.height ?? 0,
    });
    pass(
      "admin editor (zh-TW/1440)",
      testid,
      `file chooser: ${f.name}`,
      f.exif
        ? `> 2 MiB EXIF-6 photo downsized to a portrait JPEG 1500x2000 <= 2 MiB (never refused)`
        : `${role} upload 200 with the fitted ${f.fit_width}x${f.fit_height} dimensions`,
      `${body.id} ${body.content_type} ${body.size_bytes}B ${body.width}x${body.height}${optionValue ? ` (${optionValue})` : ""}`,
    );
  }
}

async function noOverflow(page: Page, label: string) {
  // [READ/MEASURE] layout assertion only; no state change.
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
    `${label} has no horizontal overflow`,
  ).toBe(true);
}

const completedBuyer: { locale: string; viewport: string }[] = [];
test.describe("PM-U product media v2", () => {
  // Buyer assertions consume the merchant's real uploaded IDs; never restart them as independent fixtures.
  test.describe.configure({ mode: "serial", timeout: 240_000 });
  test("merchant uploads 4 main + 2 option + 6 detail images with real file choosers and publishes", async ({
    page,
  }) => {
    const name = "admin editor (zh-TW/1440)";
    await page.setViewportSize({ width: 1440, height: 900 });
    page.on("pageerror", (error) =>
      pageErrors.push(`merchant ${error.name}: ${error.message}`),
    );
    await page.goto(`${origin}/en`);
    await page
      .getByRole("button", {
        name: "Sign in with identity service",
        exact: true,
      })
      .click();
    await page.waitForURL(`${origin}/**`); // back from the signed MOCK IdP with a merchant session
    pass(
      name,
      "sign-in",
      "click",
      "MOCK IdP round trip",
      "session established",
    );
    await page.goto(`${origin}/zh-TW/products/${product.id}?store=${store}`);
    const mainList = page.getByTestId("media-main-list");
    const detailList = page.getByTestId("media-detail-list");
    const skuList = page.getByTestId("media-sku-list");
    await expect(mainList, "main role list renders").toBeVisible();
    await expect(detailList, "detail role list renders").toBeVisible();
    const axisPicker = page.getByTestId("media-axis-picker");
    await expect(axisPicker, "image axis picker renders").toBeVisible();
    await expect(
      axisPicker.locator("option", { hasText: "Color" }),
      "the Color axis is selectable",
    ).toHaveCount(1);
    pass(
      name,
      "media sections",
      "render",
      "main/detail lists and axis picker",
      "all visible, Color axis offered",
    );

    await upload(page, page, "photo-input", "main", "", manifest.files.main);
    await expect(
      mainList.locator("img"),
      "4 main thumbnails after upload",
    ).toHaveCount(4);
    pass(
      name,
      "photo-input",
      "file chooser x4",
      "4 main images 200 + listed",
      `${byRole("main").length} uploaded`,
    );

    for (const value of ["Red", "Blue"]) {
      const cell = page.locator(`[data-option-value="${value}"]`);
      await expect(cell, `option cell for ${value}`).toBeVisible();
      await upload(page, cell, "photo-input-sku", "sku", value, [
        manifest.files.sku[value],
      ]);
    }
    await expect(
      skuList.locator("img"),
      "2 option images after upload",
    ).toHaveCount(2);
    pass(
      name,
      "photo-input-sku",
      "file chooser x2",
      "Red and Blue option images 200 + linked",
      `${byRole("sku").length} uploaded`,
    );

    await upload(
      page,
      page,
      "photo-input-detail",
      "detail",
      "",
      manifest.files.detail.filter((f) => !f.missing),
    );
    if (manifest.files.detail.some((f) => f.missing))
      console.log(
        "PM_CALIBRATION_DETAIL_COUNT: five real uploads; six required",
      );
    await expect(
      detailList.locator("img"),
      "6 detail thumbnails after upload",
    ).toHaveCount(6);
    const exif = byRole("detail").find((u) => u.name === "detail-2-exif.jpg")!;
    pass(
      name,
      "photo-input-detail",
      "file chooser x6",
      "6 detail images 200; EXIF photo downsized",
      `6 uploaded; EXIF stored ${exif.width}x${exif.height} ${exif.size_bytes}B`,
    );

    const published = page.waitForResponse(
      (r) =>
        r.request().method() === "PUT" &&
        new URL(r.url()).pathname ===
          `/api/stores/${store}/products/${product.id}/document`,
    );
    await page.getByTestId("product-publish").click();
    expect((await published).status(), "publish through document update").toBe(
      200,
    );
    await expect(
      page.getByTestId("product-message"),
      "publish feedback",
    ).toBeVisible();
    pass(
      name,
      "product-publish",
      "click",
      "product becomes active",
      "document PUT 200 + feedback",
    );
    // Persistence: after a full reload the server state is what the editor shows.
    await page.reload();
    await expect(
      page.getByTestId("product-status"),
      "status persists after reload",
    ).toHaveValue("active");
    await expect(
      page.getByTestId("media-main-list").locator("img"),
      "main list persists after reload",
    ).toHaveCount(4);
    await expect(
      page.getByTestId("media-detail-list").locator("img"),
      "detail list persists after reload",
    ).toHaveCount(6);
    await expect(
      page.getByTestId("media-sku-list").locator("img"),
      "option list persists after reload",
    ).toHaveCount(2);
    pass(
      name,
      "editor reload",
      "reload",
      "uploads and status survive",
      "4/6/2 images + active",
    );
    // Move the published product's last main image away through real buttons, then restore its original main order.
    // This covers the backend's legal active/zero-main state without a fixture-only API shortcut.
    const mediaCopy = productMediaCopy["zh-TW"];
    const mainIDs = byRole("main").map((image) => image.id);
    async function moveImage(
      id: string,
      from: "main" | "detail",
      label: string,
    ) {
      const response = page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          new URL(r.url()).pathname ===
            `/api/stores/${store}/products/${product.id}/images/${id}/move`,
      );
      await page
        .getByTestId(`media-${from}-list`)
        .locator(`[data-image-id="${id}"]`)
        .getByRole("button", { name: label, exact: true })
        .click();
      expect((await response).status(), "role move accepted").toBe(200);
      await expect(
        page
          .getByTestId(`media-${from === "main" ? "detail" : "main"}-list`)
          .locator(`[data-image-id="${id}"]`),
        "canonical destination contains the moved image",
      ).toHaveCount(1);
      pass(
        name,
        "move image role",
        "click",
        `${from} image moves to the other section`,
        id,
      );
    }
    for (const id of mainIDs) await moveImage(id, "main", mediaCopy.toDetail);
    await expect(
      page.getByTestId("product-main-required"),
      "active zero-main inline repair hint",
    ).toHaveText(mediaCopy.activeMainMissing);
    await expect(page.getByTestId("product-main-required")).toBeVisible();
    await expect(page.getByTestId("product-main-required")).not.toHaveAttribute(
      "role",
      /^(alert|status)$/,
    );
    await expect(
      page.getByTestId("product-save"),
      "saving as active is blocked",
    ).toBeDisabled();
    await expect(
      page.getByTestId("product-publish"),
      "publishing is blocked",
    ).toBeDisabled();
    await expect(
      page.getByTestId("photo-input"),
      "upload repair remains possible",
    ).toBeEnabled();
    await expect(
      page.getByTestId("product-unpublish"),
      "draft repair remains possible",
    ).toBeEnabled();
    await shot(page, "merchant-active-zero-main");
    for (const id of mainIDs) await moveImage(id, "detail", mediaCopy.toMain);
    await expect(
      page.getByTestId("product-main-required"),
      "main restoration clears the hint",
    ).toHaveCount(0);
    await expect(page.getByTestId("product-save")).toBeEnabled();
    await expect(page.getByTestId("product-publish")).toBeEnabled();
    await expect(
      page.getByTestId("media-main-list").locator("img"),
    ).toHaveCount(4);
    await expect(
      page.getByTestId("media-detail-list").locator("img"),
    ).toHaveCount(6);
    pass(
      name,
      "active-zero-main recovery",
      "move and restore",
      "blocking hint and active intents recover; upload/unpublish remain usable",
      "four mains restored",
    );
    await shot(page, "merchant-editor");
  });

  for (const cell of [
    { locale: "zh-TW", width: 1440, height: 900, viewport: "1440" },
    { locale: "zh-TW", width: 390, height: 844, viewport: "390" },
    { locale: "en", width: 1440, height: 900, viewport: "1440" },
    { locale: "en", width: 390, height: 844, viewport: "390" },
  ] as const) {
    test(`buyer ${cell.locale}@${cell.viewport}: 4 main thumbs, Blue option image, lazy detail stack, cart and checkout`, async ({
      page,
    }) => {
      const name = `storefront (${cell.locale}/${cell.viewport})`;
      await page.setViewportSize({ width: cell.width, height: cell.height });
      page.on("pageerror", (error) =>
        pageErrors.push(`${name} ${error.name}: ${error.message}`),
      );
      expect(byRole("main"), "merchant phase ran first").toHaveLength(4);
      expect(byRole("detail"), "merchant phase ran first").toHaveLength(6);
      expect(byRole("sku"), "merchant phase ran first").toHaveLength(2);
      const blue = byRole("sku").find((u) => u.option_value === "Blue")!;
      await page.goto(`${buyerOrigin}/${cell.locale}/products/${product.slug}`);
      await expect(
        page.getByTestId("product-main-thumbnail"),
        "exactly 4 main thumbnails",
      ).toHaveCount(4);
      const active = page.getByTestId("product-gallery-active-image");
      await expect(
        active,
        "initial selected Red variant uses its option image",
      ).toHaveAttribute(
        "data-image-id",
        byRole("sku").find((u) => u.option_value === "Red")!.id,
      );
      await page.getByTestId("product-main-thumbnail").first().click();
      await expect(active, "main thumbnail restores the cover").toHaveAttribute(
        "data-image-id",
        byRole("main")[0].id,
      );
      pass(
        name,
        "product gallery",
        "render",
        "4 main thumbnails, cover active",
        "4 thumbs, cover = main[0]",
      );

      await page
        .locator("label.sf-chip", {
          has: page.getByRole("radio", { name: "Blue", exact: true }),
        })
        .click();
      await expect(
        active,
        "choosing Blue swaps the gallery to the Blue option image",
      ).toHaveAttribute("data-image-id", blue.id);
      pass(
        name,
        "Blue variant chip",
        "click",
        "active image = Blue option image",
        blue.id,
      );

      const details = page.getByTestId("product-detail-image");
      await expect(details, "6 detail images in the stack").toHaveCount(6);
      for (const [i, detail] of (await details.all()).entries()) {
        await detail.scrollIntoViewIfNeeded();
        // [READ/MEASURE] naturalWidth is a decoded-bytes measurement; no state change.
        await expect
          .poll(
            () =>
              detail.evaluate((e: HTMLImageElement) =>
                e.complete ? e.naturalWidth : 0,
              ),
            `detail ${i} decoded`,
          )
          .toBeGreaterThan(0);
        expect(
          await detail.getAttribute("loading"),
          `detail ${i} is lazy`,
        ).toBe("lazy");
        expect(
          Number(await detail.getAttribute("width")),
          `detail ${i} has width`,
        ).toBeGreaterThan(0);
        expect(
          Number(await detail.getAttribute("height")),
          `detail ${i} has height`,
        ).toBeGreaterThan(0);
        pass(
          name,
          `product-detail-image[${i}]`,
          "scrollIntoView",
          "lazy, width+height set, decoded",
          `${await detail.getAttribute("width")}x${await detail.getAttribute("height")}`,
        );
      }
      pass(
        name,
        "detail stack",
        "scroll all",
        "6 lazy images with width/height, all decode",
        "6/6",
      );
      await noOverflow(page, name);
      await shot(page, `buyer-product-${cell.locale}-${cell.viewport}`);

      await page.getByTestId("add-to-cart").click();
      const line = page.locator(
        `[data-testid="cart-line"][data-sku="${product.skus.Blue}"]`,
      );
      await expect(line, "cart line for the Blue SKU").toHaveCount(1);
      await expect(
        line.locator("img"),
        "cart line shows the Blue option image",
      ).toHaveAttribute("src", `/media/p/${product.id}/${blue.id}`);
      pass(
        name,
        "add-to-cart",
        "click",
        "cart line uses the option image",
        blue.id,
      );

      await page.getByTestId("cart-checkout").click();
      await page.waitForURL(`**/${cell.locale}/checkout`);
      const purchase = purchaseCopy[cell.locale],
        bank = bankTransferCopy[cell.locale];
      await page
        .getByRole("button", { name: purchase.delivery, exact: true })
        .click();
      // The first available method is auto-selected; the buyer still makes the choice explicit.
      const deliverySelect = page.locator("select#delivery");
      await expect(
        deliverySelect,
        "delivery method select after Choose delivery",
      ).toBeVisible();
      const firstMethod = await deliverySelect
        .locator("option:not([disabled])")
        .first()
        .getAttribute("value");
      expect(
        firstMethod,
        "at least one available delivery method",
      ).toBeTruthy();
      await deliverySelect.selectOption(firstMethod!);
      pass(
        name,
        "delivery method",
        "selectOption",
        "a home-delivery method is chosen",
        firstMethod!,
      );
      await page
        .getByRole("button", { name: purchase.quote, exact: true })
        .click();
      await expect(
        page.getByTestId("address-section"),
        "delivery/address step reached",
      ).toBeVisible();
      await expect(
        page.getByTestId("home-payment-mode"),
        "payment step reached",
      ).toBeVisible();
      await expect(
        page.getByRole("radio", { name: bank.payBank }),
        "bank transfer is an enabled payment choice",
      ).toBeEnabled();
      // The gate stops here: no confirm, no create-order, no payment.
      expect(
        await page.getByTestId("order-section").count(),
        "no order was placed",
      ).toBe(0);
      await noOverflow(page, `${name} checkout`);
      await shot(page, `buyer-checkout-${cell.locale}-${cell.viewport}`);
      pass(
        name,
        "checkout",
        "delivery -> quote",
        "payment choices visible, no order placed",
        bank.payBank,
      );
      completedBuyer.push({ locale: cell.locale, viewport: cell.viewport });
    });
  }

  test("migrated 9-image product (post-migration layout fixture) renders 4 main + 5 detail", async ({
    page,
  }) => {
    const name = "storefront migrated (zh-TW/1440)";
    await page.setViewportSize({ width: 1440, height: 900 });
    page.on("pageerror", (error) =>
      pageErrors.push(`${name} ${error.name}: ${error.message}`),
    );
    // Post-migration LAYOUT only: the rows were seeded as 4 main + 5 detail; the 0149 upgrade path is REAL_PG.
    await page.goto(`${buyerOrigin}/zh-TW/products/${migrated.slug}`);
    await expect(
      page.getByTestId("product-main-thumbnail"),
      "exactly 4 main thumbnails",
    ).toHaveCount(4);
    await expect(
      page.getByTestId("product-gallery-active-image"),
      "cover = migrated main[0]",
    ).toHaveAttribute("data-image-id", migrated.main[0]);
    const details = page.getByTestId("product-detail-image");
    await expect(details, "5 detail images").toHaveCount(5);
    for (const [i, detail] of (await details.all()).entries()) {
      await detail.scrollIntoViewIfNeeded();
      await expect
        .poll(
          () =>
            detail.evaluate((e: HTMLImageElement) =>
              e.complete ? e.naturalWidth : 0,
            ),
          `migrated detail ${i} decoded`,
        )
        .toBeGreaterThan(0);
      pass(
        name,
        `product-detail-image[${i}]`,
        "scrollIntoView",
        "migrated detail decodes",
        "decoded",
      );
    }
    await noOverflow(page, name);
    await shot(page, "buyer-migrated");
    pass(
      name,
      "migrated product",
      "render",
      "4 main + 5 detail render without error",
      "4+5",
    );
  });
});

test.afterAll(async () => {
  if (pageErrors.length)
    console.log(`PAGE ERRORS: ${JSON.stringify(pageErrors)}`);
  await writeFile(
    path.join(evidence, "screenshots.json"),
    JSON.stringify(shots, null, 2),
    { mode: 0o600 },
  );
  await writeFile(
    path.join(evidence, "click-ledger.json"),
    JSON.stringify(ledger, null, 2),
    { mode: 0o600 },
  );
  await writeFile(
    path.join(evidence, "result.json"),
    JSON.stringify(
      {
        cases,
        page_errors: pageErrors,
        uploads: uploaded,
        buyer: completedBuyer,
        boundary:
          "production Next builds; signed MOCK IdP; synthetic buyer.example TLS edge; fixture-scope synthetic coordinates only; no order placement, no payment, no provider",
      },
      null,
      2,
    ),
    { mode: 0o600 },
  );
  expect(pageErrors, "no uncaught page errors in any phase").toEqual([]);
});
