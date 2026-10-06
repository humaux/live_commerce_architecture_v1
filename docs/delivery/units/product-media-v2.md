<!-- Purpose: unit brief for the 1688-style product media model (main images, SKU images, detail-page images).
Depends on: contracts/catalog (catalog v2), migrations 0082/0109/0111 (product_images, sizes), internal/catalog images*.go,
  apps/storefront product page, apps/admin product editor, Meta catalog feed (image_link).
Used by: PM-B (backend, Claude Sonnet) and PM-U (UI, Codex) implementers; integrator review. -->
# Unit brief — product-media-v2 (1688-style product images)

Owner decision 2026-10-06 (verbatim): 「我觉得可以不用什么需要9张图片，学习一下1688，主图4张，sku的是sku的图片，让后能够上传详情页」.
Trigger: pilot bug — a merchant product with 9 gallery images broke the storefront product page (parser cap 8 vs backend 12,
fixed in unit/storefront-image-cap). The owner wants the 1688 structure instead of one long gallery.

Reference (1688 seller workbench, read-only study 2026-10-02): 4 main images (first = cover), optional white-background
image, per-SKU images on the spec table, and a separate rich detail section of tall images (typically 750–1200 px wide).

## Model
| Role | Count | Rules | Where shown |
|---|---|---|---|
| `main` | 1–4 (≥1 to publish, unchanged rule) | position 0..3, first = cover; square recommended (no hard ratio), ≥ 600 px shortest side recommended (warn, not refuse) | storefront cards/collections (cover), product page gallery, Meta feed `image_link` (+ `additional_image_link` = main[1..3]), claim/live offer previews |
| `sku` | 0–1 per variant (SKU) | an image owned by the product and linked to one SKU; the same file may not serve two SKUs (upload twice if needed) | product page: selecting a variant shows its SKU image first in the gallery; cart line / order line / claim link thumbnail use the SKU image when present, else cover |
| `detail` | 0–20 | ordered; tall images allowed (height ≤ 6× width); 360/720/1080 size children as today (0111) | product page: full-width stack below the description, lazy-loaded, no gaps |

Byte cap stays 2 MiB per original for all roles (PG bytea pilot; `ponytail:` note — move to object storage before raising).
Accepted types unchanged (JPEG/PNG/WebP, magic-sniffed; SVG/GIF refused). EXIF orientation handling unchanged.

## Data migration (forward-only, no data loss)
- Add `role` (`main`/`sku`/`detail`) to `catalog.product_images` and a per-role position; `main` positions 0..3, `detail` 0..19.
- Add `catalog.sku_images(tenant_id, store_id, sku_id, image_id)` (one row per SKU, image must have role `sku` and belong to the
  SKU's product) — or a nullable `sku_id` on the image row; implementer picks the smaller schema and justifies it.
- Existing rows: positions 0..3 → `main` (same order); positions ≥ 4 → `detail` (same order, renumbered from 0). The pilot
  merchant's 9-image product therefore keeps 4 main + 5 detail. Migration is idempotent and asserts counts before/after.
- Drop the 0..11 single-gallery CHECK only after the new per-role CHECKs exist.

## API (merchant + buyer)
- Merchant upload takes `role` (+ `sku_id` for `sku`); reorder per role; delete; move between `main` and `detail` (one call,
  keeps the bytes). Caps enforced in SQL and Go (`MaxMainImages=4`, `MaxDetailImages=20`) with parity tests against the
  storefront/admin parsers (lesson from the pilot bug: every frontend limit must be pinned to the backend constant).
- Buyer product detail returns `images` (main, ≤4), `detail_images` (≤20) and `image_id` per variant (nullable). Product
  list/cards keep returning the cover only.
- Meta catalog feed: `image_link` = main[0], `additional_image_link` = main[1..3] (comma list), variant items use the SKU image
  when present.

## UI (PM-U, Codex)
- Admin product editor: three sections — 主圖（最多 4 張，第一張為封面，可拖曳排序）; 規格圖 (one upload cell per SKU row in the
  variants table, with "use cover" fallback shown); 詳情圖（最多 20 張，可拖曳排序，長圖預覽）. Three locales. Clear refusal copy for
  each cap and size error. Real-click flows.
- Storefront product page: main gallery (≤4) with thumbnails; choosing a variant swaps to its SKU image; detail images
  stacked after the description; mobile 390 px with no horizontal overflow; images lazy-loaded with width/height to avoid CLS.

## Acceptance (red→green; heavy gates on GitHub)
- PG: migration of a 9-image product → 4 main + 5 detail, order preserved; caps (5th main, 21st detail, second image for one
  SKU) refused in SQL and Go; cross-store/tenant image↔SKU link refused; buyer detail shape; feed image links.
- Node: storefront/admin parsers accept 4 main + 20 detail + SKU images; reject 5 main / 21 detail; parity tests vs Go constants.
- Browser (REAL uploads, owner requirement 「测试的时候需要上传图片」): merchant uploads 4 main + 2 SKU images + 6 detail images
  (generated valid JPEG/PNG, including one tall 750×4000 detail image) through the admin UI with real clicks, publishes; buyer
  opens the product page (zh-TW/en, 1440 and 390), sees 4 main thumbnails, picks the second SKU → its image shows, scrolls
  through all detail images, adds to cart, cart line shows the SKU image, checkout reaches the payment/delivery step. Also the
  migrated 9-image fixture renders without error.
- Gates: `--browser-product-editor`, `--browser-catalog-media`, `--browser-storefront`, `--browser-visual-lint`,
  `--browser-click-sweep` + full-suite shards on GitHub.

## Units and order
1. PM-B backend (Claude Sonnet): migration 0149 (next free above 0148), internal/catalog, buyer/merchant HTTP, feed. Contract
   amendment first (catalog contract) — interface before code.
2. PM-U UI (Codex) after PM-B's interface is committed (can start from the amended contract).
3. Browser regression with real uploads lands with PM-U.

## Integrator 裁决
- Caps: main 4, detail 20, SKU 1 per variant; 2 MiB per original (unchanged). White-background image: not now (YAGNI).
- Existing images ≥ position 4 become detail images; nothing is deleted.
- Rich-text detail editor: not now — detail = ordered images (1688-style long images); text stays in `description`.
- Video: not now.
