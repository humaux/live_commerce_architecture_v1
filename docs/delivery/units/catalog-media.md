# Unit catalog-media — product photos end to end, product/SKU edit + archive UI, storefront home

Role: commerce_worker (mid tier). Base `896bf24`. Worktree `.worktrees/catalog-media`, branch
`unit/catalog-media`. No delegation, **no new dependency** (Go stdlib `image/jpeg|png` + `golang.org/x/image/webp`
is NOT allowed unless already in go.mod — check; otherwise accept JPEG/PNG/WebP by magic-byte sniff and
store bytes as uploaded, no re-encode). Migration number **0082** (integrator-assigned).

**Why (output/r3-readiness/REPORT.md gaps 3, 4, 7, 10):** no product images anywhere (no column, no upload,
Meta feed `image_link` empty — internal/attribution/feed.go:31); merchants cannot rename/reprice/archive
from the UI although Go routes + BFF allowlist exist (contracts/catalog-inventory-openapi.json PATCH
products/skus, price, archive); the storefront has no home page (`apps/storefront/app/[locale]/` has no
page.tsx → buyers who open the shop host get 404); Ledger nav shows dead entries.

## Decisions (binding)
- CM1 Storage: PG table `catalog.product_images` (tenant_id, store_id, product_id, id uuid, position
  smallint 0..7, content_type in image/jpeg|image/png|image/webp, bytes bytea ≤ 2 MiB (CHECK
  octet_length), sha256 bytea, width/height nullable, created_at, version), max 8 per product (enforced in
  the definer/insert path), FORCE RLS like catalog.products, store-scoped grants mirroring catalog.products
  for the merchant writer/reader roles and SELECT for the buyer runtime role only through a definer that
  checks the store is published and the product active. `ponytail:` comment: bytea in PG is the pilot
  ceiling (single host, small catalogs); move to object storage + CDN when total image bytes > ~2 GB or
  a second host serves the storefront.
- CM2 Validation at the trust boundary (Go): size ≤ 2 MiB, magic bytes must match the declared type
  (JPEG FFD8FF, PNG 89504E47, WebP RIFF....WEBP), reject SVG/GIF/anything else, never trust filename or
  client Content-Type. Decode dimensions with stdlib `image.DecodeConfig` for JPEG/PNG (WebP: leave null).
- CM3 Admin API (scope from server auth only, catalog:write / catalog:read): `POST
  /v1/admin/stores/{store_id}/products/{product_id}/images` (multipart single file field `file`,
  Idempotency like other catalog writes — follow `command` key pattern), `GET .../images` (metadata
  list), `GET .../images/{image_id}` (bytes, `Cache-Control: private, max-age=300`), `POST
  .../images/{image_id}/delete`, `POST .../images/order` (`{ids:[...]}`). Add to
  contracts/catalog-inventory-openapi.json (integrator merges OpenAPI; you edit it in your branch).
  Admin BFF: extend `[...resource]/route.ts` allowlists for exactly these paths; multipart pass-through
  with a 2.5 MiB body cap in the BFF.
- CM4 Buyer side: buyer catalog/product projections gain `images: [{id, width, height}]` ordered by
  position (grep internal/storefront/catalog.go, internal/buyerhttp/projections.go, contracts/
  buyer-catalog-discovery-v1.md — amend it). Public bytes served by the storefront Next app at
  `/media/p/{product_id}/{image_id}` through its server-side buyer transport to a new buyer Go route
  (published store + active product only; `Cache-Control: public, max-age=86400, immutable` since the
  id changes when content changes; `X-Content-Type-Options: nosniff`). Storefront CSP `img-src` must
  allow `'self'` (check next.config.ts). Meta feed `image_link` = absolute storefront origin + that path
  for the first image (internal/attribution/feed.go; keep empty when none).
- CM5 Admin UI (Ledger.tsx / ProductPhoto.tsx): product detail panel gets photo upload (accept
  image/jpeg,image/png,image/webp; client size check, server is authority), thumbnails, delete, move
  up/down; ProductPhoto renders the real first image and falls back to the current placeholder; product
  rename/description edit, SKU price change and archive actions (product + SKU) with confirm, using the
  existing PATCH / price / archive routes and optimistic `expected_version`. Remove the dead nav items
  (網站客服 / Meta 訊息 / 平台支持 panels in Ledger.tsx ~479-489) or hide them — do not leave "not
  connected in the current build" placeholders reachable.
- CM6 Storefront home `apps/storefront/app/[locale]/page.tsx`: published store name + grid of active
  products (first image, name, lowest active SKU price, link to existing product page), empty state,
  footer reuses existing legal links. Product page shows the image gallery. Three locales.
- CM7 Comments per PROCESS.md §5 on every new file and SQL function.

## Write paths
`migrations/0082_product_images.sql`, `internal/catalog/**`, `internal/storefront/**` (catalog projection
only), `internal/buyerhttp/**` (image route + projection), `internal/httpapi/**` (image routes only),
`internal/attribution/feed.go`, `apps/admin/components/{Ledger,ProductPhoto}.tsx`, `apps/admin/lib/**`,
`apps/admin/app/api/stores/[store]/[...resource]/route.ts` (allowlist only), `apps/admin/app/globals.css`
only if unavoidable, `apps/storefront/app/[locale]/page.tsx`, `apps/storefront/app/[locale]/products/**`,
`apps/storefront/app/media/**`, `apps/storefront/lib/**`, `apps/storefront/next.config.ts` (CSP only),
`contracts/catalog-inventory-openapi.json`, `contracts/buyer-catalog-discovery-v1.md`,
`docs/engineering/dependencies.md` + depmap.

## Done when
Same static set as every unit (build, vet, gofmt, check-pkgdocs, depmap --check, admin + storefront
typecheck, test-node.sh, check_packet.py) exit 0; Go unit tests for CM2 sniffing; one author smoke via
`bash scripts/dev/test-focused.sh '<regex>'`. Evidence → `/Volumes/data/live_commerce_architecture_v1/
output/catalog-media/`. Commit on your branch; do not merge.

## Non-goals
Image resizing/thumbnails generation, CDN, categories, bulk import, video.
