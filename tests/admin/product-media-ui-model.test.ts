// Purpose: PM-U role/query and camera resize contracts, including hostile query variants.
// Depends on: Node test/assert/vm, React server rendering, typescript-api and product-media-model plus backend caps.
// Used by: local focused Node gate and media UI acceptance; no browser/provider.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { productMediaCopy } from "../../apps/admin/lib/product-media-copy.ts";
import {
  fitPhoto,
  validMediaQuery,
  mediaUploadPath,
  effectiveMediaAxis,
  mainPhotoCount,
  mediaCaps,
  mediaRecoveryKey,
  optionImageMatches,
  persistMediaPending,
  needsMainImage,
} from "../../apps/admin/lib/product-media-model.ts";
const uuid = "11111111-1111-4111-8111-111111111111";
test("main/detail/sku caps use the existing frozen image contract", () => {
  assert.deepEqual(mediaCaps, { main: 4, detail: 20, sku: 50 });
});
test("phone and tall dimensions fit2000 without enlarging small assets", () => {
  assert.deepEqual(fitPhoto(750, 4000), { width: 375, height: 2000 });
  assert.deepEqual(fitPhoto(3000, 2000), { width: 2000, height: 1333 });
  assert.deepEqual(fitPhoto(800, 800), { width: 800, height: 800 });
  for (const [w, h] of [
    [0, 20],
    [-1, 200],
    [NaN, 400],
    [100, Infinity],
  ])
    assert.throws(() => fitPhoto(w, h));
});
test("upload query requires one exact role and sku value; reads/actions carry no query", () => {
  const valid = [
    "",
    "?role=main",
    "?role=detail",
    "?role=sku&option_value=%E8%97%8D%E8%89%B2",
  ];
  for (const q of valid) assert.equal(validMediaQuery(q, true), true, q);
  for (const q of [
    "?",
    "?role=evil",
    "?role=main&role=main",
    "?role=sku",
    "?role=main&option_value=Red",
    "?role=sku&option_value=",
    "?role=sku&option_value=Red&tenant_id=" + uuid,
    "?role=sku&option_value=%00",
    "?role=sku&option_value=%ZZ",
  ])
    assert.equal(validMediaQuery(q, true), false, q);
  assert.equal(validMediaQuery("", false), true);
  assert.equal(validMediaQuery("?role=main", false), false);
});
test("role transport encodes current option values and refuses invalid identities", () => {
  assert.equal(
    mediaUploadPath(uuid, uuid, "detail"),
    `/api/stores/${uuid}/products/${uuid}/images?role=detail`,
  );
  assert.equal(
    new URL(
      mediaUploadPath(uuid, uuid, "sku", "Blue & white"),
      "https://a.invalid",
    ).searchParams.get("option_value"),
    "Blue & white",
  );
  assert.throws(() => mediaUploadPath("bad", uuid, "main"));
  assert.throws(() => mediaUploadPath(uuid, uuid, "sku"));
});
test("effective image axis follows current options, then falls back to first", () => {
  const axes = [
    { name: "Color", values: ["Red", "Blue"] },
    { name: "Size", values: ["S", "M"] },
  ];
  assert.equal(effectiveMediaAxis(axes, "Size")?.name, "Size");
  assert.equal(effectiveMediaAxis(axes, "gone")?.name, "Color");
  assert.equal(effectiveMediaAxis([], null), null);
});

test("detail and option photos cannot satisfy the main-photo recommendation", () => {
  assert.equal(
    mainPhotoCount([
      { role: "main" },
      { role: "main" },
      ...Array.from({ length: 20 }, () => ({ role: "detail" })),
      { role: "sku" },
    ]),
    2,
  );
  assert.equal(mainPhotoCount([{}, {}, {}]), 3);
});

test("PM-U copy has matching keys in ja and all currently routed locales", () => {
  const keys = Object.keys(productMediaCopy.en).sort();
  for (const locale of ["ja", "zh-TW", "zh-CN"] as const)
    assert.deepEqual(
      Object.keys(productMediaCopy[locale]).sort(),
      keys,
      locale,
    );
  assert.match(productMediaCopy.ja.mainHelp, /4/);
  assert.match(productMediaCopy.ja.detailHelp, /20/);
});

test("real-upload mode builds both Next applications on a clean CI checkout", () => {
  const source = readFileSync(
    new URL("../../scripts/dev/test-local.sh", import.meta.url),
    "utf8",
  );
  for (const app of ["admin", "storefront"]) {
    const block = [
      ...source.matchAll(
        /if (\[\[ [^\n]+ \]\]); then\n((?:(?!\nfi)[\s\S])*?)\nfi/g,
      ),
    ].find(
      (match) =>
        match[2].includes("pnpm run build:" + app) &&
        !/\bexit 0\b/.test(match[2]),
    );
    assert.ok(block, app + " build predicate exists");
    const answer = execFileSync(
      "bash",
      [
        "-c",
        `test_mode="$1"; if ${block[1]}; then printf yes; else printf no; fi`,
        "--",
        "--browser-product-media-v2",
      ],
      { encoding: "utf8" },
    );
    assert.equal(
      answer,
      "yes",
      app + " must be prepared before readiness probes",
    );
  }
  assert.match(
    source,
    /LC_BROWSER_PRODUCT_MEDIA_V2_ACCEPTANCE=1[^\n]+TestBrowserProductMediaV2RealUpload/,
  );
});

test("upload recovery journal survives a new session while remaining product/store scoped", () => {
  assert.equal(
    mediaRecoveryKey(uuid, uuid),
    `product-media-pending:${uuid}:${uuid}`,
  );
  assert.notEqual(
    mediaRecoveryKey(uuid, uuid),
    mediaRecoveryKey(uuid, "22222222-2222-4222-8222-222222222222"),
  );
  assert.throws(() => mediaRecoveryKey("bad", uuid));
});
test("an option upload receipt requires its captured axis, value and image id on canonical readback", () => {
  const links = [{ option_name: "Size", option_value: "Blue", image_id: uuid }];
  assert.equal(optionImageMatches("Size", links, "Color", "Blue", uuid), false);
  assert.equal(optionImageMatches("Size", links, "Size", "Blue", uuid), true);
  assert.equal(optionImageMatches("Size", links, "Size", "Red", uuid), false);
  assert.equal(
    optionImageMatches("Size", links, "Size", "Blue", "other"),
    false,
  );
});
test("failed durable recovery storage cannot authorize a media write", () => {
  const storage = {
    setItem() {
      throw new Error("quota");
    },
    removeItem() {
      throw new Error("disabled");
    },
  };
  assert.equal(persistMediaPending(storage, "journal", "key"), false);
  assert.equal(persistMediaPending(storage, "journal", null), false);
  const saved = new Map<string, string>();
  assert.equal(
    persistMediaPending(
      {
        setItem(k, v) {
          saved.set(k, v);
        },
        removeItem(k) {
          saved.delete(k);
        },
      },
      "journal",
      "key",
    ),
    true,
  );
  assert.equal(saved.get("journal"), "key");
});

test("an active product needs a main image after canonical media is known; repair paths stay possible", () => {
  assert.equal(
    needsMainImage("active", null),
    false,
    "pending read is not an empty media list",
  );
  assert.equal(needsMainImage("active", []), true);
  assert.equal(
    needsMainImage("active", [{ role: "detail" }, { role: "sku" }]),
    true,
  );
  assert.equal(needsMainImage("active", [{ role: "main" }]), false);
  assert.equal(
    needsMainImage("draft", []),
    false,
    "unpublish can repair an empty active product",
  );
  assert.equal(needsMainImage("archived", []), false);
});

const mediaRequire = createRequire(
  new URL("../../apps/admin/package.json", import.meta.url),
);
const mediaReact = mediaRequire("react"),
  mediaRenderer = mediaRequire("react-dom/server");
const mediaModule = { exports: {} as { ProductMediaTiles: unknown } };
runInNewContext(
  ts.transpileModule(
    readFileSync(
      new URL(
        "../../apps/admin/components/ProductMediaTiles.tsx",
        import.meta.url,
      ),
      "utf8",
    ),
    {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        jsx: ts.JsxEmit.ReactJSX,
      },
    },
  ).outputText,
  { module: mediaModule, exports: mediaModule.exports, require: mediaRequire },
);
test("empty media sections render a readable localized placeholder until photos are present", () => {
  for (const c of Object.values(productMediaCopy)) {
    for (const role of ["main", "detail"] as const) {
      const props = {
        role,
        photos: [],
        disabled: false,
        c,
        onMove() {},
        onDelete() {},
      };
      const empty = mediaRenderer.renderToStaticMarkup(
        mediaReact.createElement(mediaModule.exports.ProductMediaTiles, props),
      );
      assert.match(empty, /class="pm-empty"/);
      assert.match(empty, /<p[^>]*>[^<]+<\/p>/);
      const populated = mediaRenderer.renderToStaticMarkup(
        mediaReact.createElement(mediaModule.exports.ProductMediaTiles, {
          ...props,
          photos: [
            {
              key: "photo",
              url: "https://example.invalid/photo.png",
              width: 640,
              height: 640,
            },
          ],
        }),
      );
      assert.doesNotMatch(populated, /class="pm-empty"/);
      assert.match(populated, /data-image-id="photo"/);
    }
  }
});

test("real-upload mode never exits through the storefront MOCK shortcut", () => {
  const source = readFileSync(
    new URL("../../scripts/dev/test-local.sh", import.meta.url),
    "utf8",
  );
  const shortcut = source.match(
    /if (\[\[ [^\n]+ \]\]); then\n  # LC_SHOP_MOCK=1:/,
  );
  assert.ok(shortcut, "incumbent MOCK storefront branch exists");
  for (const [mode, mock, expected] of [
    ["--browser-product-media-v2", "0", "no"],
    ["--browser-product-media-v2", "1", "no"],
    ["--browser-storefront", "0", "no"],
    ["--browser-storefront", "1", "yes"],
  ]) {
    const actual: string = execFileSync(
      "bash",
      [
        "-c",
        `test_mode="$1"; LC_SHOP_MOCK="$2"; if ${shortcut[1]}; then printf yes; else printf no; fi`,
        "--",
        mode,
        mock,
      ],
      { encoding: "utf8" },
    );
    assert.equal(
      actual,
      expected,
      `${mode} MOCK=${mock}: only the explicit storefront shortcut may exit before PG`,
    );
  }
});
