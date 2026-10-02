# Product image renditions — R5 S1 B

Scope: product photos only. Original bytes and IDs remain unchanged. No Next image optimizer, external image URL, store ID from a caller, or public-write-on-GET.

## Frozen delivery contract

- Merchant upload commits original plus JPEG quality 82 renditions for width buckets 360/720/1080 in the same transaction. Preserve aspect ratio; never upscale a small source. JPEG composites transparency over white. Decode budget: 20 million pixels, 20,000 per axis; reject unsupported or corrupt input before storing. The existing 2 MiB upload cap remains.
- `catalog.product_image_sizes`: same tenant/store/image custody and FORCE RLS as original; composite FK with cascading deletion. Immutable bytes. Runtime may select/insert/delete, never update bytes. Definer can only read.
- Public product route accepts no query (original) or the exact raw query `w=360|720|1080`; all other queries, repetitions and encoded spellings rejected. Site and collection image routes are unchanged.
- The definer resolves the published shop from the trusted Host-derived origin, and checks active shop/product plus exact product/image ownership before either original or rendition bytes. The cache URL retains Host + path + width. No shared optimizer cache.
- Existing images: **explicit one-time merchant-authorized backfill**, per image `POST /v1/admin/stores/{store}/products/{product}/images/{image}/renditions`, `{}`, `catalog:write`, Idempotency-Key. Same image command is repeat-safe; no production job is run by this unit. Choice avoids CPU work and database mutations triggered by anonymous reads. Invalid historical files return a bounded validation failure and remain intact.
- Until backfill, valid width requests may serve the authorized original with **no-store**, never immutable. Once rendition bytes exist, their response is immutable for one day. Deleted/foreign/draft/unpublished images stay 404. Original URL remains original.
- Native img srcset/sizes for grid, rail and gallery. Preserve eager first viewport images, aspect layout, alt text and full-resolution dialog zoom.
- Catalog cards expose optional `cover_image_sizes`; detail images expose optional `sizes`: `[{width: 360|720|1080, pixel_width: actualDecodedWidth}]`. One bounded Host-scoped metadata read attaches these, without reading image bytes. HTML width descriptors use `pixel_width`, deduplicating small sources; absent metadata keeps original-only until backfill. Never pretend a 3px fixture is 360px. See [HTML source-set width requirement](https://html.spec.whatwg.org/multipage/images.html#srcset-attributes).

## Acceptance

UNIT/REAL_PG: dimensions, decode rejection/budget, all three formats, repeat uploads/backfill, cascade, RLS/definer cross-store and active/publication checks, strict width allowlist, original digest stability. Add negative tests before implementation.

REAL-STACK performance uses the same deterministic 1440×1800 JPEG catalog fixture before/after, 390×844, DPR 2, fresh contexts, original SHA checks, all visible images decoded. Existing 3×2 unit fixture is not a meaningful image-performance baseline. No padding or artificial sleeps. Seven cold browser samples, median LCP (milliseconds) after ≤ before and each sample image bodies reduced ≥70%; failed images invalidate the run. Record network/CPU settings and source digests. No claim for production devices/networks.

Regression: browser-storefront, browser-catalog-media, test-node, both tsc, check-gates, depmap and related Go/PG. Existing assertions and thresholds remain intact. Type checks/static gates need no fabricated red; feature negatives are red first, then the final full commands must pass.
