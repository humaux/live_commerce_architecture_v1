<!-- Purpose: unit brief for the 1688-style product media model (main images, SKU images, detail-page images).
Depends on: contracts/catalog (catalog v2), migrations 0082/0109/0111 (product_images, sizes), internal/catalog images*.go,
  apps/storefront product page, apps/admin product editor, Meta catalog feed (image_link).
Used by: PM-B (backend, Claude Sonnet) and PM-U (UI, Codex) implementers; integrator review. -->
# Unit brief — product-media-v2 (1688-style product images)

Owner decision 2026-10-06 (verbatim): 「我觉得可以不用什么需要9张图片，学习一下1688，主图4张，sku的是sku的图片，让后能够上传详情页」.
Trigger: pilot bug — a merchant product with 9 gallery images broke the storefront product page (parser cap 8 vs backend 12,
fixed in unit/storefront-image-cap). The owner wants the 1688 structure instead of one long gallery.

Reference (1688 seller workbench, read-only studies 2026-10-02 and 2026-10-06 via Kimi WebBridge, edit page of offer
1056679795100, nothing saved): 主图 slots with 首张主图 as cover (this offer uses 4), "≥800px 实拍大图", 1:1, <5MB;
a separate 白底图 slot; 主图视频/讲解视频; in 销售信息 the SKU table has an image column **per value of the image axis**
(「香型图片 · 添加」 — one image per 香型 value, not per SKU combination); 详情信息 = 图文详情 quick editor (up to 50
text/image blocks, drag to reorder, 插图片/插文本, 详情模板, 间距设置) plus 详情视频; left sticky nav shows per-section status
(错误 / 待优化 / 已完成).

## Model
| Role | Count | Rules | Where shown |
|---|---|---|---|
| `main` | 1–4 (≥1 to publish, unchanged rule) | position 0..3, first = cover; square recommended (no hard ratio), ≥ 600 px shortest side recommended (warn, not refuse) | storefront cards/collections (cover), product page gallery, Meta feed `image_link` (+ `additional_image_link` = main[1..3]), claim/live offer previews |
| `sku` (option-value image) | 0–1 per value of ONE image axis (the first option axis by default, merchant may pick which axis, like 1688 「香型图片」/「颜色图片」) | an image owned by the product and linked to one option value; every SKU carrying that value shows it; products without options have none | product page: selecting a variant shows its SKU image first in the gallery; cart line / order line / claim link thumbnail use the SKU image when present, else cover |
| `detail` | 0–20 | ordered; tall images allowed (height ≤ 6× width); 360/720/1080 size children as today (0111) | product page: full-width stack below the description, lazy-loaded, no gaps |

Byte cap stays 2 MiB per original for all roles (PG bytea pilot; `ponytail:` note — move to object storage before raising).
Accepted types unchanged (JPEG/PNG/WebP, magic-sniffed; SVG/GIF refused). EXIF orientation handling unchanged.

## Data migration (forward-only, no data loss)
- Add `role` (`main`/`sku`/`detail`) to `catalog.product_images` and a per-role position; `main` positions 0..3, `detail` 0..19.
- Link option-value images: `(tenant_id, store_id, product_id, option_name, option_value) -> image_id` (one row per value of the
  product's image axis; the image must have role `sku` and belong to the same product); the image axis is a product field
  (default = first option axis). Implementer picks the smallest schema and justifies it.
- Existing rows: positions 0..3 → `main` (same order); positions ≥ 4 → `detail` (same order, renumbered from 0). The pilot
  merchant's 9-image product therefore keeps 4 main + 5 detail. Migration is idempotent and asserts counts before/after.
- Drop the 0..11 single-gallery CHECK only after the new per-role CHECKs exist.

## API (merchant + buyer)
- Merchant upload takes `role` (+ `sku_id` for `sku`); reorder per role; delete; move between `main` and `detail` (one call,
  keeps the bytes). Caps enforced in SQL and Go (`MaxMainImages=4`, `MaxDetailImages=20`) with parity tests against the
  storefront/admin parsers (lesson from the pilot bug: every frontend limit must be pinned to the backend constant).
- Buyer product detail returns `images` (main, ≤4), `detail_images` (≤20), `image_axis` and per-option-value `image_id`
  (each variant resolves its image through its value on the image axis; nullable). Product
  list/cards keep returning the cover only.
- Meta catalog feed: `image_link` = main[0], `additional_image_link` = main[1..3] (comma list), variant items use the SKU image
  when present.

## UI (PM-U, Codex)
- Admin product editor: three sections — 主圖（最多 4 張，第一張為封面，可拖曳排序，建議 800px 以上正方形）; 規格圖 (one upload
  cell per value of the image axis, e.g. each 顏色, next to the option values, with "use cover" fallback shown); 詳情圖（最多 20 張，可拖曳排序，長圖預覽）. Three locales. Clear refusal copy for
  each cap and size error. Real-click flows. Phone photos often exceed 2 MiB: the admin downsizes in the browser before upload
  (longest side 2000 px, JPEG q≈0.85, EXIF orientation applied) so merchants are not refused; the server cap stays authoritative.
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
- Video: not now.
