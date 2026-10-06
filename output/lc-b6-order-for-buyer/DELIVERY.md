# lc-b6-order-for-buyer delivery
- Branch/commit: unit/lc-b6-order-for-buyer, final SHA in the integrator hand-off (code commit `bdc7f79e`, then this file)   Base: r3/integration df6be6ce   Model: Claude Sonnet 5.5 (fallback, DeepSeek balance exhausted)
- Summary: migration `migrations/0129_lc_b6_order_for_buyer.sql` = `inbox.order_for_buyer` (reservation rows, partial unique index = one live order per bundle), `claims.merchant_origin_grants`
  (single-use, 15 min, quote-bound), seven claims definers (`for_buyer_scope|peer_state|lines|begin|finish|release`, `bind_merchant_origin_grant`), the grant branch inside `claims.live_prices` and
  `claims.consume_live_prices` (same signatures), `inbox.plan_dm` order semantic key. Go: `internal/claims/live_price_grant.go`, `internal/merchanttools/order_for_buyer.go` (A15 prefill, A16 resumable
  order, pure `pickOrigins`), `internal/inbox/send_order.go` (order-pay-link/v1, link only in the sealed copy), routes in `internal/httpapi/merchanttools.go`, codes in `internal/httperror/error.go`,
  `cmd/api/inbox.go` `buildForBuyer` + `main.go`. A16 = thin wrapper over the unchanged `ManualOrders.Place` (3a reserve+eligibility+grant, 3c Place with server-only origins, bind grant to the quote right
  after CreateQuote, 3d record+audit, 4 DM); a replay completes whatever is missing (new link via the reused regenerate-link logic, one DM operation).
- Contract/interface changes: none to the frozen text. Deviations: number 0129 (contract says 0125); grants PK includes request_id; pending reservation rows are released only after a *definitive* Place refusal
  (an uncertain failure keeps them for the same key / 15 min); not_sent reasons beyond the contract: `not_requested`, `send_unavailable`, `link_unavailable`, `capability`, `takeover_changed`, `rate_limited`;
  live_price_reason values: `no_bundle|no_conversation|bundle_buyer_unverified|permission|no_live_line|live_price_unavailable|some_lines_catalog`.
- Tests: `bash scripts/dev/test-focused.sh '^(TestLiveConsole|TestLCN|TestK3LCB4|TestLiveClaims|TestKC03|TestT06|TestR2Integration|TestManualOrder|TestInbox|TestMsgTemplates|TestLiveToolsGateConsumption|TestMerchantToolsManualOrder)'`
  → run 1 PASS=99 FAIL=1 (only my new pay-link planner test: it called the planner through a WithScope that itself required inbox:reply; test-only fix), run 2 on the final tree (`gate-focused.log`)
  PASS=98 FAIL=2: `TestK3LCB4ConcurrentDuplicateRecent` and `TestLiveConsoleSendLCN08PublicReply`, both `public_reply_forbidden_content` from the random fixture product name (`t04Tag()` = 12 random hex
  chars; `msgtemplates` phoneRun `[0-9]{8,}` rejects it ~12% of processes); they passed in run 1 and touch nothing of this unit; the dedicated re-run could not get the machine PG lock in time (NOT_RUN).
  All LCN12 / KC03 / R2 upgrade / T06 / LPC01-06 / MTO tests pass in run 2. `bash scripts/dev/test-local.sh --browser-manual-order` → exit 0. DB-free: `go test ./internal/{inbox,claims,merchanttools,httpapi,httperror,storefront,msgtemplates} ./cmd/api` ok.
  Red: `output/lc-b6-order-for-buyer/red.log` (the LCN12 tests do not compile on df6be6ce) and mutation red `mutation-red.log` (fail-open peer state → FailClosed red; unbound-quote grant → GrantBranch red);
  green: `gate-focused.log` / `gate-focused-run1.log`.
- Gates run: check-gates.sh → exit 0 (gofmt, headers, registry). New gates: LCN12a-g in `tests/foundation/live_console_order_for_buyer_test.go`, real-planner DM half in `live_console_order_pay_link_test.go`,
  `internal/httpapi/for_buyer_test.go` (DB-free full router), `internal/merchanttools/order_for_buyer_test.go`, `cmd/api/for_buyer_test.go`. Pins updated: KC03 `live_claims_schema_test.go` (+7 definers 28->35, tables 9->10,
  column/table matrices, EXECUTE lists, release-only list), `r2_integration_upgrade_test.go` 63 -> 64.
- Evidence class: MOCK (REAL_PG + HTTP_PG; recording fake planner in the flow tests, real planner + loopback Graph in the DM test). No PSP, no LIVE Meta.
- Risks / money-review points (see integrator to-do): the grant makes `claims.live_prices` return the offer's live price for a bundle the cart's buyer does NOT own; everything that bounds it is SQL (peer
  comparison, live:manage, 15 min, one quote, one use, ledger cap, not purged, session not archived).
- NOT_RUN / BLOCKED: full G07 (`release-gate.sh --strict --only G07`); LCN13 retention halves for `inbox.order_for_buyer` / `claims.merchant_origin_grants` (LC-R1 follow-up); browser/UI (LC-U3);
  LIVE Meta send of the pay-link DM; PAYUNi card for manual orders (not implemented, per owner ruling).
- Integrator to-do:
  1. Money review of migration 0129: `claims.live_prices` / `consume_live_prices` rewrite (diff vs 0105 is the two marked blocks), `for_buyer_begin` eligibility order (mismatch 409 before anything is written),
     `for_buyer_finish` takes the audit lines from `live.offers.live_price_minor` at finish time (the order's frozen quote is not readable in merchant scope).
  2. Two files outside the brief's write paths, both tiny and necessary: `internal/storefront/revalidate.go` (sets transaction-local `app.quote_id` before `applyLivePrices`, so a bound grant prices only
     its own quote; CreateQuote leaves it unset) and `internal/merchanttools/manual.go` (`pipelineHooks` via context: origins into `CartInput.Origins`, bind step after CreateQuote; a plain manual order carries none)
     plus `internal/inbox/send_order.go` (new; `resolveText` still rejects `{{` in templates, this path substitutes the link itself).
  3. New privileges for `commerce_claims_writer`: SELECT `claims.bundles.purged_at`, `live.sessions(tenant_id,store_id,id,lifecycle)`, `social.conversations(7 cols)`, `inbox.bundle_peers(7 cols)` (all GUC-scoped
     policies) and table rights on the two new tables; EXECUTE for `commerce_runtime` on five definers and `commerce_buyer_runtime` on `bind_merchant_origin_grant` (all pinned in KC03).
  4. BFF allowlist += A15 (GET inbox/order-prefill) and A16 (POST orders/for-buyer); the 409 body carries `details.order_id` for `bundle_already_ordered`.
  5. A15 reads the linked customer through `social.conversation_meta`, which re-checks `inbox:read`; without it the answer is 200 with customer/delivery null. The customer/last-delivery read is N+1
     (`customers.Get` + `merchantorders.Get`), fine for a linked-customer drawer, add a definer if it shows up in latency.
  6. The pay-link DM does the A12 implicit takeover (plan_dm), by design of A1.3. A DM refusal never fails the created order (`send.state = not_sent` + reason; buyer_link is in the body for copy).
  7. Docs: contract §5.1/§11 could name 0129 and the extra reason codes; GATES.md has no new mode (LCN12 runs inside `foundation` and the `^TestLiveConsole` regex).

## Review follow-up (Opus money review f95acafa, P2-1..P2-6) — merged r3/integration first (R2 pin 65 + 0129 = 66)
- P2-1: `inbox.order_for_buyer` now stores `buyer_id` (capability owner) and `request_hash`; staleness uses `updated_at` (a resume refreshes it); a pending row is released/stolen only when no non-CANCELLED order of
  its owner exists (else 409 bundle_already_ordered with that order id); `for_buyer_release` skips such rows; `for_buyer_finish` checks the order is the owner's and raises PT409 `reservation_lost` when updated rows != the
  request's rows. P2-2: `claims.live_price_uses.unit_price_minor` written by `consume_live_prices` from the quote snapshot; finish returns it; audit lists full offer id + SKU id + consumed price (5 lines, `line_count` total).
  P2-3: finish expires the request's unconsumed grants. P2-4: same key + other body -> 409 `idempotency_conflict` (`for_buyer_begin` gained `p_request_hash`). P2-5: a begin whose eligibility fails expires earlier grants.
  P2-6: `for_buyer_peer_state` re-checks inventory:reserve. KC03 pins updated (begin signature, live_price_uses column).
- Tests: `live_console_order_for_buyer_p2_test.go` (LCN12h-k). Red on the pre-fix tree: `red-p2.log` (4/4 tests FAIL, incl. P2-1 b with a catalog-priced order so the ledger cannot mask it). Green: `gate-focused.log`
  PASS=104 FAIL=0 (regression regex incl. LCN12, KC03, R2, T06, LPC, MTO); `--browser-manual-order` exit 0; check-gates exit 0. The earlier flaky LC-B4 pair passed this time (fixed separately).
