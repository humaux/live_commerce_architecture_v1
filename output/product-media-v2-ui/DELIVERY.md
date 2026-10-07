<!-- Purpose: PM-U current CI repair, evidence binding and integrator rerun instructions.
Depends on: backend3ba093bc, merged trunkf8f01b74, source-manifest.json and focused runtime artifacts.
Used by: integrator review/GitHub acceptance; no production or provider acceptance. -->
# PM-U product-media-v2 UI — CI repair

- Branch: `unit/product-media-v2-ui`; trunk `f8f01b74ad80fc26223751ce3bfdc2cb849df6c6` merged as `62590106`. Backend base remains `3ba093bc`. Final SHA is the commit containing this file; actual value is in the main `commit-receipt.json` and handoff.
- Supersedes author `0ab80575` / int `6bf684d9` for CI37501885015/37501891939. Prior delivery/logs/manifests retained in main output/history.
- Source binding: `1b7432986d65b8429bcfb65e8d148cd2226c40fc2e2b58c5dfdaf17433f2390f` (33 source files, `source-manifest.json`). Main evidence root: `/Volumes/data/live_commerce_architecture_v1/output/product-media-v2-ui`.
- Parent Codex GPT-6 owns app/media integration and final review; independent read-only reviewer `codex-product-media-v2-ui-sub-photo-repair-review`. Actual configured reasoning/model ID is not exposed; no fabricated value. Existing storefront/test author ownership remains in prior delivery.

## Changes

`photo-preprocess.ts` preserves the same File and bytes for <=2 MiB and an oriented longest side <=2000px. Only oversized sources use canvas: JPEG quality0.85; PNG/WebP become alpha-preserving PNG, with bounded reductions until <=2 MiB. PNG logos retain transparency. Existing role/cap/UNKNOWN recovery behavior remains.

Main/detail empty sections now show plain localized copy (zh-TW/zh-CN/en; ja copy ready for the unified locale unit). Original visibility, count and refusal assertions remain. The real-upload fixture contains fitting640px and oversized3000px transparent PNGs, the750x4000 detail and >2 MiB EXIF6 phone photo. Go independently verifies stored source hashes/formats for fitting originals, decoded dimensions, transparent corners and half-transparent centers for both PNGs.

The prior PM runner registration mistakenly selected the storefront MOCK shortcut and exited before the real-upload gate. PM now selects the normal production build path for both applications and reaches `TestBrowserProductMediaV2RealUpload`. Existing explicit storefront MOCK behavior remains. Bash-condition tests cover both values of LC_SHOP_MOCK; the build predicate test now requires the production blocks that do not exit early.

Prior three-section editor, gallery/variant/cart images,40-emoji rune parity and blocking active-zero-main repair remain. No backend contract/runtime/schema/lockfile change; no push or release merge.

## Commands and exits

Evidence filenames below are prefixed `ci-37501885015-` in the main root.

| Check | Exit | Evidence |
|---|---:|---|
| Actual preprocessor Node regression before/after |1→0| `preprocess-red.log`1PASS/3FAIL → `preprocess-green.log`4/4 |
| Actual empty-tile React SSR regression |1→0| `empty-state-red.log`12PASS/1FAIL → `empty-state-green.log`13/13 |
| Runner branch regression |1→0| `runner-red.log`13PASS/1FAIL → `runner-final-green.log`14/14 |
| `node --test --experimental-transform-types tests/admin/photo-preprocess.test.ts tests/admin/product-media-ui-model.test.ts tests/admin/product-media-model.test.ts tests/admin/image-list-contract.test.ts` |0| `node-final.log`23PASS/0FAIL |
| `pnpm run typecheck:admin` |0| `typecheck-admin.log` |
| Strict owned test types: `pnpm exec tsc --noEmit --target ES2022 --module esnext --moduleResolution bundler --strict --skipLibCheck --esModuleInterop --allowImportingTsExtensions --types node --typeRoots apps/admin/node_modules/@types tests/admin/photo-preprocess.test.ts tests/admin/product-media-ui-model.test.ts tests/admin/product-media-v2.spec.ts` |0| `types-corrected-final.log` |
| `GOTOOLCHAIN=go1.27.1 go test -tags browser -run '^$' ./tests/foundation` |0| `go-compile.log` compile-only, no tests/PG |
| `pnpm run build:admin`; `pnpm run build:storefront` (synthetic platform/admin hosts/contact, identity/fixture/buyer-web disabled) |0 each| `build-admin-empty.log`, `build-storefront.log` |
| `LC_BROWSER_PRODUCT_MEDIA_V2_ACCEPTANCE=1 LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=1000s bash scripts/dev/test-focused.sh '^TestBrowserProductMediaV2RealUpload$'` |0| `browser-normal.log`; Playwright6/6 plus independent Go/PG1/1 |
| Same focused command with `LC_PM_UI_INJECT_FAULT=missing-detail` |1 expected| `browser-calibration.log`, `runtime-calibration/playwright.log`: exact `PM_CALIBRATION_DETAIL_COUNT: five real uploads; six required`; Go calibration RED |
| Same focused command with injection removed |0| `browser-final-green.log`; Playwright6/6 and Go/PG1/1 |
| `bash scripts/dev/check-gates.sh`; `git diff --check` |0 each| `gates-final.log`:73 modes, headers/vet/format |

The first focused normal attempt exposed the empty-grid problem before uploads (exit1, `browser-empty-red.log`/`runtime-empty-red`); fixed without changing its visibility assertion. One own queued process30638 was stopped before source changes (exit143, NOT_RUN). Initial Node import/standalone compiler configuration failures and the intermediate runner regex/type diagnostics are preserved; they are not semantic red/PASS evidence. Corrected commands above pass.

E3 is limited to current focused actual browser/REAL_PG with MOCK signed IdP/TLS fixture, plus Node regressions. Normal runtime ledgers prove12 real uploads, publish, active-zero-main repair, zh-TW/en×1440/390 buyer4thumbnails/Blueoption/sixdetail/cartoptionimage/delivery+payment step, and post-migration4main+5detail rendering. No order/payment/provider action. Full runtime directories are copied to main `runtime-normal`, `runtime-calibration`, `runtime-final-green` with this prefix. Independent source review: `review.md` with this prefix; no concrete P0/P1 or assertion weakening.

## CI gates — integrator runs on GitHub

- `bash scripts/dev/test-local.sh --browser-product-media-v2`
- `LC_PM_UI_INJECT_FAULT=missing-detail bash scripts/dev/test-local.sh --browser-product-media-v2` (expected1 with the exact phase marker), followed by normal mode (expected0)
- `bash scripts/dev/test-local.sh --browser-product-editor`
- `bash scripts/dev/test-local.sh --browser-catalog-core`
- `bash scripts/dev/test-local.sh --browser-catalog-media`
- `bash scripts/dev/test-local.sh --browser-storefront`
- `bash scripts/dev/test-local.sh --browser-click-sweep`
- `bash scripts/dev/test-local.sh --browser-visual-lint`

## NOT_RUN / limits

Full test-local modes, foundation, broad browser regressions and Linux/Xvfb execution are NOT_RUN locally; current GitHub rerun is required before full acceptance. Old calibration CI37501891939 failed in the wrong phase and is not credited. No shared smoke test or threshold weakened. W4-U1 remains queued until PM-U is green.

No real WebP upload fixture in this gate. Fitting EXIF originals intentionally remain unchanged; raw Go codec dimensions may differ from oriented browser dimensions. Normalized >2 MiB EXIF geometry is proven. Legacy unknown-dimension WebP limits and the frozen upload API's non-atomic axis/version race remain documented in the prior delivery. Nine-image fixture proves post-migration rendering; migration upgrade proof belongs to backend TestPMv2MigrationNineImages. Synthetic servers/PG containers are cleaned by their owned harnesses; no production state changed.
