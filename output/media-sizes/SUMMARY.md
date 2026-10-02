# media-sizes — R5 S1 option B

Date: 2026-10-03 (UTC+8). Status: **PASS — all requested local gates**, final source `56119b30` (plus documentation/evidence commit). No push, merge, deployment, production backfill, or credential access.

## Scope and commits

Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/media-sizes`; branch `unit/media-sizes`. Initial HEAD `8c0962756d98253c7668a7cd145ebecd0f59f172`, a verified descendant of the requested integration base `b7a4add` (`git merge-base --is-ancestor` exit 0). No other checkout was edited.

| Item | Status | Implementation / source commit |
|---|---|---|
| Upload produces 360/720/1080 JPEG width buckets | PASS | `592c1a7d`: quality 82, decode once, preserved aspect, alpha over white, normalized JPEG EXIF 1–8, no upscale; originals unchanged. |
| Storage, isolation, cleanup | PASS | Migration **0111**, original's tenant/store/image composite FK, cascading delete, FORCE RLS, no public writes, no byte UPDATE. Children and original commit atomically. |
| Existing images | PASS (local) | Explicit per-image merchant backfill; idempotent and audited. No anonymous lazy writes or automatic production scan. |
| Strict `?w=` and Host custody | PASS | Only exact `w=360`, `w=720`, `w=1080`. Unknown/extra/duplicate/encoded variants rejected. Active published store/product and exact image ownership checked; BFF preserves trusted Host and does not forward buyer cookies. |
| Native card / gallery / rail | PASS | `56119b30`: shared URL helper and CSS-matched sizes; original `img`, eager/lazy behavior, alt and original-resolution zoom retained. No Next optimizer. |
| Actual small-image descriptors | PASS | `5978eb71` + `56119b30`: persisted post-EXIF `pixel_width`, one bounded Host-scoped catalog metadata read, optional card `cover_image_sizes` / detail `sizes`, duplicate actual widths removed. A 3px source is **3w**, not 360w. Missing metadata keeps original-only. |
| ≥70% byte reduction and non-worse LCP | PASS | **84.2688%** image-body reduction; median LCP **3,248 → 952 ms**, seven cold samples per phase, same four source digests. |
| Independent review | PASS, static and evidence scope | Readonly review passes found no remaining P0/P1 in custody/cache/decode/backfill or final pixel-width metadata. Independent recomputation verified the final 84.2688% / 952ms pair and matching nine-case browser log. Runtime gates below were run by the primary agent. |

All source commits end with `Co-Authored-By: Codex <noreply@openai.com>`. This evidence/documentation commit follows them.

## Final commands and actual exits

All commands ran in this worktree. `0` means the command returned success, not inferred from an earlier subset. Logs beside this file are authoritative; earlier/failing runs remain separately retained.

Evidence log formatting: terminal progress-line CR/trailing spaces were normalized for Git whitespace checks; results, assertions and exit records were not changed. The first evidence staging check flagged those cosmetic log lines; the final aggregate source/evidence diff check passes.

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log`: **295 passed, 0 failed**; optional LiveKit runner NOT_RUN below. |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc-final.log` |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `storefront-tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log` |
| `bash scripts/dev/depmap.sh` | 0 | `depmap-generate.log` |
| `bash scripts/dev/depmap.sh --check` | 0 | `depmap-final.log` |
| `GOTOOLCHAIN=go1.27.1 go test -race -count=1 ./internal/catalog ./internal/buyerhttp ./internal/httpapi` | 0 | `go-final.log` |
| `bash scripts/dev/test-focused.sh '^(TestMediaSizes\|TestCatalogMedia\|TestCatalogV2\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | `pg-final.log`: **11 PASS, 0 FAIL, 0 SKIP**, real disposable PG. |
| `bash scripts/dev/test-local.sh --browser-storefront` | 0 | `browser-storefront-final.log`: **SFR01–SFR09**, including rail/gallery currentSrc and original tiny fixture, real Go/PG + synthetic edge. |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | 0 | `browser-catalog-media-final.log`: **20 cases**, zh-TW/en × desktop/390; real stack, MOCK IdP. |
| `LC_MEDIA_SIZES_PHASE=after bash scripts/dev/test-local.sh --browser-storefront` | 0 | `browser-storefront-perf-final.log`: seven paired performance samples plus all 9 SFR cases. |
| `git diff --check` | 0 | Checked before each source commit and final delivery. |

Browser detail evidence (repository-relative):

- Normal storefront: `output/playwright/storefront/20261002T185124.660871000/`.
- Catalog-media: `output/playwright/catalog-media/20261002T185226.794414000/`.
- Final performance + storefront: `output/playwright/storefront/20261002T185506.144977000/`; self-contained gate details copied beside this SUMMARY.
- Impeccable detector run once on changed product components: `design-detect.json` is `[]`, exit 0; preserved incumbent layout, no redesign.

## Paired performance

390×844, DPR 2, mobile/touch emulation, Chromium CDP 1.6 Mbps downstream / 750 Kbps upstream / 150 ms RTT, CPU 1×. Seven fresh cache-disabled contexts in each phase, `/zh-TW/products`, four visible loaded images per sample. Measures image **response body bytes**, not transfer headers. LCP is browser PerformanceObserver data; no synthetic LCP values or sleep-based estimates.

The exact same deterministic **unpadded** 1440×1800 JPEG fixture and ordered source SHA256s are checked. The original 3×2 fixture remains in ordinary regression; it is not a meaningful performance baseline. No failed images count as savings. After-phase also fetches and decodes the selected candidate and verifies its real width equals its query bucket for this large-image fixture. No page overflow allowed.

| Metric | Before (`before.json`) | Final after |
|---|---:|---:|
| Visible image bodies per sample | 387,243 B, all 7 | 60,918 B, all 7 (**−84.2688%**) |
| Median LCP | 3,248 ms | 952 ms (**not worse**) |
| Loaded images | 4/4, all 7 | 4/4, all 7; HTTP 200; real 360px candidates |

Before LCP samples: `[3108,3272,3240,3244,3336,3260,3248]`; final after: `[964,932,944,952,956,956,928]`. `before.json` and `after.json` contain all statuses, bytes and ordered original SHA256s. Screenshots `before-products-zh-TW-390.png` and `after-products-zh-TW-390.png` were visually inspected: same layout and synthetic pictures, no omitted images or layout shift workaround.

The first successful after pair (60,918 B / 932 ms median) was taken **before** the small-image metadata correction and is retained in `intermediate/`; it is not substituted for final-source acceptance. A later final-source attempt was interrupted by SIGTERM before a valid result; `red/browser-perf-interrupted.log` records it. No conclusion about the signal's sender is claimed. The isolated final rerun passed in 175.11 seconds and supplies the **952 ms** comparison above.

## Red → green and preserved failures

- `red/node.log` and `red/go.log`: feature negatives rejected the previously unsupported valid width query, exit **1** before implementation; final suites exit 0.
- `red/pg.log`: new rendition behavior against the pre-0111 schema failed upload, exit **1**; final migration/lifecycle/ACL/upgrade gates exit 0.
- `red/browser-storefront.log`: old original-only frontend failed the new width-candidate assertion, exit **1**; the final native responsive build must pass the same performance gate.
- `red/srcset-dimensions-node.log`: newly added MS4 rejected guessed descriptors and invented historical srcset, exit **1**; `srcset-dimensions-node.log` passes **4/4**, exit 0.
- `red/browser-tiny-srcset.log`: added rail load assertion exposed bucket/pixel mismatch. Fixed the producer/metadata/descriptor chain; **did not remove or relax** the `loaded()` assertion or replace the tiny fixture. Final normal browser exit 0.
- `red/pg-first-implementation.log`: cache regression (generic middleware had set no-store) and migration-count mismatch. Producer now explicitly sets immutable only for committed children; missing-child fallback remains no-store end-to-end. Upgrade expects 43 migrations because 0111 was added.
- `red/pg-metadata-fixture-role.log`: added metadata test initially used the checkout pool, which correctly lacked permission. Corrected the fixture to the real buyer-runtime pool, not the ACL.
- `red/storefront-tsc-metadata-prop.log`: gallery's old inline prop type omitted new metadata; switched it to the shared ProductDetail image type.
- `red/renditions-go.log` is an early compile diagnostic, **not** claimed as a behavioral red test. Static checks were not deliberately broken to fabricate a red/green story.

Existing catalog-media format, original digest, ordering, isolation and exact **64px versus 40px** cover identity assertions remain. The cover assertion decodes currentSrc bytes because srcset density-corrects DOM naturalWidth; it still requires exactly 64 real pixels. The old fake RIFF test fixture was replaced with actual valid 4×3 WebP bytes because full decoding is now required; original WebP metadata remains NULL as contracted.

## Backfill operation and limits

Use the normal authenticated merchant `catalog:write` API:

`POST /v1/admin/stores/{store}/products/{product}/images/{image}/renditions`

Body `{}`; mandatory `Idempotency-Key`. Get product/image IDs from existing authenticated catalog reads. The scope comes from server authentication, not a tenant supplied in this body. Use a fresh key per intended image command; reuse the same key when reconciling an unknown outcome. No UI button or production batch executor is claimed.

The explicit task was chosen over lazy generation to avoid anonymous reads triggering decoding/DB writes. Before authorized backfill, the normal frontend uses the original; direct allowed-width requests fall back to the scoped original with **no-store**. Backfill rechecks the immutable original digest under the same product lock used by deletion, inserts all children transactionally and is repeat-safe. Invalid historical bytes remain intact and fail validation; no silent fake thumbnail.

- Original compressed upload cap stays 2 MiB; decode budget 20 MP / 20,000 px per axis; at most two concurrent full decodes per API process. Waiting and inter-size work respect cancellation. These bounds are not a production load/soak result.
- JPEG quality 82 with white transparency composition; no animation support introduced. A later codec/quality change needs a versioned cache/migration decision rather than overwriting immutable child bytes.
- Added only `golang.org/x/image v0.46.0` for maintained WebP decoding and resampling; standard-library JPEG encoder. Rationale and historical decision supersession are in `docs/engineering/dependencies.md`.

## Review and traceability

- Primary: Codex, integrator/implementation/test execution; exact runtime model and reasoning setting not exposed, not guessed. Write ownership was this worktree's catalog/buyer/BFF/storefront/migration/tests/docs/evidence only.
- Readonly reviewers: `/root/orders_contract` and `/root/orders_tests`, existing explorer agents (`gpt-6-luna`, high per agent configuration); no product writes, no recursive delegation. Their reviews used initial `8c096275…`, then the evolving source and final correction; no independent runtime execution claimed.
- Humaux task `0ca5873c-ab51-4fce-8241-e115fec01c4e`; own canvas `codex-media-sizes`. Final 26 changed source files indexed under `live-commerce-media-sizes`; `MakeImageSizes` and `attachV2ImageSizes` linked to correction memory `3eb47e7c-22f2-45dc-9476-161c7b68dd1e`.
- Owned browser drivers and fixture services finished/cleaned up; the interrupted driver also exited. No foreign process or fixture was stopped.

## NOT_RUN / release boundary

- **NOT_RUN** optional `tests/media/r04-input-runner.test.mjs`: `COMMERCE_R04_LIVEKIT_BINARY` unset; unrelated live-input integration, not counted among 295 passing node tests.
- **NOT_RUN** full-repository Go/release suite, WebKit/real mobile devices, real merchant-photo/network performance, production migration/backfill/deployment, CDN/live-domain integration and production load/soak.
- Local real Go/PG plus MOCK edge/IdP acceptance does not authorize release or establish LIVE customer acceptance. Production credentials and customer business were not touched. Integration/release decision remains with the integrator.
