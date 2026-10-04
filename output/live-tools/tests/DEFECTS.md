# live-tools (R4) independent test: product defects found

Test author: independent (not the implementer). First pass base `r3/integration` 1f20d81 (incl. unit live-tools 2191be6); re-verified on the
merged test base `db36a06` (r3/integration incl. merchant-tools 0094, meta-connect 0095, storefront-integration 0093); re-verified again on
`1c8ee21` (adds 0096 worker-authority split, 0097 order-locale, 0098 buyer-comms fixes). Gate files:
`tests/foundation/live_tools_gate_test.go` (LTG01-LTG09, PG), `tests/foundation/browser_live_tools_test.go` + `tests/e2e/live-tools.spec.ts` (browser).
Defects are NOT fixed here; the failing tests stay in the branch.

## D1 (P0, unit live-tools) checkout cannot place the order of ANY cart that holds a claim-origin line

- Symptom: `checkout.Begin` on the production checkout runtime pool answers "checkout database unavailable" for every cart that has a line applied from a claim
  link (live-priced or not). Real cause (raw error captured with a temporary debug print): `ERROR: permission denied for schema claims (SQLSTATE 42501)`.
- Root cause: `checkout.Begin` calls `storefront.RevalidateQuote`, which (migration 0092 / `applyLivePrices`) calls `claims.live_prices(...)` for every cart
  line with `claim_bundle_id`. Migration 0092 grants EXECUTE on `claims.live_prices` to `commerce_buyer_runtime` only, and `commerce_checkout_runtime` has no USAGE on
  schema `claims`. The implementer's smoke revalidated through the buyer pool (`checkoutRevalidate`) and never ran `Begin`, so it never saw this.
- Impact: the core loop comment -> claim -> cart -> checkout (T12 deal loop) is broken after 0092; a buyer who redeemed a claim link can only check out after
  removing the claim line. Money-path regression (no wrong charge, but no sale).
- Browser proof on the merged base (2026-10-01): with pristine product code the T12 browser deal loop (TestBrowserE2EDealLoop, --browser-e2e) fails 3/3 at
  create-order — the checkout POST never confirms and the storefront shows its designed idempotency-recovery alert ("暫未確認結果…恢復上一筆請求"),
  run ids output/playwright/e2e/20261001T150523 / 20261001T150759 / 20261001T151001. With a temporary owner-pool diagnostic in the T12 driver granting
  exactly the two privileges below (reverted, never committed) the same run passes end to end and the whole mode exits 0 (run id 20261001T151349).
  Earlier passes of the deal loop on this base had a product mutation applied that skipped live_prices entirely — consistent.
- Fix direction: `GRANT USAGE ON SCHEMA claims TO commerce_checkout_runtime; GRANT EXECUTE ON FUNCTION claims.live_prices(uuid[],uuid[],uuid[]) TO commerce_checkout_runtime;`
  (and whatever column SELECT on `storefront.cart_lines.claim_*` the checkout role still lacks), plus the KC03 matrix row and the pool validators.
- Gate (red without the fixture grant): `TestLiveToolsGateCheckoutPlacesClaimOriginCart`, `TestLiveToolsGateOfferLifecycleAndExpiry`, `TestLiveToolsGateLinkExpiry`,
  `TestLiveToolsGateSnapshotsAndOrders`, `TestLiveToolsGatePromotionOnLivePrice`, and the order step of `TestBrowserLiveTools`. With the workaround armed
  (`LC_LTG_WORKAROUND_D1=1`, or the gitignored flag file `output/live-tools/LTG_WORKAROUND_D1.on` containing `1` for sandboxed runners that cannot pass env;
  owner-pool fixture grant of exactly those two privileges, disclosed in the test) the same tests are green, which proves D1 is the only blocker of those paths.
- Re-verified on the merged base db36a06 (2026-10-01): still present (`kimi-evidence/pg-gate-defect-baseline-merged-base.log`, PASS=5 FAIL=7, exit=1).

## D2 (P2, unit live-tools) a forged `claim_quantity` above the claimed quantity earns the live price for the whole line

- `TestLiveToolsGateForgedClaimQuantityAboveTheClaim`: claim of 2 units at the live price; the buyer database role rewrites its cart line to quantity 99 with
  `claim_quantity` 99; the quote prices 99 units at the live price (20000 instead of 30000).
- Root cause: `storefront.applyLivePrices` trusts the cart line's own `claim_quantity` (`quantity<=claim_quantity`) and `claims.live_prices` only checks that the claim
  line exists for (bundle, offer, sku), never `claims.lines.quantity`. The 0092 header promises "a forged or stale cart origin can never produce a price a legitimate
  claim would not"; amendment rule 7 lists "raising the quantity above the claimed quantity".
- Exposure: needs raw SQL as the buyer role (compromised application / SQL injection), not a browser client; SetCart and every HTTP path are safe (LTG01/LTG02 pass).
- Fix direction: pass the cart quantity to `claims.live_prices` and require `quantity <= claims.lines.quantity` (or clamp the live-priced units to the claim).
- Re-verified on the merged base db36a06: still present (red in every run, by design).

## B1 (BASE defect, not caused by live-tools) — FIXED on the merged base

- `catalog.products_default_slug()` (migration 0086, unit catalog-core) and `design.refuse_history_change()` (migration 0087, unit store-design) kept the default PUBLIC EXECUTE.
  `platform.OpenStripeRegistrarPool` refuses a login that can EXECUTE any function outside its fixed ABI ("unsafe stripe database privileges"), so every test that opened the
  registrar failed on the pre-merge base (e.g. `TestCvsPayAtPickupBegin`, `TestLiveToolsGateRefundsCappedAtPaid`). Found while running LTG08.
- On the merged base db36a06 migration 0093 (`migrations/0093_storefront_integration.sql` lines 166-174) carries exactly the two REVOKEs, and
  `TestLiveToolsGateRefundsCappedAtPaid` is green with no patch (`kimi-evidence/pg-gate-green-withworkarounds-merged-base.log`). Kept here for the record; close on merge.

## P3 (contract/implementation divergences, no gate turned red for them)

1. Amendment rule 2 says a library import seeds `max_quantity_per_claim` = 1; the implementation and the accepted deviation 5 say 3. The gate asserts 3 (accepted deviation).
   The amendment text should be corrected.
2. Amendment rule 2 reports `keyword_taken` for "keyword already in the target session, for any SKU"; the implementation returns `already_present` when the SAME keyword is on
   the SAME SKU. The gate accepts either for that case and requires `keyword_taken` for a different SKU.
3. Buyer UI (new shell): the cart page and the read-only cart summary at the top of the checkout page show catalog amounts for a claimed line, while the claim page, the quotation,
   the pay-at-pickup note and the order page show the live price (accepted deviation 3: struck-out catalog price is a follow-up). The browser gate asserts the quote/order surfaces.

## Test-code fixes made on the merged base (not product defects)

- `tests/e2e/live-tools.spec.ts` was written against the pre-0093 shell: the checkout moved from `/{locale}/products/Checkout` (now a 404 — the product slug
  catch-all) to `/{locale}/checkout` (unit storefront-integration, migration 0093; `apps/storefront/lib/routes.ts`). Both references updated.
- The keyword library is store-level and accumulates one row per matrix cell; the import assertion is `importDone(n, 0)` for cell n, not `importDone(1, 0)`.
- The spec's cell body now runs `end-run` (window close, idempotent) and closes its browser contexts in a `finally`: a failed cell previously leaked its open
  claim window, and since only one window per store can be open, the next cells cascaded on `claims-window-state` = Closed instead of failing at their own step.
- The D1 workaround can also be armed by the gitignored flag file `output/live-tools/LTG_WORKAROUND_D1.on` (the Kimi sandbox runner may only invoke the
  allowlisted `scripts/dev/*` commands and cannot pass environment variables).
- The worker-authority split (migration 0096) broke `TestBrowserLiveTools` on `1c8ee21`: `provisionMeta` passed the checkout harness' worker pool
  (`x.e.p.worker`, which 0096 turned into the SANDBOX payment authority) into `metareply.Routes`/`t06StartDispatcher`, both of which validate
  `platform.WorkerClaims`. Fixed in the test to use the claims-worker login `x.e.claims` (already the correct T21-02 authority for the meta-reply
  dispatcher); `go vet -tags browser` clean after the change. Re-run green on `1c8ee21` (`kimi-evidence/browser-green-current-base.log`, PASS 48.95s).
- Same 0096 regression, different owner: `TestBrowserE2EDealLoop` (unit T12, same `--browser-e2e` mode, runs first) now fails at
  `browser_e2e_test.go:681: metareply.Routes: unsafe runtime database role` before it can reach create-order, so on `1c8ee21` the worker-split failure
  masks D1 for T12 (on `db36a06` it reached create-order and failed there). Not this unit's gate and not fixed here; T12's driver must pass a claims-worker
  pool the same way. The live-tools browser gate (`TestBrowserLiveTools`) is unaffected and PASS on this base.
