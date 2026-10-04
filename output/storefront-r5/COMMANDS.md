# Executed commands and evidence — 2026-10-02

All commands ran in `/Volumes/data/live_commerce_architecture_v1/.worktrees/storefront-r5`, branch `unit/storefront-r5`, base `aca14d7b7794c22950ffffe9f1714117235b2675`.

| Command | Exit | Evidence / boundary |
|---|---:|---|
| `COMMERCE_BUYER_WEB_ENABLED=0 NEXT_TELEMETRY_DISABLED=1 pnpm run build:storefront` (original) | 0 | `build-before.log` |
| `node output/storefront-r5/image-spike.mjs before` (original build, successful initial run) | 0 | `before-initial.json`, `probe-before-initial.log` |
| `COMMERCE_BUYER_WEB_ENABLED=0 NEXT_TELEMETRY_DISABLED=1 pnpm run build:storefront` (option-A trial) | 0 | `build-after.log`; exact trial in `option-a-trial.patch` |
| `node output/storefront-r5/image-spike.mjs after` | 1 | `after-first.json`, `probe-after-first.log`; expected failure, not PASS |
| Same after probe, CSS-pixel-sized diagnostic screenshot confirmation | 1 | `after.json`, `probe-after.log`, `after-*.png` |
| `node /Users/luolimo/.codex/skills/impeccable/scripts/detect.mjs --json apps/storefront/components/ProductCard.tsx apps/storefront/components/ProductGallery.tsx` | 0 | `ui-detector-trial.json`; mechanical scan only, not acceptance |
| `git diff --exit-code -- apps/storefront` after applying the inverse trial changes | 0 | No remaining app source changes |
| `bash scripts/dev/test-node.sh` on restored source | 0 | `gate-node.log`: 270 PASS, 0 FAIL, 0 SKIP; separate R04 binary suite explicitly NOT_RUN |
| `pnpm --filter storefront exec tsc --noEmit` on restored source | 0 | `gate-tsc.log` (empty successful output) |
| `bash scripts/dev/check-gates.sh` on restored source | 0 | `gate-registration.log`: 55 modes documented and tracked tests registered |
| `COMMERCE_BUYER_WEB_ENABLED=0 NEXT_TELEMETRY_DISABLED=1 pnpm run build:storefront` (restored) | 0 | `build-restored.log`; `.next` restored, not left on the broken trial |
| `node output/storefront-r5/image-spike.mjs before` (restored build) | 0 | `before.json`, `probe-before.log`, `before-products-zh-TW-390x844.png`: 4/4 visible images loaded, 64,754 bytes three times |
| `bash scripts/dev/test-local.sh --browser-storefront` | NOT_RUN | Explicit user stop condition: option A failed. MOCK probe is NOT this gate. |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | NOT_RUN | Same stop condition; no Go/PG or real media-producer acceptance claimed. |
| `bash scripts/dev/test-local.sh --browser-storefront-publish` | NOT_RUN | Same stop condition; no publishing acceptance claimed. |

## Fixture and method

- Node `v24.15.0`, Next `16.3.5`, Sharp `0.35.4`, Chromium via installed Playwright `1.63.0`.
- Sharp both resolves and loads from Next's dependency scope. The incorrect top-level-only resolution result from the initial independent inspection was corrected, not used as a blocker.
- Unchanged `tests/storefront/shop-fake-api.mjs` SHA256: `3634d2aea38ce21150a00946c082e299f76234b576a5a3f0d31c8cff29d2f764`.
- Local Next **production build**, real Chromium, synthetic TLS/CONNECT edge and **MOCK Go API**. No PG, real domain, customer, PSP or carrier.
- `/zh-TW/products`, viewport **390×844**, DPR **2**, three fresh browser contexts per phase. First-screen images = image DOM boxes intersecting the initial viewport, deduplicated by `currentSrc`; sum successful image HTTP response-body bytes. Failed-response bytes are recorded separately. This is not the full-page or HTTP-header-inclusive transfer count.
- The original build's three image-LCP observations were 236/124/348 ms; the failed trial's final diagnostic observations were 212/104/96 ms. Failed-image page paints are **not comparable LCP success**, so no performance improvement/non-regression is claimed. Restored native confirmation LCP: 492/408/380 ms; timings are noisy local samples, not a calibrated production benchmark.
- The final probe source adds explicit before/after built-UI assertions and an LCP non-regression gate. Those two final guard additions were syntax-checked only, not rerun against the stopped trial; the recorded probe runs predate these added guards. The actual trial still provably used `/_next/image` (recorded resource URLs and patch).

## Harness failures retained (not product evidence)

`before-harness-error.json` / matching log: exit 1 before measurement; child-process log stream was not yet open. Fixed by awaiting its `open` event.

`before-http-error.json`, `before-tls-error.json`, `before-relay-error.json` / matching logs: exit 1 before measurement; proxy experimentation/timeouts. The final relay forwards the original RSC/prefetch headers just like the existing repository shop harness; dropping those headers made Next prefetches fetch full HTML and prevented network idle. Successful baseline and trial measurements use the same corrected TLS/CONNECT relay.

All synthetic keys stayed in process memory; task-local self-signed TLS fixture directories were removed by `finally`. Each run awaited its Next child's exit, closed the browser/edge/proxy/API and destroyed only its own sockets. No foreign process, shared cache, main checkout or other worktree was removed or changed.
