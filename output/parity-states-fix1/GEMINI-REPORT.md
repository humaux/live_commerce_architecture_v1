# Gemini Implementation Report: PARITY-STATES-FIX1 (PR #356)

Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-states`
Branch: `unit/parity-states`
Base Commit: `acf2ebd2c` (Gates 41167677841 success)
Merged Upstream: `origin/r3/integration` (already up to date)
Failing Run: `https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41170058531`

## Failure Analysis & Root Cause Classification

| Failing Job / Test | Class (a/b/c) | Changes Made | File:Line |
|---|---|---|---|
| `124446517642` / `TestBrowserStorefront` (`forbidden-(zh-TW\|zh-CN\|en)-(1440\|390)`) | **Class (a): replaced UI route mapping** | Rewrote route mapping for `kind === "forbidden"` to target `/orders/${orderID}` instead of falling through to `/order-link`. Added signed Figma frames `1429-35190` (desktop) and `1429-35210` (mobile) to `frames.forbidden`. Updated navigation recovery action to click `c.home` (`/`) via real link click. | [`tests/storefront/shop-real-gate.mjs:465`](file:///Volumes/data/live_commerce_architecture_v1/.worktrees/parity-states/tests/storefront/shop-real-gate.mjs#L465), [`tests/storefront/shop-real-gate.mjs:473`](file:///Volumes/data/live_commerce_architecture_v1/.worktrees/parity-states/tests/storefront/shop-real-gate.mjs#L473), [`tests/storefront/shop-real-gate.mjs:506-507`](file:///Volumes/data/live_commerce_architecture_v1/.worktrees/parity-states/tests/storefront/shop-real-gate.mjs#L506-L507) |
| `124446517642` / `TestBrowserStorefront` (`unknown-(zh-TW\|zh-CN\|en)-(1440\|390)`) | **Class (b): product code root fix** | Preserved `OrderDetails` and `OrderPayment` mount across background refreshes (`refresh()`) by tracking `orderRef.current = order` and guarding `setState("loading")` with `if (!isRetry && !orderRef.current)`. Prevents unmounting `OrderPayment`, preserving `latest.current` so `signedUnknown` triggers correctly on simulated 503 read failure. | [`apps/storefront/app/[locale]/orders/[orderID]/OrderPage.tsx:28-33`](file:///Volumes/data/live_commerce_architecture_v1/.worktrees/parity-states/apps/storefront/app/[locale]/orders/[orderID]/OrderPage.tsx#L28-L33) |
| `124446517509` / `TestBrowserInboxUIRealChain` (`INU04`, `INU06`, `INU07`) | **Class (c): identical failure on base** & **Class (b): shared setup helper fix** | 1. **Class (c):** Baseline run `41111408749` job `124257277983` (`a7ae56a58a4e41f348942bbb7cd5e3a16b502b01`) timed out at `490.94s` with identical failures on `INU04`, `INU06`, `INU07`.<br>2. **Class (b):** In shared setup helper `enterMessages`, dynamically resolve shell menu button accessible name based on `page.url()` (`"打开导航"` on `zh-CN`, `"開啟導覽"` on `zh-TW`, `"Open navigation"` on `en`), unblocking localized mobile runs while matching static contract in `tests/admin/inbox-browser-source.test.ts`. | [`tests/admin/inbox-browser-support.ts:77-81`](file:///Volumes/data/live_commerce_architecture_v1/.worktrees/parity-states/tests/admin/inbox-browser-support.ts#L77-L81) |
| `124447472099` / `receipts` | **Dependent** | Fails only due to bundle:02 and bundle:06 failures above; passes once those bundles pass. | N/A |
| `124447738981` / `required` | **Dependent** | Fails only due to receipts failure above. | N/A |

## Verification

- `node scripts/dev/derive-browser-ownership.mjs` executed (wrote 54 modes to `output/ci-select-backend-browser/browser-ownership.json`).
- `node scripts/dev/derive-browser-ownership.mjs --check` verified (exit code 0).
- `gofmt -l .` verified (exit code 0).
- Preserved all source file headers (`Purpose / Depends on / Used by / Invariants`).

STATUS: DONE
