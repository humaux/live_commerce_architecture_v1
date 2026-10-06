<!-- Purpose: current PM-U implementation/addendum handoff and evidence boundaries.
Depends on: frozen backend3ba093bc, main evidence logs/source-manifest.json and GitHub gates.
Used by: integrator review/CI; no production/provider acceptance. -->
# PM-U product-media-v2 UI delivery

- Branch `unit/product-media-v2-ui`; backend base `3ba093bc006ab985fd3578e9f5e78cc09295c3e5` as requested by the addendum. Final commit is the commit containing this file; actual SHA in handoff/commit-receipt.json.
- Supersedes author handoff c62aec88. Prior source/log/doc evidence preserved in main output; rollback ref `codex/pm-ui-before-addendum-c62aec88`. Own UI commit replayed onto3ba as4fea09c6; no push/release merge.
- Current source binding SHA256 `8a7af90a8f0958f88012cd9484db1d4a1082cc8a31722b58aa3a9e99624077b3`;32source files in main source-manifest.json. Delivery-only documentation added after source checks.
- Main evidence root `/Volumes/data/live_commerce_architecture_v1/output/product-media-v2-ui/`.

## Implemented

Admin closed BFF media action/query allowlist; Main(max4,first cover,reorder)/Option(one image per value on selected image option)/Detail(max20,tall,reorder) sections. EXIF-aware sequential browser decode/re-encodeJPEGq0.85,<=2000px and<=2MiB before upload. UNKNOWN retains immutable File/key; durable store/product journal survives re-auth and refuses dispatch when storage fails. Canonical axis/value/image verification covers edit and staged creation.

Buyer4main thumbnails, current variant image, lazy detail stack below description with original dimensions and cart server image_id. en/zh-TW/zh-CN wired; ja copy-only, owner assigned Japanese routing to unified locale unit.

**PM-B addendum:** image-list text limits already use `[...s].length`; retained test reads Go bounds and accepts a40-emoji value/30-emoji name, rejects one extra code point. Editor uses the Manager's canonical media head, distinguishing unread/failed reads from known zero. An active product with no main images shows a blocking alert; active save/publish are disabled and the staged hook checks resulting status, including already-active products. Upload/move and draft/unpublish repair remain possible. Stale pre-mutation reads cannot replace the new head. Real-click CI scenario moves all4published main images to detail, asserts blocked active intents + usable repairs, then moves them back in original order before buyer/PG readback.

## Author/review ownership

Codex GPT-6 parent owns admin/wiring/addendum and final review. Separate storefront child fd2f8f3011de6ad167e47b4d5b57070007730c56 (base7918) and independent Kimi K3 browser author a7ca55320d545b23f41cbb15ec282c4b327d2cc4 (base7918); parent applied scoped patches, corrected integration findings, and obtained independent source/harness re-reviews. Child worktrees retained. Backend/runtime/migration edits in3ba are inherited from PM-B, not authored here.

## Local evidence (MODEL_ONLY / compile-only)

| Command | Exit | Evidence |
|---|---:|---|
| `pnpm install --offline --frozen-lockfile` |0| `deps.log` (initial dependency setup; lock unchanged) |
| `node --test --experimental-strip-types tests/admin/product-media-ui-model.test.ts` |1→0| original `red-model.log`→`green-model.log`; query/geometry/caps |
| same focused media command |1→0| `red-ci-selector.log`→`green-ci-selector.log`; clean-CI both-app build predicates |
| same focused media command |1→0| `red-boundary-axis.log`, `red-storage.log`→`green-boundary-axis-storage.log`; recovery/axis negatives |
| `node --test --experimental-strip-types tests/admin/product-media-model.test.ts` |1→0| `red-rune-parity.log`→`green-rune-parity.log`;40emoji code-point bounds (retained) |
| new known/unknown/role/draft blocking tests |1→0| `addendum-red.log`→`addendum-green.log`17PASS/0FAIL |
| `bash scripts/dev/test-node.sh` |0| `addendum-node.log`478PASS/0FAIL |
| `node --test --test-reporter=spec --experimental-strip-types tests/admin/product-media-ui-model.test.ts tests/admin/product-media-model.test.ts tests/admin/shell-registry.test.ts` |0| `addendum-focused-final.log`26PASS/0FAIL |
| storefront Node |0| parent `storefront-unit.log`11/0; unchanged child `storefront/storefront-node.log`230/0; child `storefront/red.log`6FAIL→`storefront/green.log`11PASS |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` |0| `addendum-types-final.log` |
| `pnpm --filter @live-commerce/storefront exec tsc --noEmit` |0| `storefront-types-final.log` (storefront source unchanged) |
| `pnpm exec tsc --noEmit --target ES2022 --module esnext --moduleResolution bundler --strict --skipLibCheck --esModuleInterop --allowImportingTsExtensions --types node --typeRoots apps/admin/node_modules/@types tests/admin/product-media-v2.spec.ts` |0| `addendum-browser-types.log` |
| `GOMAXPROCS=2 GOTOOLCHAIN=go1.27.1 go test -p 1 -tags browser -run '^$' ./tests/foundation` |0| `addendum-go-compile.log`; no tests executed, noPG |
| `bash scripts/dev/check-gates.sh` |0| `addendum-gates-final.log`;72modes and header ratchet pass |
| PM detector one pass / `git diff --check` |0| historical `detect.json` before addendum has no findings; current diff clean (visual browser gate pending) |

Independent source re-review reports no remaining concrete P0/P1 or test weakening. E3 applies to MODEL_ONLY Node checks; E1 applies to types/compile. **No browser/PG product acceptance claimed.**

## CI gates (NOT_RUN locally)

Integrator pushes/runs `.github/workflows/gates.yml`:

- `bash scripts/dev/test-local.sh --browser-product-media-v2`
- `bash scripts/dev/test-local.sh --browser-product-editor`
- `bash scripts/dev/test-local.sh --browser-catalog-core`
- `bash scripts/dev/test-local.sh --browser-storefront`
- `bash scripts/dev/test-local.sh --browser-click-sweep`
- `bash scripts/dev/test-local.sh --browser-visual-lint`

New-gate calibration on GitHub: `LC_PM_UI_INJECT_FAULT=missing-detail bash scripts/dev/test-local.sh --browser-product-media-v2` requiredexit1 after five real detail uploads, explicit `PM_CALIBRATION_DETAIL_COUNT` marker and six-detail assertion. Earlier unrelated failures do not count. Clearenv normalmode requiredexit0. Both are NOT_RUN by author.

The mode runs realJPEG/PNG uploads4main+2option+6detail incl750x4000 and>2MiBEXIF6, merchant publish, active-zero-main repair, zh-TW/en×1440/390 buyer option gallery/cart/detail/delivery/payment and post-migration4main+5detail fixture; independent Go PG readback. No order/payment placement or provider action.

## Limitations / integrator follow-up

- All six heavy modes, production Next builds, actual browser downsize/EXIF/upload/390px layout and PG runtime remain NOT_RUN. GitHub required before merge. No local test-local/test-focused/browser/PG process was started.
- Frozen upload API has no expected-axis/version field: concurrent axis change cannot be prevented atomically; canonical binding mismatch stops/requests inspection, no blind retry.
- Legacy unknown-dimension WebP cannot promise intrinsic detail CLS prevention; new normalized uploads have measured geometry.
- Nine-image browser fixture proves post-migration rendering; upgrade proof belongs to backend `TestPMv2MigrationNineImages`.
- Backend3ba review/migration corrections are now the branch base. Backend CI remains integrator-owned; prior37459547565 was cancelled (historical, not a PASS). Integrator applies current trunk for shared Linux native-Chromium regressions.
- Heavy run evidence under output/playwright/product-media-v2 must be retained from GitHub artifacts. Durable main logs/manifests here; original child assumptions in browser-author/DELIVERY.md are superseded by parent corrections. No task-owned running test servers/containers/browser remain.
