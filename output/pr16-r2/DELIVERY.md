<!-- Purpose: PR16 round-two source, causal regression evidence and output-preservation handoff.
Depends on: comments4223464571/4223464580, real CartProvider/client/journal and registered browser gates.
Used by: integrator pre-push review and required PR checks; not production acceptance. -->
# PR #16 round 2

- Branch `unit/pr1-ui-followups`; base `997be8e0`. Fetch and merges of own remote/trunk were already up to date.
- Tested source **`2425349aa4e7c917a81fb2c7517f36f586708292`**. Final delivery commit is evidence-only.
- Codex-1, no delegates; actual host model identifier unavailable. Only three runtime/test files changed. Deferred doc comment **4223464585 is untouched**.

## Fixes

**4223464571 — EDIT route readiness,375px.** The existing English edit flow now really clicks expand/collapse, checks `aria-expanded` false→true→false and checklist visibility both ways, and records an `edit/en/375/readiness toggle` ledger row. The subsequent one-command merge-patch assertion and existing axe checks remain unchanged. Mutation calibration disabled the handler **only in edit mode**, leaving create functional: the new assertion at acceptance.ts:544 failed as intended. The original form source was restored byte-for-byte and SHA-checked before final gates; no ProductDocumentForm change was committed.

**4223464580 — out-of-order buyer session response.** CartProvider increments a monotonic generation for refresh and commands. Stale session responses are discarded before adopting a context, and stale cart results cannot replace newer state. An active command keeps its recovery context; opportunistic reads do not supersede it. Retry snapshots the current context ref and still calls the existing journal writer, preserving the same B context/key/body. Unmount invalidates pending reads. Two actual-source Node counterexamples control the HTTP completion order: older refresh A after newer refresh B, and older refresh A after mutation B lost its acknowledgement. Both prove B remains published and its lost write replays the original receipt; existing four first-write/rotated-context cases remain green. This is MOCK hook/network/lock-host evidence, not a claim that browser navigator.locks permits every synthetic interleaving.

## Commands and evidence

| Command | Exit | Result |
|---|---:|---|
| `node --test --experimental-strip-types apps/storefront/tests/cart-provider-retry.test.mjs` on old provider | **1** | `cart-red.log`:2 new failures,4 original passes |
| Same command after fix | 0 | `cart-green.log`:6/6 |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-product-editor` with edit-only toggle defect | **1 expected** | `edit-toggle-red.log` + `evidence/edit-toggle-red-playwright.log`: new EDIT assertion fails,11 dependent cases NOT_RUN; separate CC12 passed |
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log`:1156 tests,0 failures |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc.log` |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `storefront-tsc.log` (additional changed-app check) |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`:82 documented modes,header/architecture checks |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-product-editor` restored source | 0 | `browser-product-editor.log`:12/12 + frozen CC12 2/2 |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-order` | 0 | `browser-order.log`:26 cases,6 buyers,7 orders; exact order/hold/job/receipt/reserve counts |

Browser/PG runs were strictly serial, with source fixed during each run. No timeout/retry/threshold was widened. Original browser artifacts: catalog-core/20261008T210437.410511000 (product),catalog-core/20261008T210513.134716000 (CC12),buyer-order-3260051908. Safe log excerpts and the current click ledger are committed in `evidence/`. SOURCE-SHA256 binds the tested files.

## Output preservation

Before testing, the existing three evidence directories were archived locally. After copying this run's evidence, `git checkout -- output/product-ui-v2 output/product-ui-v2-fix` restored tracked outputs. The archive also restored the previously untracked files byte-for-byte. All three directory comparisons were equal; `git diff --exit-code -- output` passed. The inherited58 output status entries matched before/after. These prior outputs were **not staged**. Local archive/status snapshots and the task-owned extracted comparison copy remain untracked for recovery: the environment rejected the attempted cleanup before execution, and no alternative deletion mechanism was used. No other worktree or shared cache was cleaned.

## Handoff / NOT_RUN

Local requested batch is **E3**: actual-source Node MOCK plus production Next/real Go/isolated PG browser fixtures; not independent review or LIVE acceptance. All owned gate processes have ended. No push/deploy or real provider/customer action.

NOT_RUN: required PR CI and independent review on the new head; full G07/foundation (storefront runtime unit; integrator/CI coverage remains required), LIVE/payment/production acceptance. The unrelated R04 media runner is reported NOT_RUN by test-node because `COMMERCE_R04_LIVEKIT_BINARY` is unset. CI should include the PR-selected modes and the two browser modes above. Deferred documentation P2 stays in FOLLOWUPS.
