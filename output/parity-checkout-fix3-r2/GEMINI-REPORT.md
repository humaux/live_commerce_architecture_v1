# GEMINI-REPORT — PARITY-CHECKOUT-FIX3 (R2)

Card: PARITY-CHECKOUT-FIX3 (PR #351, branch `unit/parity-checkout`)
Failing run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41105720054 (head `7c2e866c3270c24ed978b3d7edf60a2c72001fae`)
Base merged: `origin/r3/integration` (`71e403e8fb7165720f36dc390344fcb2526b9e46` -> merge commit `1446559e8`)

| Failing job / test | Class | What you changed, file:line |
|---|---|---|
| `124237319015` (`gate (bundle:06)` / `--browser-home-cod`) / `TestBrowserHomeCod` (`COD-255 gap-up: server due refresh required` at `tests/storefront/home-cod-buyer.mjs:255`) | (b) | `apps/storefront/components/OrderFlow.tsx:231-240`: always call `refreshCodQuote(context, currentQuote, found)` when loading options so a merchant COD fee change during the initial options read (`gap-up`/`gap-down`/`gap-zero` before COD radio selection) refreshes the authoritative quote `cod_due_minor`, while falling back to `currentQuote` on non-COD initial review (`!optionRefresh && selected !== "cash_on_delivery"`) only when `GET /api/buyer/quotes/{id}` returned 200 with a still-stale COD due (`reason instanceof BuyerClientError && reason.status === undefined`, preserving pinned card re-quote in `checkout-v2.mjs:531-550` and failing closed on transport/5xx errors). |
| `124237319091` (`gate (--browser-click-sweep@5/10)`) / `NOT_RUN` (runner artifact download failed after 5 retries) | infrastructure (`BLOCKED_EXTERNAL`) | None (GitHub Actions `prebuilt` artifact download failure before test execution; `test_count=0`). |
| `124237319763` (`gate (--browser-click-sweep@7/10)`) / `NOT_RUN` (runner artifact download failed after 5 retries) | infrastructure (`BLOCKED_EXTERNAL`) | None (GitHub Actions `prebuilt` artifact download failure before test execution; `test_count=0`). |
| `124237319491` (`gate (--browser-visual-lint@3/4)`) / `NOT_RUN` (runner artifact download failed after 5 retries) | infrastructure (`BLOCKED_EXTERNAL`) | None (GitHub Actions `prebuilt` artifact download failure before test execution; `test_count=0`). |
| `124237318093` (`gate (--browser-webkit)`) / `NOT_RUN` (runner artifact download failed after 5 retries) | infrastructure (`BLOCKED_EXTERNAL`) | None (GitHub Actions `prebuilt` artifact download failure before test execution; `test_count=0`). |
| `124250898768` (`aggregate`), `124250898837` (`receipts`), `124252653668` (`required`) | derived | `output/ci-select-backend-browser/browser-ownership.json:1` (regenerated via `node scripts/dev/derive-browser-ownership.mjs` and verified with `--check`); `output/parity-checkout/FIX3.md:1-34`. |

STATUS: DONE
