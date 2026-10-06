# Independent money review: live price kept on pause (Opus)

- Reviewer: Claude Opus 5.5, read-only (independent of the Sonnet implementer). Date 2026-10-07.
- Reviewed: worktree `.worktrees/live-price-keep-on-pause`, HEAD 056c7e21 (code identical to c319b691: `git diff --quiet c319b691 056c7e21 -- internal tests migrations`), base 4f2f5998.
- Rule under review: owner decision 2026-10-07 「已经认领的买家保留直播价，只拒绝新的认领。」
- Local evidence (DB-free only): clean tree; `gofmt -l` clean; `go vet ./internal/claims/ ./internal/storefront/ ./tests/foundation/` clean;
  `go test -count=1 ./internal/claims/ ./internal/storefront/ ./internal/merchanttools/` ok. Two mutations of `skuWinners` (drop the active preference; let only
  pending lines compete), each run in an isolated scratch copy, were caught by `TestRedeemPlanningOneLinePerSKU`. One reviewer probe (below) shows the P1. PG gates were NOT_RUN locally;
  I relied on the implementer's logs (`green-final.log` with the SHA stamped, `red*.log`).

## Verdict: MERGE-AFTER-FIX

No P0. One P1 (a Go-only fix of about 8 lines plus tests). The SQL change is correct and minimal.

## P1

### P1-1 `skuWinners` flips to the superseded typo line once the replacement offer is paused, and overwrites the cart line the replacement set
- Evidence: `internal/claims/buyer.go:368-383`. The rank is: active, then lowest offer id. The comment at `:366` says a repeated redeem can never let a paused line
  overwrite the cart line its active replacement set. That only holds while the replacement is still ACTIVE. Once every offer of the SKU is paused (end-of-live batch
  deactivate, W3-06B), the rank drops to lowest offer id, and offer ids are random UUIDs.
- Reviewer probe (scratch copy, real `splitPending`):
  - Typo line `A11`: lower id, pending, paused.
  - Corrected line `A1`: applied x3.
  - While `A1` is active, nothing is applied (correct).
  - After `A1` is also paused, the result is `apply=[A11 x2]`. `RedeemLink` then sets the cart SKU to 2 (an absolute quantity) and replaces the origin with A11
    (`buyer.go` RedeemLink origins/mergeCart). B1 shows the A11 line as `pending && available`, so the storefront offers the action.
- Why it matters (money, both directions):
  1. Before the order: the cart quantity drops from 3 to 2 and switches to A11's price (catalog, if the typo offer had no price). The buyer loses the A1 price they were granted.
  2. After A1 was ordered: the superseded typo claim becomes redeemable. The buyer gets 2 more units at a live price, which KC04 typo recovery meant to be replaced
     (contract §12: the typo line "apply only if that offer is reactivated").
  3. The trigger is an ordinary pause, the action this unit tells merchants is harmless. No test covers it. The unit table has "two paused, the lowest is already applied"
     (`claims_test.go:459`) but not "two paused, the HIGHER id was applied". The PG degenerate case (`live_price_keep_on_pause_test.go:236`) pauses both before any redeem.
- Minimal fix: rank active, then already applied (`!pending`), then lowest offer id. I verified it in scratch: existing `TestRedeemPlanning*` stay green and the probe
  applies nothing. The PG flow could still miss it, so add a PG case as well.
  ```go
  case line.offerActive && cur.offerActive:
      return nil, command.ErrConflict
  case line.offerActive != cur.offerActive:
      if line.offerActive { winners[line.skuID] = line }
  case line.pending != cur.pending:
      if !line.pending { winners[line.skuID] = line }   // the line already carrying the SKU keeps it
  case line.offerID < cur.offerID:
      winners[line.skuID] = line
  ```
  Tests to add:
  - Unit case: "two paused, the higher id applied, the lower pending" means nothing is applied and 1 is skipped.
  - PG case in `TestLivePriceKeepOnPauseOneClaimPerSKU`:
    1. Redeem while A1 is active.
    2. Pause A1.
    3. Preview: the A11 line has `available=false`.
    4. Redeem again: the cart stays at x3 with origin A1, and the quote is live at `ltgLive`.
  - Update the comment at `buyer.go:363-367` and amendment rule 4 (`contracts/live-keyword-claims-v1.md:1384`): active, then applied, then lowest id.
- Residual after the fix (P2-1): the fix does not cover these cases.
  - Neither line was ever applied, then both offers were paused. This is the reminder flow: 0144 re-issues links for never-opened claims after the live.
  - A1 was raised after its redeem and then paused.

  In both cases the rank is still a UUID coin flip.

## P2

- **P2-1 Arbitrary tie-break when every competing line is paused and pending.**
  - Evidence: `buyer.go:377`, `live_price_keep_on_pause_test.go:236-262`. Contract rule 4 calls this case "degenerate". It is realistic: typo recovery, then an
    end-of-live batch deactivate, then a reminder link (0144 header lines 5-6), then the buyer opens the link for the first time.
  - Why: about half the time the typo claim (its quantity and its price) wins over the corrected claim.
  - Fix, owner to pick one:
    - (a) Let the latest claim win, which needs a time order from `preview_link` / `redeem_link`. That changes their signature, so the KC03 pin has to change.
    - (b) When there is no active line and 2 or more lines are pending, apply none (pre-0158 behaviour for this case; any cart origin stays priced).
    - (c) Keep the current rule and document it as a coin flip.
- **P2-2 The W3-04B late-deactivation race is unreachable. The contract text and DELIVERY risk #1 describe a race that cannot happen.**
  - Evidence:
    - The poller ingests and plans the reply in ONE transaction (`internal/claimsintake/poller.go:168-190`).
    - Ingest locks the offer `FOR SHARE` (`internal/claims/ingest.go:180-181`, also on the Meta path where the lookup is by id).
    - The only two writers of `live.offers.active` take `FOR NO KEY UPDATE` first, so they wait for that transaction to commit (`merchant.go:482`, `keyword_tools.go:433`; no
      other `UPDATE live.offers` in `migrations/` or `internal/`).
    - So at `plan_claim_reply` time, `claim_sold_out_facts` (`0151:139-142`) always sees the claim's offer as active. The `NOT o.active` clause is dead on the auto-reply
      path. SO03 (`TestSoldOutReplyPausedOffer`) only calls the helper directly after an owner-fixture UPDATE.
    - A retry after a rollback re-ingests: the claim is refused (OFFER_INACTIVE), with no bundle and no reply.
  - Rating: P2 (documentation, plus a latent contradiction).
    - No buyer can hold a claim line that was accepted before the pause and still get the sold-out text.
    - If planning ever leaves the ingest transaction, for example through the deferred River leg of 0144, `NOT o.active` would contradict the owner rule.
  - Minimal fix:
    - Rewrite amendment rule 7 (`contracts/live-keyword-claims-v1.md:1393`) and the W3-04B sentence (`:1227-1228`) to say "unreachable: the reply is planned in the
      ingest transaction, which holds the offer FOR SHARE".
    - In a later migration owned by W3-04B, drop `NOT o.active OR` from `integration.claim_sold_out_facts`. The ACCEPTED claim event already proves the offer was
      active at acceptance, so no new claim-event flag is needed.
- **P2-3 A price edit on a paused offer changes the price claimants pay.** The price is evaluated at quote time (amendment rule 2, `0158:84-85`).
  - This matches rule 3 for active offers. `checkout.Begin` re-runs RevalidateQuote and fails closed on a change (LTG03: conflict, zero facts), so no charge changes silently.
  - Not a P1, but the owner wording 保留直播价 could mean "the price at claim time". Ask the owner to confirm.
  - Freezing the price per line would need a `claims.lines` snapshot column, which is out of scope here.
  - Missing test: reprice a paused offer, then the claimant's quote shows the new price and an old quote fails closed at Begin.
- **P2-4 Merchant re-issue renews the price window of a paused offer.** The amendment rule 2 claim "Expiry is not extended" holds only for buyer actions.
  - `claims.issue_link` (`0060:595-599`) resets `expires_at` to now + 72 h. Three actions call it: the LC-B4 manual send, the 0144 reminder (never-opened claims), and the
    release path (`p_release`, where the next opener binds).
  - This is pre-existing for active offers, and it is merchant-initiated, so no buyer exploit. Add one sentence to rule 2 and to the `deactivate_has_claims` copy.
- **P2-5 Merchant order-for-buyer can use a superseded paused line, so it does not always pick the same line as the buyer.**
  - `merchanttools/order_for_buyer.go:133-146` takes the first QUALIFYING line per SKU. If the active line's quantity or remaining quantity does not cover the request, it
    falls through to the paused typo line, which 0158 now prices (`0158:152`).
  - So the comment at `0158:160-161` ("picks the same claim line as the buyer's redeem") holds only when the active line qualifies.
  - Each line stays within its own cap and the action is merchant-initiated. Either skip superseded lines in `pickOrigins` (reuse `skuWinners`) or reword the comment.
- **P2-6 Gaps in the evidence.**
  - `TestLiveConsoleOrderForBuyerPausedOfferKeepsLivePrice` has no red log; it is absent from `red.log` and `red2.log`.
  - `red*.log` carry no base-SHA header. `green-final.log` has one.
- **P2-7 Stale contract rows.**
  - The KC11 row (`contracts/live-keyword-claims-v1.md:948`) still says "inactive offer ... skipped and still pending, applied after reactivation".
  - The W3-04B text at `:1227-1228` is also stale (see P2-2).
- **P2-8 `ORDER BY ... o.active DESC` now comes before `LIMIT 50` in `for_buyer_lines`** (`0158:162`). With more than 50 lines it changes which lines are cut. Negligible.

## Checks that passed (with evidence)

1. **Bodies.**
   - Latest definitions before 0158:
     - `claims.live_prices`: 0129:396.
     - `claims.preview_live_prices`: 0092:111, VOLATILE via 0101:35.
     - `claims.for_buyer_lines`: 0129:164.
     - Search: `grep -rniE "create (or replace )?function claims\.(live_prices|preview_live_prices|for_buyer_lines)"` over all migrations. The functions are also not
       patched through a `regprocedure` pattern, and the parallel branches 0159/0160 do not touch them.
   - Diff against 0158:
     - `live_prices`: only the `o.active` clause and its comment change.
     - `preview_live_prices`: only the `o.active` clause, plus STABLE becomes VOLATILE (the same as 0101).
     - `for_buyer_lines`: `CASE WHEN o.active AND ... THEN ... END` becomes `o.live_price_minor`, plus `ORDER BY o.active DESC`.
   - Owner, SECURITY DEFINER, search_path and EXECUTE are unchanged (CREATE OR REPLACE, then ALTER OWNER). `consume_live_prices` (0129:452) never read `active`.
2. **No path for a buyer who never claimed.**
   - Writers of `claims.lines`: only `internal/claims/ingest.go:224/228`. The manual and Meta paths both go through `offerReason`, which returns OFFER_INACTIVE (`ingest.go:344`).
   - Cart origins are written only by `RedeemLink`. The merchant grant is per bundle and per claim line (0129).
   - Reminders, LC-B4 sends and direct checkout use the same functions, as do the cart API, the storefront cart and the quote.
   - Bound owner and unexpired link: `0158:70-71`. Tests (e) and (g).
   - Cap per claim line across orders: `0158:81-83`. `consume_live_prices` locks the line and re-checks it (0129).
   - No stacking: one origin per cart SKU, and one line per cart line in `live_prices`.
   - Clearing the price ends the live price: `ClearedPriceStillEndsIt` and LTG03.
3. **`skuWinners` result.** It is deterministic and independent of input order. Its KC04 wire shape (B1 `available`, B2 `skipped{sku_id, reason}`) is unchanged
   (`apps/storefront/lib/claim-contract.ts:40,97`). B1 (`buyer.go:143-151`) and B2 (`splitPending`) rank the same set (all lines of the bundle), so preview and redeem agree.
   The one defect is P1-1.
4. **Gate changes.**
   - LTG03: the pause now asserts the exact live price, bundle and offer. The fail-closed half moved to clearing the price, with the same conflict, zero holds or orders,
     then a catalog re-quote.
   - `TestKeywordToolsBatchDeactivateIsAPause`: Amy is live at quantity 2, Bob is applied and live at quantity 1, a new claim is refused, and reactivation changes nothing.
   - `TestLiveToolsExpiryOffersAndForgedOrigin`: now also asserts `live_claim`, the live price and the catalog price.
   - `TestRedeemPlanning`: the paused line is applied at 5, and is `sold_out` at stock 4.
   - KC04 typo recovery is untouched. Every change is at least as strict, and nothing unrelated got weaker.
5. **New tests (a)-(g).**
   - They check exact values: price 20000, quantities 2 and 1, order totals, and ledger quantities and row counts.
   - Red on unchanged code: `red.log` (5 FAIL) and `red2.log` (1 FAIL). `red-unit.log` ran against a no-op `skuWinners`.
6. **Pins.**
   - KC03: no test file changed. The KC03 ACL and signature pins pass in `green-final.log`.
   - R2: the computed count is 76 numbered files from 0070 plus 8 post-River files from 0015, which is 84 and matches `r2_integration_upgrade_test.go:71`.
     With 0159 and 0160 it will be 86 at merge (integrator).

## NOT_RUN (reviewer)
PG suites, including the R2 upgrade gate and the full foundation suite, are left to CI. No browser runs.
