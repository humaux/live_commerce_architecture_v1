<!-- Purpose: PM-U author handoff and evidence boundaries.
Depends on: frozen product-media-v2 backend7918, main evidence logs, source-manifest.json and GitHub gates.
Used by: integrator review/CI; no production/provider acceptance. -->
# PM-U product-media-v2 UI delivery

- Branch: `unit/product-media-v2-ui`; base `7918ecbb992d15963d3f9a5f61bed63a91872a01`. Commit: the commit containing this file (final SHA reported in handoff).
- Source evidence binding SHA256: `c34d8bd82da5a725b08fe56ad91b7d4a126dd7ab4eb4bc36ef8f2f658bd01ae3` (32 files in main `source-manifest.json`; documentation-only delivery added after the source checks).
- Model/ownership: Codex GPT-6 root; storefront child Codex `fd2f8f3011de6ad167e47b4d5b57070007730c56`; independent Kimi K3 test-author `a7ca55320d545b23f41cbb15ec282c4b327d2cc4`; parent applied scoped source patches, then corrected and independently re-reviewed integration findings. Frozen base for both children7918. Separate child worktrees retained for review.

Admin BFF admits move/option-images/image-axis actions and exact role/value upload queries. The product editor has Main(max4/first cover/reorder), Option(one cell per axis value), Detail(max20/tall/reorder) sections. Sequential EXIF-aware browser decoding normalizes to JPEGq0.85, <=2000px and <=2MiB before sending. Immutable File/key are reused for explicit UNKNOWN recovery; durable store/product journals survive re-auth and storage failures refuse dispatch. Canonical axis/value/image readback prevents false success in both edit and staged creation.

Buyer main thumbnails remain4; current variant shows its linked image, details render below description with lazy loading and original dimensions, cart thumbnails prefer server image_id. English/zh-TW/zh-CN wired; **ja copy delivered only**, owner chose unified locale unit to add Japanese routing. No backend locale expansion.

Independent real-click gate uploads4main+2option+6detail including750x4000 and >2MiB EXIF6 JPEG, publishes through the edit document PUT, then checks zh-TW/en ×1440/390, actual option/cart images and delivery/payment controls without order/payment placement. It also renders the post-migration4+5 fixture. Go independently reads PG images/links/status; source files are valid JPEG/PNG. Upload observation is one chooser/response per file. Runtime gate has **NOT_RUN** status until GitHub.

## Local checks and red/green

All logs live at `/Volumes/data/live_commerce_architecture_v1/output/product-media-v2-ui/`.

| Command | Exit | Evidence/result |
|---|---:|---|
| `pnpm install --offline --frozen-lockfile` |0| `deps.log` |
| `node --test --experimental-strip-types tests/admin/product-media-ui-model.test.ts` |1→0| `red-model.log`→`green-model.log`; role/query/geometry |
| same focused Node command |1→0| `red-ci-selector.log`→`green-ci-selector.log`; both actual Next build predicates |
| same focused Node command |1→0| `red-boundary-axis.log`, `red-storage.log`→`green-boundary-axis-storage.log`; durable recovery and axis receipt binding |
| `node --test --experimental-strip-types tests/admin/product-media-model.test.ts` |1→0| `red-rune-parity.log`→`green-rune-parity.log`; Go-rune Unicode limits |
| `bash scripts/dev/test-node.sh` |0| `node-delivery.log`,477PASS/0FAIL |
| `node --test --test-reporter=spec --experimental-strip-types tests/admin/product-media-ui-model.test.ts tests/admin/product-media-model.test.ts tests/admin/shell-registry.test.ts` |0| `focused-final.log`,25PASS/0FAIL |
| storefront focused Node tests / full storefront Node suite |0| `storefront/red.log`6FAIL→`storefront/green.log`11PASS; `storefront-unit.log` parent11PASS/0FAIL; `storefront/storefront-node.log` child230PASS/0FAIL (unchanged storefront patch) |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` |0| `types-delivery.log` |
| `pnpm --filter @live-commerce/storefront exec tsc --noEmit` |0| `storefront-types-final.log` |
| `pnpm exec tsc --noEmit --target ES2022 --module esnext --moduleResolution bundler --strict --skipLibCheck --esModuleInterop --allowImportingTsExtensions --types node --typeRoots apps/admin/node_modules/@types tests/admin/product-media-v2.spec.ts` |0| `browser-types-green.log`; earlier invocation lacked Node type roots (`browser-types-first.log`,exit1), corrected invocation |
| `GOMAXPROCS=2 GOTOOLCHAIN=go1.27.1 go test -p 1 -tags browser -run '^$' ./tests/foundation` |0| `go-compile-final.log`; compile-only **no tests executed**, no PG |
| `bash scripts/dev/check-gates.sh` |0| `gates-final.log`;72registered modes, headers ratchet passes |
| PM UI detector one pass / `git diff --check` |0| `detect.json` has no findings; final diff clean |

Independent read-only re-review found no remaining concrete source P0/P1 after fixing recovery fencing, storage refusal, axis false-success, test response/publish/gallery mismatches and FK-unsafe fixture cleanup. This is source review, **not browser acceptance**.

## CI gates

Integrator pushes/runs `.github/workflows/gates.yml`; no heavy gate ran locally.

- `bash scripts/dev/test-local.sh --browser-product-media-v2`
- `bash scripts/dev/test-local.sh --browser-product-editor`
- `bash scripts/dev/test-local.sh --browser-catalog-core`
- `bash scripts/dev/test-local.sh --browser-storefront`
- `bash scripts/dev/test-local.sh --browser-click-sweep`
- `bash scripts/dev/test-local.sh --browser-visual-lint`

New-gate calibration on GitHub first: `LC_PM_UI_INJECT_FAULT=missing-detail bash scripts/dev/test-local.sh --browser-product-media-v2` must **exit1** after the explicit `PM_CALIBRATION_DETAIL_COUNT` marker and six-detail assertion (five real detail uploads). Unrelated earlier failures cannot count as calibration. Then clear the env and run the normal mode, required exit0. Both are **NOT_RUN**, so no browser red/green claimed yet.

## NOT_RUN / limitations / integrator follow-up

- All six heavy modes, production Next builds, actual browser processing/real upload/EXIF/390px layout, PG runtime and browser calibration are NOT_RUN by author; GitHub required before merging. Evidence classes: MODEL_ONLY for Node; E1 compile/types; prospective BROWSER+REAL_PG with MOCK IdP/edge. No SANDBOX/LIVE provider, real-device or production claim.
- Backend API request/response unchanged. `role+option_value` upload has no expected-axis/version field: concurrent server axis change cannot be prevented atomically by this frozen contract. UI stops/requests inspection when canonical binding differs; no blind retry against another axis.
- Legacy unknown-dimension WebP details cannot promise intrinsic-dimension CLS prevention; new normalized uploads have measured dimensions.
- 9-image browser fixture is **post-migration layout rendering**; migration upgrade proof belongs to backend `TestPMv2MigrationNineImages`.
- Backend CI37459547565 was cancelled, not PASS. Backend branch subsequently advanced to3ba093bc (0149 review corrections); integrator owns that dependency integration and backend CI.
- Latest r3/integration6f285fe1 adds Linux native-Chromium MOU startup fix; integrator should apply current trunk when running affected shared regressions. This author branch inherited heartbeat/Xvfb fixes via7918/8c; no local test-local/test-focused or PG mode was started.
- Main evidence logs/hash manifest are durable. Child author DELIVERY is retained under `browser-author/DELIVERY.md`; its original assumptions were corrected by parent as described above. Worker-generated packet-check artifact was not imported. No running task-owned test servers, PG fixtures or browser processes remain.
